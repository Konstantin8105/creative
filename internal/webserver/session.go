package webserver

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Konstantin8105/creative"
)

// Tab represents a single chat tab within a session.
type Tab struct {
	ID        string
	Modes     []string
	Label     string
	Chat      *creative.Chat
	Folders   []string
	TempDir   string
	CreatedAt time.Time
}

// PrimaryMode returns the first mode name.
func (t *Tab) PrimaryMode() string {
	if len(t.Modes) > 0 {
		return t.Modes[0]
	}
	return ""
}

// IsCombo returns true if this tab combines multiple modes.
func (t *Tab) IsCombo() bool {
	return len(t.Modes) > 1
}

// Session represents a user session containing multiple independent tabs.
type Session struct {
	Tabs         map[string]*Tab
	CreatedAt    time.Time
	LastActivity time.Time
}

// TabInfo is a serializable summary of a tab for the API.
type TabInfo struct {
	ID    string   `json:"id"`
	Modes []string `json:"modes"`
	Label string   `json:"label"`
}

// SessionManager manages user sessions with multi-tab support and TTL-based cleanup.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	cfg      *creative.Config
	ttl      time.Duration
	stopCh   chan struct{}
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(cfg *creative.Config, ttl time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		cfg:      cfg,
		ttl:      ttl,
		stopCh:   make(chan struct{}),
	}
	go sm.cleanupLoop()
	return sm
}

// CreateTab creates a new single-mode tab (backward-compatible wrapper).
func (sm *SessionManager) CreateTab(sessionID, modeName string) (tabID string, err error) {
	return sm.createTabWithModes(sessionID, []string{modeName})
}

// CreateComboTab creates a new tab combining multiple modes.
// The prompt is taken from the first mode; book folders are merged from all modes.
func (sm *SessionManager) CreateComboTab(sessionID string, modeNames []string) (tabID string, err error) {
	return sm.createTabWithModes(sessionID, modeNames)
}

// createTabWithModes creates a tab from one or more mode names.
func (sm *SessionManager) createTabWithModes(sessionID string, modeNames []string) (tabID string, err error) {
	if len(modeNames) == 0 {
		return "", fmt.Errorf("at least one mode is required")
	}

	// Find all mode configs (no lock needed — cfg is read-only after creation)
	modeCfgs := make([]*creative.ModeConfig, 0, len(modeNames))
	for _, name := range modeNames {
		var found *creative.ModeConfig
		for i := range sm.cfg.Modes {
			if sm.cfg.Modes[i].Name == name {
				found = &sm.cfg.Modes[i]
				break
			}
		}
		if found == nil {
			return "", fmt.Errorf("mode %q not found", name)
		}
		modeCfgs = append(modeCfgs, found)
	}

	// First mode provides the prompt
	primary := modeCfgs[0]
	prompt := primary.GetPrompt()
	prvAI := creative.NewRouterAI(sm.cfg.Provider)
	ch := creative.NewChat(prvAI)
	ch.AddSystem(prompt)

	// Merge folders from all modes (dedup)
	folderSet := make(map[string]struct{})
	for _, mc := range modeCfgs {
		for _, f := range mc.Folders {
			folderSet[f] = struct{}{}
		}
	}
	mergedFolders := make([]string, 0, len(folderSet))
	for f := range folderSet {
		mergedFolders = append(mergedFolders, f)
	}

	tabID = generateID()

	// Compute temp directory path for list files (not created yet — lazy)
	tempDir := filepath.Join(os.TempDir(), "creative-lists", tabID)

	// Add temp dir to book folders so BookTools can see list files
	mergedFolders = append(mergedFolders, tempDir)

	allTools := append(
		creative.BookTools(mergedFolders...),
		creative.ListAddTool(tempDir)...,
	)
	ch.SetTools(allTools)

	// Generate label
	labels := make([]string, len(modeCfgs))
	for i, mc := range modeCfgs {
		labels[i] = mc.Label
	}
	label := buildComboLabel(labels)

	modeNamesCopy := make([]string, len(modeNames))
	copy(modeNamesCopy, modeNames)
	tab := &Tab{
		ID:        tabID,
		Modes:     modeNamesCopy,
		Label:     label,
		Chat:      ch,
		Folders:   mergedFolders,
		TempDir:   tempDir,
		CreatedAt: time.Now(),
	}

	// One lock acquisition for session lookup/create + tab insertion
	sm.mu.Lock()
	sess, ok := sm.sessions[sessionID]
	if !ok {
		sess = &Session{
			Tabs:         make(map[string]*Tab),
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
		}
		sm.sessions[sessionID] = sess
	}
	sess.LastActivity = time.Now()
	sess.Tabs[tabID] = tab
	sm.mu.Unlock()

	return tabID, nil
}

// buildComboLabel joins mode labels with " | " separator.
// If the result exceeds 45 characters, it is truncated with "…".
func buildComboLabel(labels []string) string {
	if len(labels) == 1 {
		return labels[0]
	}
	label := strings.Join(labels, " | ")
	runes := []rune(label)
	if len(runes) > 45 {
		runes = append(runes[:42], '…')
		label = string(runes)
	}
	return label
}

// CloseTab closes a tab. Returns the sessionID for cleanup tracking.
func (sm *SessionManager) CloseTab(sessionID, tabID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess, ok := sm.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}

	if _, ok := sess.Tabs[tabID]; !ok {
		return fmt.Errorf("tab not found")
	}

	delete(sess.Tabs, tabID)
	sess.LastActivity = time.Now()

	// If no tabs left, delete the session
	if len(sess.Tabs) == 0 {
		delete(sm.sessions, sessionID)
		log.Printf("[session] deleted: %s (no tabs remaining)", sessionID[:min(len(sessionID), 8)])
	}

	return nil
}

// ListTabs returns a list of tab info for the given session.
func (sm *SessionManager) ListTabs(sessionID string) ([]TabInfo, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess, ok := sm.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	sess.LastActivity = time.Now()

	infos := make([]TabInfo, 0, len(sess.Tabs))
	for _, tab := range sess.Tabs {
		infos = append(infos, TabInfo{
			ID:    tab.ID,
			Modes: tab.Modes,
			Label: tab.Label,
		})
	}
	return infos, nil
}

// GetChat returns the chat for a specific tab in a session.
// Before returning, it sets creative.BooksFolder for the tab's book tools.
func (sm *SessionManager) GetChat(sessionID, tabID string) (*creative.Chat, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess, ok := sm.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	tab, ok := sess.Tabs[tabID]
	if !ok {
		return nil, fmt.Errorf("tab not found")
	}

	sess.LastActivity = time.Now()

	return tab.Chat, nil
}

// CloseSession immediately removes a session.
func (sm *SessionManager) CloseSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	log.Printf("[session] CloseSession: %s", sessionID)
	delete(sm.sessions, sessionID)
}

// Heartbeat updates the session's LastActivity timestamp.
func (sm *SessionManager) Heartbeat(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.sessions[sessionID]; ok {
		s.LastActivity = time.Now()
	}
}

func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			sm.cleanup()
		case <-sm.stopCh:
			return
		}
	}
}

func (sm *SessionManager) cleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for id, s := range sm.sessions {
		if now.Sub(s.LastActivity) > sm.ttl {
			delete(sm.sessions, id)
			log.Printf("[session] expired: %s (age: %v, inactive: %v)",
				id[:min(len(id), 8)], now.Sub(s.CreatedAt), now.Sub(s.LastActivity))
		}
	}
}

// Stop stops the cleanup goroutine.
func (sm *SessionManager) Stop() {
	close(sm.stopCh)
}

func generateID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
