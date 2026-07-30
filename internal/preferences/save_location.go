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
	LastSaveDirectory string `json:"lastSaveDirectory"`
	LastRepository    string `json:"lastRepository,omitempty"`
	RepositoryWidth   int    `json:"repositoryWidth,omitempty"`
}

// Store persists small, non-sensitive GUI preferences for the current user.
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
	value.LastRepository = filepath.Clean(absolute)
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
