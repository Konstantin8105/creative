package webserver

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Konstantin8105/creative"
)

func newTestConfig(t *testing.T) *creative.Config {
	t.Helper()

	dir := t.TempDir()

	// Create a prompt file
	promtPath := filepath.Join(dir, "test.promt")
	if err := os.WriteFile(promtPath, []byte("You are a test assistant."), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a second prompt file
	promtPath2 := filepath.Join(dir, "test2.promt")
	if err := os.WriteFile(promtPath2, []byte("You are another test assistant."), 0644); err != nil {
		t.Fatal(err)
	}

	// Create books folders
	booksDir1 := filepath.Join(dir, "books1")
	if err := os.MkdirAll(booksDir1, 0755); err != nil {
		t.Fatal(err)
	}
	booksDir2 := filepath.Join(dir, "books2")
	if err := os.MkdirAll(booksDir2, 0755); err != nil {
		t.Fatal(err)
	}

	// Use absolute paths for prompt files
	absPrompt1 := filepath.ToSlash(promtPath)
	absPrompt2 := filepath.ToSlash(promtPath2)
	absBooks1 := filepath.ToSlash(booksDir1)
	absBooks2 := filepath.ToSlash(booksDir2)

	jsonPath := filepath.Join(dir, "config.json")
	jsonContent := `{
		"provider": {
			"endpoint": "http://localhost:11434/v1/",
			"model": "test-model",
			"context_size": 4096,
			"timeout": "30s"
		},
		"modes": [
			{
				"name": "test",
				"label": "Test Mode",
				"prompt_file": "` + absPrompt1 + `",
				"books_folder": ["` + absBooks1 + `"]
			},
			{
				"name": "test2",
				"label": "Test Mode 2",
				"prompt_file": "` + absPrompt2 + `",
				"books_folder": ["` + absBooks2 + `"]
			}
		]
	}`
	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := creative.LoadConfig(jsonPath)
	if err != nil {
		t.Fatal(err)
	}

	return cfg
}

func TestNewSessionManager(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	if sm == nil {
		t.Fatal("NewSessionManager returned nil")
	}
	sm.Stop()
}

func TestTabCreateAndList(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	// Create tabs in different sessions
	tab1, err := sm.CreateTab("session_1", "test", "")
	if err != nil {
		t.Fatal(err)
	}
	if tab1 == "" {
		t.Fatal("expected non-empty tab ID")
	}

	// Second tab in same session
	tab2, err := sm.CreateTab("session_1", "test", "")
	if err != nil {
		t.Fatal(err)
	}
	if tab2 == tab1 {
		t.Error("expected different tab IDs for different tabs")
	}

	// Tab in different session
	tab3, err := sm.CreateTab("session_2", "test", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = tab3

	// List tabs per session
	tabs1, err := sm.ListTabs("session_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs1) != 2 {
		t.Fatalf("expected 2 tabs in session_1, got %d", len(tabs1))
	}

	tabs2, err := sm.ListTabs("session_2")
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs2) != 1 {
		t.Fatalf("expected 1 tab in session_2, got %d", len(tabs2))
	}
}

func TestListTabs_SessionNotFound(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	_, err := sm.ListTabs("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestCreateTab(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	tabID, err := sm.CreateTab("session_a", "test", "")
	if err != nil {
		t.Fatal(err)
	}
	if tabID == "" {
		t.Fatal("expected non-empty tab ID")
	}

	// Tab should be listable
	tabs, err := sm.ListTabs("session_a")
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabs))
	}
	if len(tabs[0].Modes) != 1 || tabs[0].Modes[0] != "test" {
		t.Errorf("tab modes = %v, want [test]", tabs[0].Modes)
	}
}

func TestCreateTab_UnknownMode(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	_, err := sm.CreateTab("session_b", "nonexistent", "")
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestCloseTab(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	tabID, err := sm.CreateTab("session_c", "test", "")
	if err != nil {
		t.Fatal(err)
	}

	// Close the tab
	if err := sm.CloseTab("session_c", tabID); err != nil {
		t.Fatal(err)
	}

	// Session should be deleted (no tabs) — ListTabs should fail
	_, err = sm.ListTabs("session_c")
	if err == nil {
		t.Error("expected session to be deleted after closing last tab")
	}
}

func TestCloseTab_NotFound(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	if err := sm.CloseTab("nonexistent", "tab_x"); err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestGetChat(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	tabID, err := sm.CreateTab("session_d", "test", "")
	if err != nil {
		t.Fatal(err)
	}

	chat, err := sm.GetChat("session_d", tabID)
	if err != nil {
		t.Fatal(err)
	}
	if chat == nil {
		t.Fatal("GetChat returned nil")
	}
}

func TestGetChat_SessionNotFound(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	_, err := sm.GetChat("nonexistent", "tab_x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHeartbeat(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	// Create a tab to establish session
	_, err := sm.CreateTab("session_e", "test", "")
	if err != nil {
		t.Fatal(err)
	}

	// Heartbeat should not panic or error
	sm.Heartbeat("session_e")

	// Session should still be alive
	tabs, err := sm.ListTabs("session_e")
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs) != 1 {
		t.Errorf("expected 1 tab, got %d", len(tabs))
	}
}

func TestCloseSession(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	// Create a tab to establish session
	_, err := sm.CreateTab("session_f", "test", "")
	if err != nil {
		t.Fatal(err)
	}

	sm.CloseSession("session_f")

	// Session should be deleted
	_, err = sm.ListTabs("session_f")
	if err == nil {
		t.Error("expected session to be deleted after CloseSession")
	}
}

func TestConcurrentTabs(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := 		sm.CreateTab("concurrent", "test", "")
			if err != nil {
				t.Errorf("CreateTab: %v", err)
			}
		}()
	}

	wg.Wait()

	tabs, err := sm.ListTabs("concurrent")
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs) != n {
		t.Errorf("expected %d tabs, got %d", n, len(tabs))
	}
}

func TestCreateComboTab(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	tabID, err := sm.CreateComboTab("combo_session", []string{"test", "test2"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if tabID == "" {
		t.Fatal("expected non-empty tab ID")
	}

	tabs, err := sm.ListTabs("combo_session")
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabs))
	}
	if len(tabs[0].Modes) != 2 {
		t.Errorf("expected 2 modes, got %d", len(tabs[0].Modes))
	}
	if tabs[0].Modes[0] != "test" || tabs[0].Modes[1] != "test2" {
		t.Errorf("modes = %v, want [test test2]", tabs[0].Modes)
	}
	// Label should be combined
	expectedLabel := "Test Mode | Test Mode 2"
	if tabs[0].Label != expectedLabel {
		t.Errorf("label = %q, want %q", tabs[0].Label, expectedLabel)
	}
}

func TestCreateComboTab_SingleMode(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	tabID, err := sm.CreateComboTab("single_combo", []string{"test"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if tabID == "" {
		t.Fatal("expected non-empty tab ID")
	}

	tabs, err := sm.ListTabs("single_combo")
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabs))
	}
	if len(tabs[0].Modes) != 1 || tabs[0].Modes[0] != "test" {
		t.Errorf("modes = %v, want [test]", tabs[0].Modes)
	}
	// Label should be just the mode label
	if tabs[0].Label != "Test Mode" {
		t.Errorf("label = %q, want %q", tabs[0].Label, "Test Mode")
	}
}

func TestCreateComboTab_UnknownMode(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	_, err := sm.CreateComboTab("combo_bad", []string{"test", "nonexistent"}, "")
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestCreateComboTab_EmptyModes(t *testing.T) {
	cfg := newTestConfig(t)
	sm := NewSessionManager(cfg, 1*time.Hour, true)
	defer sm.Stop()

	_, err := sm.CreateComboTab("combo_empty", []string{}, "")
	if err == nil {
		t.Fatal("expected error for empty modes")
	}
}

func TestBuildComboLabel(t *testing.T) {
	tests := []struct {
		labels []string
		want   string
	}{
		{[]string{"Single"}, "Single"},
		{[]string{"A", "B"}, "A | B"},
		{[]string{"A", "B", "C"}, "A | B | C"},
		{[]string{"Very long label A", "Very long label B", "Very long label C"},
			"Very long label A | Very long label B | Ve…"},
	}
	for _, tt := range tests {
		got := buildComboLabel(tt.labels)
		if got != tt.want {
			t.Errorf("buildComboLabel(%v) = %q, want %q", tt.labels, got, tt.want)
		}
	}
}
