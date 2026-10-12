package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/sbeyeler/loxone2hue/internal/config"
	"github.com/sbeyeler/loxone2hue/internal/hue"
	"github.com/sbeyeler/loxone2hue/internal/logging"
	"github.com/sbeyeler/loxone2hue/internal/loxone"
	"github.com/sbeyeler/loxone2hue/internal/models"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	// feedbackDropped reports how many feedback jobs the WebSocket hub had to
	// discard; wired by NewServer, nil in isolation.
	feedbackDropped func() int64
	hueClient       *hue.Client
	mappingManager  *loxone.MappingManager
	udpSender       *loxone.UDPSender
	httpSender      *loxone.HTTPSender
}

// NewHandlers creates a new handlers instance
func NewHandlers(hueClient *hue.Client, mappingManager *loxone.MappingManager, udpSender *loxone.UDPSender, httpSender *loxone.HTTPSender) *Handlers {
	return &Handlers{
		hueClient:      hueClient,
		mappingManager: mappingManager,
		udpSender:      udpSender,
		httpSender:     httpSender,
	}
}

// saveConfig persists the configuration and reports failure to the client.
// On failure it runs rollback so memory and disk do not drift apart: a
// mapping that only exists in memory "works" until the next restart and is
// then gone, which is exactly the failure mode a read-only config volume
// used to produce silently. Returns false when the response has been sent.
func saveConfig(w http.ResponseWriter, what string, rollback func()) bool {
	err := config.Save()
	if err == nil {
		return true
	}

	log.Error().Err(err).Str("path", config.GetPath()).Msgf("Failed to save config after %s", what)
	if rollback != nil {
		rollback()
	}
	errorResponse(w, http.StatusInternalServerError,
		fmt.Sprintf("%s not saved: configuration file is not writable (%v)", what, err))
	return false
}

// JSON response helper
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Error response helper
func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

// Health returns the service health status
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	var feedbackDropped int64
	if h.feedbackDropped != nil {
		feedbackDropped = h.feedbackDropped()
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":                "healthy",
		"timestamp":             time.Now().UTC(),
		"hue_configured":        h.hueClient.IsConfigured(),
		"dropped_bridge_events": h.hueClient.DroppedEvents(),
		"dropped_feedback":      feedbackDropped,
	})
}

// TestBridgeConnection tests network connectivity to a HUE bridge
func (h *Handlers) TestBridgeConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BridgeIP string `json:"bridge_ip"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.BridgeIP == "" {
		errorResponse(w, http.StatusBadRequest, "bridge_ip required")
		return
	}

	results := make(map[string]interface{})
	results["bridge_ip"] = req.BridgeIP

	// Test 1: DNS/IP resolution
	dnsResult := make(map[string]interface{})
	ips, err := net.LookupIP(req.BridgeIP)
	if err != nil {
		dnsResult["success"] = false
		dnsResult["error"] = err.Error()
	} else {
		ipStrings := make([]string, len(ips))
		for i, ip := range ips {
			ipStrings[i] = ip.String()
		}
		dnsResult["success"] = true
		dnsResult["addresses"] = ipStrings
	}
	results["dns_lookup"] = dnsResult

	// Test 2: TCP connection to port 443 (HTTPS)
	tcp443Result := make(map[string]interface{})
	tcpConn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:443", req.BridgeIP), 5*time.Second)
	if err != nil {
		tcp443Result["success"] = false
		tcp443Result["error"] = err.Error()
	} else {
		tcpConn.Close()
		tcp443Result["success"] = true
	}
	results["tcp_443"] = tcp443Result

	// Test 3: TCP connection to port 80 (HTTP)
	tcp80Result := make(map[string]interface{})
	tcpConn80, err := net.DialTimeout("tcp", fmt.Sprintf("%s:80", req.BridgeIP), 5*time.Second)
	if err != nil {
		tcp80Result["success"] = false
		tcp80Result["error"] = err.Error()
	} else {
		tcpConn80.Close()
		tcp80Result["success"] = true
	}
	results["tcp_80"] = tcp80Result

	// Test 4: HTTPS request to bridge API
	httpsResult := make(map[string]interface{})
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	resp, err := client.Get(fmt.Sprintf("https://%s/api/config", req.BridgeIP))
	if err != nil {
		httpsResult["success"] = false
		httpsResult["error"] = err.Error()
	} else {
		resp.Body.Close()
		httpsResult["success"] = true
		httpsResult["status_code"] = resp.StatusCode
	}
	results["https_request"] = httpsResult

	log.Info().Interface("results", results).Msg("Bridge connection test completed")
	jsonResponse(w, http.StatusOK, results)
}

// GetBridge returns bridge information
func (h *Handlers) GetBridge(w http.ResponseWriter, r *http.Request) {
	if !h.hueClient.IsConfigured() {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"configured": false,
		})
		return
	}

	info, err := h.hueClient.GetBridgeInfo()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"configured": true,
		"info":       info,
	})
}

// DiscoverBridges discovers HUE bridges on the network
func (h *Handlers) DiscoverBridges(w http.ResponseWriter, r *http.Request) {
	bridges, err := hue.DiscoverBridges(5 * time.Second)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"bridges": bridges,
	})
}

// PairBridge pairs with a HUE bridge
func (h *Handlers) PairBridge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BridgeIP string `json:"bridge_ip"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.BridgeIP == "" {
		errorResponse(w, http.StatusBadRequest, "bridge_ip required")
		return
	}

	// Pair through a throwaway client. Pointing the shared client at the new
	// address first meant a failed attempt (typo, link button not pressed)
	// left every REST call going to the wrong bridge until the next restart.
	appKey, err := hue.NewClient(req.BridgeIP, "").Pair("Loxone2HUE", "gateway")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Save configuration. The pairing itself succeeded, but a key that is not
	// on disk is lost at the next restart, so treat that as a failure and
	// leave the previously configured bridge in place.
	prevHue := config.GetHue()
	config.UpdateHue(req.BridgeIP, appKey)
	if !saveConfig(w, "pairing", func() { config.UpdateHue(prevHue.BridgeIP, prevHue.ApplicationKey) }) {
		return
	}

	// Bring the event stream up against the freshly paired bridge, so status
	// feedback to Loxone starts without restarting the gateway.
	h.hueClient.Configure(req.BridgeIP, appKey)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"application_key": appKey,
	})
}

// GetDevices returns all lights
func (h *Handlers) GetDevices(w http.ResponseWriter, r *http.Request) {
	lights, err := h.hueClient.GetLights()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"devices": lights,
	})
}

// GetDevice returns a single light
func (h *Handlers) GetDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	light, err := h.hueClient.GetLight(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, light)
}

// SetDevice updates a light's state
func (h *Handlers) SetDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var cmd models.DeviceCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.hueClient.SetLightState(id, cmd); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetGroups returns all groups
func (h *Handlers) GetGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.hueClient.GetGroups()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"groups": groups,
	})
}

// GetGroup returns a single group
func (h *Handlers) GetGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	groups, err := h.hueClient.GetGroups()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, g := range groups {
		if g.ID == id {
			jsonResponse(w, http.StatusOK, g)
			return
		}
	}

	errorResponse(w, http.StatusNotFound, "group not found")
}

// SetGroup updates a group's state
func (h *Handlers) SetGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var cmd models.DeviceCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.hueClient.SetGroupState(id, cmd); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetSensors returns all sensors
func (h *Handlers) GetSensors(w http.ResponseWriter, r *http.Request) {
	sensors, err := h.hueClient.GetSensors()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"sensors": sensors,
	})
}

// GetScenes returns all scenes
func (h *Handlers) GetScenes(w http.ResponseWriter, r *http.Request) {
	scenes, err := h.hueClient.GetScenes()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"scenes": scenes,
	})
}

// ActivateScene activates a scene
func (h *Handlers) ActivateScene(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := h.hueClient.ActivateScene(id); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetMappings returns all mappings
func (h *Handlers) GetMappings(w http.ResponseWriter, r *http.Request) {
	mappings := config.GetMappings()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"mappings": mappings,
	})
}

// CreateMapping creates a new mapping
func (h *Handlers) CreateMapping(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var mapping models.Mapping
	if err := json.Unmarshal(body, &mapping); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// A missing "enabled" decodes to false, which is indistinguishable from an
	// explicit false. Probe the raw body so a client can create a disabled
	// mapping while omitting the field still means enabled.
	var probe struct {
		Enabled *bool `json:"enabled"`
	}
	_ = json.Unmarshal(body, &probe)

	mappings := config.GetMappings()
	for _, m := range mappings {
		if m.LoxoneID == mapping.LoxoneID {
			errorResponse(w, http.StatusConflict, fmt.Sprintf("mapping with loxone_id '%s' already exists", mapping.LoxoneID))
			return
		}
	}

	mapping.ID = uuid.New().String()
	if probe.Enabled == nil {
		mapping.Enabled = true
	}

	// Auto-assign miniserver if only one is configured and none was provided
	if mapping.MiniserverID == "" {
		if miniservers := config.GetLoxone().Miniservers; len(miniservers) == 1 {
			mapping.MiniserverID = miniservers[0].ID
		}
	}

	prev := config.GetMappings()
	mappings = append(mappings, mapping)
	config.UpdateMappings(mappings)
	h.mappingManager.Add(&mapping)

	if !saveConfig(w, "mapping", func() {
		config.UpdateMappings(prev)
		h.mappingManager.Load(prev)
	}) {
		return
	}

	jsonResponse(w, http.StatusCreated, mapping)
}

// UpdateMapping updates an existing mapping
func (h *Handlers) UpdateMapping(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var update models.Mapping
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	prev := config.GetMappings()
	mappings := config.GetMappings()
	found := false
	for i, m := range mappings {
		if m.ID == id {
			update.ID = id
			mappings[i] = update
			found = true
			break
		}
	}

	if !found {
		errorResponse(w, http.StatusNotFound, "mapping not found")
		return
	}

	config.UpdateMappings(mappings)
	h.mappingManager.Load(mappings)

	if !saveConfig(w, "mapping update", func() {
		config.UpdateMappings(prev)
		h.mappingManager.Load(prev)
	}) {
		return
	}

	jsonResponse(w, http.StatusOK, update)
}

// DeleteMapping deletes a mapping
func (h *Handlers) DeleteMapping(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	mappings := config.GetMappings()
	newMappings := make([]models.Mapping, 0, len(mappings))
	found := false

	for _, m := range mappings {
		if m.ID == id {
			found = true
			continue
		}
		newMappings = append(newMappings, m)
	}

	if !found {
		errorResponse(w, http.StatusNotFound, "mapping not found")
		return
	}

	config.UpdateMappings(newMappings)
	h.mappingManager.Remove(id)

	if !saveConfig(w, "mapping deletion", func() {
		config.UpdateMappings(mappings)
		h.mappingManager.Load(mappings)
	}) {
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// maskedPasswordSentinel is returned in place of a stored Loxone HTTP password.
// A client that submits this value back in UpdateConfig signals "keep unchanged".
const maskedPasswordSentinel = config.MaskedPassword

// GetConfig returns the current configuration
func (h *Handlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := config.Snapshot()

	// Mask Loxone HTTP passwords so they are never sent to clients in clear
	// text. The snapshot is already a copy, so masking in place is safe.
	safeLoxone := cfg.Loxone
	for i := range safeLoxone.Miniservers {
		if safeLoxone.Miniservers[i].HTTPPassword != "" {
			safeLoxone.Miniservers[i].HTTPPassword = maskedPasswordSentinel
		}
	}

	auth := config.GetAuth()

	// Don't expose sensitive data
	safeConfig := map[string]interface{}{
		"server": cfg.Server,
		"auth": map[string]interface{}{
			"enabled":  auth.Enabled(),
			"username": auth.User(),
		},
		"hue": map[string]interface{}{
			"bridge_ip":  cfg.Hue.BridgeIP,
			"configured": cfg.Hue.ApplicationKey != "",
		},
		"loxone":  safeLoxone,
		"logging": cfg.Logging,
	}

	jsonResponse(w, http.StatusOK, safeConfig)
}

// UpdateConfig updates the configuration
func (h *Handlers) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var update struct {
		Loxone *config.LoxoneConfig `json:"loxone,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if update.Loxone != nil {
		// Merges under the config lock and restores masked passwords
		config.UpdateLoxone(*update.Loxone)

		// Reconfigure the senders from the merged config, so they get the real
		// passwords rather than the sentinel from the request.
		merged := config.GetLoxone()

		if h.udpSender != nil {
			h.udpSender.Configure(merged.Miniservers)
		}

		if h.httpSender != nil {
			h.httpSender.Configure(merged.Miniservers)
		}
	}

	if err := config.Save(); err != nil {
		errorResponse(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetLogs returns recent log entries from the in-memory buffer
func (h *Handlers) GetLogs(w http.ResponseWriter, r *http.Request) {
	level := r.URL.Query().Get("level")
	search := r.URL.Query().Get("search")
	limitStr := r.URL.Query().Get("limit")

	limit := 200
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	entries := logging.Buffer.GetEntries(level, search, limit)
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
	})
}

// MappingsBackup represents a backup of mappings
type MappingsBackup struct {
	Version   string           `json:"version"`
	CreatedAt time.Time        `json:"created_at"`
	Mappings  []models.Mapping `json:"mappings"`
}

// TestUDP sends a test UDP message to a specific Loxone Miniserver
func (h *Handlers) TestUDP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LoxoneID     string `json:"loxone_id"`
		Property     string `json:"property"`
		Value        string `json:"value"`
		MiniserverID string `json:"miniserver_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.LoxoneID == "" || req.Property == "" {
		errorResponse(w, http.StatusBadRequest, "loxone_id and property required")
		return
	}

	if req.MiniserverID == "" {
		errorResponse(w, http.StatusBadRequest, "miniserver_id required")
		return
	}

	// Validate property + value
	valid, errMsg := validateUDPValue(req.Property, req.Value)
	if !valid {
		errorResponse(w, http.StatusBadRequest, errMsg)
		return
	}

	if h.udpSender == nil || !h.udpSender.IsEnabledFor(req.MiniserverID) {
		errorResponse(w, http.StatusBadRequest, "UDP feedback is not enabled for this miniserver")
		return
	}

	h.udpSender.Send(req.MiniserverID, req.LoxoneID, req.Property, req.Value)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "sent",
		"message": fmt.Sprintf("%s/%s:%s", req.LoxoneID, req.Property, req.Value),
	})
}

// validateUDPValue validates a value for a given UDP property
func validateUDPValue(property, value string) (bool, string) {
	switch property {
	case "on":
		if value != "0" && value != "1" {
			return false, "on: Wert muss 0 oder 1 sein"
		}
	case "bri":
		v, err := strconv.Atoi(value)
		if err != nil || v < 0 || v > 100 {
			return false, "bri: Wert muss eine Ganzzahl zwischen 0 und 100 sein"
		}
	case "ct":
		v, err := strconv.Atoi(value)
		if err != nil || v < 153 || v > 500 {
			return false, "ct: Wert muss eine Ganzzahl zwischen 153 und 500 sein"
		}
	case "color_x", "color_y":
		v, err := strconv.ParseFloat(value, 64)
		if err != nil || v < 0 || v > 1 {
			return false, property + ": Wert muss eine Dezimalzahl zwischen 0 und 1 sein"
		}
	default:
		return false, "Unbekannte Eigenschaft: " + property
	}
	return true, ""
}

// ExportMappings exports all mappings as a downloadable JSON file
func (h *Handlers) ExportMappings(w http.ResponseWriter, r *http.Request) {
	mappings := config.GetMappings()

	backup := MappingsBackup{
		Version:   "1.0",
		CreatedAt: time.Now().UTC(),
		Mappings:  mappings,
	}

	// Set headers for file download
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=loxone2hue-mappings-backup.json")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.Encode(backup)
}

// isUsableMapping reports whether an imported entry carries the fields a
// mapping needs to do anything. Entries failing this are counted as skipped
// instead of being written into the configuration as dead weight.
func isUsableMapping(m models.Mapping) bool {
	return m.LoxoneID != "" && m.HueID != "" && m.HueType != ""
}

// ImportMappings imports mappings from a JSON backup
func (h *Handlers) ImportMappings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode   string         `json:"mode"` // "replace" or "merge"
		Backup MappingsBackup `json:"backup"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Backup.Version == "" {
		errorResponse(w, http.StatusBadRequest, "invalid backup format: missing version")
		return
	}

	importedMappings := req.Backup.Mappings
	if importedMappings == nil {
		importedMappings = []models.Mapping{}
	}

	var resultMappings []models.Mapping
	var imported, skipped, updated int

	switch req.Mode {
	case "replace":
		// Replace all existing mappings
		resultMappings = make([]models.Mapping, 0, len(importedMappings))
		for _, m := range importedMappings {
			if !isUsableMapping(m) {
				skipped++
				continue
			}
			if m.ID == "" {
				m.ID = uuid.New().String()
			}
			resultMappings = append(resultMappings, m)
			imported++
		}

	case "merge":
		// Merge with existing mappings (skip duplicates by loxone_id)
		existingMappings := config.GetMappings()
		existingByLoxoneID := make(map[string]int)
		for i, m := range existingMappings {
			existingByLoxoneID[m.LoxoneID] = i
		}

		resultMappings = existingMappings

		for _, newMapping := range importedMappings {
			if !isUsableMapping(newMapping) {
				skipped++
				continue
			}
			if idx, exists := existingByLoxoneID[newMapping.LoxoneID]; exists {
				// Update existing mapping
				newMapping.ID = resultMappings[idx].ID
				resultMappings[idx] = newMapping
				updated++
			} else {
				// Add new mapping
				if newMapping.ID == "" {
					newMapping.ID = uuid.New().String()
				}
				resultMappings = append(resultMappings, newMapping)
				imported++
			}
		}

	default:
		errorResponse(w, http.StatusBadRequest, "invalid mode: use 'replace' or 'merge'")
		return
	}

	// Update config and mapping manager
	config.UpdateMappings(resultMappings)
	h.mappingManager.Load(resultMappings)

	if err := config.Save(); err != nil {
		log.Error().Err(err).Msg("Failed to save config after import")
		errorResponse(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"imported": imported,
		"updated":  updated,
		"skipped":  skipped,
		"total":    len(resultMappings),
	})
}
