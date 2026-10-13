package loxone

import (
	"math"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
)

// TestToDeviceCommandAcceptsJSONNumbers pins down the type mismatch between the
// two command paths: the text parser stores int/float64 directly, while JSON
// decodes every number as float64. Asserting on .(int) silently dropped
// color_temp for every JSON and WebSocket client.
func TestToDeviceCommandAcceptsJSONNumbers(t *testing.T) {
	p := NewCommandParser()

	// Exactly what a JSON client sends: numbers decoded as float64.
	cmd := &models.LoxoneCommand{
		Type:   "command",
		Target: "light_1",
		Action: "set",
		Params: map[string]interface{}{
			"on":         true,
			"brightness": float64(80),
			"color_temp": float64(300),
		},
	}

	dc := p.ToDeviceCommand(cmd)

	if dc.On == nil || !*dc.On {
		t.Error("on was dropped")
	}
	if dc.Brightness == nil || *dc.Brightness != 80 {
		t.Errorf("brightness = %v, want 80", dc.Brightness)
	}
	if dc.ColorTemp == nil {
		t.Fatal("color_temp was dropped for a JSON command")
	}
	if *dc.ColorTemp != 300 {
		t.Errorf("color_temp = %d, want 300", *dc.ColorTemp)
	}
}

// TestToDeviceCommandStillAcceptsTextTypes makes sure the coercion did not
// break the text parser path, which produces native Go types.
func TestToDeviceCommandStillAcceptsTextTypes(t *testing.T) {
	p := NewCommandParser()

	cmd, err := p.ParseText("SET light_1 CT 3000")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	dc := p.ToDeviceCommand(cmd)
	if dc.ColorTemp == nil {
		t.Fatal("color_temp was dropped for a text command")
	}
	// 3000 K is above the Kelvin threshold and converts to mirek.
	if *dc.ColorTemp != 1000000/3000 {
		t.Errorf("color_temp = %d, want %d", *dc.ColorTemp, 1000000/3000)
	}
}

func TestIntParamForms(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  int
		ok    bool
	}{
		{"int", 5, 5, true},
		{"float64 from JSON", float64(5), 5, true},
		{"string", "5", 5, true},
		{"missing", nil, 0, false},
		{"not a number", "abc", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &models.LoxoneCommand{Params: map[string]interface{}{}}
			if tc.value != nil {
				cmd.Params["mood_number"] = tc.value
			}

			got, ok := cmd.IntParam("mood_number")
			if ok != tc.ok || got != tc.want {
				t.Errorf("IntParam() = (%d, %v), want (%d, %v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// TestHexToXY checks the sRGB -> xy conversion against the published Philips
// HUE primaries. The previous hand-rolled pow() approximated the 2.4 exponent
// linearly, which skewed every mixed colour.
func TestHexToXY(t *testing.T) {
	cases := []struct {
		hex    string
		x, y   float64
		reason string
	}{
		{"#FF0000", 0.7006, 0.2993, "HUE red primary"},
		{"#00FF00", 0.1724, 0.7468, "HUE green primary"},
		{"#0000FF", 0.1355, 0.0399, "HUE blue primary"},
		{"#FFFFFF", 0.3227, 0.3290, "D65 white point"},
		// A mixed colour is what actually exercises the gamma curve: for the
		// pure primaries and for grey the exponent cancels out.
		{"#FF8000", 0.6112, 0.3750, "orange, exercises gamma"},
	}

	const tolerance = 0.001

	for _, tc := range cases {
		xy := hexToXY(tc.hex)
		if xy == nil {
			t.Errorf("%s (%s): got nil", tc.hex, tc.reason)
			continue
		}
		if math.Abs(xy[0]-tc.x) > tolerance || math.Abs(xy[1]-tc.y) > tolerance {
			t.Errorf("%s (%s): got (%.4f, %.4f), want (%.4f, %.4f)",
				tc.hex, tc.reason, xy[0], xy[1], tc.x, tc.y)
		}
	}
}

func TestHexToXYRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "#12345", "#GGGGGG", "nonsense"} {
		if xy := hexToXY(in); xy != nil {
			t.Errorf("hexToXY(%q) = %v, want nil", in, xy)
		}
	}
}

func TestGammaExpand(t *testing.T) {
	// Reference values for the sRGB transfer function.
	cases := []struct{ in, want float64 }{
		{0.0, 0.0},
		{1.0, 1.0},
		{0.5, 0.21404},
		{0.04, 0.00310},
	}

	for _, tc := range cases {
		if got := gammaExpand(tc.in); math.Abs(got-tc.want) > 0.0001 {
			t.Errorf("gammaExpand(%v) = %.5f, want %.5f", tc.in, got, tc.want)
		}
	}
}

// TestToDeviceCommandBrightnessImpliesPower covers the exported Virtual Output
// template, which drives a light with a single analog "SET <id> BRI <v>".
// The HUE API neither switches a lamp on for a dimming write nor accepts 0, so
// a bare brightness has to carry the power state itself.
func TestToDeviceCommandBrightnessImpliesPower(t *testing.T) {
	p := NewCommandParser()

	parse := func(text string) models.DeviceCommand {
		cmd, err := p.ParseText(text)
		if err != nil {
			t.Fatalf("ParseText(%q): %v", text, err)
		}
		return p.ToDeviceCommand(cmd)
	}

	// Dimmer at 60 while the lamp is off: must switch on.
	dc := parse("SET wz_decke BRI 60")
	if dc.On == nil || !*dc.On {
		t.Errorf("BRI 60: on = %v, want true", dc.On)
	}
	if dc.Brightness == nil || *dc.Brightness != 60 {
		t.Errorf("BRI 60: brightness = %v, want 60", dc.Brightness)
	}

	// Dimmer at 0: must switch off and must not write dimming 0, which the
	// bridge would clamp to the minimum brightness.
	dc = parse("SET wz_decke BRI 0")
	if dc.On == nil || *dc.On {
		t.Errorf("BRI 0: on = %v, want false", dc.On)
	}
	if dc.Brightness != nil {
		t.Errorf("BRI 0: brightness = %v, want none", *dc.Brightness)
	}

	// Out-of-range values are clamped rather than rejected by the bridge.
	dc = parse("SET wz_decke BRI 140")
	if dc.Brightness == nil || *dc.Brightness != 100 {
		t.Errorf("BRI 140: brightness = %v, want 100", dc.Brightness)
	}

	// An explicit power state wins over the derived one (JSON clients).
	dc = p.ToDeviceCommand(&models.LoxoneCommand{
		Type: "command", Target: "wz_decke", Action: "set",
		Params: map[string]interface{}{"on": false, "brightness": float64(60)},
	})
	if dc.On == nil || *dc.On {
		t.Errorf("explicit on=false with brightness: on = %v, want false", dc.On)
	}
}
