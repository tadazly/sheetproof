package ugit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureMigratesLegacyUGXLSXExecutableAndOnlyTouchesXLSXTools(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	globalConfig := filepath.Join(t.TempDir(), ".gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	c := client{gitPath: gitPath}
	ctx := context.Background()

	runConfig(t, c, "config", "--global", "--add", "difftool.*.csv_Custom.cmd", "'/tools/csv' \"$LOCAL\" \"$REMOTE\"")
	runConfig(t, c, "config", "--global", "--add", "difftool.*.xlsx_BeyondCompare.cmd", "'/old/bcomp' \"$LOCAL\" \"$REMOTE\"")
	runConfig(t, c, "config", "--global", "--add", "mergetool.*.xlsx_Custom.cmd", "'/old/ugxlsx' compare --left \"$LOCAL\" --right \"$REMOTE\"")

	legacy, err := c.inspect(ctx, createExecutable(t, "current location/SheetProof"))
	if err != nil {
		t.Fatal(err)
	}
	if !legacy.NeedsUpdate || legacy.Configured || !contains(legacy.ExistingPaths, "/old/ugxlsx") {
		t.Fatalf("legacy configuration was not detected: %+v", legacy)
	}

	firstExecutable := createExecutable(t, "first location/SheetProof")
	first, err := c.configure(ctx, firstExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Configured {
		t.Fatal("first configuration was not verified")
	}
	if first.GitPath != gitPath {
		t.Fatalf("Git path = %q, want %q", first.GitPath, gitPath)
	}
	if len(first.ConfigOrigins) != 1 || !strings.Contains(
		filepath.ToSlash(first.ConfigOrigins[0]), filepath.ToSlash(globalConfig),
	) {
		t.Fatalf("config origins = %v, want %s", first.ConfigOrigins, globalConfig)
	}
	assertConfigValue(t, c, "difftool.*.csv_Custom.cmd", "'/tools/csv' \"$LOCAL\" \"$REMOTE\"")
	assertConfigMissing(t, c, "difftool.*.xlsx_BeyondCompare.cmd")

	secondExecutable := createExecutable(t, "moved location/SheetProof")
	beforeMove, err := c.inspect(ctx, secondExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if !beforeMove.NeedsUpdate || beforeMove.Configured {
		t.Fatalf("moved status = %+v, want NeedsUpdate", beforeMove)
	}
	afterMove, err := c.configure(ctx, secondExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if !afterMove.Configured {
		t.Fatal("moved configuration was not verified")
	}
	if !contains(afterMove.ExistingPaths, secondExecutable) {
		t.Fatalf("existing paths = %v, want unquoted executable path %q", afterMove.ExistingPaths, secondExecutable)
	}
	for _, entry := range afterMove.Existing {
		if strings.Contains(entry.Value, firstExecutable) {
			t.Fatalf("old executable path remains in %s", entry.Key)
		}
	}
	assertConfigValue(
		t,
		c,
		diffCommandKey,
		"'\""+secondExecutable+"\"'"+` compare --left "$LOCAL" --right "$REMOTE"`,
	)
	assertConfigValue(
		t,
		c,
		diffFallbackKey,
		"'"+secondExecutable+`' compare --left "$LOCAL" --right "$REMOTE"`,
	)
	assertConfigValue(
		t,
		c,
		mergeCommandKey,
		"'"+secondExecutable+`' compare --left "$LOCAL" --right "$REMOTE" --base "$BASE" --output "$MERGED"`,
	)
	assertConfigValue(t, c, mergeTrustExitKey, "false")
}

func TestInspectRecognizesExactConfiguration(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), ".gitconfig"))
	c := client{gitPath: gitPath}
	executable := createExecutable(t, "SheetProof")

	empty, err := c.inspect(context.Background(), executable)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Configured || empty.NeedsUpdate {
		t.Fatalf("empty status = %+v", empty)
	}

	for _, entry := range desiredEntries(executable) {
		runConfig(t, c, "config", "--global", "--add", entry.Key, entry.Value)
	}
	configured, err := c.inspect(context.Background(), executable)
	if err != nil {
		t.Fatal(err)
	}
	if !configured.Configured || configured.NeedsUpdate {
		t.Fatalf("configured status = %+v", configured)
	}
}

func TestGitExecutableCandidatesCoverMacAndWindowsUGitBundles(t *testing.T) {
	mac := gitExecutableCandidates("darwin", func(string) string { return "" })
	if !contains(mac, "/Applications/UGit.app/Contents/Resources/app/git/bin/git") {
		t.Fatalf("mac candidates = %v", mac)
	}
	windows := gitExecutableCandidates("windows", func(key string) string {
		switch key {
		case "LOCALAPPDATA":
			return `C:\Users\tester\AppData\Local`
		case "ProgramFiles":
			return `C:\Program Files`
		case "ProgramFiles(x86)":
			return `C:\Program Files (x86)`
		default:
			return ""
		}
	})
	if !contains(windows, `C:\Users\tester\AppData\Local\Programs\UGit\resources\app\git\cmd\git.exe`) {
		t.Fatalf("windows candidates = %v", windows)
	}
	if !contains(windows, `C:\Program Files\UGit\resources\app\git\mingw64\bin\git.exe`) {
		t.Fatalf("windows bundled Git candidate missing: %v", windows)
	}
	if !contains(windows, `C:\Program Files\Git\cmd\git.exe`) {
		t.Fatalf("windows Git candidate missing: %v", windows)
	}
}

func createExecutable(t *testing.T, relative string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func runConfig(t *testing.T, c client, args ...string) {
	t.Helper()
	_, _, err := c.run(context.Background(), args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func assertConfigValue(t *testing.T, c client, key, want string) {
	t.Helper()
	output, _, err := c.run(context.Background(), "config", "--global", "--get", key)
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	if got := strings.TrimSuffix(string(output), "\n"); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}

func assertConfigMissing(t *testing.T, c client, key string) {
	t.Helper()
	_, exitCode, err := c.run(context.Background(), "config", "--global", "--get", key)
	if exitCode != 1 {
		t.Fatalf("%s still exists: exit=%d err=%v", key, exitCode, err)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
