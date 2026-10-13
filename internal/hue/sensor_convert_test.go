package hue

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
)

// TestConvertHueSensor covers the CLIP v2 payload of every sensor kind the
// gateway knows, including the newer *_report fields that replaced the flat
// values on current bridge firmware.
func TestConvertHueSensor(t *testing.T) {
	owners := map[string]string{
		"dev-1":  "Flur Sensor",
		"room-1": "Wohnzimmer",
	}
	deviceOwner := `"owner":{"rid":"dev-1","rtype":"device"}`

	intp := func(v int) *int { return &v }
	boolp := func(v bool) *bool { return &v }
	strp := func(v string) *string { return &v }
	floatp := func(v float64) *float64 { return &v }

	cases := []struct {
		name       string
		sensorType string
		raw        string
		want       models.Sensor
		nameSuffix string
	}{
		{
			name:       "motion from motion_report",
			sensorType: "motion",
			raw:        `{"id":"s1",` + deviceOwner + `,"enabled":true,"motion":{"motion_report":{"changed":"2026-10-12T21:44:39Z","motion":true}}}`,
			want: models.Sensor{ID: "s1", Type: "motion", DeviceID: "dev-1", Owner: "Flur Sensor", Scope: models.ScopeDevice,
				State: models.SensorState{Motion: boolp(true), Enabled: true, LastUpdated: "2026-10-12T21:44:39Z"}},
		},
		{
			name:       "motion from legacy flag",
			sensorType: "motion",
			raw:        `{"id":"s1",` + deviceOwner + `,"enabled":false,"motion":{"motion":true,"motion_valid":true}}`,
			want: models.Sensor{ID: "s1", Type: "motion", DeviceID: "dev-1", Owner: "Flur Sensor", Scope: models.ScopeDevice,
				State: models.SensorState{Motion: boolp(true), Enabled: false}},
		},
		{
			name:       "temperature",
			sensorType: "temperature",
			raw:        `{"id":"s2",` + deviceOwner + `,"enabled":true,"temperature":{"temperature":21.5,"temperature_report":{"changed":"t1","temperature":21.5}}}`,
			want: models.Sensor{ID: "s2", Type: "temperature", DeviceID: "dev-1", Owner: "Flur Sensor", Scope: models.ScopeDevice,
				State: models.SensorState{Temperature: floatp(21.5), Enabled: true, LastUpdated: "t1"}},
			nameSuffix: " Temperature",
		},
		{
			name:       "light level with lux",
			sensorType: "light_level",
			raw:        `{"id":"s3",` + deviceOwner + `,"enabled":true,"light":{"light_level_report":{"changed":"t2","light_level":20001}}}`,
			want: models.Sensor{ID: "s3", Type: "light_level", DeviceID: "dev-1", Owner: "Flur Sensor", Scope: models.ScopeDevice,
				State: models.SensorState{LightLevel: intp(20001), Lux: intp(100), Enabled: true, LastUpdated: "t2"}},
			nameSuffix: " Light Level",
		},
		{
			name:       "button",
			sensorType: "button",
			raw:        `{"id":"s4",` + deviceOwner + `,"metadata":{"control_id":2},"button":{"button_report":{"updated":"t3","event":"short_release"}}}`,
			want: models.Sensor{ID: "s4", Type: "button", DeviceID: "dev-1", Owner: "Flur Sensor", Scope: models.ScopeDevice,
				State: models.SensorState{ButtonEvent: strp("short_release"), ControlID: intp(2), LastUpdated: "t3"}},
			nameSuffix: " Button 2",
		},
		{
			name:       "contact",
			sensorType: "contact",
			raw:        `{"id":"s5",` + deviceOwner + `,"enabled":true,"contact_report":{"changed":"t4","state":"no_contact"}}`,
			want: models.Sensor{ID: "s5", Type: "contact", DeviceID: "dev-1", Owner: "Flur Sensor", Scope: models.ScopeDevice,
				State: models.SensorState{ContactState: strp("no_contact"), Enabled: true, LastUpdated: "t4"}},
			nameSuffix: " Contact",
		},
		{
			name:       "rotary counter clockwise is negative",
			sensorType: "relative_rotary",
			raw:        `{"id":"s6",` + deviceOwner + `,"relative_rotary":{"rotary_report":{"updated":"t5","action":"start","rotation":{"direction":"counter_clockwise","steps":3,"duration":400}}}}`,
			want: models.Sensor{ID: "s6", Type: "relative_rotary", DeviceID: "dev-1", Owner: "Flur Sensor", Scope: models.ScopeDevice,
				State: models.SensorState{RotaryAction: strp("start"), RotarySteps: intp(-3), LastUpdated: "t5"}},
			nameSuffix: " Rotary",
		},
		{
			name:       "device power",
			sensorType: "device_power",
			raw:        `{"id":"s7",` + deviceOwner + `,"power_state":{"battery_level":85,"battery_state":"low"}}`,
			want: models.Sensor{ID: "s7", Type: "device_power", DeviceID: "dev-1", Owner: "Flur Sensor", Scope: models.ScopeDevice,
				State: models.SensorState{BatteryLevel: intp(85), BatteryState: strp("low")}},
			nameSuffix: " Battery",
		},
		{
			name:       "room motion aggregate",
			sensorType: "grouped_motion",
			raw:        `{"id":"gm1","owner":{"rid":"room-1","rtype":"room"},"enabled":true,"motion":{"motion_report":{"changed":"t6","motion":false}}}`,
			want: models.Sensor{ID: "gm1", Type: "grouped_motion", DeviceID: "room-1", Owner: "Wohnzimmer", Scope: models.ScopeRoom,
				State: models.SensorState{Motion: boolp(false), Enabled: true, LastUpdated: "t6"}},
			nameSuffix: " Bewegung",
		},
		{
			name:       "home light level aggregate without owner name",
			sensorType: "grouped_light_level",
			raw:        `{"id":"gl1","owner":{"rid":"home-1","rtype":"bridge_home"},"enabled":true,"light":{"light_level_report":{"changed":"t7","light_level":1}}}`,
			want: models.Sensor{ID: "gl1", Type: "grouped_light_level", DeviceID: "home-1", Owner: homeName, Scope: models.ScopeHome,
				State: models.SensorState{LightLevel: intp(1), Lux: intp(1), Enabled: true, LastUpdated: "t7"}},
			nameSuffix: " Helligkeit",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertHueSensor(json.RawMessage(tc.raw), tc.sensorType, owners)
			if got == nil {
				t.Fatal("convertHueSensor returned nil")
			}
			// The name is owner plus a kind-specific suffix; check it apart
			// from the rest so the comparison below can ignore it.
			if !strings.HasPrefix(got.Name, tc.want.Owner) {
				t.Errorf("name %q does not start with owner %q", got.Name, tc.want.Owner)
			}
			if tc.nameSuffix != "" && !strings.HasSuffix(got.Name, tc.nameSuffix) {
				t.Errorf("name %q lacks suffix %q", got.Name, tc.nameSuffix)
			}
			got.Name = ""
			if gotJSON, wantJSON := toJSON(t, got), toJSON(t, &tc.want); gotJSON != wantJSON {
				t.Errorf("sensor = %s\nwant     %s", gotJSON, wantJSON)
			}
		})
	}
}

// TestConvertHueSensorFallbackName: a sensor whose device is unknown still
// needs a usable name in the UI and a device scope.
func TestConvertHueSensorFallbackName(t *testing.T) {
	got := convertHueSensor(json.RawMessage(`{"id":"0123456789abcdef","type":"temperature","temperature":{"temperature":20}}`), "temperature", nil)
	if got == nil {
		t.Fatal("convertHueSensor returned nil")
	}
	if got.Name != "Sensor 01234567 Temperature" {
		t.Errorf("fallback name = %q, want %q", got.Name, "Sensor 01234567 Temperature")
	}
	if got.Scope != models.ScopeDevice {
		t.Errorf("scope = %q, want device", got.Scope)
	}
}

func toJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
