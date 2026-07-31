//go:build windows

package ugit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindGitExecutablePrefersVersionedUGitBundleOverPath(t *testing.T) {
	localAppData := t.TempDir()
	older := filepath.Join(localAppData, "UGit", "app-5.9.0", "resources", "app", "git", "cmd", "git.exe")
	latest := filepath.Join(localAppData, "UGit", "app-5.51.0", "resources", "app", "git", "cmd", "git.exe")
	latestMingw := filepath.Join(localAppData, "UGit", "app-5.51.0", "resources", "app", "git", "mingw64", "bin", "git.exe")
	for _, path := range []string{older, latest, latestMingw} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	pathGit := filepath.Join(t.TempDir(), "git.exe")
	if err := os.WriteFile(pathGit, []byte("path git"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := findGitExecutableFor(
		"windows",
		func(key string) string {
			if key == "LOCALAPPDATA" {
				return localAppData
			}
			return ""
		},
		func(string) (string, error) { return pathGit, nil },
		os.Stat,
		filepath.Glob,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != latest {
		t.Fatalf("Git path = %q, want latest UGit bundle %q", got, latest)
	}
}

func TestFindGitExecutableFallsBackToPathWhenUGitIsMissing(t *testing.T) {
	pathGit := filepath.Join(t.TempDir(), "git.exe")
	got, err := findGitExecutableFor(
		"windows",
		func(string) string { return t.TempDir() },
		func(string) (string, error) { return pathGit, nil },
		func(string) (os.FileInfo, error) { return nil, errors.New("missing") },
		func(string) ([]string, error) { return nil, nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != pathGit {
		t.Fatalf("Git path = %q, want PATH Git %q", got, pathGit)
	}
}
