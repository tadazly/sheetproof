package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
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
		info.DefaultRef = branches[0].FullName
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
	normalized, err := normalizeRelativePath(relative)
	if err != nil {
		return "", err
	}
	if ref.FullName == "" || (ref.Kind != LocalBranch && ref.Kind != RemoteBranch) {
		return "", errors.New("无效的 Git 引用")
	}
	object := ref.FullName + ":" + normalized
	if _, err := runGit(r.root, nil, "cat-file", "-e", object); err != nil {
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
	if _, err := runGit(r.root, temp, "show", "--no-textconv", "--no-ext-diff", object); err != nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	commandArgs := append([]string{"-C", directory}, args...)
	command := exec.CommandContext(ctx, "git", commandArgs...)
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
