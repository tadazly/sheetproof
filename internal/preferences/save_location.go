package preferences

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const preferencesFilename = "preferences.json"

type savePreferences struct {
	LastSaveDirectory string `json:"lastSaveDirectory"`
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
	if saved := s.load(); directoryExists(saved) {
		return saved
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
	data, err := json.MarshalIndent(savePreferences{LastSaveDirectory: directory}, "", "  ")
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

func (s Store) load() string {
	if s.path == "" {
		return ""
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return ""
	}
	var value savePreferences
	if json.Unmarshal(data, &value) != nil {
		return ""
	}
	return filepath.Clean(value.LastSaveDirectory)
}

func directoryExists(path string) bool {
	if path == "" || path == "." {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
