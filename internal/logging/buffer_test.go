package logging

import (
	"testing"
	"time"
)

// TestLogBufferParsesZerologAndKeepsNewestFirst covers the ring buffer the
// settings page reads: zerolog JSON lines become entries with upper-case
// level and extra fields, the buffer wraps around at its size, and reads
// come back newest first.
func TestLogBufferParsesZerologAndKeepsNewestFirst(t *testing.T) {
	b := NewLogBuffer(3)

	lines := []string{
		`{"level":"info","time":"2026-10-13T20:00:01Z","message":"eins","miniserver":"Haus"}`,
		`{"level":"warn","time":"2026-10-13T20:00:02Z","message":"zwei"}`,
		`not json at all`,
		`{"level":"debug","time":"2026-10-13T20:00:03Z","message":"drei"}`,
		`{"level":"error","time":"2026-10-13T20:00:04.5Z","message":"vier","status":500}`,
	}
	for _, l := range lines {
		if n, err := b.Write([]byte(l)); err != nil || n != len(l) {
			t.Fatalf("Write(%q) = %d, %v", l, n, err)
		}
	}

	got := b.GetEntries("", "", 0)
	if len(got) != 3 {
		t.Fatalf("entries = %d, want 3 (buffer size)", len(got))
	}
	if got[0].Message != "vier" || got[1].Message != "drei" || got[2].Message != "zwei" {
		t.Errorf("order = %s, %s, %s; want vier, drei, zwei (newest first, 'eins' evicted)", got[0].Message, got[1].Message, got[2].Message)
	}
	if got[0].Level != "ERROR" || got[0].Source != "SYSTEM" {
		t.Errorf("entry = %+v, want level ERROR and source SYSTEM", got[0])
	}
	if got[0].Timestamp.Format(time.RFC3339Nano) != "2026-10-13T20:00:04.5Z" {
		t.Errorf("timestamp = %v, want the zerolog time", got[0].Timestamp)
	}
	if got[0].Fields["status"] != float64(500) {
		t.Errorf("fields = %v, want status=500 kept as extra field", got[0].Fields)
	}
	if _, ok := got[0].Fields["message"]; ok {
		t.Error("message must not be duplicated into fields")
	}
}

// TestLogBufferFilters covers the level, search and limit parameters of
// GET /api/logs. Search matches the message and string fields, case-insensitively.
func TestLogBufferFilters(t *testing.T) {
	b := NewLogBuffer(10)
	for _, l := range []string{
		`{"level":"info","message":"UDP feedback sent","miniserver":"Haus","msg":"wz/on:1"}`,
		`{"level":"info","message":"HTTP feedback sent","miniserver":"Garage"}`,
		`{"level":"warn","message":"Failed to send UDP feedback"}`,
		`{"level":"info","message":"Configuration loaded"}`,
	} {
		b.Write([]byte(l))
	}

	if got := b.GetEntries("WARN", "", 0); len(got) != 1 || got[0].Message != "Failed to send UDP feedback" {
		t.Errorf("level filter = %+v", got)
	}
	if got := b.GetEntries("warn", "", 0); len(got) != 1 {
		t.Errorf("level filter is not case-insensitive: %+v", got)
	}
	if got := b.GetEntries("", "feedback", 0); len(got) != 3 {
		t.Errorf("search feedback = %d entries, want 3", len(got))
	}
	if got := b.GetEntries("", "garage", 0); len(got) != 1 || got[0].Message != "HTTP feedback sent" {
		t.Errorf("search in string fields = %+v", got)
	}
	if got := b.GetEntries("info", "feedback", 1); len(got) != 1 || got[0].Message != "HTTP feedback sent" {
		t.Errorf("level+search+limit = %+v, want the newest HTTP entry only", got)
	}
	if got := b.GetEntries("", "", 2); len(got) != 2 {
		t.Errorf("limit = %d entries, want 2", len(got))
	}
	if got := b.GetEntries("", "nichts", 0); len(got) != 0 {
		t.Errorf("no match = %+v, want empty", got)
	}
}
