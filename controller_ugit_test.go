package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	coreapp "github.com/tadazly/sheetproof/internal/app"
	"github.com/tadazly/sheetproof/internal/ugit"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestUGitBatchRequiresCurrentScanAndConfirmationWithoutOpeningRepository(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "repo")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	controllerGit(t, root, "init", "--quiet")
	controllerGit(t, root, "config", "--local", "difftool.*.xlsx_BeyondCompare.cmd", "'/old/tool' args")
	executable := filepath.Join(t.TempDir(), "SheetProof")
	if err := os.WriteFile(executable, []byte("test executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	scan, err := ugit.ScanRepositories(context.Background(), directory, executable)
	if err != nil || len(scan.Repositories) != 1 {
		t.Fatalf("scan = %+v, %v", scan, err)
	}
	c := NewController("", "", coreapp.Options{Locale: "zh-CN"})
	c.ctx = context.WithValue(context.Background(), "frontend", true)
	c.ugitScan, c.ugitScanID = &scan, 9
	confirmations := 0
	c.dialog = func(_ context.Context, options runtime.MessageDialogOptions) (string, error) {
		confirmations++
		if !strings.Contains(options.Message, "全局 XLSX") || !strings.Contains(options.Message, "其他文件类型") {
			t.Fatalf("incomplete confirmation: %s", options.Message)
		}
		return "取消", nil
	}
	for _, input := range []struct {
		id    uint64
		paths []string
	}{{8, []string{root}}, {9, nil}, {9, []string{directory}}, {9, []string{root, root}}} {
		if _, err := c.RepairUGitRepositories(input.id, input.paths); err == nil {
			t.Fatal("invalid batch accepted")
		}
	}
	if confirmations != 0 {
		t.Fatal("invalid selection reached confirmation")
	}
	configPath := filepath.Join(root, ".git", "config")
	before, _ := os.ReadFile(configPath)
	result, err := c.RepairUGitRepositories(9, []string{root})
	after, _ := os.ReadFile(configPath)
	if err != nil || !result.Cancelled || !slices.Equal(before, after) {
		t.Fatalf("cancel did not preserve configuration: %+v, %v", result, err)
	}
	if _, err := os.Stat(os.Getenv("GIT_CONFIG_GLOBAL")); !os.IsNotExist(err) {
		t.Fatal("cancel wrote global configuration")
	}
	c.dialog = func(_ context.Context, _ runtime.MessageDialogOptions) (string, error) {
		return "修复所选仓库", nil
	}
	result, err = c.RepairUGitRepositories(9, []string{root})
	if err != nil || len(result.Results) != 1 || result.Results[0].Status != "repaired" {
		t.Fatalf("repair = %+v, %v", result, err)
	}
	if c.repo != nil || c.session != nil || c.ugitScan != nil {
		t.Fatal("batch opened a repository/session or retained an actionable stale scan")
	}
	if _, err := c.RepairUGitRepositories(9, []string{root}); err == nil {
		t.Fatal("consumed scan accepted")
	}
}

func TestUGitOperationSerializationAndScanCancellation(t *testing.T) {
	c := NewController("", "", coreapp.Options{})
	ctx, _, finish, err := c.beginUGitOperation()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := c.beginUGitOperation(); err == nil {
		t.Fatal("overlapping UGit operations accepted")
	}
	c.mu.Lock()
	c.ugitScanCancel = c.ugitOperationCancel
	c.mu.Unlock()
	c.CancelUGitRepositoryScan()
	if ctx.Err() != context.Canceled {
		t.Fatal("scan cancellation did not reach its context")
	}
	finish()
	_, _, finish, err = c.beginUGitOperation()
	if err != nil {
		t.Fatal(err)
	}
	finish()
	c.shutdown(context.Background())
	if _, _, _, err := c.beginUGitOperation(); err == nil {
		t.Fatal("operation accepted after shutdown")
	}
}
