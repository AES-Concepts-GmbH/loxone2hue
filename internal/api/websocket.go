package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/hue"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/loxone"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Reject handshakes carrying a foreign Origin, so a random web page cannot
	// open a control channel to the gateway.
	CheckOrigin: isSameOriginRequest,
}

// WebSocketHub manages WebSocket connections
type WebSocketHub struct {
	clients    map[*WebSocketClient]bool
	broadcast  chan []byte
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mu         sync.RWMutex

	// feedback carries bridge events to the worker that talks to Loxone.
	// Feedback does network I/O (HTTP with timeouts), so it must not run on
	// the goroutine that drains the bridge's event channel.
	feedback            chan hue.Event
	droppedFeedback     atomic.Int64
	lastFeedbackDropLog atomic.Int64

	hueClient      *hue.Client
	mappingManager *loxone.MappingManager
	commandParser  *loxone.CommandParser
	udpSender      *loxone.UDPSender
	httpSender     *loxone.HTTPSender
}

// WebSocketClient represents a connected WebSocket client.
//
// send is never closed: both the hub and the client's own readPump produce
// messages for it, so closing it would race with an in-flight send. Teardown
// is signalled by closing done instead, which writePump selects on.
type WebSocketClient struct {
	hub       *WebSocketHub
	conn      *websocket.Conn
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
	clientID  string
	isLoxone  bool
}

// close signals that the client is gone and wakes writePump.
// Safe to call more than once and from any goroutine.
func (c *WebSocketClient) close() {
	c.closeOnce.Do(func() { close(c.done) })
}

// trySend queues a message without ever blocking. It reports false when the
// client is gone or cannot keep up, in which case the message is dropped.
func (c *WebSocketClient) trySend(message []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}

	select {
	case c.send <- message:
		return true
	default:
		// Buffer full — the client is too slow for the event rate.
		return false
	}
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub(hueClient *hue.Client, mappingManager *loxone.MappingManager, udpSender *loxone.UDPSender, httpSender *loxone.HTTPSender) *WebSocketHub {
	return &WebSocketHub{
		clients:        make(map[*WebSocketClient]bool),
		broadcast:      make(chan []byte, 256),
		register:       make(chan *WebSocketClient),
		unregister:     make(chan *WebSocketClient),
		feedback:       make(chan hue.Event, feedbackQueueSize),
		hueClient:      hueClient,
		mappingManager: mappingManager,
		commandParser:  loxone.NewCommandParser(),
		udpSender:      udpSender,
		httpSender:     httpSender,
	}
}

// Run starts the hub's event loop
func (h *WebSocketHub) Run(ctx context.Context) {
	// Forward HUE events to WebSocket clients and queue them for Loxone
	go h.forwardHueEvents(ctx)
	go h.runFeedback(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Info().Str("client", client.clientID).Bool("loxone", client.isLoxone).Msg("Client connected")

		case client := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, client)
			h.mu.Unlock()
			client.close()
			log.Info().Str("client", client.clientID).Msg("Client disconnected")

		case message := <-h.broadcast:
			// Snapshot under the read lock, then send outside it, so the map is
			// never mutated while only holding RLock.
			h.mu.RLock()
			clients := make([]*WebSocketClient, 0, len(h.clients))
			for client := range h.clients {
				clients = append(clients, client)
			}
			h.mu.RUnlock()

			var slow []*WebSocketClient
			for _, client := range clients {
				if !client.trySend(message) {
					slow = append(slow, client)
				}
			}

			if len(slow) > 0 {
				h.mu.Lock()
				for _, client := range slow {
					delete(h.clients, client)
				}
				h.mu.Unlock()
				for _, client := range slow {
					log.Warn().Str("client", client.clientID).Msg("Dropping client that cannot keep up")
					client.close()
				}
			}
		}
	}
}

// feedbackQueueSize bounds how many bridge events may wait for the feedback
// worker. The bridge's own channel holds 100; this is deliberately larger so a
// burst survives a slow Miniserver without pushing back on the event stream.
const feedbackQueueSize = 256

// feedbackDropLogInterval throttles the warning about a full feedback queue.
const feedbackDropLogInterval = 10 * time.Second

// forwardHueEvents forwards HUE events to connected clients and hands them to
// the feedback worker. Everything here must be non-blocking: this goroutine
// is the only consumer of the bridge's event channel, and the bridge drops
// events once that channel fills.
func (h *WebSocketHub) forwardHueEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-h.hueClient.Events():
			if data := h.uiMessageFor(event); data != nil {
				h.broadcast <- data
			}
			h.queueFeedback(event)
		}
	}
}

// uiMessageFor builds the WebSocket message the Web UI receives for a bridge
// event, or nil when the event has no UI representation.
//
// The payload is always the converted cache object, never the raw CLIP event:
// the UI merges "status" into its LightState, and a raw {"on":{"on":false}}
// used to leave light.state.on an always-truthy object.
func (h *WebSocketHub) uiMessageFor(event hue.Event) []byte {
	var msg interface{}

	switch {
	case event.Type == "light":
		light, ok := h.hueClient.LightSnapshot(event.ID)
		if !ok {
			return nil
		}
		msg = models.LoxoneStatus{
			Type:   "status",
			Device: event.ID,
			State:  light.State,
		}

	case event.Type == "grouped_light":
		groupID := h.hueClient.GroupIDForGroupedLight(event.ID)
		if groupID == "" {
			return nil
		}
		group, ok := h.hueClient.GroupSnapshot(groupID)
		if !ok {
			return nil
		}
		msg = map[string]interface{}{
			"type":  "group_update",
			"group": group,
		}

	case hue.IsSensorType(event.Type):
		sensor := h.hueClient.GetSensor(event.ID)
		if sensor == nil {
			return nil
		}
		msg = map[string]interface{}{
			"type":   "sensor_update",
			"sensor": sensor,
		}

	default:
		return nil
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil
	}
	return data
}

// queueFeedback hands an event to the feedback worker without blocking.
func (h *WebSocketHub) queueFeedback(event hue.Event) {
	select {
	case h.feedback <- event:
		return
	default:
	}

	total := h.droppedFeedback.Add(1)
	now := time.Now().UnixNano()
	last := h.lastFeedbackDropLog.Load()
	if now-last < int64(feedbackDropLogInterval) || !h.lastFeedbackDropLog.CompareAndSwap(last, now) {
		return
	}
	log.Warn().
		Str("id", event.ID).
		Str("type", event.Type).
		Int64("dropped_total", total).
		Msg("Feedback queue full, Loxone will not receive this update (is the Miniserver reachable?)")
}

// runFeedback is the single worker that sends UDP and HTTP feedback to Loxone.
// One worker keeps updates for a device in order (on before brightness).
func (h *WebSocketHub) runFeedback(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-h.feedback:
			h.sendFeedback(event)
		}
	}
}

// DroppedFeedback reports how many bridge events never reached the feedback
// worker because its queue was full.
func (h *WebSocketHub) DroppedFeedback() int64 {
	return h.droppedFeedback.Load()
}

// feedbackTarget holds a loxoneID and optionally the miniserver to send to
type feedbackTarget struct {
	loxoneID     string
	miniserverID string // empty = send to all (send_all targets)
	feedbackUDP  bool   // whether to send UDP feedback
	feedbackHTTP bool   // whether to send HTTP feedback
}

// targetFromMapping creates a feedbackTarget from a mapping, using the mapping's feedback flags.
func targetFromMapping(m *models.Mapping) feedbackTarget {
	return feedbackTarget{
		loxoneID:     m.LoxoneID,
		miniserverID: m.MiniserverID,
		feedbackUDP:  m.ShouldFeedbackUDP(),
		feedbackHTTP: m.ShouldFeedbackHTTP(),
	}
}

// feedbackTargets turns the mappings of one HUE resource into feedback targets,
// skipping scenes and mood entries — those are inputs to the gateway, not
// destinations for a status update.
func feedbackTargets(mappings []*models.Mapping) []feedbackTarget {
	var targets []feedbackTarget
	for _, m := range mappings {
		if m.HueType == "scene" || strings.Contains(m.LoxoneID, "_mood_") {
			continue
		}
		targets = append(targets, targetFromMapping(m))
	}
	return targets
}

// targetSendAll creates a feedbackTarget for send_all (no mapping, both protocols enabled).
func targetSendAll(loxoneID string) feedbackTarget {
	return feedbackTarget{
		loxoneID:     loxoneID,
		feedbackUDP:  true,
		feedbackHTTP: true,
	}
}

// sendFeedback extracts changed properties from a HUE event and sends them via UDP and/or HTTP
func (h *WebSocketHub) sendFeedback(event hue.Event) {
	// Handle sensor events separately
	if hue.IsSensorType(event.Type) {
		h.sendSensorFeedback(event)
		return
	}

	// Extract event data fields via JSON roundtrip
	type eventData struct {
		On *struct {
			On bool `json:"on"`
		} `json:"on,omitempty"`
		Dimming *struct {
			Brightness float64 `json:"brightness"`
		} `json:"dimming,omitempty"`
		ColorTemperature *struct {
			Mirek int `json:"mirek"`
		} `json:"color_temperature,omitempty"`
		Color *struct {
			XY struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
			} `json:"xy"`
		} `json:"color,omitempty"`
	}

	raw, err := json.Marshal(event.Data)
	if err != nil {
		return
	}

	var ed eventData
	if err := json.Unmarshal(raw, &ed); err != nil {
		return
	}

	udpEnabled := h.udpSender != nil && h.udpSender.IsEnabled()
	httpEnabled := h.httpSender != nil && h.httpSender.IsEnabled()
	if !udpEnabled && !httpEnabled {
		return
	}

	// Resolve the event to the HUE resource that mappings refer to. Group
	// mappings store the room or zone ID; the bridge reports the aggregated
	// group state on the room's grouped_light service, so map that back.
	// Single-light events deliberately do not fan out to the groups containing
	// the light: that reported one lamp's state as the room's state.
	hueID := event.ID
	var name string
	switch event.Type {
	case "light":
		if light, ok := h.hueClient.LightSnapshot(event.ID); ok {
			name = light.Name
		}
	case "grouped_light":
		hueID = h.hueClient.GroupIDForGroupedLight(event.ID)
		if hueID == "" {
			return
		}
		name = h.hueClient.GetGroupName(hueID)
	default:
		return
	}

	// Collect all targets that should receive this update (skip scenes and mood mappings).
	// A HUE resource can carry several mappings, and each one is a target.
	var targets []feedbackTarget
	if direct := feedbackTargets(h.mappingManager.GetAllByHueID(hueID)); len(direct) > 0 {
		targets = append(targets, direct...)
	} else if name != "" && ((udpEnabled && h.udpSender.HasSendAll()) || (httpEnabled && h.httpSender.HasSendAll())) {
		// No mapping found — generate LoxoneID from device name, send to all send_all targets
		targets = append(targets, targetSendAll(sanitizeName(name)))
	}

	if len(targets) == 0 {
		return
	}

	// Send per changed property per target via UDP and/or HTTP
	for _, target := range targets {
		send := func(property string, value interface{}) {
			if udpEnabled && target.feedbackUDP {
				if target.miniserverID != "" {
					h.udpSender.Send(target.miniserverID, target.loxoneID, property, value)
				} else {
					h.udpSender.SendToAll(target.loxoneID, property, value)
				}
			}
			if httpEnabled && target.feedbackHTTP {
				if target.miniserverID != "" {
					h.httpSender.Send(target.miniserverID, target.loxoneID, property, value)
				} else {
					h.httpSender.SendToAll(target.loxoneID, property, value)
				}
			}
		}

		if ed.On != nil {
			onVal := 0
			if ed.On.On {
				onVal = 1
			}
			send("on", onVal)
		}
		if ed.Dimming != nil {
			send("bri", int(ed.Dimming.Brightness))
		}
		if ed.ColorTemperature != nil {
			send("ct", ed.ColorTemperature.Mirek)
		}
		if ed.Color != nil {
			send("color_x", fmt.Sprintf("%.4f", ed.Color.XY.X))
			send("color_y", fmt.Sprintf("%.4f", ed.Color.XY.Y))
		}
	}
}

// sendSensorFeedback handles feedback for sensor events via UDP and/or HTTP
func (h *WebSocketHub) sendSensorFeedback(event hue.Event) {
	udpEnabled := h.udpSender != nil && h.udpSender.IsEnabled()
	httpEnabled := h.httpSender != nil && h.httpSender.IsEnabled()
	if !udpEnabled && !httpEnabled {
		return
	}

	// Find target for this sensor
	var targets []feedbackTarget

	if sensorTargets := feedbackTargets(h.mappingManager.GetAllByHueID(event.ID)); len(sensorTargets) > 0 {
		targets = append(targets, sensorTargets...)
	} else if (udpEnabled && h.udpSender.HasSendAll()) || (httpEnabled && h.httpSender.HasSendAll()) {
		if name := h.hueClient.GetSensorName(event.ID); name != "" {
			targets = append(targets, targetSendAll(sanitizeName(name)))
		}
	}

	if len(targets) == 0 {
		return
	}

	// Extract sensor data via JSON roundtrip
	raw, err := json.Marshal(event.Data)
	if err != nil {
		return
	}

	var item hue.EventItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return
	}

	for _, target := range targets {
		send := func(property string, value interface{}) {
			if udpEnabled && target.feedbackUDP {
				if target.miniserverID != "" {
					h.udpSender.Send(target.miniserverID, target.loxoneID, property, value)
				} else {
					h.udpSender.SendToAll(target.loxoneID, property, value)
				}
			}
			if httpEnabled && target.feedbackHTTP {
				if target.miniserverID != "" {
					h.httpSender.Send(target.miniserverID, target.loxoneID, property, value)
				} else {
					h.httpSender.SendToAll(target.loxoneID, property, value)
				}
			}
		}

		// Switch on the kind, so the room/home aggregates (grouped_motion,
		// grouped_light_level) reach Loxone under the same properties as the
		// per-device sensors.
		switch hue.SensorKind(event.Type) {
		case hue.KindMotion:
			if item.Motion != nil {
				motion := item.Motion.Motion
				if item.Motion.MotionReport != nil {
					motion = item.Motion.MotionReport.Motion
				}
				val := 0
				if motion {
					val = 1
				}
				send("motion", val)
			}

		case hue.KindTemperature:
			if item.Temperature != nil {
				temp := item.Temperature.Temperature
				if item.Temperature.TemperatureReport != nil {
					temp = item.Temperature.TemperatureReport.Temperature
				}
				send("temperature", fmt.Sprintf("%.1f", temp))
			}

		case hue.KindLightLevel:
			if item.LightSensor != nil {
				level := item.LightSensor.LightLevel
				if item.LightSensor.LightLevelReport != nil {
					level = item.LightSensor.LightLevelReport.LightLevel
				}
				// light_level is the bridge's raw logarithmic value (kept for
				// existing Loxone configurations), lux the converted reading.
				send("light_level", level)
				send("lux", models.LuxFromLightLevel(level))
			}

		case hue.KindButton:
			if item.Button != nil {
				event := item.Button.LastEvent
				if item.Button.ButtonReport != nil {
					event = item.Button.ButtonReport.Event
				}
				if event != "" {
					// Convert button event to numeric code for Loxone
					// initial_press=0, repeat=1, short_release=2, long_release=3, long_press=4
					buttonCode := buttonEventToCode(event)
					send("button", buttonCode)
				}
			}

		case hue.KindContact:
			if item.ContactReport != nil {
				val := 0
				if item.ContactReport.State == "no_contact" {
					val = 1
				}
				send("contact", val)
			}

		case hue.KindRotary:
			if item.RelativeRotary != nil && item.RelativeRotary.RotaryReport != nil {
				steps := item.RelativeRotary.RotaryReport.Rotation.Steps
				if item.RelativeRotary.RotaryReport.Rotation.Direction == "counter_clockwise" {
					steps = -steps
				}
				send("rotary", steps)
			}

		case hue.KindPower:
			if item.PowerState != nil {
				send("battery", item.PowerState.BatteryLevel)
			}
		}
	}
}

// buttonEventToCode converts a HUE button event string to a numeric code
func buttonEventToCode(event string) int {
	switch event {
	case "initial_press":
		return 0
	case "repeat":
		return 1
	case "short_release":
		return 2
	case "long_release":
		return 3
	case "long_press":
		return 4
	default:
		return -1
	}
}

// HandleWebSocket handles WebSocket upgrade requests and HTTP command requests
func (h *WebSocketHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if this is an HTTP command request (for testing via browser)
	cmd := r.URL.Query().Get("cmd")
	if cmd != "" {
		h.handleHTTPCommand(w, r, cmd)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	// Check if this is a Loxone connection
	isLoxone := r.URL.Query().Get("type") == "loxone"
	clientID := r.URL.Query().Get("id")
	if clientID == "" {
		clientID = conn.RemoteAddr().String()
	}

	client := &WebSocketClient{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 256),
		done:     make(chan struct{}),
		clientID: clientID,
		isLoxone: isLoxone,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

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

// BroadcastStatus sends a status update to all connected clients
func (h *WebSocketHub) BroadcastStatus(deviceID string, state interface{}) {
	status := models.LoxoneStatus{
		Type:   "status",
		Device: deviceID,
		State:  state,
	}

	data, err := json.Marshal(status)
	if err != nil {
		return
	}

	h.broadcast <- data
}

// readPump reads messages from the WebSocket connection
func (c *WebSocketClient) readPump() {
	defer func() {
		// Close first so writePump exits even if the hub is already shutting
		// down and never drains unregister.
		c.close()
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Msg("WebSocket read error")
			}
			break
		}

		c.handleMessage(message)
	}
}

// handleMessage processes incoming messages
func (c *WebSocketClient) handleMessage(message []byte) {
	// Try to parse as JSON command first
	cmd, err := c.hub.commandParser.ParseJSON(message)
	if err != nil {
		// Try text format
		cmd, err = c.hub.commandParser.ParseText(string(message))
		if err != nil {
			log.Warn().Str("message", string(message)).Err(err).Msg("Failed to parse command")
			c.sendError("invalid command format")
			return
		}
	}

	log.Debug().
		Str("type", cmd.Type).
		Str("target", cmd.Target).
		Str("action", cmd.Action).
		Msg("Received command")

	res, err := c.hub.executeCommand(cmd)
	if err != nil {
		logCommandError("websocket", cmd, err)
		c.sendError(err.Error())
		return
	}

	if res.State != nil {
		status := models.LoxoneStatus{
			Type:   "status",
			Device: cmd.Target,
			State:  *res.State,
		}
		data, _ := json.Marshal(status)
		c.trySend(data)
		return
	}

	c.sendAck(cmd.Target)
}

func (c *WebSocketClient) sendAck(target string) {
	msg := map[string]interface{}{
		"type":   "ack",
		"target": target,
	}
	data, _ := json.Marshal(msg)
	c.trySend(data)
}

func (c *WebSocketClient) sendError(message string) {
	msg := map[string]interface{}{
		"type":    "error",
		"message": message,
	}
	data, _ := json.Marshal(msg)
	c.trySend(data)
}

// writePump writes messages to the WebSocket connection
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-c.done:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return

		case message := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
)
