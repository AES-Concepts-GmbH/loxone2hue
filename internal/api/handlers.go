package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/hue"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/logging"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/loxone"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// mutationMu serializes configuration changes, including persistence and rollback.
var mutationMu sync.Mutex

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

// decodeJSON reads a JSON request body into v. It answers 413 when the body
// exceeded the limit set by bodyLimitMiddleware and 400 (with msg) for
// anything else that is not valid JSON, and reports whether decoding worked.
func decodeJSON(w http.ResponseWriter, r *http.Request, v interface{}, msg string) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			errorResponse(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("request body exceeds %d bytes", tooLarge.Limit))
			return false
		}
		errorResponse(w, http.StatusBadRequest, msg)
		return false
	}
	return true
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

// isPrivateHost reports whether host (an IP, a hostname, optionally with a
// port) lies in a private, loopback or link-local network. The bridge test
// and the pairing make the gateway open connections to whatever address the
// caller supplies; a Hue Bridge is always on the LAN, so anything else is
// refused rather than letting the gateway probe arbitrary hosts.
func isPrivateHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return isPrivateIP(ip)
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, a := range addrs {
		if !isPrivateIP(a.IP) {
			return false
		}
	}
	return true
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
}

// requirePrivateBridge rejects bridge addresses outside the local network.
func requirePrivateBridge(w http.ResponseWriter, bridgeIP string) bool {
	if isPrivateHost(bridgeIP) {
		return true
	}
	errorResponse(w, http.StatusBadRequest, "bridge_ip must be an address in the local network (private IP range)")
	return false
}

// TestBridgeConnection tests network connectivity to a HUE bridge
func (h *Handlers) TestBridgeConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BridgeIP string `json:"bridge_ip"`
	}

	if !decodeJSON(w, r, &req, "invalid request body") {
		return
	}

	if req.BridgeIP == "" {
		errorResponse(w, http.StatusBadRequest, "bridge_ip required")
		return
	}
	if !requirePrivateBridge(w, req.BridgeIP) {
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
	mutationMu.Lock()
	defer mutationMu.Unlock()
	var req struct {
		BridgeIP string `json:"bridge_ip"`
	}

	if !decodeJSON(w, r, &req, "invalid request body") {
		return
	}

	if req.BridgeIP == "" {
		errorResponse(w, http.StatusBadRequest, "bridge_ip required")
		return
	}
	if !requirePrivateBridge(w, req.BridgeIP) {
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
	candidate := config.Snapshot()
	candidate.Hue = config.HueConfig{BridgeIP: req.BridgeIP, ApplicationKey: appKey}
	if !commitConfig(w, candidate) {
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
	if !decodeJSON(w, r, &cmd, "invalid request body") {
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
	if !decodeJSON(w, r, &cmd, "invalid request body") {
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
	mutationMu.Lock()
	defer mutationMu.Unlock()
	var update struct {
		Loxone *config.LoxoneConfig `json:"loxone,omitempty"`
	}

	if !decodeJSON(w, r, &update, "invalid request body") {
		return
	}

	candidate := config.Snapshot()
	if update.Loxone != nil {
		stored := map[string]string{}
		for _, ms := range candidate.Loxone.Miniservers {
			stored[ms.ID] = ms.HTTPPassword
		}
		for i := range update.Loxone.Miniservers {
			ms := &update.Loxone.Miniservers[i]
			if ms.HTTPPassword == config.MaskedPassword {
				ms.HTTPPassword = stored[ms.ID]
			}
		}
		candidate.Loxone = *update.Loxone
		if candidate.Loxone.Miniservers == nil {
			candidate.Loxone.Miniservers = []config.MiniserverConfig{}
		}
	}
	if !commitConfig(w, candidate) {
		return
	}

	if update.Loxone != nil {
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
