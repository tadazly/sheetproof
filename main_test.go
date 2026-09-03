package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsWebviewOptionsReserveZoomForTheWorksheet(t *testing.T) {
	settings := windowsWebviewOptions()
	if settings.IsZoomControlEnabled {
		t.Fatal("WebView page zoom must remain disabled")
	}
	if !settings.DisablePinchZoom {
		t.Fatal("WebView pinch zoom must remain disabled")
	}
}

func TestExecutableDiagnosticFieldsIdentifyTheExactBuildWithoutItsDirectory(t *testing.T) {
	fields := executableDiagnosticFields()
	name, _ := fields["executable"].(string)
	if name == "" || name != filepath.Base(name) {
		t.Fatalf("executable = %q", name)
	}
	hash, _ := fields["executableSha256"].(string)
	if len(hash) != 64 || strings.ContainsAny(hash, "\\/") {
		t.Fatalf("executableSha256 = %q", hash)
	}
}

func TestWailsBindingHelperIsExcludedFromRuntimeDiagnostics(t *testing.T) {
	for _, name := range []string{"wailsbindings", "wailsbindings.exe", "WAILSBINDINGS.EXE"} {
		if !isWailsBindingsExecutable(name) {
			t.Fatalf("expected %q to be recognized as the binding helper", name)
		}
	}
	for _, name := range []string{"SheetProof.exe", "SheetProof-Diagnostic.exe", "wailsbindings-old.exe"} {
		if isWailsBindingsExecutable(name) {
			t.Fatalf("did not expect %q to be recognized as the binding helper", name)
		}
	}
}
