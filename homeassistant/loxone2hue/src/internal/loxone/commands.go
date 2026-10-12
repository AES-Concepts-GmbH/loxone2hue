package loxone

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sbeyeler/loxone2hue/internal/models"
)

// CommandParser parses Loxone commands
type CommandParser struct{}

// NewCommandParser creates a new command parser
func NewCommandParser() *CommandParser {
	return &CommandParser{}
}

// ParseJSON parses a JSON command from Loxone
func (p *CommandParser) ParseJSON(data []byte) (*models.LoxoneCommand, error) {
	var cmd models.LoxoneCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

// ParseText parses a simple text command
// Format: SET <target> <property> [value]
// Examples:
//   - SET light_1 ON
//   - SET light_1 OFF
//   - SET light_1 BRI 80
//   - SET light_1 COLOR #FF5500
//   - SET light_1 CT 3000
//   - SET group_1 SCENE relax
//   - GET light_1 STATUS
//   - SCENE <scene_mapping_id>       - Activate a scene by mapping ID
//   - MOOD <target> <mood_number>    - Activate scene for mood number (0=off)
func (p *CommandParser) ParseText(text string) (*models.LoxoneCommand, error) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid command format: %s", text)
	}

	cmdType := strings.ToUpper(parts[0])

	// Handle SCENE command (only 2 parts needed)
	if cmdType == "SCENE" {
		if len(parts) < 2 {
			return nil, fmt.Errorf("scene mapping ID required")
		}
		return &models.LoxoneCommand{
			Type:   "command",
			Target: parts[1],
			Action: "scene",
			Params: map[string]interface{}{
				"scene_id": parts[1],
			},
		}, nil
	}

	// Handle MOOD command: MOOD <target> <mood_number>
	if cmdType == "MOOD" {
		if len(parts) < 3 {
			return nil, fmt.Errorf("mood command requires target and mood number")
		}
		target := parts[1]
		moodNum, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid mood number: %s", parts[2])
		}
		return &models.LoxoneCommand{
			Type:   "command",
			Target: target,
			Action: "mood",
			Params: map[string]interface{}{
				"mood_number": moodNum,
			},
		}, nil
	}

	// Other commands need at least 3 parts
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid command format: %s", text)
	}

	target := parts[1]
	action := strings.ToUpper(parts[2])

	cmd := &models.LoxoneCommand{
		Target: target,
		Params: make(map[string]interface{}),
	}

	switch cmdType {
	case "SET":
		cmd.Type = "command"
		cmd.Action = "set"

		switch action {
		case "ON":
			cmd.Params["on"] = true
		case "OFF":
			cmd.Params["on"] = false
		case "BRI":
			if len(parts) < 4 {
				return nil, fmt.Errorf("brightness value required")
			}
			bri, err := strconv.ParseFloat(parts[3], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid brightness value: %s", parts[3])
			}
			cmd.Params["brightness"] = bri
		case "COLOR":
			if len(parts) < 4 {
				return nil, fmt.Errorf("color value required")
			}
			cmd.Params["color"] = parts[3]
		case "CT":
			if len(parts) < 4 {
				return nil, fmt.Errorf("color temperature value required")
			}
			ct, err := strconv.Atoi(parts[3])
			if err != nil {
				return nil, fmt.Errorf("invalid color temperature: %s", parts[3])
			}
			cmd.Params["color_temp"] = ct
		case "SCENE":
			if len(parts) < 4 {
				return nil, fmt.Errorf("scene ID required")
			}
			cmd.Action = "scene"
			cmd.Params["scene_id"] = parts[3]
		default:
			return nil, fmt.Errorf("unknown action: %s", action)
		}

	case "GET":
		cmd.Type = "query"
		cmd.Action = action

	default:
		return nil, fmt.Errorf("unknown command type: %s", cmdType)
	}

	return cmd, nil
}

// ToDeviceCommand converts Loxone command params to a DeviceCommand
func (p *CommandParser) ToDeviceCommand(cmd *models.LoxoneCommand) models.DeviceCommand {
	dc := models.DeviceCommand{}

	if on, ok := cmd.BoolParam("on"); ok {
		dc.On = &on
	}

	if bri, ok := cmd.FloatParam("brightness"); ok {
		// Loxone dimmers drive a single analog output: 0 means off, anything
		// above means on at that level. The HUE API does not infer either —
		// a dimming write neither turns a lamp on nor accepts 0 — so derive
		// the power state here unless the command set it explicitly.
		switch {
		case bri <= 0:
			if dc.On == nil {
				off := false
				dc.On = &off
			}
			// No dimming write: 0 is rejected by the bridge and would be
			// clamped to the minimum brightness instead of switching off.
		default:
			if bri > 100 {
				bri = 100
			}
			dc.Brightness = &bri
			if dc.On == nil {
				on := true
				dc.On = &on
			}
		}
	}

	if ct, ok := cmd.IntParam("color_temp"); ok {
		// Convert Kelvin to Mirek if needed
		mirek := ct
		if ct > 1000 {
			// Assume Kelvin, convert to Mirek
			mirek = 1000000 / ct
		}
		dc.ColorTemp = &mirek
	}

	if color, ok := cmd.Params["color"].(string); ok {
		xy := hexToXY(color)
		if xy != nil {
			dc.Color = &models.Color{XY: *xy}
		}
	}

	return dc
}

// hexToXY converts a hex color string to XY color space
// This is a simplified conversion - real implementation would need
// proper color space transformation based on gamut
func hexToXY(hex string) *[2]float64 {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return nil
	}

	r, err := strconv.ParseInt(hex[0:2], 16, 64)
	if err != nil {
		return nil
	}
	g, err := strconv.ParseInt(hex[2:4], 16, 64)
	if err != nil {
		return nil
	}
	b, err := strconv.ParseInt(hex[4:6], 16, 64)
	if err != nil {
		return nil
	}

	// Normalize to 0-1
	rNorm := float64(r) / 255.0
	gNorm := float64(g) / 255.0
	bNorm := float64(b) / 255.0

	// Apply gamma correction
	rNorm = gammaExpand(rNorm)
	gNorm = gammaExpand(gNorm)
	bNorm = gammaExpand(bNorm)

	// Convert to XYZ
	X := rNorm*0.664511 + gNorm*0.154324 + bNorm*0.162028
	Y := rNorm*0.283881 + gNorm*0.668433 + bNorm*0.047685
	Z := rNorm*0.000088 + gNorm*0.072310 + bNorm*0.986039

	// Convert to xy
	sum := X + Y + Z
	if sum == 0 {
		return &[2]float64{0.33, 0.33}
	}

	x := X / sum
	y := Y / sum

	return &[2]float64{x, y}
}

// gammaExpand converts an sRGB channel value to linear light.
func gammaExpand(v float64) float64 {
	if v > 0.04045 {
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return v / 12.92
}
