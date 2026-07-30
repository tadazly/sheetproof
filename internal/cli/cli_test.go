package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ug-tools/ugxlsx/internal/app"
	"github.com/ug-tools/ugxlsx/internal/testutil"
)

func TestDiffJSONWithUnicodeAndSpaces(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"diff", "--left", pair.Left, "--right", pair.Right, "--format", "json"}, &stdout, &stderr, noGUI)
	if code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	var result struct {
		Equal           bool `json:"equal"`
		DifferenceCount int  `json:"differenceCount"`
		Sheets          []struct {
			Name string `json:"name"`
		} `json:"sheets"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if result.Equal || result.DifferenceCount < 1 || len(result.Sheets) < 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestDiffErrorsAndCompareArguments(t *testing.T) {
	dir := t.TempDir()
	pair, err := testutil.CreatePair(dir)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
		code int
	}{
		{"missing args", []string{"diff"}, ExitRuntime},
		{"bad format", []string{"diff", "--left", pair.Left, "--right", pair.Right, "--format", "xml"}, ExitRuntime},
		{"same path", []string{"diff", "--left", pair.Left, "--right", pair.Left}, ExitRuntime},
		{"unsupported", []string{"diff", "--left", filepath.Join(dir, "a.xls"), "--right", pair.Right}, ExitUnsupported},
		{"missing file", []string{"diff", "--left", filepath.Join(dir, "missing.xlsx"), "--right", pair.Right}, ExitRead},
		{"half compare", []string{"compare", "--left", pair.Left}, ExitRuntime},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := Run(test.args, &stdout, &stderr, noGUI); got != test.code {
				t.Fatalf("exit = %d, want %d; stderr=%s", got, test.code, stderr.String())
			}
		})
	}
}

func noGUI(_, _ string, _ app.Options) error { return nil }
