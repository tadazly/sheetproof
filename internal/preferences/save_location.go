package preferences

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const preferencesFilename = "preferences.json"

type savePreferences struct {
	LastSaveDirectory string                                      `json:"lastSaveDirectory"`
	LastRepository    string                                      `json:"lastRepository,omitempty"`
	RepositoryWidth   int                                         `json:"repositoryWidth,omitempty"`
	RepositoryRefs    map[string]string                           `json:"repositoryRefs,omitempty"`
	RepositoryIndexes map[string]map[string]repositoryIndexRecord `json:"repositoryIndexes,omitempty"`
}

type repositoryIndexRecord struct {
	Signature string   `json:"signature"`
	Files     []string `json:"files"`
}

// Store persists non-sensitive GUI preferences and lightweight repository
// indexes for the current user.
type Store struct {
	path string
}

func NewStore() Store {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return Store{}
	}
	return Store{path: filepath.Join(configDir, "ugxlsx", preferencesFilename)}
}

// NewStoreAt is primarily useful for isolated tests and portable packaging.
func NewStoreAt(path string) Store {
	return Store{path: path}
}

// SaveDirectory returns the last successful Save As directory, or the platform
// Downloads directory on first use. Missing and corrupt preferences are ignored.
func (s Store) SaveDirectory() string {
	if saved := s.load().LastSaveDirectory; directoryExists(saved) {
		return filepath.Clean(saved)
	}
	if downloads := platformDownloadsDirectory(); directoryExists(downloads) {
		return downloads
	}
	home, err := os.UserHomeDir()
	if err == nil && directoryExists(home) {
		return home
	}
	return ""
}

// RecordSaveTarget records the parent directory only after a workbook was
// successfully saved.
func (s Store) RecordSaveTarget(target string) error {
	if s.path == "" {
		return errors.New("user configuration directory is unavailable")
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	directory := filepath.Dir(absolute)
	if !directoryExists(directory) {
		return os.ErrNotExist
	}
	value := s.load()
	value.LastSaveDirectory = directory
	return s.write(value)
}

// LastRepository returns the last successfully opened repository path. The
// caller intentionally receives missing paths too, so startup can report that
// the recent repository is no longer available.
func (s Store) LastRepository() string {
	value := s.load().LastRepository
	if value == "" {
		return ""
	}
	return filepath.Clean(value)
}

func (s Store) RecordRepository(root string) error {
	if s.path == "" {
		return errors.New("user configuration directory is unavailable")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if !directoryExists(absolute) {
		return os.ErrNotExist
	}
	value := s.load()
	cleaned := filepath.Clean(absolute)
	if value.LastRepository == cleaned {
		return nil
	}
	value.LastRepository = cleaned
	return s.write(value)
}

// RepositoryRef returns the last comparison ref selected for a repository.
func (s Store) RepositoryRef(root string) string {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	return s.load().RepositoryRefs[filepath.Clean(absolute)]
}

func (s Store) RecordRepositoryRef(root, ref string) error {
	if s.path == "" {
		return errors.New("user configuration directory is unavailable")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if !directoryExists(absolute) {
		return os.ErrNotExist
	}
	if ref == "" {
		return errors.New("repository comparison ref is empty")
	}
	value := s.load()
	if value.RepositoryRefs == nil {
		value.RepositoryRefs = make(map[string]string)
	}
	rootKey := filepath.Clean(absolute)
	if value.RepositoryRefs[rootKey] == ref {
		return nil
	}
	value.RepositoryRefs[rootKey] = ref
	return s.write(value)
}

func (s Store) RepositoryIndex(root, ref, signature string) ([]string, bool) {
	absolute, err := filepath.Abs(root)
	if err != nil || ref == "" || signature == "" {
		return nil, false
	}
	byRef := s.load().RepositoryIndexes[filepath.Clean(absolute)]
	record, exists := byRef[ref]
	if !exists || record.Signature != signature {
		return nil, false
	}
	return append([]string{}, record.Files...), true
}

func (s Store) RecordRepositoryIndex(root, ref, signature string, files []string) error {
	if s.path == "" {
		return errors.New("user configuration directory is unavailable")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if !directoryExists(absolute) {
		return os.ErrNotExist
	}
	if ref == "" || signature == "" {
		return errors.New("repository index ref and signature are required")
	}
	value := s.load()
	if value.RepositoryIndexes == nil {
		value.RepositoryIndexes = make(map[string]map[string]repositoryIndexRecord)
	}
	rootKey := filepath.Clean(absolute)
	if value.RepositoryIndexes[rootKey] == nil {
		value.RepositoryIndexes[rootKey] = make(map[string]repositoryIndexRecord)
	}
	value.RepositoryIndexes[rootKey][ref] = repositoryIndexRecord{
		Signature: signature,
		Files:     append([]string{}, files...),
	}
	return s.write(value)
}

func (s Store) RepositoryWidth() int {
	width := s.load().RepositoryWidth
	if width < 180 || width > 520 {
		return 280
	}
	return width
}

func (s Store) RecordRepositoryWidth(width int) error {
	if width < 180 || width > 520 {
		return fmt.Errorf("repository sidebar width must be between 180 and 520")
	}
	value := s.load()
	value.RepositoryWidth = width
	return s.write(value)
}

func (s Store) write(value savePreferences) error {
	if s.path == "" {
		return errors.New("user configuration directory is unavailable")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), ".preferences-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, s.path); err == nil {
		return nil
	}
	// Windows does not replace an existing destination with os.Rename. The
	// preference is non-critical, so fall back to a direct small-file update.
	return os.WriteFile(s.path, data, 0o600)
}

func (s Store) load() savePreferences {
	if s.path == "" {
		return savePreferences{}
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return savePreferences{}
	}
	var value savePreferences
	if json.Unmarshal(data, &value) != nil {
		return savePreferences{}
	}
	return value
}

func directoryExists(path string) bool {
	if path == "" || path == "." {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
