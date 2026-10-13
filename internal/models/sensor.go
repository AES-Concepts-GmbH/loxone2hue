package models

import "math"

// Sensor represents a HUE sensor resource.
//
// Type is the CLIP v2 resource type. Besides the per-device sensors ("motion",
// "temperature", "light_level", "button", "contact", "relative_rotary",
// "device_power") the bridge aggregates motion and light level per room and
// for the whole home ("grouped_motion", "grouped_light_level"). Those carry
// the same payload as their per-device counterparts and differ only in owner.
type Sensor struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	DeviceID string `json:"device_id,omitempty"`
	Owner    string `json:"owner,omitempty"` // parent device, room or home name
	// Scope says what the sensor reports on: "device" (a physical sensor),
	// "room" or "home" (aggregated by the bridge).
	Scope string      `json:"scope,omitempty"`
	State SensorState `json:"state"`
}

// Sensor scopes, derived from the owner's resource type.
const (
	ScopeDevice = "device"
	ScopeRoom   = "room"
	ScopeHome   = "home"
)

// SensorState represents the current state of a sensor
type SensorState struct {
	// Motion sensor
	Motion *bool `json:"motion,omitempty"`

	// Temperature sensor (Celsius)
	Temperature *float64 `json:"temperature,omitempty"`

	// Ambient light, as the bridge reports it: 10000 * log10(lux) + 1.
	LightLevel *int `json:"light_level,omitempty"`
	// Lux is LightLevel converted to lux, set whenever LightLevel is.
	Lux *int `json:"lux,omitempty"`

	// Button
	ButtonEvent *string `json:"button_event,omitempty"` // "initial_press", "repeat", "short_release", "long_release", "long_press"
	ControlID   *int    `json:"control_id,omitempty"`   // button index (1-based)

	// Contact sensor
	ContactState *string `json:"contact_state,omitempty"` // "contact", "no_contact"

	// Rotary (e.g., Hue Tap Dial)
	RotaryAction *string `json:"rotary_action,omitempty"` // "start", "repeat"
	RotarySteps  *int    `json:"rotary_steps,omitempty"`  // positive=clockwise, negative=counter-clockwise

	// Device power / battery
	BatteryLevel *int    `json:"battery_level,omitempty"` // 0-100
	BatteryState *string `json:"battery_state,omitempty"` // "normal", "low", "critical"

	// Common
	Enabled     bool   `json:"enabled"`
	LastUpdated string `json:"last_updated,omitempty"`
}

// LuxFromLightLevel converts the bridge's logarithmic light level
// (10000 * log10(lux) + 1) to lux, rounded to whole lux.
func LuxFromLightLevel(level int) int {
	if level <= 0 {
		return 0
	}
	return int(math.Round(math.Pow(10, float64(level-1)/10000)))
}

// SetLightLevel stores a raw light level together with its lux value.
func (s *SensorState) SetLightLevel(level int) {
	lux := LuxFromLightLevel(level)
	s.LightLevel = &level
	s.Lux = &lux
}
