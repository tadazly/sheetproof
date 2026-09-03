package diagnostics

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerWritesStructuredEventsAndRotates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "sheetproof.log")
	log := New(path, 360)
	log.Record("zoom.apply", map[string]any{"from": 100, "to": 110, "origin": "wheel"})
	log.Error(strings.Repeat("x", 240))
	log.Record("zoom.settled", map[string]any{"to": 110})

	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated log: %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("expected current log entry")
	}
	var item entry
	if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
		t.Fatalf("decode JSONL: %v", err)
	}
	if item.Event != "zoom.settled" || item.Fields["to"] != float64(110) {
		t.Fatalf("unexpected event: %#v", item)
	}
}

func TestNormalizeEventNameAndMessageBounds(t *testing.T) {
	if got := normalizeEventName("zoom apply/unsafe"); got != "zoom_apply_unsafe" {
		t.Fatalf("normalize event = %q", got)
	}
	if got := truncate(strings.Repeat("界", maxMessageRunes+10), maxMessageRunes); len([]rune(got)) != maxMessageRunes {
		t.Fatalf("truncated message has %d runes", len([]rune(got)))
	}
}

func TestClearRemovesCurrentAndRotatedLogs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sheetproof.log")
	log := New(path, 140)
	log.Record("first", map[string]any{"value": strings.Repeat("x", 100)})
	log.Record("second", map[string]any{"value": strings.Repeat("y", 100)})
	log.Clear()
	for _, candidate := range []string{path, path + ".1"} {
		if _, err := os.Stat(candidate); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, got %v", candidate, err)
		}
	}
}

func TestDiscardLoggerDoesNotWrite(t *testing.T) {
	log := NewDiscard()
	log.Record("process.start", map[string]any{"pid": 42})
	log.Error("ignored")
	log.Clear()
	if log.Path() != "" || log.Session() != "" {
		t.Fatalf("discard logger unexpectedly exposes path %q or session %q", log.Path(), log.Session())
	}
}
