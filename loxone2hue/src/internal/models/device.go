package models

// Light represents a HUE light device
type Light struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Type         string       `json:"type"`
	ModelID      string       `json:"model_id"`
	ProductName  string       `json:"product_name"`
	State        LightState   `json:"state"`
	Capabilities Capabilities `json:"capabilities,omitempty"`
}

// LightState represents the current state of a light
type LightState struct {
	On         bool    `json:"on"`
	Brightness float64 `json:"brightness"`           // 0-100
	ColorTemp  int     `json:"color_temp,omitempty"` // Mirek (153-500)
	Color      *Color  `json:"color,omitempty"`
	Reachable  bool    `json:"reachable"`
}

// Color represents color in XY color space
type Color struct {
	XY     [2]float64 `json:"xy"`
	Gamut  string     `json:"gamut,omitempty"`
	HexRGB string     `json:"hex_rgb,omitempty"`
}

// Capabilities describes what a light can do
type Capabilities struct {
	SupportsColor     bool `json:"supports_color"`
	SupportsColorTemp bool `json:"supports_color_temp"`
	SupportsDimming   bool `json:"supports_dimming"`
}

// DeviceCommand represents a command to control a device
type DeviceCommand struct {
	On         *bool    `json:"on,omitempty"`
	Brightness *float64 `json:"brightness,omitempty"`
	ColorTemp  *int     `json:"color_temp,omitempty"`
	Color      *Color   `json:"color,omitempty"`
}
