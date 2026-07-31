package ugit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

const (
	diffCommandKey       = "difftool.*.xlsx_Custom.cmd"
	mergeCommandKey      = "mergetool.*.xlsx_Custom.cmd"
	mergeTrustExitKey    = "mergetool.*.xlsx_Custom.trustexitcode"
	managedKeyExpression = `^(difftool|mergetool)\.\*\.xlsx_[0-9A-Za-z]+\.(cmd|trustexitcode)$`
	commandTimeout       = 10 * time.Second
)

type Entry struct {
	Key   string
	Value string
}

type Inspection struct {
	Configured     bool
	NeedsUpdate    bool
	ExecutablePath string
	ExistingPaths  []string
	Existing       []Entry
}

type client struct {
	gitPath string
}

// CurrentExecutablePath returns a stable absolute executable path suitable for
// persisting in UGit's Git configuration.
func CurrentExecutablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("读取当前应用路径: %w", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("规范化当前应用路径: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
	}
	if strings.Contains(path, "'") {
		return "", errors.New("应用路径包含单引号，UGit 无法安全解析；请将应用移动到不含单引号的固定目录")
	}
	cleanPath := filepath.Clean(path)
	tempDir := filepath.Clean(os.TempDir()) + string(os.PathSeparator)
	inTemp := strings.HasPrefix(cleanPath, tempDir)
	if runtime.GOOS == "windows" {
		inTemp = strings.HasPrefix(strings.ToLower(cleanPath), strings.ToLower(tempDir))
	}
	if inTemp ||
		strings.Contains(filepath.ToSlash(path), "/AppTranslocation/") {
		return "", errors.New("当前应用位于临时目录；请先移动到固定位置并重新启动，再配置 UGit")
	}
	return path, nil
}

func Inspect(ctx context.Context, executablePath string) (Inspection, error) {
	c, err := newClient()
	if err != nil {
		return Inspection{}, err
	}
	return c.inspect(ctx, executablePath)
}

// Configure replaces only UGit's global *.xlsx diff/merge registrations. Any
// existing XLSX tool entries are snapshotted and restored if a later write or
// verification step fails.
func Configure(ctx context.Context, executablePath string) (Inspection, error) {
	c, err := newClient()
	if err != nil {
		return Inspection{}, err
	}
	return c.configure(ctx, executablePath)
}

func newClient() (client, error) {
	gitPath, err := findGitExecutable()
	if err != nil {
		return client{}, err
	}
	return client{gitPath: gitPath}, nil
}

func findGitExecutable() (string, error) {
	if path, err := exec.LookPath("git"); err == nil {
		return path, nil
	}
	for _, candidate := range gitExecutableCandidates(runtime.GOOS, os.Getenv) {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("未找到 Git；请先安装 Git，或从 UGit 启动一次本应用后重试")
}

func gitExecutableCandidates(goos string, getenv func(string) string) []string {
	switch goos {
	case "darwin":
		return []string{
			"/usr/bin/git",
			"/Applications/UGit.app/Contents/Resources/app/git/bin/git",
		}
	case "windows":
		var candidates []string
		add := func(root string, elements ...string) {
			if strings.TrimSpace(root) == "" {
				return
			}
			candidates = append(candidates, windowsJoin(root, elements...))
		}
		localAppData := getenv("LOCALAPPDATA")
		programFiles := getenv("ProgramFiles")
		programFilesX86 := getenv("ProgramFiles(x86)")
		add(localAppData, "Programs", "UGit", "resources", "app", "git", "cmd", "git.exe")
		add(localAppData, "Programs", "UGit", "resources", "app", "git", "mingw64", "bin", "git.exe")
		add(localAppData, "UGit", "resources", "app", "git", "cmd", "git.exe")
		add(localAppData, "UGit", "resources", "app", "git", "mingw64", "bin", "git.exe")
		add(localAppData, "Programs", "Git", "cmd", "git.exe")
		add(programFiles, "UGit", "resources", "app", "git", "cmd", "git.exe")
		add(programFiles, "UGit", "resources", "app", "git", "mingw64", "bin", "git.exe")
		add(programFilesX86, "UGit", "resources", "app", "git", "cmd", "git.exe")
		add(programFilesX86, "UGit", "resources", "app", "git", "mingw64", "bin", "git.exe")
		add(programFiles, "Git", "cmd", "git.exe")
		add(programFilesX86, "Git", "cmd", "git.exe")
		return candidates
	default:
		return []string{"/usr/bin/git", "/usr/local/bin/git"}
	}
}

func windowsJoin(root string, elements ...string) string {
	path := strings.TrimRight(root, `\/`)
	for _, element := range elements {
		path += `\` + strings.Trim(element, `\/`)
	}
	return path
}

func (c client) inspect(ctx context.Context, executablePath string) (Inspection, error) {
	if err := validateExecutablePath(executablePath); err != nil {
		return Inspection{}, err
	}
	entries, err := c.listManagedEntries(ctx)
	if err != nil {
		return Inspection{}, err
	}
	desired := desiredEntries(executablePath)
	configured := entriesEqual(entries, desired)
	return Inspection{
		Configured:     configured,
		NeedsUpdate:    len(entries) > 0 && !configured,
		ExecutablePath: executablePath,
		ExistingPaths:  existingPaths(entries),
		Existing:       entries,
	}, nil
}

func (c client) configure(ctx context.Context, executablePath string) (Inspection, error) {
	before, err := c.inspect(ctx, executablePath)
	if err != nil || before.Configured {
		return before, err
	}
	desired := desiredEntries(executablePath)
	keys := uniqueKeys(append(slices.Clone(before.Existing), desired...))
	rollback := func(cause error) (Inspection, error) {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		defer cancel()
		if rollbackErr := c.restore(rollbackCtx, keys, before.Existing); rollbackErr != nil {
			return Inspection{}, errors.Join(cause, fmt.Errorf("恢复原 UGit 配置失败: %w", rollbackErr))
		}
		return Inspection{}, cause
	}

	for _, key := range keys {
		if err := c.unsetAll(ctx, key); err != nil {
			return rollback(fmt.Errorf("清理旧 UGit 配置 %s: %w", key, err))
		}
	}
	for _, entry := range desired {
		if err := c.add(ctx, entry); err != nil {
			return rollback(fmt.Errorf("写入 UGit 配置 %s: %w", entry.Key, err))
		}
	}
	after, err := c.inspect(ctx, executablePath)
	if err != nil {
		return rollback(fmt.Errorf("验证 UGit 配置: %w", err))
	}
	if !after.Configured {
		return rollback(errors.New("UGit 配置写入后校验不一致"))
	}
	return after, nil
}

func (c client) restore(ctx context.Context, keys []string, entries []Entry) error {
	for _, key := range keys {
		if err := c.unsetAll(ctx, key); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		if err := c.add(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

func (c client) listManagedEntries(ctx context.Context) ([]Entry, error) {
	output, exitCode, err := c.run(
		ctx,
		"config", "--global", "-z", "--get-regexp", managedKeyExpression,
	)
	if exitCode == 1 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, record := range bytes.Split(output, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		key, value, found := bytes.Cut(record, []byte{'\n'})
		if !found || len(key) == 0 {
			return nil, errors.New("Git 返回了无法识别的 UGit 工具配置")
		}
		entries = append(entries, Entry{Key: string(key), Value: string(value)})
	}
	return entries, nil
}

func (c client) unsetAll(ctx context.Context, key string) error {
	_, exitCode, err := c.run(ctx, "config", "--global", "--unset-all", key)
	if exitCode == 1 || exitCode == 5 {
		return nil
	}
	return err
}

func (c client) add(ctx context.Context, entry Entry) error {
	_, _, err := c.run(ctx, "config", "--global", "--add", entry.Key, entry.Value)
	return err
}

func (c client) run(parent context.Context, args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, c.gitPath, args...)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return output, -1, fmt.Errorf("Git 配置命令超时: %w", ctx.Err())
	}
	if err == nil {
		return output, 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return output, exitErr.ExitCode(), err
		}
		return output, exitErr.ExitCode(), fmt.Errorf("%w: %s", err, message)
	}
	return output, -1, err
}

func validateExecutablePath(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("UGit 工具路径必须是绝对路径")
	}
	if strings.Contains(path, "'") {
		return errors.New("应用路径包含单引号，UGit 无法安全解析")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("当前应用可执行文件不可用: %w", err)
	}
	if info.IsDir() {
		return errors.New("UGit 工具路径不能是目录")
	}
	return nil
}

func desiredEntries(executablePath string) []Entry {
	quotedPath := "'" + executablePath + "'"
	return []Entry{
		{
			Key:   diffCommandKey,
			Value: quotedPath + ` compare --left "$LOCAL" --right "$REMOTE"`,
		},
		{
			Key:   mergeCommandKey,
			Value: quotedPath + ` compare --left "$LOCAL" --right "$REMOTE" --output "$MERGED"`,
		},
		{Key: mergeTrustExitKey, Value: "false"},
	}
}

func entriesEqual(actual, desired []Entry) bool {
	if len(actual) != len(desired) {
		return false
	}
	counts := make(map[Entry]int, len(actual))
	for _, entry := range actual {
		counts[entry]++
	}
	for _, entry := range desired {
		if counts[entry] == 0 {
			return false
		}
		counts[entry]--
	}
	return true
}

func uniqueKeys(entries []Entry) []string {
	seen := make(map[string]struct{}, len(entries))
	var result []string
	for _, entry := range entries {
		if _, exists := seen[entry.Key]; exists {
			continue
		}
		seen[entry.Key] = struct{}{}
		result = append(result, entry.Key)
	}
	return result
}

func existingPaths(entries []Entry) []string {
	seen := make(map[string]struct{})
	var paths []string
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Key, ".cmd") {
			continue
		}
		value := strings.TrimSpace(entry.Value)
		if !strings.HasPrefix(value, "'") {
			continue
		}
		end := strings.Index(value[1:], "' ")
		if end < 0 {
			continue
		}
		path := value[1 : end+1]
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}
