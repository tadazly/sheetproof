package diagnostics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

const (
	defaultMaxBytes = int64(2 * 1024 * 1024)
	maxMessageRunes = 2000
)

var invalidEventName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type entry struct {
	Timestamp string         `json:"timestamp"`
	Session   string         `json:"session"`
	Level     string         `json:"level"`
	Event     string         `json:"event"`
	Message   string         `json:"message,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// Logger writes bounded JSONL diagnostics and implements the Wails logger interface.
type Logger struct {
	mu       sync.Mutex
	path     string
	session  string
	maxBytes int64
	now      func() time.Time
	discard  bool
}

func DefaultPath() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate user cache: %w", err)
	}
	return filepath.Join(root, "SheetProof", "logs", "sheetproof.log"), nil
}

func NewDefault() *Logger {
	path, _ := DefaultPath()
	return New(path, defaultMaxBytes)
}

func New(path string, maxBytes int64) *Logger {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	now := time.Now
	return &Logger{
		path:     path,
		session:  fmt.Sprintf("%s-%d", now().UTC().Format("20060102T150405.000000000Z"), os.Getpid()),
		maxBytes: maxBytes,
		now:      now,
	}
}

// NewDiscard returns a logger that intentionally drops every event. It is used
// by Wails' binding helper so build-time launches cannot pollute the user's
// runtime diagnostic log.
func NewDiscard() *Logger {
	return &Logger{discard: true}
}

func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *Logger) Session() string {
	if l == nil {
		return ""
	}
	return l.session
}

func (l *Logger) Record(event string, fields map[string]any) {
	l.write("INFO", event, "", fields)
}

func (l *Logger) Clear() {
	if l == nil || l.path == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = os.Remove(l.path)
	_ = os.Remove(l.path + ".1")
}

func (l *Logger) Print(message string)   { l.write("PRINT", "wails.print", message, nil) }
func (l *Logger) Trace(message string)   { l.write("TRACE", "wails.trace", message, nil) }
func (l *Logger) Debug(message string)   { l.write("DEBUG", "wails.debug", message, nil) }
func (l *Logger) Info(message string)    { l.write("INFO", "wails.info", message, nil) }
func (l *Logger) Warning(message string) { l.write("WARN", "wails.warning", message, nil) }
func (l *Logger) Error(message string)   { l.write("ERROR", "wails.error", message, nil) }

func (l *Logger) Fatal(message string) {
	l.write("FATAL", "wails.fatal", message, nil)
	os.Exit(1)
}

func (l *Logger) write(level, eventName, message string, fields map[string]any) {
	if l == nil || l.discard {
		return
	}
	item := entry{
		Timestamp: l.now().UTC().Format(time.RFC3339Nano),
		Session:   l.session,
		Level:     level,
		Event:     normalizeEventName(eventName),
		Message:   truncate(message, maxMessageRunes),
		Fields:    fields,
	}
	data, err := json.Marshal(item)
	if err != nil {
		return
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.path == "" {
		_, _ = os.Stderr.Write(data)
		return
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		_, _ = os.Stderr.Write(data)
		return
	}
	l.rotateIfNeeded(int64(len(data)))
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		_, _ = os.Stderr.Write(data)
		return
	}
	_, _ = file.Write(data)
	_ = file.Close()
}

func (l *Logger) rotateIfNeeded(incoming int64) {
	info, err := os.Stat(l.path)
	if err != nil || info.Size()+incoming <= l.maxBytes {
		return
	}
	backup := l.path + ".1"
	_ = os.Remove(backup)
	if err := os.Rename(l.path, backup); err != nil {
		_ = os.Truncate(l.path, 0)
	}
}

func normalizeEventName(value string) string {
	value = invalidEventName.ReplaceAllString(value, "_")
	value = truncate(value, 80)
	if value == "" {
		return "unknown"
	}
	return value
}

func truncate(value string, maximum int) string {
	characters := []rune(value)
	if len(characters) <= maximum {
		return value
	}
	return string(characters[:maximum])
}
