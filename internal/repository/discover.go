package repository

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type DiscoveryIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type Discovery struct {
	Directory string
	Roots     []string
	Issues    []DiscoveryIssue
}

// Discover finds worktrees (including nested repositories and .git files),
// without entering Git metadata or following directory links outside the scan.
func Discover(ctx context.Context, directory string) (Discovery, error) {
	return DiscoverWithGit(ctx, directory, "git")
}

// DiscoverWithGit keeps discovery on the integration's selected Git executable.
func DiscoverWithGit(ctx context.Context, directory, gitExecutable string) (Discovery, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return Discovery{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return Discovery{}, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return Discovery{}, err
	}
	if !info.IsDir() {
		return Discovery{}, fmt.Errorf("请选择要扫描的目录")
	}
	if filepath.Base(root) == ".git" {
		return Discovery{}, fmt.Errorf("请选择工作区或其上级目录，不要选择 .git 目录")
	}
	result := Discovery{Directory: root, Roots: []string{}, Issues: []DiscoveryIssue{}}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			if path == root {
				return walkErr
			}
			result.Issues = append(result.Issues, DiscoveryIssue{Path: path, Message: walkErr.Error()})
			return nil
		}
		if entry.Name() != ".git" {
			return nil
		}
		parent := filepath.Dir(path)
		if entry.Type()&os.ModeSymlink != 0 {
			result.Issues = append(result.Issues, DiscoveryIssue{Path: parent, Message: "已跳过链接形式的 .git 目录"})
			return nil
		}
		found, err := FindRootWithGit(ctx, parent, gitExecutable)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			result.Issues = append(result.Issues, DiscoveryIssue{Path: parent, Message: err.Error()})
		} else if found == parent || (runtime.GOOS == "windows" && strings.EqualFold(found, parent)) {
			result.Roots = append(result.Roots, found)
		} else {
			result.Issues = append(result.Issues, DiscoveryIssue{Path: parent, Message: ".git 不属于有效的独立工作区"})
		}
		if entry.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	return result, err
}
