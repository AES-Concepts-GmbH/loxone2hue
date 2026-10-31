package hue

import (
	"encoding/json"
	"fmt"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
)

type hueLight struct {
	ID    string `json:"id"`
	Owner *struct {
		RID   string `json:"rid"`
		RType string `json:"rtype"`
	} `json:"owner,omitempty"`
	Metadata struct {
		Name      string `json:"name"`
		Archetype string `json:"archetype"`
	} `json:"metadata"`
	ProductData struct {
		ModelID     string `json:"model_id"`
		ProductName string `json:"product_name"`
	} `json:"product_data"`
	On struct {
		On bool `json:"on"`
	} `json:"on"`
	Dimming *struct {
		Brightness float64 `json:"brightness"`
	} `json:"dimming,omitempty"`
	ColorTemperature *struct {
		Mirek      int  `json:"mirek"`
		MirekValid bool `json:"mirek_valid"`
	} `json:"color_temperature,omitempty"`
	Color *struct {
		XY struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"xy"`
		Gamut struct {
			Red   struct{ X, Y float64 } `json:"red"`
			Green struct{ X, Y float64 } `json:"green"`
			Blue  struct{ X, Y float64 } `json:"blue"`
		} `json:"gamut"`
		GamutType string `json:"gamut_type"`
	} `json:"color,omitempty"`
}

type hueRoom struct {
	ID       string `json:"id"`
	Metadata struct {
		Name      string `json:"name"`
		Archetype string `json:"archetype"`
	} `json:"metadata"`
	Children []struct {
		RID   string `json:"rid"`
		RType string `json:"rtype"`
	} `json:"children"`
	Services []struct {
		RID   string `json:"rid"`
		RType string `json:"rtype"`
	} `json:"services"`
}

type hueScene struct {
	ID       string `json:"id"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Group struct {
		RID   string `json:"rid"`
		RType string `json:"rtype"`
	} `json:"group"`
}

func convertHueLight(hl hueLight) *models.Light {
	light := &models.Light{
		ID:          hl.ID,
		Name:        hl.Metadata.Name,
		Type:        hl.Metadata.Archetype,
		ModelID:     hl.ProductData.ModelID,
		ProductName: hl.ProductData.ProductName,
		State: models.LightState{
			On:        hl.On.On,
			Reachable: true,
		},
		Capabilities: models.Capabilities{},
	}

	if hl.Dimming != nil {
		light.State.Brightness = hl.Dimming.Brightness
		light.Capabilities.SupportsDimming = true
	}

	if hl.ColorTemperature != nil && hl.ColorTemperature.MirekValid {
		light.State.ColorTemp = hl.ColorTemperature.Mirek
		light.Capabilities.SupportsColorTemp = true
	}

	if hl.Color != nil {
		light.State.Color = &models.Color{
			XY:    [2]float64{hl.Color.XY.X, hl.Color.XY.Y},
			Gamut: hl.Color.GamutType,
		}
		light.Capabilities.SupportsColor = true
	}

	return light
}

func convertHueRoom(hr hueRoom, deviceToLightID map[string]string) *models.Group {
	group := &models.Group{
		ID:     hr.ID,
		Name:   hr.Metadata.Name,
		Type:   "room",
		Lights: make([]string, 0),
	}

	for _, child := range hr.Children {
		if child.RType == "device" {
			// Map device ID to light ID
			if lightID, ok := deviceToLightID[child.RID]; ok {
				group.Lights = append(group.Lights, lightID)
			}
		}
	}

	for _, svc := range hr.Services {
		if svc.RType == "grouped_light" {
			group.GroupedLightID = svc.RID
			break
		}
	}

	return group
}

func convertHueScene(hs hueScene) *models.Scene {
	return &models.Scene{
		ID:      hs.ID,
		Name:    hs.Metadata.Name,
		GroupID: hs.Group.RID,
		Type:    hs.Group.RType,
	}
}

// convertHueSensor converts raw JSON sensor data into a Sensor model
// sensorNameSuffix is appended to the owner name to tell a device's sensors
// apart ("Flur Motion", "Flur Temperature"). The aggregated room sensors get
// German suffixes to match the UI wording for rooms.
var sensorNameSuffix = map[string]string{
	"motion":              " Motion",
	"temperature":         " Temperature",
	"light_level":         " Light Level",
	"contact":             " Contact",
	"grouped_motion":      " Bewegung",
	"grouped_light_level": " Helligkeit",
}

// scopeForOwner derives the sensor scope from the owner's resource type.
func scopeForOwner(rtype string) string {
	switch rtype {
	case "room", "zone":
		return models.ScopeRoom
	case "bridge_home":
		return models.ScopeHome
	default:
		return models.ScopeDevice
	}
}

func convertHueSensor(raw json.RawMessage, sensorType string, ownerNames map[string]string) *models.Sensor {
	// Common fields present in all sensor types
	var base struct {
		ID    string `json:"id"`
		Owner *struct {
			RID   string `json:"rid"`
			RType string `json:"rtype"`
		} `json:"owner,omitempty"`
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		return nil
	}

	sensor := &models.Sensor{
		ID:    base.ID,
		Type:  sensorType,
		Scope: models.ScopeDevice,
		State: models.SensorState{
			Enabled: base.Enabled,
		},
	}

	if base.Owner != nil {
		sensor.DeviceID = base.Owner.RID
		sensor.Scope = scopeForOwner(base.Owner.RType)
		if name, ok := ownerNames[base.Owner.RID]; ok {
			sensor.Owner = name
			sensor.Name = name
		} else if base.Owner.RType == "bridge_home" {
			sensor.Owner = homeName
			sensor.Name = homeName
		}
	}

	// Fallback base name when the owning device or room is unknown; the
	// kind-specific suffix below is appended to it like to any other name.
	if sensor.Name == "" {
		shortID := sensor.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		sensor.Name = fmt.Sprintf("Sensor %s", shortID)
	}

	switch SensorKind(sensorType) {
	case KindMotion:
		var m struct {
			Motion struct {
				Motion       bool `json:"motion"`
				MotionValid  bool `json:"motion_valid"`
				MotionReport *struct {
					Changed string `json:"changed"`
					Motion  bool   `json:"motion"`
				} `json:"motion_report,omitempty"`
			} `json:"motion"`
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &m); err == nil {
			// Newer firmware (and the aggregated grouped_motion) only fills
			// motion_report; the top-level motion flag is legacy and may be
			// absent, so the report wins when present.
			motion := m.Motion.Motion
			if m.Motion.MotionReport != nil {
				motion = m.Motion.MotionReport.Motion
				sensor.State.LastUpdated = m.Motion.MotionReport.Changed
			}
			sensor.State.Motion = &motion
			sensor.State.Enabled = m.Enabled
			sensor.Name = sensor.Name + sensorNameSuffix[sensorType]
		}

	case KindTemperature:
		var t struct {
			Temperature struct {
				Temperature       float64 `json:"temperature"`
				TemperatureValid  bool    `json:"temperature_valid"`
				TemperatureReport *struct {
					Changed     string  `json:"changed"`
					Temperature float64 `json:"temperature"`
				} `json:"temperature_report,omitempty"`
			} `json:"temperature"`
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &t); err == nil {
			temp := t.Temperature.Temperature
			sensor.State.Temperature = &temp
			sensor.State.Enabled = t.Enabled
			if t.Temperature.TemperatureReport != nil {
				sensor.State.LastUpdated = t.Temperature.TemperatureReport.Changed
			}
			sensor.Name = sensor.Name + sensorNameSuffix[sensorType]
		}

	case KindLightLevel:
		var l struct {
			Light struct {
				LightLevel       int  `json:"light_level"`
				LightLevelValid  bool `json:"light_level_valid"`
				LightLevelReport *struct {
					Changed    string `json:"changed"`
					LightLevel int    `json:"light_level"`
				} `json:"light_level_report,omitempty"`
			} `json:"light"`
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &l); err == nil {
			level := l.Light.LightLevel
			if l.Light.LightLevelReport != nil {
				level = l.Light.LightLevelReport.LightLevel
				sensor.State.LastUpdated = l.Light.LightLevelReport.Changed
			}
			sensor.State.SetLightLevel(level)
			sensor.State.Enabled = l.Enabled
			sensor.Name = sensor.Name + sensorNameSuffix[sensorType]
		}

	case KindButton:
		var b struct {
			Metadata struct {
				ControlID int `json:"control_id"`
			} `json:"metadata"`
			Button struct {
				ButtonReport *struct {
					Updated string `json:"updated"`
					Event   string `json:"event"`
				} `json:"button_report,omitempty"`
				LastEvent string `json:"last_event"`
			} `json:"button"`
		}
		if err := json.Unmarshal(raw, &b); err == nil {
			controlID := b.Metadata.ControlID
			sensor.State.ControlID = &controlID
			event := b.Button.LastEvent
			if b.Button.ButtonReport != nil {
				event = b.Button.ButtonReport.Event
				sensor.State.LastUpdated = b.Button.ButtonReport.Updated
			}
			if event != "" {
				sensor.State.ButtonEvent = &event
			}
			sensor.Name = fmt.Sprintf("%s Button %d", sensor.Name, controlID)
		}

	case KindContact:
		var ct struct {
			ContactReport *struct {
				Changed string `json:"changed"`
				State   string `json:"state"`
			} `json:"contact_report,omitempty"`
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &ct); err == nil {
			sensor.State.Enabled = ct.Enabled
			if ct.ContactReport != nil {
				sensor.State.ContactState = &ct.ContactReport.State
				sensor.State.LastUpdated = ct.ContactReport.Changed
			}
			sensor.Name = sensor.Name + sensorNameSuffix[sensorType]
		}

	case KindRotary:
		var rr struct {
			RelativeRotary struct {
				RotaryReport *struct {
					Updated  string `json:"updated"`
					Action   string `json:"action"`
					Rotation struct {
						Direction string `json:"direction"`
						Steps     int    `json:"steps"`
						Duration  int    `json:"duration"`
					} `json:"rotation"`
				} `json:"rotary_report,omitempty"`
				LastEvent *struct {
					Action   string `json:"action"`
					Rotation struct {
						Direction string `json:"direction"`
						Steps     int    `json:"steps"`
					} `json:"rotation"`
				} `json:"last_event,omitempty"`
			} `json:"relative_rotary"`
		}
		if err := json.Unmarshal(raw, &rr); err == nil {
			if rr.RelativeRotary.RotaryReport != nil {
				sensor.State.RotaryAction = &rr.RelativeRotary.RotaryReport.Action
				steps := rr.RelativeRotary.RotaryReport.Rotation.Steps
				if rr.RelativeRotary.RotaryReport.Rotation.Direction == "counter_clockwise" {
					steps = -steps
				}
				sensor.State.RotarySteps = &steps
				sensor.State.LastUpdated = rr.RelativeRotary.RotaryReport.Updated
			}
			sensor.Name = sensor.Name + " Rotary"
		}

	case KindPower:
		var dp struct {
			PowerState struct {
				BatteryLevel int    `json:"battery_level"`
				BatteryState string `json:"battery_state"`
			} `json:"power_state"`
		}
		if err := json.Unmarshal(raw, &dp); err == nil {
			sensor.State.BatteryLevel = &dp.PowerState.BatteryLevel
			sensor.State.BatteryState = &dp.PowerState.BatteryState
			sensor.Name = sensor.Name + " Battery"
		}
	}

	return sensor
}
