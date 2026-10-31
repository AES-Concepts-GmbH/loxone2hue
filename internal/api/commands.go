package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"net/http"
	"strings"
)

// cmdError is an execution failure together with the HTTP status it maps to.
// The WebSocket path only forwards the message.
type cmdError struct {
	status int
	msg    string
}

func (e *cmdError) Error() string { return e.msg }

func badRequest(format string, a ...interface{}) error {
	return &cmdError{status: http.StatusBadRequest, msg: fmt.Sprintf(format, a...)}
}

func notFound(format string, a ...interface{}) error {
	return &cmdError{status: http.StatusNotFound, msg: fmt.Sprintf(format, a...)}
}

func execFailed(err error) error {
	return &cmdError{status: http.StatusInternalServerError, msg: err.Error()}
}

// commandErrorStatus maps an error from executeCommand to an HTTP status.
func commandErrorStatus(err error) int {
	var ce *cmdError
	if errors.As(err, &ce) {
		return ce.status
	}
	return http.StatusInternalServerError
}

// commandResult is what a successfully executed command reports back.
type commandResult struct {
	HueID   string
	HueType string
	// State is set for STATUS queries.
	State *models.LightState
}

// isHueResourceID reports whether s looks like a raw HUE v2 resource ID.
func isHueResourceID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// executeCommand runs a parsed Loxone command against the bridge. Both
// transports (GET /ws?cmd=... and the WebSocket) go through here, so they
// cannot drift apart again: the two copies had already diverged (STATUS only
// on one side, different answers for the same error).
//
// A target that matches no mapping is accepted only when it is a HUE resource
// ID itself; anything else is a typo in a Loxone ID and gets a 404 instead of
// being sent to the bridge as a light ID.
func (h *WebSocketHub) executeCommand(cmd *models.LoxoneCommand) (commandResult, error) {
	action := strings.ToLower(cmd.Action)

	switch action {
	case "scene":
		return h.executeScene(cmd)
	case "mood":
		return h.executeMood(cmd)
	}

	hueID, hueType, ok := h.mappingManager.ResolveTarget(cmd.Target)
	if !ok {
		if !isHueResourceID(cmd.Target) {
			return commandResult{}, notFound("unknown target '%s': no mapping with this loxone_id", cmd.Target)
		}
		hueID, hueType = cmd.Target, "light"
	}
	res := commandResult{HueID: hueID, HueType: hueType}

	switch action {
	case "set":
		deviceCmd := h.commandParser.ToDeviceCommand(cmd)
		var err error
		switch hueType {
		case "light":
			err = h.hueClient.SetLightState(hueID, deviceCmd)
		case "group":
			err = h.hueClient.SetGroupState(hueID, deviceCmd)
		default:
			// Previously a silent no-op that still answered "ok".
			return res, badRequest("target '%s' is a %s mapping and cannot be set; use SCENE or MOOD", cmd.Target, hueType)
		}
		if err != nil {
			return res, execFailed(err)
		}
		return res, nil

	case "status":
		if hueType != "light" {
			return res, badRequest("STATUS is only available for light mappings, '%s' is a %s", cmd.Target, hueType)
		}
		light, err := h.hueClient.GetLight(hueID)
		if err != nil {
			return res, execFailed(err)
		}
		res.State = &light.State
		return res, nil

	default:
		return res, badRequest("unsupported action: %s", cmd.Action)
	}
}

// executeScene activates a scene by scene mapping or raw HUE scene ID.
func (h *WebSocketHub) executeScene(cmd *models.LoxoneCommand) (commandResult, error) {
	sceneID, ok := cmd.Params["scene_id"].(string)
	if !ok || sceneID == "" {
		return commandResult{}, badRequest("scene_id required")
	}

	if hueID, hueType, resolved := h.mappingManager.ResolveTarget(sceneID); resolved && hueType == "scene" {
		sceneID = hueID
	} else if !isHueResourceID(sceneID) {
		return commandResult{}, notFound("unknown scene '%s': no scene mapping with this loxone_id", sceneID)
	}

	res := commandResult{HueID: sceneID, HueType: "scene"}
	if err := h.hueClient.ActivateScene(sceneID); err != nil {
		return res, execFailed(err)
	}
	return res, nil
}

// executeMood handles MOOD <target> <n>: 0 switches the target off, n > 0
// activates the n-th scene mapping of the target.
func (h *WebSocketHub) executeMood(cmd *models.LoxoneCommand) (commandResult, error) {
	moodNum, ok := cmd.IntParam("mood_number")
	if !ok {
		return commandResult{}, badRequest("mood_number required")
	}

	hueID, hueType, resolved := h.mappingManager.ResolveMood(cmd.Target, moodNum)
	if !resolved {
		return commandResult{}, notFound("no mapping found for mood %d of '%s'", moodNum, cmd.Target)
	}
	res := commandResult{HueID: hueID, HueType: hueType}

	if moodNum == 0 {
		off := false
		offCmd := models.DeviceCommand{On: &off}
		var err error
		switch hueType {
		case "light":
			err = h.hueClient.SetLightState(hueID, offCmd)
		case "group":
			err = h.hueClient.SetGroupState(hueID, offCmd)
		default:
			return res, badRequest("mood 0 of '%s' must point at a light or group, not a %s", cmd.Target, hueType)
		}
		if err != nil {
			return res, execFailed(err)
		}
		return res, nil
	}

	if hueType != "scene" {
		return res, badRequest("mood mapping must be a scene")
	}
	if err := h.hueClient.ActivateScene(hueID); err != nil {
		return res, execFailed(err)
	}
	return res, nil
}

// logCommandError logs an execution failure at a level matching its cause:
// client mistakes as warnings, bridge failures as errors.
func logCommandError(transport string, cmd *models.LoxoneCommand, err error) {
	ev := log.Warn()
	if commandErrorStatus(err) >= http.StatusInternalServerError {
		ev = log.Error()
	}
	ev.Err(err).Str("transport", transport).Str("target", cmd.Target).Str("action", cmd.Action).Msg("Command failed")
}

// handleHTTPCommand processes HTTP command requests (for testing and Loxone virtual outputs)
func (h *WebSocketHub) handleHTTPCommand(w http.ResponseWriter, r *http.Request, cmdStr string) {
	w.Header().Set("Content-Type", "application/json")

	cmd, err := h.commandParser.ParseText(cmdStr)
	if err != nil {
		log.Warn().Str("command", cmdStr).Err(err).Msg("Failed to parse HTTP command")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid command format: " + err.Error()})
		return
	}

	log.Debug().
		Str("type", cmd.Type).
		Str("target", cmd.Target).
		Str("action", cmd.Action).
		Interface("params", cmd.Params).
		Msg("Received HTTP command")

	res, err := h.executeCommand(cmd)
	if err != nil {
		logCommandError("http", cmd, err)
		w.WriteHeader(commandErrorStatus(err))
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	resp := map[string]interface{}{
		"status":   "ok",
		"target":   cmd.Target,
		"action":   cmd.Action,
		"hue_id":   res.HueID,
		"hue_type": res.HueType,
	}
	if res.State != nil {
		resp["state"] = res.State
	}
	json.NewEncoder(w).Encode(resp)
}
