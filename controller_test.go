package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	coreapp "github.com/ug-tools/ugxlsx/internal/app"
	"github.com/ug-tools/ugxlsx/internal/preferences"
	"github.com/ug-tools/ugxlsx/internal/repository"
	"github.com/ug-tools/ugxlsx/internal/testutil"
	"github.com/xuri/excelize/v2"
)

func TestControllerAsyncBootstrapAndViewportAPI(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	controller := NewController(pair.Left, pair.Right, coreapp.Options{})
	controller.startup(context.Background())
	deadline := time.Now().Add(5 * time.Second)
	for controller.Bootstrap().Loading && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	state := controller.Bootstrap()
	if state.Loading || !state.HasSession || state.Error != "" {
		t.Fatalf("bootstrap state = %+v", state)
	}
	summary, err := controller.Summary()
	if err != nil || summary.Diff.DifferenceCount != 7 {
		t.Fatalf("summary = %+v, err=%v", summary.Diff, err)
	}
	region, err := controller.Region("数据 表", 1, 10, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(region.Cells) != 100 {
		t.Fatalf("region cells = %d", len(region.Cells))
	}
	if _, err := controller.CopyRightToLeft("数据 表", 1, 1); err != nil {
		t.Fatal(err)
	}
	controller.shutdown(context.Background())
}

func TestControllerShutdownCancelsDifferenceIndexBeforeWaiting(t *testing.T) {
	controller := NewController("", "", coreapp.Options{})
	indexContext, cancel := context.WithCancel(context.Background())
	controller.differenceIndexCancel = cancel
	controller.wg.Add(1)
	indexStopped := make(chan struct{})
	go func() {
		defer controller.wg.Done()
		defer close(indexStopped)
		<-indexContext.Done()
	}()

	shutdownDone := make(chan struct{})
	go func() {
		controller.shutdown(context.Background())
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown waited for the difference index instead of cancelling it")
	}
	select {
	case <-indexStopped:
	default:
		t.Fatal("difference index worker did not observe shutdown cancellation")
	}
}

func TestControllerRepositoryWorkflowPreservesBranchAndSavesOnlyWorktreeFile(t *testing.T) {
	root, relative := createControllerRepository(t)
	preferencePath := filepath.Join(t.TempDir(), "preferences.json")
	controller := NewController("", "", coreapp.Options{})
	controller.prefs = preferences.NewStoreAt(preferencePath)
	result, err := controller.OpenRepository(filepath.Join(root, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Repository.Path == "" || result.Repository.SelectedFile != "" {
		t.Fatalf("open repository result = %+v", result.Repository)
	}
	if result.Repository.SelectedRef != "refs/remotes/origin/main" ||
		len(result.Repository.DifferenceFiles) != 0 {
		t.Fatalf("initial repository defaults = %+v", result.Repository)
	}
	result, err = controller.SelectRepositoryRef("refs/heads/develop")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Repository.DifferenceIndexing || len(result.Repository.DifferenceFiles) != 0 {
		t.Fatalf("uncached semantic index should start in background: %+v", result.Repository)
	}
	result = waitForDifferenceIndex(t, controller)
	if len(result.Repository.DifferenceFiles) != 1 ||
		result.Repository.DifferenceFiles[0] != relative {
		t.Fatalf("exact repository difference index = %+v", result.Repository.DifferenceFiles)
	}
	reopenedRepo, info, err := repository.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	develop, err := reopenedRepo.ResolveReference("refs/heads/develop", info.Branches)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := reopenedRepo.DifferenceIndexSignature(develop, info.Files)
	if err != nil {
		t.Fatal(err)
	}
	cached, exists := controller.prefs.RepositoryIndex(reopenedRepo.Root(), develop.FullName, signature)
	if !exists || strings.Join(cached, "|") != strings.Join(result.Repository.DifferenceFiles, "|") {
		t.Fatalf("persisted repository index = %#v, exists=%t", cached, exists)
	}
	cachedController := NewController("", "", coreapp.Options{})
	cachedController.prefs = preferences.NewStoreAt(preferencePath)
	cachedResult, err := cachedController.OpenRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if cachedResult.Repository.SelectedRef != develop.FullName ||
		cachedResult.Repository.DifferenceIndexing ||
		len(cachedResult.Repository.DifferenceFiles) != 1 ||
		cachedResult.Repository.DifferenceFiles[0] != relative {
		t.Fatalf("reopened cached repository index = %+v", cachedResult.Repository)
	}
	cachedController.shutdown(context.Background())
	result, err = controller.SelectRepositoryFile(relative)
	if err != nil {
		t.Fatal(err)
	}
	if result.Repository.SelectedRef != "refs/heads/develop" || result.Repository.RightState != "ready" {
		t.Fatalf("default comparison = %+v", result.Repository)
	}
	if result.Summary == nil || result.Summary.Diff.DifferenceCount == 0 {
		t.Fatalf("repository comparison summary = %+v", result.Summary)
	}
	tempRight := controller.tempRight
	if tempRight == "" || strings.HasPrefix(tempRight, root+string(filepath.Separator)) {
		t.Fatalf("right ref temp path = %q", tempRight)
	}
	beforeBranch := strings.TrimSpace(controllerGit(t, root, "branch", "--show-current"))

	if _, err := controller.EditLeft("数据 表", 4, 1, "工具内编辑", "text"); err != nil {
		t.Fatal(err)
	}
	result, err = controller.SelectRepositoryRef("refs/remotes/origin/develop")
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary == nil || !result.Summary.Dirty {
		t.Fatal("switching only the comparison ref discarded left edits")
	}
	result, err = controller.SelectRepositoryRef("refs/heads/missing")
	if err != nil {
		t.Fatal(err)
	}
	if result.Repository.RightState != "missing" || result.Repository.ComparisonActive {
		t.Fatalf("missing branch file state = %+v", result.Repository)
	}
	result = waitForDifferenceIndex(t, controller)
	if len(result.Repository.DifferenceFiles) != 0 {
		t.Fatalf("missing branch should not appear in common differences: %+v", result.Repository.DifferenceFiles)
	}
	if result.Summary == nil || !result.Summary.Dirty || result.Summary.Diff.RightFile != "" ||
		result.Summary.Diff.DifferenceCount != 0 {
		t.Fatalf("left session after missing right = %+v", result.Summary)
	}
	result, err = controller.SelectRepositoryRef("develop")
	if err != nil {
		t.Fatal(err)
	}
	if result.Repository.RightState != "ready" {
		t.Fatalf("reselected right state = %+v", result.Repository)
	}
	if _, err := controller.CopyRightToLeft("数据 表", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Save(); err != nil {
		t.Fatal(err)
	}
	if afterBranch := strings.TrimSpace(controllerGit(t, root, "branch", "--show-current")); afterBranch != beforeBranch {
		t.Fatalf("repository workflow switched branch from %q to %q", beforeBranch, afterBranch)
	}
	status := controllerGit(t, root, "status", "--porcelain=v1", "--", relative)
	if !strings.Contains(status, relative) {
		t.Fatalf("saved worktree file is not modified: %q", status)
	}
	if cached := controllerGit(t, root, "diff", "--cached", "--name-only"); strings.TrimSpace(cached) != "" {
		t.Fatalf("save staged files: %q", cached)
	}
	if count := strings.TrimSpace(controllerGit(t, root, "rev-list", "--count", "HEAD")); count != "2" {
		t.Fatalf("save created a commit, commit count = %s", count)
	}
	lastTemp := controller.tempRight
	controller.shutdown(context.Background())
	if _, err := os.Stat(lastTemp); lastTemp != "" && !os.IsNotExist(err) {
		t.Fatalf("temporary right file was not cleaned: %v", err)
	}
}

func TestControllerRestoresLastRepositoryAndFallsBackWhenItMoves(t *testing.T) {
	root, _ := createControllerRepository(t)
	preferencePath := filepath.Join(t.TempDir(), "config", "preferences.json")
	first := NewController("", "", coreapp.Options{})
	first.prefs = preferences.NewStoreAt(preferencePath)
	if _, err := first.OpenRepository(root); err != nil {
		t.Fatal(err)
	}
	if _, err := first.SelectRepositoryRef("refs/remotes/origin/develop"); err != nil {
		t.Fatal(err)
	}
	first.shutdown(context.Background())

	restored := NewController("", "", coreapp.Options{})
	restored.prefs = preferences.NewStoreAt(preferencePath)
	restored.startup(context.Background())
	waitForBootstrap(t, restored)
	state := restored.Bootstrap()
	if state.Repository == nil || state.Mode != modeRepository || state.Error != "" {
		t.Fatalf("restored bootstrap = %+v", state)
	}
	if state.Repository.SelectedRef != "refs/remotes/origin/develop" {
		t.Fatalf("restored repository ref = %q", state.Repository.SelectedRef)
	}
	restored.shutdown(context.Background())

	controllerGit(t, root, "update-ref", "-d", "refs/remotes/origin/develop")
	fallback := NewController("", "", coreapp.Options{})
	fallback.prefs = preferences.NewStoreAt(preferencePath)
	fallback.startup(context.Background())
	waitForBootstrap(t, fallback)
	state = fallback.Bootstrap()
	if state.Repository == nil || state.Repository.SelectedRef != "refs/remotes/origin/main" {
		t.Fatalf("missing preferred ref fallback = %+v", state)
	}
	fallback.shutdown(context.Background())

	moved := root + "-moved"
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	unavailable := NewController("", "", coreapp.Options{})
	unavailable.prefs = preferences.NewStoreAt(preferencePath)
	unavailable.startup(context.Background())
	waitForBootstrap(t, unavailable)
	state = unavailable.Bootstrap()
	if state.Repository != nil || state.HasSession || !strings.Contains(state.Error, "最近打开的仓库不可用") {
		t.Fatalf("unavailable recent repository bootstrap = %+v", state)
	}
}

func TestControllerUnsavedRepositorySwitchOffersCancelSaveAndDiscard(t *testing.T) {
	root, relative := createControllerRepository(t)
	controller := NewController("", "", coreapp.Options{})
	controller.prefs = preferences.NewStoreAt(filepath.Join(t.TempDir(), "preferences.json"))
	if _, err := controller.OpenRepository(root); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.SelectRepositoryFile(relative); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.EditLeft("数据 表", 4, 1, "需要保存", "text"); err != nil {
		t.Fatal(err)
	}
	controller.switchPrompt = func() (string, error) { return "取消", nil }
	if _, err := controller.OpenRepository(root); err == nil || !strings.Contains(err.Error(), "已取消切换") {
		t.Fatalf("cancel switch error = %v", err)
	}
	result, err := controller.Repository()
	if err != nil || result.Repository.SelectedFile != relative || result.Summary == nil || !result.Summary.Dirty {
		t.Fatalf("cancel did not preserve current selection: %+v, %v", result, err)
	}

	controller.switchPrompt = func() (string, error) { return "保存并继续", nil }
	if _, err := controller.OpenRepository(root); err != nil {
		t.Fatal(err)
	}
	if got := controllerWorkbookCell(t, filepath.Join(root, filepath.FromSlash(relative)), "A4"); got != "需要保存" {
		t.Fatalf("save-and-continue value = %q", got)
	}

	if _, err := controller.SelectRepositoryFile(relative); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.EditLeft("数据 表", 5, 1, "应该丢弃", "text"); err != nil {
		t.Fatal(err)
	}
	controller.switchPrompt = func() (string, error) { return "不保存并继续", nil }
	if _, err := controller.OpenRepository(root); err != nil {
		t.Fatal(err)
	}
	if got := controllerWorkbookCell(t, filepath.Join(root, filepath.FromSlash(relative)), "A5"); got != "" {
		t.Fatalf("discard-and-continue unexpectedly saved %q", got)
	}
	controller.shutdown(context.Background())
}

func TestControllerDirectoryDropUsesFirstItemAndRejectsFiles(t *testing.T) {
	root, _ := createControllerRepository(t)
	controller := NewController("", "", coreapp.Options{})
	controller.prefs = preferences.NewStoreAt(filepath.Join(t.TempDir(), "preferences.json"))
	controller.handleFileDrop(0, 0, []string{filepath.Join(root, "config"), t.TempDir()})
	controller.wg.Wait()
	state := controller.Bootstrap()
	if state.Repository == nil || !strings.Contains(state.Repository.Notice, "其余拖入项已忽略") {
		t.Fatalf("multi-directory drop state = %+v", state)
	}
	controller.shutdown(context.Background())

	fileController := NewController("", "", coreapp.Options{})
	fileController.prefs = preferences.NewStoreAt(filepath.Join(t.TempDir(), "preferences.json"))
	fileController.handleFileDrop(0, 0, []string{filepath.Join(root, "README.md")})
	fileController.wg.Wait()
	state = fileController.Bootstrap()
	if state.Repository != nil || !strings.Contains(state.Error, "不能选择文件") {
		t.Fatalf("file drop state = %+v", state)
	}
}

func createControllerRepository(t *testing.T) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo with 中文")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	controllerGit(t, root, "init", "-b", "main")
	controllerGit(t, root, "config", "user.email", "test@example.com")
	controllerGit(t, root, "config", "user.name", "ugxlsx test")
	controllerGit(t, root, "remote", "add", "origin", ".")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	controllerGit(t, root, "add", "README.md")
	controllerGit(t, root, "commit", "-m", "base")
	controllerGit(t, root, "branch", "missing")

	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	relative := "config/中文 表.xlsx"
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	copyControllerFile(t, pair.Left, target)
	styleOnlyRelative := "config/仅样式变化.xlsx"
	styleOnlyTarget := filepath.Join(root, filepath.FromSlash(styleOnlyRelative))
	writeControllerWorkbook(t, styleOnlyTarget, false)
	controllerGit(t, root, "add", relative, styleOnlyRelative)
	controllerGit(t, root, "commit", "-m", "main workbook")
	mainHash := strings.TrimSpace(controllerGit(t, root, "rev-parse", "HEAD"))
	controllerGit(t, root, "update-ref", "refs/remotes/origin/main", mainHash)
	controllerGit(t, root, "branch", "--set-upstream-to=origin/main", "main")
	controllerGit(t, root, "branch", "develop")
	controllerGit(t, root, "switch", "develop")
	copyControllerFile(t, pair.Right, target)
	writeControllerWorkbook(t, styleOnlyTarget, true)
	controllerGit(t, root, "add", relative, styleOnlyRelative)
	controllerGit(t, root, "commit", "-m", "develop workbook")
	hash := strings.TrimSpace(controllerGit(t, root, "rev-parse", "HEAD"))
	controllerGit(t, root, "update-ref", "refs/remotes/origin/develop", hash)
	controllerGit(t, root, "switch", "main")
	return root, relative
}

func writeControllerWorkbook(t *testing.T, path string, styled bool) {
	t.Helper()
	file := excelize.NewFile()
	defer file.Close()
	if err := file.SetCellStr("Sheet1", "A1", "语义相同"); err != nil {
		t.Fatal(err)
	}
	if styled {
		style, err := file.NewStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Color: []string{"#FFF2CC"}, Pattern: 1},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := file.SetCellStyle("Sheet1", "A1", "A1", style); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
}

func copyControllerFile(t *testing.T, source, target string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func controllerGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func controllerWorkbookCell(t *testing.T, path, axis string) string {
	t.Helper()
	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	value, err := file.GetCellValue("数据 表", axis)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func waitForBootstrap(t *testing.T, controller *Controller) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for controller.Bootstrap().Loading && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if controller.Bootstrap().Loading {
		t.Fatal("controller bootstrap timed out")
	}
}

func waitForDifferenceIndex(t *testing.T, controller *Controller) RepositoryResult {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		result, err := controller.Repository()
		if err != nil {
			t.Fatal(err)
		}
		if !result.Repository.DifferenceIndexing {
			return result
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("repository difference index timed out")
	return RepositoryResult{}
}
