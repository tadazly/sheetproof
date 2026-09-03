package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tadazly/sheetproof/internal/diagnostics"
)

func TestRecordDiagnosticsBoundsAndSanitizesFrontendData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sheetproof.log")
	controller := &Controller{diagnosticLog: diagnostics.New(path, 1024*1024)}
	controller.RecordDiagnostics([]FrontendDiagnosticEvent{{
		ID:        "session-1",
		Timestamp: "2026-09-03T01:02:03Z",
		Name:      "zoom.apply/unsafe",
		Fields: map[string]any{
			"from percent": 100,
			"note":         strings.Repeat("x", maxFrontendDiagnosticText+20),
			"nested":       map[string]any{"cell": "must-not-be-logged"},
		},
	}})

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("expected diagnostic event")
	}
	var item struct {
		Event  string         `json:"event"`
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.Event != "frontend.zoom.apply_unsafe" {
		t.Fatalf("event = %q", item.Event)
	}
	if item.Fields["from_percent"] != float64(100) {
		t.Fatalf("fields = %#v", item.Fields)
	}
	if item.Fields["nested"] != "unsupported:map[string]interface {}" {
		t.Fatalf("nested field leaked: %#v", item.Fields["nested"])
	}
	if got := item.Fields["note"].(string); len([]rune(got)) != maxFrontendDiagnosticText {
		t.Fatalf("note has %d runes", len([]rune(got)))
	}
}
