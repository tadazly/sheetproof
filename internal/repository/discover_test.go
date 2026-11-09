package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDiscoverNestedRepositoriesAndGitFiles(t *testing.T) {
	directory := t.TempDir()
	outer := filepath.Join(directory, "outer")
	nested := filepath.Join(outer, "nested")
	separate := filepath.Join(directory, "separate")
	for _, root := range []string{outer, nested, separate} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		args := []string{"init", "--quiet"}
		if root == separate {
			args = append(args, "--separate-git-dir", filepath.Join(t.TempDir(), "metadata"))
		}
		if _, err := runGit(root, nil, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Nested metadata is never searched; invalid worktree markers are reported.
	for _, marker := range []string{filepath.Join(outer, ".git", "hidden", ".git"), filepath.Join(directory, "broken", ".git")} {
		if err := os.MkdirAll(marker, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	outside := t.TempDir()
	if _, err := runGit(outside, nil, "init", "--quiet"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "linked")); err != nil {
		t.Logf("directory link fixture unavailable: %v", err)
	}
	result, err := Discover(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{outer, nested, separate}
	slices.Sort(want)
	slices.Sort(result.Roots)
	if !slices.Equal(want, result.Roots) || len(result.Issues) != 1 {
		t.Fatalf("discover = %+v, want roots %v and one invalid marker", result, want)
	}
	if _, err := Discover(context.Background(), filepath.Join(outer, ".git")); err == nil {
		t.Fatal("Git metadata accepted as scan root")
	}
}

func TestDiscoverCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Discover(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled scan = %v", err)
	}
}
