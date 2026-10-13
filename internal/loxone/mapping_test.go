package loxone

import (
	"sort"
	"testing"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
)

func loxoneIDs(mappings []*models.Mapping) []string {
	ids := make([]string, 0, len(mappings))
	for _, m := range mappings {
		ids = append(ids, m.LoxoneID)
	}
	sort.Strings(ids)
	return ids
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestGetAllByHueIDReturnsEveryMapping covers the case the previous 1:1 index
// silently dropped: one HUE resource mapped to several Loxone IDs. Only the
// last one loaded used to receive feedback.
func TestGetAllByHueIDReturnsEveryMapping(t *testing.T) {
	m := NewMappingManager()
	m.Load([]models.Mapping{
		{ID: "1", LoxoneID: "wohnzimmer", HueID: "hue-group-1", HueType: "group", Enabled: true},
		{ID: "2", LoxoneID: "wohnzimmer_alt", HueID: "hue-group-1", HueType: "group", Enabled: true},
		{ID: "3", LoxoneID: "kueche", HueID: "hue-group-2", HueType: "group", Enabled: true},
	})

	got := loxoneIDs(m.GetAllByHueID("hue-group-1"))
	if want := []string{"wohnzimmer", "wohnzimmer_alt"}; !equal(got, want) {
		t.Errorf("GetAllByHueID = %v, want %v", got, want)
	}

	if got := loxoneIDs(m.GetAllByHueID("hue-group-2")); !equal(got, []string{"kueche"}) {
		t.Errorf("second group = %v", got)
	}

	if got := m.GetAllByHueID("unknown"); got != nil {
		t.Errorf("unknown HUE ID = %v, want nil", got)
	}
}

// TestRemoveKeepsSiblingMappings makes sure deleting one mapping does not take
// the other mappings of the same HUE resource with it.
func TestRemoveKeepsSiblingMappings(t *testing.T) {
	m := NewMappingManager()
	m.Load([]models.Mapping{
		{ID: "1", LoxoneID: "wohnzimmer", HueID: "hue-group-1", HueType: "group", Enabled: true},
		{ID: "2", LoxoneID: "wohnzimmer_alt", HueID: "hue-group-1", HueType: "group", Enabled: true},
	})

	m.Remove("1")

	got := loxoneIDs(m.GetAllByHueID("hue-group-1"))
	if want := []string{"wohnzimmer_alt"}; !equal(got, want) {
		t.Errorf("after Remove = %v, want %v", got, want)
	}
	if m.GetByLoxoneID("wohnzimmer") != nil {
		t.Error("removed mapping still resolvable by Loxone ID")
	}

	// Removing the last one clears the entry entirely.
	m.Remove("2")
	if got := m.GetAllByHueID("hue-group-1"); got != nil {
		t.Errorf("after removing both = %v, want nil", got)
	}
}

func TestDisabledMappingsAreNotIndexed(t *testing.T) {
	m := NewMappingManager()
	m.Load([]models.Mapping{
		{ID: "1", LoxoneID: "aktiv", HueID: "hue-1", HueType: "light", Enabled: true},
		{ID: "2", LoxoneID: "inaktiv", HueID: "hue-1", HueType: "light", Enabled: false},
	})

	if got := loxoneIDs(m.GetAllByHueID("hue-1")); !equal(got, []string{"aktiv"}) {
		t.Errorf("GetAllByHueID = %v, want only the enabled mapping", got)
	}
}

// TestGetAllByHueIDReturnsCopy guards against a caller mutating the index.
func TestGetAllByHueIDReturnsCopy(t *testing.T) {
	m := NewMappingManager()
	m.Load([]models.Mapping{
		{ID: "1", LoxoneID: "a", HueID: "hue-1", HueType: "light", Enabled: true},
		{ID: "2", LoxoneID: "b", HueID: "hue-1", HueType: "light", Enabled: true},
	})

	got := m.GetAllByHueID("hue-1")
	got[0] = nil

	if again := m.GetAllByHueID("hue-1"); again[0] == nil || len(again) != 2 {
		t.Error("caller mutated the manager's own slice")
	}
}

func TestResolveMoodAndTarget(t *testing.T) {
	m := NewMappingManager()
	m.Load([]models.Mapping{
		{ID: "1", LoxoneID: "buero", HueID: "grp-1", HueType: "group", Enabled: true},
		{ID: "2", LoxoneID: "buero_mood_1", HueID: "scene-1", HueType: "scene", Enabled: true},
		{ID: "3", LoxoneID: "buero_mood_12", HueID: "scene-12", HueType: "scene", Enabled: true},
	})

	if id, typ, ok := m.ResolveTarget("buero"); !ok || id != "grp-1" || typ != "group" {
		t.Errorf("ResolveTarget = (%q, %q, %v)", id, typ, ok)
	}

	// Mood 0 falls back to the base group so it can be switched off.
	if id, typ, ok := m.ResolveMood("buero", 0); !ok || id != "grp-1" || typ != "group" {
		t.Errorf("ResolveMood(0) = (%q, %q, %v)", id, typ, ok)
	}

	if id, _, ok := m.ResolveMood("buero", 1); !ok || id != "scene-1" {
		t.Errorf("ResolveMood(1) = (%q, %v)", id, ok)
	}

	// Two-digit moods must not be truncated by the itoa helper.
	if id, _, ok := m.ResolveMood("buero", 12); !ok || id != "scene-12" {
		t.Errorf("ResolveMood(12) = (%q, %v)", id, ok)
	}

	if _, _, ok := m.ResolveMood("buero", 99); ok {
		t.Error("ResolveMood(99) resolved although no mapping exists")
	}
}
