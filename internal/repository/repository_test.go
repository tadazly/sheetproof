package repository

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestRepositoryDiscoveryBranchesScanAndObjectRead(t *testing.T) {
	root := filepath.Join(t.TempDir(), "中文 仓库")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-b", "main")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "user.name", "ugxlsx test")
	git(t, root, "remote", "add", "origin", ".")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "README.md")
	git(t, root, "commit", "-m", "base")
	git(t, root, "branch", "missing")

	relative := filepath.ToSlash(filepath.Join("配置 目录", "奖励 表.xlsx"))
	writeWorkbook(t, filepath.Join(root, filepath.FromSlash(relative)), "main")
	git(t, root, "add", relative)
	git(t, root, "commit", "-m", "main workbook")
	mainHash := strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
	git(t, root, "update-ref", "refs/remotes/origin/main", mainHash)
	git(t, root, "branch", "--set-upstream-to=origin/main", "main")
	git(t, root, "branch", "feature10")
	git(t, root, "branch", "feature2")
	git(t, root, "branch", "develop")
	git(t, root, "switch", "develop")
	writeWorkbook(t, filepath.Join(root, filepath.FromSlash(relative)), "develop")
	git(t, root, "add", relative)
	git(t, root, "commit", "-m", "develop workbook")
	developHash := strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
	git(t, root, "update-ref", "refs/remotes/origin/develop", developHash)
	git(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/develop")
	git(t, root, "switch", "main")

	ignored := filepath.Join(root, ".git", "ignored.xlsx")
	if err := os.WriteFile(ignored, []byte("not a workbook"), 0o644); err != nil {
		t.Fatal(err)
	}
	subdirectory := filepath.Join(root, "配置 目录")
	foundRoot, err := FindRoot(subdirectory)
	canonicalRoot, canonicalErr := filepath.EvalSymlinks(root)
	if err != nil || canonicalErr != nil || foundRoot != canonicalRoot {
		t.Fatalf("FindRoot(%q) = %q, %v; want %q (%v)", subdirectory, foundRoot, err, canonicalRoot, canonicalErr)
	}
	repo, info, err := Open(subdirectory)
	if err != nil {
		t.Fatal(err)
	}
	if info.CurrentBranch != "main" || info.Detached {
		t.Fatalf("branch state = %q detached=%t", info.CurrentBranch, info.Detached)
	}
	if len(info.Files) != 1 || info.Files[0] != relative {
		t.Fatalf("files = %#v", info.Files)
	}
	gotNames := make([]string, len(info.Branches))
	for index, branch := range info.Branches {
		gotNames[index] = string(branch.Kind) + ":" + branch.Name
	}
	wantNames := []string{
		"local:develop", "local:feature2", "local:feature10", "local:missing",
		"remote:origin/develop", "remote:origin/main",
	}
	if strings.Join(gotNames, "|") != strings.Join(wantNames, "|") {
		t.Fatalf("branches = %#v, want %#v", gotNames, wantNames)
	}
	if info.DefaultRef != "refs/remotes/origin/main" {
		t.Fatalf("default ref = %q", info.DefaultRef)
	}
	git(t, root, "branch", "--unset-upstream", "main")
	refreshed, err := repo.Refresh()
	if err != nil || refreshed.DefaultRef != "refs/remotes/origin/main" {
		t.Fatalf("matching remote fallback = %q, %v", refreshed.DefaultRef, err)
	}

	develop, err := repo.ResolveReference("develop", info.Branches)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := repo.ChangedCommonXLSX(develop, info.Files)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0] != relative {
		t.Fatalf("changed XLSX candidates = %#v", candidates)
	}
	beforeBranch := strings.TrimSpace(git(t, root, "branch", "--show-current"))
	beforeStatus := git(t, root, "status", "--porcelain=v1")
	temp, err := repo.ReadReferenceFile(develop, relative)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(temp) })
	if filepath.Dir(temp) == root || strings.HasPrefix(temp, root+string(filepath.Separator)) {
		t.Fatalf("temporary ref file was created inside repository: %s", temp)
	}
	if got := readWorkbook(t, temp); got != "develop" {
		t.Fatalf("ref workbook value = %q", got)
	}
	if after := strings.TrimSpace(git(t, root, "branch", "--show-current")); after != beforeBranch {
		t.Fatalf("reading ref switched branch from %q to %q", beforeBranch, after)
	}
	if after := git(t, root, "status", "--porcelain=v1"); after != beforeStatus {
		t.Fatalf("reading ref changed worktree status:\nbefore %q\nafter %q", beforeStatus, after)
	}

	missing, err := repo.ResolveReference("missing", info.Branches)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReadReferenceFile(missing, relative); !IsMissingFile(err) {
		t.Fatalf("missing ref path error = %v", err)
	}
	candidates, err = repo.ChangedCommonXLSX(missing, info.Files)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("missing-ref candidates = %#v, %v", candidates, err)
	}

	signatureBefore, err := repo.DifferenceIndexSignature(develop, info.Files)
	if err != nil {
		t.Fatal(err)
	}
	stableBefore, err := repo.DifferenceIndexSignatureExcluding(develop, info.Files, relative)
	if err != nil {
		t.Fatal(err)
	}
	writeWorkbook(t, filepath.Join(root, filepath.FromSlash(relative)), "uncommitted")
	signatureAfter, err := repo.DifferenceIndexSignature(develop, info.Files)
	if err != nil {
		t.Fatal(err)
	}
	stableAfter, err := repo.DifferenceIndexSignatureExcluding(develop, info.Files, relative)
	if err != nil {
		t.Fatal(err)
	}
	if signatureBefore == signatureAfter {
		t.Fatal("difference index signature did not change after worktree edit")
	}
	if stableBefore != stableAfter {
		t.Fatal("signature excluding the saved workbook changed after only that workbook was edited")
	}
	modified, err := repo.FileModified(relative)
	if err != nil || !modified {
		t.Fatalf("FileModified = %t, %v", modified, err)
	}
	if got := readWorkbook(t, filepath.Join(root, filepath.FromSlash(relative))); got != "uncommitted" {
		t.Fatalf("working tree value = %q", got)
	}
}

func TestRepositoryRejectsOrdinaryDirectoryAndFiles(t *testing.T) {
	plain := t.TempDir()
	if _, err := FindRoot(plain); err == nil || !strings.Contains(err.Error(), "不是有效的 Git 仓库") {
		t.Fatalf("ordinary directory error = %v", err)
	}
	file := filepath.Join(plain, "file.xlsx")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FindRoot(file); err == nil || !strings.Contains(err.Error(), "不能选择文件") {
		t.Fatalf("file error = %v", err)
	}
}

func TestRepositoryDetachedHeadAndArgumentSafety(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-b", "main")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "user.name", "ugxlsx test")
	relative := "目录/$(touch injected).xlsx"
	writeWorkbook(t, filepath.Join(root, filepath.FromSlash(relative)), "safe")
	git(t, root, "add", relative)
	git(t, root, "commit", "-m", "safe")
	git(t, root, "branch", "evil;touch-injected")
	git(t, root, "checkout", "--detach", "HEAD")
	repo, info, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Detached || info.CurrentBranch == "" {
		t.Fatalf("detached state = %+v", info)
	}
	branch, err := repo.ResolveReference("evil;touch-injected", info.Branches)
	if err != nil {
		t.Fatal(err)
	}
	temp, err := repo.ReadReferenceFile(branch, relative)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(temp)
	if _, err := os.Stat(filepath.Join(root, "injected")); !os.IsNotExist(err) {
		t.Fatalf("branch or path was interpreted by a shell: %v", err)
	}
	if _, err := repo.ResolveRelativePath("../outside.xlsx"); err == nil {
		t.Fatal("path traversal was accepted")
	}
}

func git(t *testing.T, directory string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", directory}, args...)
	command := exec.Command("git", commandArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func writeWorkbook(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file := excelize.NewFile()
	defer file.Close()
	if err := file.SetCellStr("Sheet1", "A1", value); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
}

func readWorkbook(t *testing.T, path string) string {
	t.Helper()
	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	value, err := file.GetCellValue("Sheet1", "A1")
	if err != nil {
		t.Fatal(err)
	}
	return value
}
