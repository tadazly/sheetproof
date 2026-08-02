package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tadazly/sheetproof/internal/app"
	"github.com/tadazly/sheetproof/internal/testutil"
	"github.com/tadazly/sheetproof/internal/workbook"
)

func TestHelpUsesCurrentSheetProofCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"help"}, &stdout, &stderr, noGUI); code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	usage := stdout.String()
	if !strings.Contains(usage, "sheetproof compare") || !strings.Contains(usage, "SheetProof") {
		t.Fatalf("help does not use current product command:\n%s", usage)
	}
	if strings.Contains(strings.ToLower(usage), "ugxlsx") {
		t.Fatalf("help still uses the legacy product name:\n%s", usage)
	}
}

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

func TestCompareAcceptsGitNullDeviceAndCleansPlaceholder(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, nullDevice := range []string{"/dev/null", "NUL", "NUL:"} {
		t.Run(nullDevice, func(t *testing.T) {
			var launchedLeft, launchedRight string
			launcher := func(left, right string, _ app.Options) error {
				launchedLeft, launchedRight = left, right
				if left != pair.Left {
					t.Fatalf("left = %q, want %q", left, pair.Left)
				}
				if right == nullDevice || filepath.Ext(right) != ".xlsx" {
					t.Fatalf("right was not adapted to an XLSX placeholder: %q", right)
				}
				if _, err := os.Stat(right); err != nil {
					t.Fatalf("placeholder unavailable while GUI is running: %v", err)
				}
				return nil
			}
			var stdout, stderr bytes.Buffer
			code := Run(
				[]string{"compare", "--left", pair.Left, "--right", nullDevice},
				&stdout,
				&stderr,
				launcher,
			)
			if code != ExitOK {
				t.Fatalf("exit = %d stderr=%s", code, stderr.String())
			}
			if launchedLeft == "" || launchedRight == "" {
				t.Fatal("launcher was not called")
			}
			if _, err := os.Stat(launchedRight); !os.IsNotExist(err) {
				t.Fatalf("placeholder was not cleaned after GUI exit: %v", err)
			}
		})
	}
}

func TestCompareAcceptsGitNullDeviceOnLeft(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var placeholder string
	launcher := func(left, right string, _ app.Options) error {
		placeholder = left
		if left == "/dev/null" || filepath.Ext(left) != ".xlsx" {
			t.Fatalf("left was not adapted to an XLSX placeholder: %q", left)
		}
		if right != pair.Right {
			t.Fatalf("right = %q, want %q", right, pair.Right)
		}
		file, snapshot, err := (workbook.Reader{}).Open(left)
		if err != nil {
			t.Fatalf("open placeholder: %v", err)
		}
		defer file.Close()
		if len(snapshot.Sheets) == 0 {
			t.Fatal("placeholder has no worksheets")
		}
		sourceFile, source, err := (workbook.Reader{}).Open(right)
		if err != nil {
			t.Fatalf("open source: %v", err)
		}
		defer sourceFile.Close()
		if len(snapshot.Sheets) != len(source.Sheets) {
			t.Fatalf("placeholder sheet count = %d, want %d", len(snapshot.Sheets), len(source.Sheets))
		}
		for index := range source.Sheets {
			if snapshot.Sheets[index].Name != source.Sheets[index].Name {
				t.Fatalf(
					"placeholder sheet %d = %q, want %q",
					index,
					snapshot.Sheets[index].Name,
					source.Sheets[index].Name,
				)
			}
		}
		return nil
	}
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"compare", "--left", "/dev/null", "--right", pair.Right},
		&stdout,
		&stderr,
		launcher,
	)
	if code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(placeholder); !os.IsNotExist(err) {
		t.Fatalf("placeholder was not cleaned after GUI exit: %v", err)
	}
}

func TestCompareRejectsTwoGitNullDevices(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"compare", "--left", "/dev/null", "--right", "NUL"},
		&stdout,
		&stderr,
		noGUI,
	)
	if code != ExitRuntime {
		t.Fatalf("exit = %d, want %d; stderr=%s", code, ExitRuntime, stderr.String())
	}
}

func TestCompareForcesGitDiffToolInvocationReadOnly(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIFF_PATH_COUNTER", "1")
	t.Setenv("GIT_DIFF_PATH_TOTAL", "1")
	var launched app.Options
	launcher := func(_, _ string, options app.Options) error {
		launched = options
		return nil
	}
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"compare", "--left", pair.Left, "--right", pair.Right},
		&stdout,
		&stderr,
		launcher,
	)
	if code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if !launched.GitDiff || !launched.ReadonlyLeft {
		t.Fatalf("Git difftool options = %+v, want GitDiff and ReadonlyLeft", launched)
	}
}

func TestUGitSpreadsheetCompareListAlwaysLaunchesReadOnly(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	listDir := t.TempDir()
	listPath := filepath.Join(listDir, "SpreadsheetCompare-123.txt")
	if err := os.WriteFile(listPath, []byte(pair.Left+"\r\n"+pair.Right+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var launchedLeft, launchedRight string
	var launched app.Options
	launcher := func(left, right string, options app.Options) error {
		launchedLeft, launchedRight, launched = left, right, options
		return nil
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{listPath}, &stdout, &stderr, launcher)
	if code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if launchedLeft != pair.Left || launchedRight != pair.Right {
		t.Fatalf("launched paths = %q / %q", launchedLeft, launchedRight)
	}
	if !launched.GitDiff || !launched.ReadonlyLeft || launched.UGitWorktree || launched.LeftLabel != "选中版本" || launched.RightLabel != "工作区" {
		t.Fatalf("UGit direct options = %+v", launched)
	}
}

func TestUGitSpreadsheetCompareMakesVerifiedWorktreeEditableOnLeft(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo 中文")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runCLIGit(t, root, "init", "-b", "main")
	runCLIGit(t, root, "config", "user.email", "test@example.com")
	runCLIGit(t, root, "config", "user.name", "SheetProof test")
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(root, "配置 目录", "reward.xlsx")
	if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFileForCLITest(t, pair.Left, worktree)
	runCLIGit(t, root, "add", "--", "配置 目录/reward.xlsx")
	runCLIGit(t, root, "commit", "-m", "base")

	diffDir := filepath.Join(root, ".git", "ugit", "diff")
	if err := os.MkdirAll(diffDir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(diffDir, "temp-HEAD-配置 目录-reward.xlsx")
	copyFileForCLITest(t, pair.Right, snapshot)
	listPath := filepath.Join(diffDir, "SpreadsheetCompare-123.txt")
	if err := os.WriteFile(listPath, []byte(snapshot+"\r\n"+worktree+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var launchedLeft, launchedRight string
	var launched app.Options
	launcher := func(left, right string, options app.Options) error {
		launchedLeft, launchedRight, launched = left, right, options
		return nil
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{listPath}, &stdout, &stderr, launcher); code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if launchedLeft != worktree || launchedRight != snapshot {
		t.Fatalf("launched paths = %q / %q, want worktree / snapshot", launchedLeft, launchedRight)
	}
	if launched.ReadonlyLeft || launched.GitDiff || !launched.UGitWorktree {
		t.Fatalf("UGit worktree options = %+v, want editable UGitWorktree session", launched)
	}
	if launched.LeftLabel != "当前工作区" || launched.RightLabel != "HEAD" {
		t.Fatalf("UGit worktree labels = %q / %q", launched.LeftLabel, launched.RightLabel)
	}
}

func TestUGitSpreadsheetCompareKeepsTwoSnapshotsReadOnlyInVerifiedDiffDirectory(t *testing.T) {
	root := t.TempDir()
	runCLIGit(t, root, "init", "-b", "main")
	diffDir := filepath.Join(root, ".git", "ugit", "diff")
	if err := os.MkdirAll(diffDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pair, err := testutil.CreatePair(diffDir)
	if err != nil {
		t.Fatal(err)
	}
	listPath := filepath.Join(diffDir, "SpreadsheetCompare-456.txt")
	if err := os.WriteFile(listPath, []byte(pair.Left+"\n"+pair.Right+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var launched app.Options
	launcher := func(_, _ string, options app.Options) error {
		launched = options
		return nil
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{listPath}, &stdout, &stderr, launcher); code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if !launched.GitDiff || !launched.ReadonlyLeft || launched.UGitWorktree {
		t.Fatalf("two-snapshot options = %+v, want read-only Git diff", launched)
	}
}

func TestUGitSpreadsheetCompareRequiresMatchingRepositoryGitDirectory(t *testing.T) {
	root := t.TempDir()
	runCLIGit(t, root, "init", "-b", "main")
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(root, "reward.xlsx")
	copyFileForCLITest(t, pair.Left, worktree)
	foreignDiffDir := filepath.Join(t.TempDir(), "ugit", "diff")
	if err := os.MkdirAll(foreignDiffDir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(foreignDiffDir, "temp-HEAD-reward.xlsx")
	copyFileForCLITest(t, pair.Right, snapshot)
	listPath := filepath.Join(foreignDiffDir, "SpreadsheetCompare-789.txt")
	if err := os.WriteFile(listPath, []byte(snapshot+"\n"+worktree+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var launchedLeft, launchedRight string
	var launched app.Options
	launcher := func(left, right string, options app.Options) error {
		launchedLeft, launchedRight, launched = left, right, options
		return nil
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{listPath}, &stdout, &stderr, launcher); code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if launchedLeft != snapshot || launchedRight != worktree {
		t.Fatalf("unverified paths were reordered: %q / %q", launchedLeft, launchedRight)
	}
	if !launched.GitDiff || !launched.ReadonlyLeft || launched.UGitWorktree {
		t.Fatalf("mismatched Git directory options = %+v, want read-only fallback", launched)
	}
}

func TestUGitSpreadsheetCompareRecognizesLinkedWorktreeGitDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "main")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runCLIGit(t, root, "init", "-b", "main")
	runCLIGit(t, root, "config", "user.email", "test@example.com")
	runCLIGit(t, root, "config", "user.name", "SheetProof test")
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mainFile := filepath.Join(root, "reward.xlsx")
	copyFileForCLITest(t, pair.Left, mainFile)
	runCLIGit(t, root, "add", "reward.xlsx")
	runCLIGit(t, root, "commit", "-m", "base")

	linked := filepath.Join(filepath.Dir(root), "linked 中文")
	runCLIGit(t, root, "worktree", "add", "-b", "linked-test", linked)
	gitDirectory := runCLIGitOutput(t, linked, "rev-parse", "--absolute-git-dir")
	diffDir := filepath.Join(gitDirectory, "ugit", "diff")
	if err := os.MkdirAll(diffDir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(diffDir, "temp-HEAD-reward.xlsx")
	copyFileForCLITest(t, pair.Right, snapshot)
	worktree := filepath.Join(linked, "reward.xlsx")
	listPath := filepath.Join(diffDir, "SpreadsheetCompare-linked.txt")
	if err := os.WriteFile(listPath, []byte(snapshot+"\n"+worktree+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var launchedLeft, launchedRight string
	var launched app.Options
	launcher := func(left, right string, options app.Options) error {
		launchedLeft, launchedRight, launched = left, right, options
		return nil
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{listPath}, &stdout, &stderr, launcher); code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if launchedLeft != worktree || launchedRight != snapshot || !launched.UGitWorktree || launched.ReadonlyLeft {
		t.Fatalf("linked worktree launch = %q / %q %+v", launchedLeft, launchedRight, launched)
	}
}

func TestComparePassesMergeBaseMetadata(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(t.TempDir(), "base.xlsx")
	data, err := os.ReadFile(pair.Right)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base, data, 0o600); err != nil {
		t.Fatal(err)
	}
	var launched app.Options
	launcher := func(_, _ string, options app.Options) error {
		launched = options
		return nil
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"compare", "--left", pair.Left, "--right", pair.Right,
		"--base", base, "--output", filepath.Join(t.TempDir(), "merged.xlsx"),
	}, &stdout, &stderr, launcher)
	if code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if !launched.GitMerge || launched.MergeBase != base {
		t.Fatalf("merge options = %+v", launched)
	}
}

func TestCompareUsesUGitEnvironmentLabelsWhenFlagsAreAbsent(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCAL_TITLE", "正式服-外网")
	t.Setenv("REMOTE_TITLE", "小屋分支")
	var launched app.Options
	launcher := func(_, _ string, options app.Options) error {
		launched = options
		return nil
	}
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"compare", "--left", pair.Left, "--right", pair.Right},
		&stdout,
		&stderr,
		launcher,
	)
	if code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if launched.LeftLabel != "正式服-外网" || launched.RightLabel != "小屋分支" {
		t.Fatalf("UGit labels = %q / %q", launched.LeftLabel, launched.RightLabel)
	}
}

func TestCompareExplicitLabelsOverrideUGitEnvironment(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCAL_TITLE", "UGit 左侧")
	t.Setenv("REMOTE_TITLE", "UGit 右侧")
	var launched app.Options
	launcher := func(_, _ string, options app.Options) error {
		launched = options
		return nil
	}
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{
			"compare",
			"--left", pair.Left,
			"--right", pair.Right,
			"--left-label", "自定义左侧",
			"--right-label", "自定义右侧",
		},
		&stdout,
		&stderr,
		launcher,
	)
	if code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if launched.LeftLabel != "自定义左侧" || launched.RightLabel != "自定义右侧" {
		t.Fatalf("explicit labels = %q / %q", launched.LeftLabel, launched.RightLabel)
	}
}

func TestCompareDoesNotConfusePartialGitEnvironmentWithDiffTool(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIFF_PATH_COUNTER", "1")
	t.Setenv("GIT_DIFF_PATH_TOTAL", "")
	var launched app.Options
	launcher := func(_, _ string, options app.Options) error {
		launched = options
		return nil
	}
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"compare", "--left", pair.Left, "--right", pair.Right},
		&stdout,
		&stderr,
		launcher,
	)
	if code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if launched.GitDiff || launched.ReadonlyLeft {
		t.Fatalf("ordinary compare options = %+v, want editable non-Git comparison", launched)
	}
}

func TestRepositoryCommandValidatesAndPassesExactFileAndRef(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo 中文")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runCLIGit(t, root, "init", "-b", "main")
	runCLIGit(t, root, "config", "user.email", "test@example.com")
	runCLIGit(t, root, "config", "user.name", "SheetProof test")
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

func runCLIGitOutput(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(bytes.TrimSpace(output))
}

func copyFileForCLITest(t *testing.T, source, target string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func noGUI(_, _ string, _ app.Options) error { return nil }
