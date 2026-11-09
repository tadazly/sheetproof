package ugit

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
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

func TestConfigureRepairsRepositoryOverridesAndPreservesOtherTools(t *testing.T) {
	for _, scenario := range []string{"none", "xlsx", "wildcard", "both"} {
		t.Run(scenario, func(t *testing.T) {
			c, executable := isolatedClient(t)
			root := t.TempDir()
			runConfig(t, c, "-C", root, "init", "--quiet")
			c.repositoryRoot = root
			for _, entry := range desiredEntries(executable) {
				if err := c.add(context.Background(), entry); err != nil {
					t.Fatal(err)
				}
			}
			addLocal := func(key, value string) {
				t.Helper()
				runConfig(t, c, "-C", root, "config", "--local", "--add", key, value)
			}
			csvKey := "difftool.*.csv_Custom.cmd"
			addLocal(csvKey, "'/tools/csv' args")
			if scenario == "xlsx" || scenario == "both" {
				for _, tool := range []string{"difftool", "mergetool"} {
					addLocal(tool+".*.xlsx_BeyondCompare.cmd", "'/old/bcomp' args")
					addLocal(tool+".*.xlsx_BeyondCompare.cmd", "'/old/bcomp2' args")
					addLocal(tool+".*.xlsx_BeyondCompare.scope", "tables/**")
				}
				addLocal("mergetool.*.xlsx_BeyondCompare.trustexitcode", "true")
			}
			if scenario == "wildcard" || scenario == "both" {
				for _, tool := range []string{"difftool", "mergetool"} {
					addLocal(tool+".*.*_BeyondCompare.cmd", "'/tools/bcomp' args")
					addLocal(tool+".*.*_BeyondCompare.scope", "tables/**")
				}
			}
			before, err := c.inspect(context.Background(), executable)
			if err != nil {
				t.Fatal(err)
			}
			if before.Configured != (scenario == "none") || before.LocalOverride != (scenario != "none") {
				t.Fatalf("unexpected inspection: %+v", before)
			}
			after, err := c.configure(context.Background(), executable)
			if err != nil || !after.Configured {
				t.Fatalf("configure = %+v, %v", after, err)
			}
			local, err := c.listEntries(context.Background(), "local", `^(difftool|mergetool)\.`)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range local {
				if strings.Contains(entry.Key, ".xlsx_BeyondCompare.") {
					t.Fatalf("old XLSX rule remains: %+v", entry)
				}
			}
			assertEntry := func(key, value string) {
				t.Helper()
				if !slices.ContainsFunc(local, func(entry Entry) bool { return entry.Key == key && entry.Value == value }) {
					t.Fatalf("missing %s = %s in %+v", key, value, local)
				}
			}
			assertEntry(csvKey, "'/tools/csv' args")
			if scenario == "wildcard" || scenario == "both" {
				for _, tool := range []string{"difftool", "mergetool"} {
					assertEntry(tool+".*.*_BeyondCompare.cmd", "'/tools/bcomp' args")
					assertEntry(tool+".*.*_BeyondCompare.scope", "tables/**")
				}
			}
			if scenario != "none" {
				for _, entry := range desiredEntries(executable) {
					assertEntry(entry.Key, entry.Value)
				}
			}
			// A second click must perform no mutations, including wildcard-only migration.
			base := c
			c.execute = func(ctx context.Context, args ...string) ([]byte, int, error) {
				if slices.Contains(args, "--add") || slices.Contains(args, "--unset-all") {
					t.Fatalf("idempotent configure wrote %v", args)
				}
				return base.run(ctx, args...)
			}
			if _, err := c.configure(context.Background(), executable); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConfigureRepairsDirectDiffOrder(t *testing.T) {
	c, executable := isolatedClient(t)
	entries := desiredEntries(executable)
	entries[0], entries[1] = entries[1], entries[0]
	for _, entry := range entries {
		if err := c.add(context.Background(), entry); err != nil {
			t.Fatal(err)
		}
	}
	before, err := c.inspect(context.Background(), executable)
	if err != nil || before.Configured {
		t.Fatalf("wrong tool order accepted: %+v, %v", before, err)
	}
	after, err := c.configure(context.Background(), executable)
	if err != nil || !after.Configured || after.Existing[0].Key != diffCommandKey {
		t.Fatalf("order not repaired: %+v, %v", after, err)
	}
}

func TestConfigureRestoresBothScopesAfterPartialWrite(t *testing.T) {
	c, executable := isolatedClient(t)
	c.repositoryRoot = t.TempDir()
	runConfig(t, c, "-C", c.repositoryRoot, "init", "--quiet")
	for _, scope := range []string{"global", "local"} {
		for _, entry := range []Entry{
			{Key: "difftool.*.xlsx_BeyondCompare.cmd", Value: "'/old/bcomp' args", Scope: scope},
			{Key: "difftool.*.xlsx_BeyondCompare.cmd", Value: "'/old/bcomp2' args", Scope: scope},
			{Key: "difftool.*.xlsx_BeyondCompare.scope", Value: "tables/**", Scope: scope},
			{Key: "difftool.*.*_BeyondCompare.cmd", Value: "'/tools/bcomp' args", Scope: scope},
		} {
			if err := c.add(context.Background(), entry); err != nil {
				t.Fatal(err)
			}
		}
	}
	before, err := c.inspect(context.Background(), executable)
	if err != nil {
		t.Fatal(err)
	}
	base, failed := c, false
	c.execute = func(ctx context.Context, args ...string) ([]byte, int, error) {
		if !failed && slices.Contains(args, "-C") && slices.Contains(args, "--add") && slices.Contains(args, mergeCommandKey) {
			failed = true
			return nil, 1, errors.New("simulated write failure")
		}
		return base.run(ctx, args...)
	}
	if _, err := c.configure(context.Background(), executable); err == nil || !failed {
		t.Fatalf("expected partial write failure, got %v", err)
	}
	after, err := c.inspect(context.Background(), executable)
	// XLSX order is compared through Existing. Wildcard vs XLSX placement does
	// not affect UGit's suffix priority, so compare the private snapshot by key.
	slices.SortStableFunc(before.localEntries, func(a, b Entry) int { return strings.Compare(a.Key, b.Key) })
	slices.SortStableFunc(after.localEntries, func(a, b Entry) int { return strings.Compare(a.Key, b.Key) })
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("rollback mismatch: before=%+v after=%+v err=%v", before, after, err)
	}
}

func TestConfigureRestoresDeletionWhenCancellationArrivesAfterGitWrites(t *testing.T) {
	for _, targetScope := range []string{"global", "local"} {
		t.Run(targetScope, func(t *testing.T) {
			c, executable := isolatedClient(t)
			c.repositoryRoot = t.TempDir()
			runConfig(t, c, "-C", c.repositoryRoot, "init", "--quiet")
			legacyKey := "difftool.*.xlsx_BeyondCompare.cmd"
			for _, scope := range []string{"global", "local"} {
				for _, value := range []string{"'/old/first' args", "'/old/second' args"} {
					if err := c.add(context.Background(), Entry{Key: legacyKey, Value: value, Scope: scope}); err != nil {
						t.Fatal(err)
					}
				}
			}
			before, err := c.inspect(context.Background(), executable)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			base, cancelled := c, false
			c.execute = func(ctx context.Context, args ...string) ([]byte, int, error) {
				output, code, err := base.run(ctx, args...)
				local := slices.Contains(args, "-C")
				if !cancelled && err == nil && local == (targetScope == "local") &&
					slices.Contains(args, "--unset-all") && slices.Contains(args, legacyKey) {
					cancelled = true
					cancel()
					return output, -1, context.Canceled
				}
				return output, code, err
			}
			if _, err := c.configure(ctx, executable); !errors.Is(err, context.Canceled) || !cancelled {
				t.Fatalf("expected cancellation after a successful deletion, got %v", err)
			}
			after, err := base.inspect(context.Background(), executable)
			if err != nil || !reflect.DeepEqual(before.Existing, after.Existing) {
				t.Fatalf("cancelled deletion was not restored: before=%+v after=%+v err=%v", before.Existing, after.Existing, err)
			}
		})
	}
}

func TestRestoreDoesNotDuplicateValuesWhenCleanupFails(t *testing.T) {
	c, _ := isolatedClient(t)
	entry := Entry{Key: "difftool.*.xlsx_BeyondCompare.cmd", Value: "'/old/tool' args", Scope: "global"}
	if err := c.add(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	base := c
	c.execute = func(ctx context.Context, args ...string) ([]byte, int, error) {
		if slices.Contains(args, "--unset-all") {
			return nil, -1, errors.New("simulated cleanup failure")
		}
		return base.run(ctx, args...)
	}
	if err := c.restore(context.Background(), []Entry{entry}, []Entry{entry}); err == nil {
		t.Fatal("expected rollback cleanup error")
	}
	entries, err := base.listEntries(context.Background(), "global", managedKeyExpression)
	if err != nil || len(entries) != 1 || entries[0].Value != entry.Value {
		t.Fatalf("failed rollback duplicated original entries: %+v, %v", entries, err)
	}
}

func isolatedClient(t *testing.T) (client, string) {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	return client{gitPath: gitPath}, createExecutable(t, "SheetProof")
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
