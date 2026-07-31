package preferences

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	preferencesFilename      = "preferences.json"
	maxRecentRepositoryCount = 10
)

type savePreferences struct {
	LastSaveDirectory  string                                      `json:"lastSaveDirectory"`
	LastRepository     string                                      `json:"lastRepository,omitempty"`
	RecentRepositories []string                                    `json:"recentRepositories,omitempty"`
	RepositoryWidth    int                                         `json:"repositoryWidth,omitempty"`
	RepositoryRefs     map[string]string                           `json:"repositoryRefs,omitempty"`
	RepositoryIndexes  map[string]map[string]repositoryIndexRecord `json:"repositoryIndexes,omitempty"`
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
	recent := s.RecentRepositories()
	if len(recent) > 0 {
		return recent[0]
	}
	return ""
}

// RecentRepositories returns up to ten repositories in most-recently-opened
// order. The legacy lastRepository value is retained as a migration fallback.
func (s Store) RecentRepositories() []string {
	value := s.load()
	paths := value.RecentRepositories
	if len(paths) == 0 && value.LastRepository != "" {
		paths = []string{value.LastRepository}
	}
	result := make([]string, 0, min(len(paths), maxRecentRepositoryCount))
	for _, path := range paths {
		if path == "" {
			continue
		}
		cleaned := filepath.Clean(path)
		duplicate := false
		for _, existing := range result {
			if repositoryPathsEqual(existing, cleaned) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, cleaned)
		}
		if len(result) == maxRecentRepositoryCount {
			break
		}
	}
	return result
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
	recent := make([]string, 0, maxRecentRepositoryCount)
	recent = append(recent, cleaned)
	for _, path := range value.RecentRepositories {
		path = filepath.Clean(path)
		if path == "." || repositoryPathsEqual(path, cleaned) {
			continue
		}
		recent = append(recent, path)
		if len(recent) == maxRecentRepositoryCount {
			break
		}
	}
	if len(value.RecentRepositories) == 0 && value.LastRepository != "" &&
		!repositoryPathsEqual(value.LastRepository, cleaned) {
		recent = append(recent, filepath.Clean(value.LastRepository))
	}
	if len(recent) > maxRecentRepositoryCount {
		recent = recent[:maxRecentRepositoryCount]
	}
	if value.LastRepository == cleaned && slicesEqualPaths(value.RecentRepositories, recent) {
		return nil
	}
	value.LastRepository = cleaned
	value.RecentRepositories = recent
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

func repositoryPathsEqual(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func slicesEqualPaths(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !repositoryPathsEqual(left[index], right[index]) {
			return false
		}
	}
	return true
}
