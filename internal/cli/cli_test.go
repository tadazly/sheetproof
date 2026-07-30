package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
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

func TestRepositoryCommandValidatesAndPassesExactFileAndRef(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo 中文")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runCLIGit(t, root, "init", "-b", "main")
	runCLIGit(t, root, "config", "user.email", "test@example.com")
	runCLIGit(t, root, "config", "user.name", "ugxlsx test")
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	relative := "配置 目录/reward.xlsx"
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(pair.Left)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatal(err)
	}
	runCLIGit(t, root, "add", relative)
	runCLIGit(t, root, "commit", "-m", "workbook")
	runCLIGit(t, root, "branch", "develop")

	var launched app.Options
	launcher := func(left, right string, options app.Options) error {
		if left != "" || right != "" {
			t.Fatalf("repo command passed direct files: %q %q", left, right)
		}
		launched = options
		return nil
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"repo", "--path", root, "--file", relative, "--ref", "develop"}, &stdout, &stderr, launcher)
	if code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if launched.RepositoryPath == "" || launched.RepositoryFile != relative || launched.RepositoryRef != "refs/heads/develop" {
		t.Fatalf("repository launch options = %+v", launched)
	}

	for _, test := range []struct {
		name string
		args []string
		code int
	}{
		{"missing path", []string{"repo"}, ExitRuntime},
		{"ordinary directory", []string{"repo", "--path", t.TempDir()}, ExitRuntime},
		{"invalid file", []string{"repo", "--path", root, "--file", "missing.xlsx"}, ExitRead},
		{"invalid ref", []string{"repo", "--path", root, "--ref", "does-not-exist"}, ExitRuntime},
	} {
		t.Run(test.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			if got := Run(test.args, &stdout, &stderr, noGUI); got != test.code {
				t.Fatalf("exit = %d, want %d; stderr=%s", got, test.code, stderr.String())
			}
		})
	}
}

func runCLIGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func noGUI(_, _ string, _ app.Options) error { return nil }
