package preferences

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRecordsAndLoadsLastSaveDirectory(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "第二次 保存")
	for _, directory := range []string{first, second} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	store := NewStoreAt(filepath.Join(root, "config", preferencesFilename))

	if err := store.RecordSaveTarget(filepath.Join(first, "原文件.xlsx")); err != nil {
		t.Fatal(err)
	}
	if got := store.SaveDirectory(); got != first {
		t.Fatalf("first save directory = %q, want %q", got, first)
	}
	if err := store.RecordSaveTarget(filepath.Join(second, "另存.xlsx")); err != nil {
		t.Fatal(err)
	}
	if got := NewStoreAt(store.path).SaveDirectory(); got != second {
		t.Fatalf("persisted save directory = %q, want %q", got, second)
	}
}

func TestStoreFallsBackWhenPreferenceIsInvalid(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, preferencesFilename)
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := NewStoreAt(path).SaveDirectory()
	if got == "" {
		t.Fatal("expected Downloads or home directory fallback")
	}
	if info, err := os.Stat(got); err != nil || !info.IsDir() {
		t.Fatalf("fallback directory %q is not usable: %v", got, err)
	}
}

func TestStoreDoesNotRecordMissingDirectory(t *testing.T) {
	root := t.TempDir()
	store := NewStoreAt(filepath.Join(root, "config", preferencesFilename))
	err := store.RecordSaveTarget(filepath.Join(root, "missing", "file.xlsx"))
	if err == nil {
		t.Fatal("expected missing directory error")
	}
	if _, statErr := os.Stat(store.path); !os.IsNotExist(statErr) {
		t.Fatalf("preference file unexpectedly created: %v", statErr)
	}
}

func TestStoreRemembersRepositoryAndSidebarWidthWithoutLosingSaveDirectory(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, "中文 仓库")
	saveDirectory := filepath.Join(root, "saved")
	for _, directory := range []string{repository, saveDirectory} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	store := NewStoreAt(filepath.Join(root, "config", preferencesFilename))
	if err := store.RecordSaveTarget(filepath.Join(saveDirectory, "book.xlsx")); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordRepository(repository); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordRepositoryWidth(336); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordRepositoryRef(repository, "refs/remotes/origin/develop"); err != nil {
		t.Fatal(err)
	}
	indexFiles := []string{"config/a.xlsx", "中文/b.xlsx"}
	if err := store.RecordRepositoryIndex(
		repository, "refs/remotes/origin/develop", "signature-1", indexFiles,
	); err != nil {
		t.Fatal(err)
	}
	reopened := NewStoreAt(store.path)
	if got := reopened.LastRepository(); got != repository {
		t.Fatalf("last repository = %q, want %q", got, repository)
	}
	if got := reopened.RepositoryWidth(); got != 336 {
		t.Fatalf("repository width = %d", got)
	}
	if got := reopened.SaveDirectory(); got != saveDirectory {
		t.Fatalf("save directory was lost: %q", got)
	}
	if got := reopened.RepositoryRef(repository); got != "refs/remotes/origin/develop" {
		t.Fatalf("repository ref = %q", got)
	}
	if got, exists := reopened.RepositoryIndex(
		repository, "refs/remotes/origin/develop", "signature-1",
	); !exists || len(got) != 2 || got[1] != indexFiles[1] {
		t.Fatalf("repository index = %#v, exists=%t", got, exists)
	}
	if _, exists := reopened.RepositoryIndex(
		repository, "refs/remotes/origin/develop", "stale-signature",
	); exists {
		t.Fatal("stale repository index was reused")
	}
	otherRepository := filepath.Join(root, "other")
	if err := os.Mkdir(otherRepository, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := reopened.RepositoryRef(otherRepository); got != "" {
		t.Fatalf("unrelated repository ref = %q", got)
	}
}

func TestStoreKeepsTenRecentRepositoriesInMostRecentOrder(t *testing.T) {
	root := t.TempDir()
	store := NewStoreAt(filepath.Join(root, "config", preferencesFilename))
	repositories := make([]string, 12)
	for index := range repositories {
		repositories[index] = filepath.Join(root, fmt.Sprintf("repo-%02d", index))
		if err := os.Mkdir(repositories[index], 0o755); err != nil {
			t.Fatal(err)
		}
		if err := store.RecordRepository(repositories[index]); err != nil {
			t.Fatal(err)
		}
	}

	recent := NewStoreAt(store.path).RecentRepositories()
	if len(recent) != 10 {
		t.Fatalf("recent repository count = %d, want 10: %#v", len(recent), recent)
	}
	if recent[0] != repositories[11] || recent[9] != repositories[2] {
		t.Fatalf("recent repository order = %#v", recent)
	}
	if err := store.RecordRepository(repositories[5]); err != nil {
		t.Fatal(err)
	}
	recent = store.RecentRepositories()
	if recent[0] != repositories[5] || len(recent) != 10 {
		t.Fatalf("reopened repository was not moved to the front: %#v", recent)
	}
	seen := 0
	for _, path := range recent {
		if path == repositories[5] {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("reopened repository appears %d times: %#v", seen, recent)
	}
}

func TestStoreMigratesLegacyLastRepositoryToRecentRepositories(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, "legacy")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	preferencePath := filepath.Join(root, preferencesFilename)
	data := []byte(fmt.Sprintf(`{"lastRepository":%q}`, repository))
	if err := os.WriteFile(preferencePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStoreAt(preferencePath)
	if got := store.RecentRepositories(); len(got) != 1 || got[0] != repository {
		t.Fatalf("legacy recent repositories = %#v", got)
	}
	if got := store.LastRepository(); got != repository {
		t.Fatalf("legacy last repository = %q", got)
	}
}

func TestStoreMigratesLegacyUGXLSXPreferencesWithoutLosingData(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, legacyConfigDirectory, preferencesFilename)
	currentPath := filepath.Join(root, currentConfigDirectory, preferencesFilename)
	repository := filepath.Join(root, "repository")
	saveDirectory := filepath.Join(root, "saved")
	for _, directory := range []string{filepath.Dir(legacyPath), repository, saveDirectory} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	legacyData := []byte(fmt.Sprintf(
		`{"lastSaveDirectory":%q,"lastRepository":%q,"recentRepositories":[%q],"repositoryWidth":344,"repositoryRefs":{%q:%q}}`,
		saveDirectory, repository, repository, repository, "refs/remotes/origin/release",
	))
	if err := os.WriteFile(legacyPath, legacyData, 0o600); err != nil {
		t.Fatal(err)
	}

	store := Store{path: currentPath, legacyPath: legacyPath}
	if got := store.SaveDirectory(); got != saveDirectory {
		t.Fatalf("migrated save directory = %q, want %q", got, saveDirectory)
	}
	if got := store.LastRepository(); got != repository {
		t.Fatalf("migrated recent repository = %q, want %q", got, repository)
	}
	if got := store.RepositoryWidth(); got != 344 {
		t.Fatalf("migrated repository width = %d, want 344", got)
	}
	if got := store.RepositoryRef(repository); got != "refs/remotes/origin/release" {
		t.Fatalf("migrated repository ref = %q", got)
	}
	if _, err := os.Stat(currentPath); err != nil {
		t.Fatalf("SheetProof preferences were not created: %v", err)
	}
	if got, err := os.ReadFile(legacyPath); err != nil || string(got) != string(legacyData) {
		t.Fatalf("legacy preferences changed: err=%v data=%q", err, got)
	}

	if err := store.RecordRepositoryWidth(360); err != nil {
		t.Fatal(err)
	}
	if got := NewStoreAt(currentPath).RepositoryWidth(); got != 360 {
		t.Fatalf("new preference path was not used for writes: %d", got)
	}
}
