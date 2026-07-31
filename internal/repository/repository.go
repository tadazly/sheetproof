package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ug-tools/ugxlsx/internal/backgroundcmd"
)

const commandTimeout = 15 * time.Second

type BranchKind string

const (
	LocalBranch  BranchKind = "local"
	RemoteBranch BranchKind = "remote"
)

type Branch struct {
	Name     string     `json:"name"`
	FullName string     `json:"fullName"`
	Kind     BranchKind `json:"kind"`
}

type Info struct {
	Root          string   `json:"root"`
	Name          string   `json:"name"`
	CurrentBranch string   `json:"currentBranch"`
	Detached      bool     `json:"detached"`
	Dirty         bool     `json:"dirty"`
	Operation     string   `json:"operation"`
	Files         []string `json:"files"`
	Branches      []Branch `json:"branches"`
	DefaultRef    string   `json:"defaultRef"`
	GitDirectory  string   `json:"-"`
}

type Repository struct {
	root string
}

type MissingFileError struct {
	Ref  string
	Path string
}

func (e *MissingFileError) Error() string {
	return fmt.Sprintf("%s:%s does not exist", e.Ref, e.Path)
}

func IsMissingFile(err error) bool {
	var target *MissingFileError
	return errors.As(err, &target)
}

func Open(path string) (*Repository, Info, error) {
	root, err := FindRoot(path)
	if err != nil {
		return nil, Info{}, err
	}
	repo := &Repository{root: root}
	info, err := repo.Refresh()
	if err != nil {
		return nil, Info{}, err
	}
	return repo, info, nil
}

func FindRoot(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("仓库路径不能为空")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("解析仓库路径失败: %w", err)
	}
	stat, err := os.Stat(absolute)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("仓库目录不存在: %s", absolute)
		}
		return "", fmt.Errorf("无法访问仓库目录 %s: %w", absolute, err)
	}
	if !stat.IsDir() {
		return "", fmt.Errorf("请选择 Git 仓库目录，不能选择文件: %s", absolute)
	}
	output, err := runGit(absolute, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("该目录不是有效的 Git 仓库: %s", absolute)
	}
	root := strings.TrimSpace(string(output))
	if root == "" {
		return "", fmt.Errorf("该目录不是有效的 Git 仓库: %s", absolute)
	}
	resolved, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("解析 Git 仓库根目录失败: %w", err)
	}
	return filepath.Clean(resolved), nil
}

func (r *Repository) Root() string {
	return r.root
}

func (r *Repository) Refresh() (Info, error) {
	branch, detached, err := r.currentBranch()
	if err != nil {
		return Info{}, err
	}
	branches, err := r.branches(branch, detached)
	if err != nil {
		return Info{}, err
	}
	files, err := r.ScanXLSX()
	if err != nil {
		return Info{}, err
	}
	dirtyOutput, err := runGit(r.root, nil, "status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return Info{}, fmt.Errorf("读取 Git 工作区状态失败: %w", err)
	}
	gitDirOutput, err := runGit(r.root, nil, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return Info{}, fmt.Errorf("读取 Git 元数据目录失败: %w", err)
	}
	gitDir := strings.TrimSpace(string(gitDirOutput))
	info := Info{
		Root:          r.root,
		Name:          filepath.Base(r.root),
		CurrentBranch: branch,
		Detached:      detached,
		Dirty:         len(bytes.TrimSpace(dirtyOutput)) > 0,
		Operation:     operationAt(gitDir),
		Files:         files,
		Branches:      branches,
		GitDirectory:  gitDir,
	}
	if len(branches) > 0 {
		info.DefaultRef = r.defaultReference(branch, detached, branches)
	}
	return info, nil
}

func (r *Repository) ScanXLSX() ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(r.root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".xlsx") {
			return nil
		}
		relative, err := filepath.Rel(r.root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描仓库 XLSX 文件失败: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return naturalLess(files[i], files[j]) })
	return files, nil
}

func (r *Repository) ResolveRelativePath(relative string) (string, error) {
	normalized, err := normalizeRelativePath(relative)
	if err != nil {
		return "", err
	}
	absolute := filepath.Join(r.root, filepath.FromSlash(normalized))
	check, err := filepath.Rel(r.root, absolute)
	if err != nil || check == ".." || strings.HasPrefix(check, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("文件路径超出仓库范围: %s", relative)
	}
	return absolute, nil
}

func (r *Repository) ResolveReference(value string, branches []Branch) (Branch, error) {
	for _, branch := range branches {
		if value == branch.FullName {
			return branch, nil
		}
	}
	var matches []Branch
	for _, branch := range branches {
		if value == branch.Name {
			matches = append(matches, branch)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return Branch{}, fmt.Errorf("分支名称存在歧义，请使用完整引用: %s", value)
	}
	return Branch{}, fmt.Errorf("无效的对比分支: %s", value)
}

func (r *Repository) ReadReferenceFile(ref Branch, relative string) (string, error) {
	return r.ReadReferenceFileContext(context.Background(), ref, relative)
}

// ReadReferenceFileContext exports a validated Git object while allowing
// background callers to cancel the Git processes during shutdown.
func (r *Repository) ReadReferenceFileContext(
	ctx context.Context,
	ref Branch,
	relative string,
) (string, error) {
	normalized, err := normalizeRelativePath(relative)
	if err != nil {
		return "", err
	}
	if ref.FullName == "" || (ref.Kind != LocalBranch && ref.Kind != RemoteBranch) {
		return "", errors.New("无效的 Git 引用")
	}
	object := ref.FullName + ":" + normalized
	if _, err := runGitContext(ctx, r.root, nil, "cat-file", "-e", object); err != nil {
		if errors.Is(err, context.Canceled) {
			return "", err
		}
		return "", &MissingFileError{Ref: ref.Name, Path: normalized}
	}
	temp, err := os.CreateTemp("", "ugxlsx-repository-*.xlsx")
	if err != nil {
		return "", fmt.Errorf("创建分支工作簿临时文件失败: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}
	if _, err := runGitContext(ctx, r.root, temp, "show", "--no-textconv", "--no-ext-diff", object); err != nil {
		cleanup()
		return "", fmt.Errorf("读取分支 %s 中的 %s 失败: %w", ref.Name, normalized, err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return "", fmt.Errorf("写入分支工作簿临时文件失败: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("关闭分支工作簿临时文件失败: %w", err)
	}
	return tempPath, nil
}

func (r *Repository) FileModified(relative string) (bool, error) {
	normalized, err := normalizeRelativePath(relative)
	if err != nil {
		return false, err
	}
	output, err := runGit(r.root, nil, "status", "--porcelain=v1", "--untracked-files=all", "--", normalized)
	if err != nil {
		return false, fmt.Errorf("读取文件 Git 状态失败: %w", err)
	}
	return len(bytes.TrimSpace(output)) > 0, nil
}

// ChangedCommonXLSX returns worktree XLSX paths which also exist in the
// selected reference and whose Git content differs. It intentionally avoids
// opening workbooks; exact cell differences are calculated only after a file
// is selected.
func (r *Repository) ChangedCommonXLSX(ref Branch, worktreeFiles []string) ([]string, error) {
	return r.ChangedCommonXLSXContext(context.Background(), ref, worktreeFiles)
}

// ChangedCommonXLSXContext is the cancellable form used by the background
// semantic difference index.
func (r *Repository) ChangedCommonXLSXContext(
	ctx context.Context,
	ref Branch,
	worktreeFiles []string,
) ([]string, error) {
	if ref.FullName == "" || (ref.Kind != LocalBranch && ref.Kind != RemoteBranch) {
		return nil, errors.New("无效的 Git 引用")
	}
	files := make([]string, 0, len(worktreeFiles))
	worktreeSet := make(map[string]struct{}, len(worktreeFiles))
	for _, file := range worktreeFiles {
		normalized, err := normalizeRelativePath(file)
		if err != nil {
			return nil, err
		}
		if _, exists := worktreeSet[normalized]; exists {
			continue
		}
		worktreeSet[normalized] = struct{}{}
		files = append(files, normalized)
	}

	diffOutput, err := runGitContext(
		ctx, r.root, nil,
		"diff", "--name-only", "-z", "--no-renames", "--no-textconv", "--no-ext-diff",
		ref.FullName, "--",
	)
	if err != nil {
		return nil, fmt.Errorf("读取引用与工作区文件变化失败: %w", err)
	}
	changed := make(map[string]struct{}, len(files))
	for _, path := range splitNULPaths(diffOutput) {
		normalized := filepath.ToSlash(path)
		if _, exists := worktreeSet[normalized]; exists {
			changed[normalized] = struct{}{}
		}
	}

	treeOutput, err := runGitContext(ctx, r.root, nil, "ls-tree", "-r", "-z", "--name-only", ref.FullName)
	if err != nil {
		return nil, fmt.Errorf("读取引用中的文件列表失败: %w", err)
	}
	referenceFiles := make(map[string]struct{})
	for _, path := range splitNULPaths(treeOutput) {
		if strings.EqualFold(filepath.Ext(path), ".xlsx") {
			referenceFiles[filepath.ToSlash(path)] = struct{}{}
		}
	}
	result := make([]string, 0, len(changed))
	for _, file := range files {
		_, changedInGit := changed[file]
		_, existsInReference := referenceFiles[file]
		if changedInGit && existsInReference {
			result = append(result, file)
		}
	}
	return result, nil
}

// DifferenceIndexSignature identifies the local and reference content used by
// the exact semantic differing-workbook index. Worktree metadata keeps startup
// cheap while still invalidating normal edits, saves, additions, and removals.
func (r *Repository) DifferenceIndexSignature(ref Branch, worktreeFiles []string) (string, error) {
	return r.differenceIndexSignatureContext(context.Background(), ref, worktreeFiles, "")
}

// DifferenceIndexSignatureContext is the cancellable form used to verify a
// completed background index before it is cached.
func (r *Repository) DifferenceIndexSignatureContext(
	ctx context.Context,
	ref Branch,
	worktreeFiles []string,
) (string, error) {
	return r.differenceIndexSignatureContext(ctx, ref, worktreeFiles, "")
}

// DifferenceIndexSignatureExcluding fingerprints every input except one
// worktree path. Controllers use it around an in-app save to prove that no
// other workbook, HEAD, or comparison ref changed before incrementally
// updating the cached differing-workbook list.
func (r *Repository) DifferenceIndexSignatureExcluding(
	ref Branch,
	worktreeFiles []string,
	excluded string,
) (string, error) {
	normalized, err := normalizeRelativePath(excluded)
	if err != nil {
		return "", err
	}
	return r.differenceIndexSignatureContext(
		context.Background(), ref, worktreeFiles, normalized,
	)
}

func (r *Repository) differenceIndexSignatureContext(
	ctx context.Context,
	ref Branch,
	worktreeFiles []string,
	excluded string,
) (string, error) {
	if ref.FullName == "" || (ref.Kind != LocalBranch && ref.Kind != RemoteBranch) {
		return "", errors.New("无效的 Git 引用")
	}
	refOID, err := runGitContext(ctx, r.root, nil, "rev-parse", "--verify", ref.FullName+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("读取对比分支版本失败: %w", err)
	}
	headOID, err := runGitContext(ctx, r.root, nil, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", fmt.Errorf("读取当前工作区版本失败: %w", err)
	}
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "ugxlsx-semantic-difference-index-v2\x00%s\x00%s\x00",
		bytes.TrimSpace(headOID), bytes.TrimSpace(refOID))
	for _, file := range worktreeFiles {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		normalized, normalizeErr := normalizeRelativePath(file)
		if normalizeErr != nil {
			return "", normalizeErr
		}
		if normalized == excluded {
			continue
		}
		absolute, resolveErr := r.ResolveRelativePath(normalized)
		if resolveErr != nil {
			return "", resolveErr
		}
		info, statErr := os.Stat(absolute)
		if statErr != nil {
			return "", fmt.Errorf("读取工作区表格状态失败 %s: %w", normalized, statErr)
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%d\x00%d\x00", normalized, info.Size(), info.ModTime().UnixNano())
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (r *Repository) currentBranch() (string, bool, error) {
	output, err := runGit(r.root, nil, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		return strings.TrimSpace(string(output)), false, nil
	}
	head, headErr := runGit(r.root, nil, "rev-parse", "--short", "HEAD")
	if headErr != nil {
		return "", false, fmt.Errorf("读取当前 Git 分支失败: %w", headErr)
	}
	return strings.TrimSpace(string(head)), true, nil
}

func (r *Repository) defaultReference(current string, detached bool, branches []Branch) string {
	if !detached {
		if output, err := runGit(r.root, nil, "rev-parse", "--symbolic-full-name", "@{upstream}"); err == nil {
			upstream := strings.TrimSpace(string(output))
			for _, branch := range branches {
				if branch.Kind == RemoteBranch && branch.FullName == upstream {
					return branch.FullName
				}
			}
		}
		for _, branch := range branches {
			if branch.Kind != RemoteBranch {
				continue
			}
			parts := strings.SplitN(branch.Name, "/", 2)
			if len(parts) == 2 && parts[1] == current {
				return branch.FullName
			}
		}
	}
	if len(branches) > 0 {
		return branches[0].FullName
	}
	return ""
}

func (r *Repository) branches(current string, detached bool) ([]Branch, error) {
	output, err := runGit(r.root, nil,
		"for-each-ref", "--format=%(refname)%00%(refname:short)%00%(symref)",
		"refs/heads", "refs/remotes")
	if err != nil {
		return nil, fmt.Errorf("读取 Git 分支列表失败: %w", err)
	}
	locals := make([]Branch, 0)
	remotes := make([]Branch, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x00")
		if len(parts) < 3 || parts[2] != "" {
			continue
		}
		full, short := parts[0], parts[1]
		switch {
		case strings.HasPrefix(full, "refs/heads/"):
			if !detached && short == current {
				continue
			}
			locals = append(locals, Branch{Name: short, FullName: full, Kind: LocalBranch})
		case strings.HasPrefix(full, "refs/remotes/"):
			if strings.HasSuffix(short, "/HEAD") {
				continue
			}
			remotes = append(remotes, Branch{Name: short, FullName: full, Kind: RemoteBranch})
		}
	}
	less := func(items []Branch) {
		sort.Slice(items, func(i, j int) bool { return naturalLess(items[i].Name, items[j].Name) })
	}
	less(locals)
	less(remotes)
	return append(locals, remotes...), nil
}

func splitNULPaths(output []byte) []string {
	parts := bytes.Split(output, []byte{0})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) > 0 {
			result = append(result, string(part))
		}
	}
	return result
}

func normalizeRelativePath(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", errors.New("表格相对路径不能为空")
	}
	if filepath.IsAbs(value) || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("表格路径必须是仓库内相对路径: %s", value)
	}
	normalized := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", fmt.Errorf("表格路径超出仓库范围: %s", value)
	}
	if !strings.EqualFold(filepath.Ext(normalized), ".xlsx") {
		return "", fmt.Errorf("仅支持仓库中的 .xlsx 文件: %s", value)
	}
	return normalized, nil
}

func operationAt(gitDir string) string {
	checks := []struct {
		name string
		path string
	}{
		{"merge", "MERGE_HEAD"},
		{"rebase", "rebase-merge"},
		{"rebase", "rebase-apply"},
		{"cherry-pick", "CHERRY_PICK_HEAD"},
		{"revert", "REVERT_HEAD"},
		{"bisect", "BISECT_LOG"},
	}
	for _, check := range checks {
		if _, err := os.Stat(filepath.Join(gitDir, check.path)); err == nil {
			return check.name
		}
	}
	return ""
}

func runGit(directory string, stdout io.Writer, args ...string) ([]byte, error) {
	return runGitContext(context.Background(), directory, stdout, args...)
}

func runGitContext(parent context.Context, directory string, stdout io.Writer, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()
	commandArgs := append([]string{"-C", directory}, args...)
	command := backgroundcmd.CommandContext(ctx, "git", commandArgs...)
	var output bytes.Buffer
	var stderr bytes.Buffer
	if stdout == nil {
		command.Stdout = &output
	} else {
		command.Stdout = stdout
	}
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("Git 命令超时")
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, errors.New(message)
	}
	return output.Bytes(), nil
}

func naturalLess(left, right string) bool {
	a, b := []rune(strings.ToLower(left)), []rune(strings.ToLower(right))
	for i, j := 0, 0; i < len(a) && j < len(b); {
		if unicode.IsDigit(a[i]) && unicode.IsDigit(b[j]) {
			startI, startJ := i, j
			for i < len(a) && unicode.IsDigit(a[i]) {
				i++
			}
			for j < len(b) && unicode.IsDigit(b[j]) {
				j++
			}
			an, _ := strconv.ParseUint(string(a[startI:i]), 10, 64)
			bn, _ := strconv.ParseUint(string(b[startJ:j]), 10, 64)
			if an != bn {
				return an < bn
			}
			continue
		}
		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
	return len(a) < len(b)
}
