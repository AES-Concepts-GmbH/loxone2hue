package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GetMappings returns all mappings
func (h *Handlers) GetMappings(w http.ResponseWriter, r *http.Request) {
	mappings := config.GetMappings()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"mappings": mappings,
	})
}

// CreateMapping creates a new mapping
func (h *Handlers) CreateMapping(w http.ResponseWriter, r *http.Request) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			errorResponse(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("request body exceeds %d bytes", tooLarge.Limit))
			return
		}
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var mapping models.Mapping
	if err := json.Unmarshal(body, &mapping); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !isUsableMapping(mapping) {
		errorResponse(w, http.StatusBadRequest, "invalid mapping fields or resource type")
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

	if err := validateMappings([]models.Mapping{mapping}, config.GetLoxone().Miniservers); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	candidate := config.Snapshot()
	candidate.Mappings = append(mappings, mapping)
	if !commitConfig(w, candidate) {
		return
	}
	h.mappingManager.Load(candidate.Mappings)

	jsonResponse(w, http.StatusCreated, mapping)
}

// UpdateMapping updates an existing mapping
func (h *Handlers) UpdateMapping(w http.ResponseWriter, r *http.Request) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	vars := mux.Vars(r)
	id := vars["id"]

	var update models.Mapping
	if !decodeJSON(w, r, &update, "invalid request body") {
		return
	}

	if !isUsableMapping(update) {
		errorResponse(w, http.StatusBadRequest, "loxone_id, hue_id and hue_type required")
		return
	}

	mappings := config.GetMappings()
	found := false
	for i, m := range mappings {
		if m.ID == id {
			update.ID = id
			mappings[i] = update
			found = true
			continue
		}
		// Same check as on create: two mappings with one loxone_id would make
		// the Loxone command ambiguous and the mapping manager keep only one.
		if m.LoxoneID == update.LoxoneID {
			errorResponse(w, http.StatusConflict, fmt.Sprintf("mapping with loxone_id '%s' already exists", update.LoxoneID))
			return
		}
	}

	if !found {
		errorResponse(w, http.StatusNotFound, "mapping not found")
		return
	}

	if err := validateMappings(mappings, config.GetLoxone().Miniservers); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	candidate := config.Snapshot()
	candidate.Mappings = mappings
	if !commitConfig(w, candidate) {
		return
	}
	h.mappingManager.Load(mappings)

	jsonResponse(w, http.StatusOK, update)
}

// DeleteMapping deletes a mapping
func (h *Handlers) DeleteMapping(w http.ResponseWriter, r *http.Request) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
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

	candidate := config.Snapshot()
	candidate.Mappings = newMappings
	if !commitConfig(w, candidate) {
		return
	}
	h.mappingManager.Load(newMappings)

	jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
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

	if !decodeJSON(w, r, &req, "invalid request body") {
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
	return strings.TrimSpace(m.LoxoneID) != "" && strings.TrimSpace(m.HueID) != "" && (m.HueType == "light" || m.HueType == "group" || m.HueType == "scene" || m.HueType == "sensor")
}

// ImportMappings imports mappings from a JSON backup
func (h *Handlers) ImportMappings(w http.ResponseWriter, r *http.Request) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	var req struct {
		Mode   string         `json:"mode"` // "replace" or "merge"
		Backup MappingsBackup `json:"backup"`
	}

	if !decodeJSON(w, r, &req, "invalid request body") {
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
				existingByLoxoneID[newMapping.LoxoneID] = len(resultMappings)
				resultMappings = append(resultMappings, newMapping)
				imported++
			}
		}

	default:
		errorResponse(w, http.StatusBadRequest, "invalid mode: use 'replace' or 'merge'")
		return
	}

	if err := validateMappings(resultMappings, config.GetLoxone().Miniservers); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	candidate := config.Snapshot()
	candidate.Mappings = resultMappings
	if !commitConfig(w, candidate) {
		return
	}
	h.mappingManager.Load(resultMappings)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"imported": imported,
		"updated":  updated,
		"skipped":  skipped,
		"total":    len(resultMappings),
	})
}
