package ugit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestScanAndRepairUseSelectedGitWithoutPATH(t *testing.T) {
	for _, removeBeforeScan := range []bool{true, false} {
		name := "before-repair"
		if removeBeforeScan {
			name = "before-scan"
		}
		t.Run(name, func(t *testing.T) {
			c, executable := isolatedClient(t)
			directory := t.TempDir()
			root := scanRepo(t, c, directory, "legacy", Entry{Key: "difftool.*.xlsx_BeyondCompare.cmd", Value: "'/old/tool' args"})
			clearPATH := func() {
				t.Setenv("PATH", "")
				if _, err := exec.LookPath("git"); err == nil {
					t.Fatal("test requires Git to be absent from PATH")
				}
			}
			if removeBeforeScan {
				clearPATH()
			}
			scan, err := c.scanRepositories(context.Background(), directory, executable)
			if err != nil || scan.Inspected != 1 || len(scan.Repositories) != 1 || len(scan.Issues) != 0 {
				t.Fatalf("scan with selected Git = %+v, %v", scan, err)
			}
			if !removeBeforeScan {
				clearPATH()
			}
			results, err := scan.Repair(context.Background(), []string{root})
			if err != nil || len(results) != 1 || results[0].Status != "repaired" {
				t.Fatalf("repair with selected Git = %+v, %v", results, err)
			}
			c.repositoryRoot = root
			inspection, err := c.inspect(context.Background(), executable)
			if err != nil || !inspection.Configured {
				t.Fatalf("configuration was not repaired: %+v, %v", inspection, err)
			}
		})
	}
}

func TestScanSerializesUnknownToolPathsAsEmptyArray(t *testing.T) {
	c, executable := isolatedClient(t)
	directory := t.TempDir()
	scanRepo(t, c, directory, "unknown-command", Entry{Key: "difftool.*.xlsx_BeyondCompare.cmd", Value: "/old/bcompare"})
	scan, err := c.scanRepositories(context.Background(), directory, executable)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(scan)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Repositories []struct {
			Tools []string `json:"tools"`
		} `json:"repositories"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Repositories) != 1 || payload.Repositories[0].Tools == nil {
		t.Fatalf("tool list must be a JSON array: %s", data)
	}
}

func scanRepo(t *testing.T, c client, directory, name string, entries ...Entry) string {
	t.Helper()
	root := filepath.Join(directory, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runConfig(t, c, "-C", root, "init", "--quiet")
	c.repositoryRoot = root
	for _, entry := range entries {
		entry.Scope = "local"
		if err := c.add(context.Background(), entry); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func configBytes(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestScanIsReadOnlyAndRepairOnlyAffectsSelectedUnchangedRepositories(t *testing.T) {
	c, executable := isolatedClient(t)
	directory := t.TempDir()
	legacy := Entry{Key: "difftool.*.xlsx_BeyondCompare.cmd", Value: "'/old/tool' args"}
	old := scanRepo(t, c, directory, "old", legacy, Entry{Key: "difftool.*.xlsx_BeyondCompare.scope", Value: "tables/**"})
	wildcard := scanRepo(t, c, directory, "wildcard",
		Entry{Key: "difftool.*.*_BeyondCompare.cmd", Value: "'/old/wildcard' args"},
		Entry{Key: "difftool.*.*_BeyondCompare.scope", Value: "tables/**"})
	changed := scanRepo(t, c, directory, "changed", legacy)
	good := scanRepo(t, c, directory, "good", desiredEntries(executable)...)
	clean := scanRepo(t, c, directory, "clean", Entry{Key: "difftool.*.csv_Custom.cmd", Value: "'/csv/tool' args"})
	before := make(map[string][]byte)
	for _, root := range []string{old, wildcard, changed, good, clean} {
		before[root] = configBytes(t, root)
	}
	scan, err := c.scanRepositories(context.Background(), directory, executable)
	if err != nil || len(scan.Repositories) != 3 || scan.Inspected != 5 {
		t.Fatalf("scan = %+v, %v", scan, err)
	}
	for root, data := range before {
		if !slices.Equal(data, configBytes(t, root)) {
			t.Fatalf("scan changed %s", root)
		}
	}
	if _, err := os.Stat(os.Getenv("GIT_CONFIG_GLOBAL")); !os.IsNotExist(err) {
		t.Fatalf("scan wrote global configuration: %v", err)
	}
	for _, paths := range [][]string{nil, {good}, {old, old}, {filepath.Join(directory, "outside")}} {
		if _, err := scan.Repair(context.Background(), paths); err == nil {
			t.Fatalf("invalid selection accepted: %v", paths)
		}
	}
	c.repositoryRoot = changed
	if err := c.add(context.Background(), Entry{Key: legacy.Key, Value: "'/new/user-choice' args", Scope: "local"}); err != nil {
		t.Fatal(err)
	}
	changedBefore := configBytes(t, changed)
	results, err := scan.Repair(context.Background(), []string{old, changed})
	if err != nil || results[0].Status != "repaired" || results[1].Status != "changed" {
		t.Fatalf("repair = %+v, %v", results, err)
	}
	for _, root := range []string{wildcard, good, clean} {
		if !slices.Equal(before[root], configBytes(t, root)) {
			t.Fatalf("unselected repository changed: %s", root)
		}
	}
	if !slices.Equal(changedBefore, configBytes(t, changed)) {
		t.Fatal("concurrent user configuration overwritten")
	}
	again, err := c.scanRepositories(context.Background(), directory, executable)
	if err != nil || len(again.Repositories) != 2 {
		t.Fatalf("rescan = %+v, %v", again, err)
	}
	for _, item := range again.Repositories {
		if item.Path != wildcard && item.Path != changed {
			t.Fatalf("already configured repository listed: %+v", item)
		}
	}
}

func TestBatchRepairContinuesAfterOneWriteFails(t *testing.T) {
	c, executable := isolatedClient(t)
	directory := t.TempDir()
	legacy := Entry{Key: "difftool.*.xlsx_BeyondCompare.cmd", Value: "'/old/tool' args"}
	broken := scanRepo(t, c, directory, "broken", legacy)
	healthy := scanRepo(t, c, directory, "healthy", legacy)
	scan, err := c.scanRepositories(context.Background(), directory, executable)
	if err != nil {
		t.Fatal(err)
	}
	base, failed := c, false
	scan.client.execute = func(ctx context.Context, args ...string) ([]byte, int, error) {
		if !failed && slices.Contains(args, broken) && slices.Contains(args, "--add") && slices.Contains(args, diffFallbackKey) {
			failed = true
			return nil, 1, errors.New("simulated write failure")
		}
		return base.run(ctx, args...)
	}
	results, err := scan.Repair(context.Background(), []string{broken, healthy})
	if err != nil || !failed || results[0].Status != "failed" || results[1].Status != "repaired" {
		t.Fatalf("partial failure = %+v, %v", results, err)
	}
	c.repositoryRoot = broken
	old, err := c.listEntries(context.Background(), "local", managedKeyExpression)
	if err != nil || len(old) != 1 || old[0].Key != legacy.Key || old[0].Value != legacy.Value {
		t.Fatalf("failed repository not restored: %+v, %v", old, err)
	}
}
