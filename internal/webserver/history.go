package webserver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Konstantin8105/creative"
)

// HistoryWriter writes chat message histories to JSON files.
type HistoryWriter struct {
	mu       sync.Mutex
	filePath string
	enabled  bool
}

// sanitizeFilenamePart replaces any character not in [a-zA-Z0-9._-] with '_'.
// Guarantees a valid filename component on both Windows and Linux.
func sanitizeFilenamePart(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
}

// NewHistoryWriter creates a new HistoryWriter.
// The file is written to "history/{safeIP}.{safeSession}.{safeTab}.json".
// When enabled is false, Save is a no-op.
func NewHistoryWriter(ip, sessionID, tabID string, enabled bool) *HistoryWriter {
	dir := "history"
	safeIP := sanitizeFilenamePart(ip)
	safeSess := sanitizeFilenamePart(sessionID)
	safeTab := sanitizeFilenamePart(tabID)
	filename := fmt.Sprintf("%s.%s.%s.json", safeIP, safeSess, safeTab)
	return &HistoryWriter{
		filePath: filepath.Join(dir, filename),
		enabled:  enabled,
	}
}

// Save writes the message history to a JSON file with indentation.
// If the writer is disabled, it is a no-op.
func (hw *HistoryWriter) Save(messages []creative.ChatMessage) error {
	if !hw.enabled {
		return nil
	}
	hw.mu.Lock()
	defer hw.mu.Unlock()
	if err := os.MkdirAll("history", 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(hw.filePath, data, 0644)
}

// FilePath returns the target file path. Used for logging.
func (hw *HistoryWriter) FilePath() string {
	return hw.filePath
}
