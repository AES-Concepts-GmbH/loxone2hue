package models

// Group represents a HUE room or zone
type Group struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Type   string     `json:"type"` // "room" or "zone"
	Lights []string   `json:"lights"`
	State  GroupState `json:"state"`
	Scenes []Scene    `json:"scenes,omitempty"`
	// GroupedLightID is the grouped_light service the bridge exposes for this
	// room or zone. Group commands go to it, and the bridge reports the
	// aggregated group state through it.
	GroupedLightID string `json:"grouped_light_id,omitempty"`
}

// GroupState represents the aggregated state of a group
type GroupState struct {
	AllOn      bool    `json:"all_on"`
	AnyOn      bool    `json:"any_on"`
	Brightness float64 `json:"brightness,omitempty"`
}

// Scene represents a HUE scene
type Scene struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	GroupID string `json:"group_id"`
	Type    string `json:"type"`
}
