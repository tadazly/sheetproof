package preferences

import (
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
