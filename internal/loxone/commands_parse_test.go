package loxone

import (
	"reflect"
	"testing"
)

// TestParseText pins the text protocol a Loxone Virtueller Ausgang speaks
// (GET /ws?cmd=...). Every accepted form and every rejection here is part of
// the contract with existing Loxone configurations.
func TestParseText(t *testing.T) {
	p := NewCommandParser()

	cases := []struct {
		text   string
		typ    string
		target string
		action string
		params map[string]interface{}
	}{
		{"SET wz_decke ON", "command", "wz_decke", "set", map[string]interface{}{"on": true}},
		{"SET wz_decke OFF", "command", "wz_decke", "set", map[string]interface{}{"on": false}},
		// Lower case and extra whitespace come from hand-typed Loxone commands.
		{"  set   wz_decke   on ", "command", "wz_decke", "set", map[string]interface{}{"on": true}},
		{"SET wz_decke BRI 80", "command", "wz_decke", "set", map[string]interface{}{"brightness": 80.0}},
		{"SET wz_decke BRI 42.5", "command", "wz_decke", "set", map[string]interface{}{"brightness": 42.5}},
		{"SET wz_decke CT 3000", "command", "wz_decke", "set", map[string]interface{}{"color_temp": 3000}},
		{"SET wz_decke COLOR #FF5500", "command", "wz_decke", "set", map[string]interface{}{"color": "#FF5500"}},
		{"SET wz SCENE relax", "command", "wz", "scene", map[string]interface{}{"scene_id": "relax"}},
		{"GET wz_decke STATUS", "query", "wz_decke", "STATUS", map[string]interface{}{}},
		{"SCENE abend", "command", "abend", "scene", map[string]interface{}{"scene_id": "abend"}},
		{"MOOD wz 2", "command", "wz", "mood", map[string]interface{}{"mood_number": 2}},
		{"MOOD wz 0", "command", "wz", "mood", map[string]interface{}{"mood_number": 0}},
	}

	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			cmd, err := p.ParseText(tc.text)
			if err != nil {
				t.Fatalf("ParseText(%q): %v", tc.text, err)
			}
			if cmd.Type != tc.typ || cmd.Target != tc.target || cmd.Action != tc.action {
				t.Errorf("got %s/%s/%s, want %s/%s/%s", cmd.Type, cmd.Target, cmd.Action, tc.typ, tc.target, tc.action)
			}
			if !reflect.DeepEqual(cmd.Params, tc.params) {
				t.Errorf("params = %#v, want %#v", cmd.Params, tc.params)
			}
		})
	}
}

// TestParseTextRejectsMalformedCommands: a malformed command must produce an
// error (and thereby a 400 to Loxone), never a half-parsed command that ends
// up as a bridge call.
func TestParseTextRejectsMalformedCommands(t *testing.T) {
	p := NewCommandParser()

	for _, text := range []string{
		"",
		"SET",
		"SET wz_decke",
		"SET wz_decke BRI",
		"SET wz_decke BRI hell",
		"SET wz_decke CT warm",
		"SET wz_decke COLOR",
		"SET wz SCENE",
		"SET wz_decke BLINK",
		"TOGGLE wz_decke ON",
		"MOOD wz",
		"MOOD wz zwei",
		"SCENE",
	} {
		if cmd, err := p.ParseText(text); err == nil {
			t.Errorf("ParseText(%q) = %+v, want error", text, cmd)
		}
	}
}
