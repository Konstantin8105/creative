package webserver

import (
	"log"
	"sync"
	"time"

	"github.com/Konstantin8105/creative"
)

// session represents a single user chat session.
type session struct {
	chat      *creative.Chat
	createdAt time.Time
}

// SessionManager manages user sessions with a TTL-based cleanup.
// Sessions expire after the configured TTL and are removed by a periodic cleanup goroutine.
// Each session gets its own *creative.Chat instance, created via the factory function.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*session
	ttl      time.Duration
	stopCh   chan struct{}
	factory  func() *creative.Chat
}

// NewSessionManager creates a new SessionManager.
// ttl controls how long a session lives after creation.
// factory is called to create a new *creative.Chat for each new session.
func NewSessionManager(ttl time.Duration, factory func() *creative.Chat) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*session),
		ttl:      ttl,
		stopCh:   make(chan struct{}),
		factory:  factory,
	}
	go sm.cleanupLoop()
	return sm
}

// GetOrCreate returns the Chat for an existing session, or creates a new one.
func (sm *SessionManager) GetOrCreate(id string) *creative.Chat {
	// Fast path: check existing
	sm.mu.RLock()
	s, ok := sm.sessions[id]
	sm.mu.RUnlock()
	if ok {
		return s.chat
	}

	// Slow path: create new session
	ch := sm.factory()
	sm.mu.Lock()
	// Double-check after acquiring write lock
	if s, ok := sm.sessions[id]; ok {
		sm.mu.Unlock()
		return s.chat
	}
	sm.sessions[id] = &session{
		chat:      ch,
		createdAt: time.Now(),
	}
	sm.mu.Unlock()
	return ch
}

// cleanupLoop periodically removes expired sessions.
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
		if now.Sub(s.createdAt) > sm.ttl {
			delete(sm.sessions, id)
			log.Printf("[session] expired and removed: %s", id[:min(len(id), 8)])
		}
	}
}

// Stop stops the cleanup goroutine.
func (sm *SessionManager) Stop() {
	close(sm.stopCh)
}

// Count returns the number of active sessions.
func (sm *SessionManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}
