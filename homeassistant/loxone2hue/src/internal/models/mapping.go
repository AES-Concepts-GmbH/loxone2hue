package models

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Mapping represents a mapping between Loxone and HUE resources
type Mapping struct {
	ID           string `json:"id" yaml:"id"`
	Name         string `json:"name" yaml:"name"`
	LoxoneID     string `json:"loxone_id" yaml:"loxone_id"` // Loxone UUID or custom ID
	HueID        string `json:"hue_id" yaml:"hue_id"`       // HUE resource ID
	HueType      string `json:"hue_type" yaml:"hue_type"`   // "light", "group", "scene"
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	FeedbackUDP  *bool  `json:"feedback_udp,omitempty" yaml:"feedback_udp,omitempty"`
	FeedbackHTTP *bool  `json:"feedback_http,omitempty" yaml:"feedback_http,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	MiniserverID string `json:"miniserver_id,omitempty" yaml:"miniserver_id,omitempty"`
}

// ShouldFeedbackUDP returns true if UDP feedback should be sent for this mapping.
// Defaults to true when FeedbackUDP is nil (not explicitly configured).
func (m *Mapping) ShouldFeedbackUDP() bool {
	if m.FeedbackUDP == nil {
		return true
	}
	return *m.FeedbackUDP
}

// ShouldFeedbackHTTP returns true if HTTP feedback should be sent for this mapping.
// Defaults to true when FeedbackHTTP is nil (not explicitly configured).
func (m *Mapping) ShouldFeedbackHTTP() bool {
	if m.FeedbackHTTP == nil {
		return true
	}
	return *m.FeedbackHTTP
}

// LoxoneCommand represents an incoming command from Loxone
type LoxoneCommand struct {
	Type   string                 `json:"type"`   // "command" or "query"
	Target string                 `json:"target"` // Mapped device/group ID
	Action string                 `json:"action"` // "set", "get", "scene"
	Params map[string]interface{} `json:"params,omitempty"`
}

// Params arrive from two directions with different Go types: the text parser
// produces int/float64/bool directly, while JSON decodes every number as
// float64. The accessors below accept both, so a command means the same thing
// whether it came in as text or as JSON.

// IntParam reads a numeric parameter as an int.
func (c *LoxoneCommand) IntParam(key string) (int, bool) {
	switch v := c.Params[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n), true
		}
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n, true
		}
	}
	return 0, false
}

// FloatParam reads a numeric parameter as a float64.
func (c *LoxoneCommand) FloatParam(key string) (float64, bool) {
	switch v := c.Params[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// BoolParam reads a boolean parameter, also accepting the numeric and textual
// forms a Loxone virtual output may send.
func (c *LoxoneCommand) BoolParam(key string) (bool, bool) {
	switch v := c.Params[key].(type) {
	case bool:
		return v, true
	case float64:
		return v != 0, true
	case int:
		return v != 0, true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f != 0, true
		}
	case string:
		switch strings.ToLower(v) {
		case "true", "on", "1":
			return true, true
		case "false", "off", "0":
			return false, true
		}
	}
	return false, false
}

// LoxoneStatus represents a status update sent to Loxone
type LoxoneStatus struct {
	Type   string      `json:"type"` // "status"
	Device string      `json:"device"`
	State  interface{} `json:"state"`
}

// WebSocketMessage is a generic WebSocket message wrapper
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}
