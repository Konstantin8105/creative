package webserver

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/Konstantin8105/creative"
)

// Tab represents a single chat tab within a session.
type Tab struct {
	ID          string
	Mode        string
	Label       string
	Chat        *creative.Chat
	BooksFolder string
	CreatedAt   time.Time
}

// Session represents a user session containing multiple independent tabs.
type Session struct {
	Tabs         map[string]*Tab
	CreatedAt    time.Time
	LastActivity time.Time
}

// TabInfo is a serializable summary of a tab for the API.
type TabInfo struct {
	ID    string `json:"id"`
	Mode  string `json:"mode"`
	Label string `json:"label"`
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

// CreateTab creates a new tab in the given session for the given mode.
// The session is created if it doesn't exist — entire operation is atomic.
func (sm *SessionManager) CreateTab(sessionID, modeName string) (tabID string, err error) {
	// Find the mode config (no lock needed — cfg is read-only after creation)
	var modeCfg *creative.ModeConfig
	for i := range sm.cfg.Modes {
		if sm.cfg.Modes[i].Name == modeName {
			modeCfg = &sm.cfg.Modes[i]
			break
		}
	}
	if modeCfg == nil {
		return "", fmt.Errorf("mode %q not found", modeName)
	}

	// Resolve prompt and create chat (I/O — no lock needed)
	prompt := modeCfg.GetPrompt()
	prvAI := creative.NewRouterAI(sm.cfg.Provider)
	ch := creative.NewChat(prvAI)
	ch.AddSystem(prompt)

	ch.SetTools(creative.BookTools(modeCfg.BooksFolder))

	tabID = generateID()
	tab := &Tab{
		ID:          tabID,
		Mode:        modeName,
		Label:       modeCfg.Label,
		Chat:        ch,
		BooksFolder: modeCfg.BooksFolder,
		CreatedAt:   time.Now(),
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
			Mode:  tab.Mode,
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
		if now.Sub(s.LastActivity) > sm.ttl || now.Sub(s.CreatedAt) > 24*time.Hour {
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
