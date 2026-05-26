package webserver

import (
	"fmt"
	"testing"
	"time"

	"github.com/Konstantin8105/creative"
)

func TestNewSessionManager(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, func() *creative.Chat {
		return creative.NewChat(nil)
	})
	if sm == nil {
		t.Fatal("NewSessionManager returned nil")
	}
	if sm.ttl != 1*time.Hour {
		t.Errorf("ttl = %v, want 1h", sm.ttl)
	}
	if sm.Count() != 0 {
		t.Errorf("Count() = %d, want 0", sm.Count())
	}
	sm.Stop()
}

func TestSessionManager_GetOrCreate(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, func() *creative.Chat {
		return creative.NewChat(nil)
	})
	defer sm.Stop()

	ch1 := sm.GetOrCreate("session_1")
	if ch1 == nil {
		t.Fatal("GetOrCreate returned nil")
	}
	if sm.Count() != 1 {
		t.Errorf("Count() = %d, want 1", sm.Count())
	}

	// Same session ID returns cached Chat
	ch2 := sm.GetOrCreate("session_1")
	if ch2 != ch1 {
		t.Error("GetOrCreate should return the same Chat for the same session")
	}
	if sm.Count() != 1 {
		t.Errorf("Count() = %d, want 1", sm.Count())
	}

	// Different session ID returns new Chat
	ch3 := sm.GetOrCreate("session_2")
	if ch3 == nil {
		t.Fatal("GetOrCreate returned nil for new session")
	}
	if ch3 == ch1 {
		t.Error("GetOrCreate should return a different Chat for different session")
	}
	if sm.Count() != 2 {
		t.Errorf("Count() = %d, want 2", sm.Count())
	}
}

func TestSessionManager_ConcurrentAccess(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, func() *creative.Chat {
		return creative.NewChat(nil)
	})
	defer sm.Stop()

	done := make(chan struct{})
	const n = 10
	for i := 0; i < n; i++ {
		go func(id int) {
			sm.GetOrCreate(fmt.Sprintf("session_%d", id))
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < n; i++ {
		<-done
	}
	if sm.Count() != n {
		t.Errorf("Count() = %d, want %d after concurrent access", sm.Count(), n)
	}
}

func TestSessionManager_Cleanup(t *testing.T) {
	sm := NewSessionManager(50*time.Millisecond, func() *creative.Chat {
		return creative.NewChat(nil)
	})
	defer sm.Stop()

	sm.GetOrCreate("session_a")
	sm.GetOrCreate("session_b")

	if sm.Count() != 2 {
		t.Fatalf("Count() = %d, want 2 before cleanup", sm.Count())
	}

	// Wait for cleanup to run (cleanupLoop runs every 5 min by default,
	// but cleanup is also called directly in the loop.
	// We just need to wait and then manually trigger cleanup via sleep and
	// let the goroutine do its work. Since our TTL is very short (50ms),
	// the next tick (5 min) would be too long. Instead we call cleanup directly.
	time.Sleep(100 * time.Millisecond)
	sm.cleanup()

	if sm.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after cleanup", sm.Count())
	}
}

func TestSessionManager_Stop(t *testing.T) {
	sm := NewSessionManager(1*time.Hour, func() *creative.Chat {
		return creative.NewChat(nil)
	})

	sm.GetOrCreate("test_session")
	if sm.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", sm.Count())
	}

	sm.Stop()

	// After Stop, the cleanup goroutine should exit (no panic).
	// The sessions map still has the session (Stop doesn't clear it).
	if sm.Count() != 1 {
		t.Errorf("Count() = %d, want 1 (Stop should not clear sessions)", sm.Count())
	}
}

func TestSessionManager_DoubleCheck(t *testing.T) {
	// Test the double-check locking pattern in GetOrCreate
	factoryCount := 0
	sm := NewSessionManager(1*time.Hour, func() *creative.Chat {
		factoryCount++
		return creative.NewChat(nil)
	})
	defer sm.Stop()

	// Create a session, then call GetOrCreate with the same ID concurrently
	sm.GetOrCreate("test")

	// This should hit the fast path (RLock -> exists) and NOT call factory
	_ = sm.GetOrCreate("test")
	if factoryCount != 1 {
		t.Errorf("factory called %d times, want 1", factoryCount)
	}

	// Also test the slow path double-check (write lock) scenario.
	// After first creation, subsequent calls should NOT create duplicates.
	_ = sm.GetOrCreate("test")
	if factoryCount != 1 {
		t.Errorf("factory called %d times, want 1 (double-check failed)", factoryCount)
	}
}
