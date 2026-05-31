package webserver

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/Konstantin8105/creative"
	"github.com/russross/blackfriday/v2"
)

//go:embed static/index.html
var indexHTML []byte

func sseEvent(w http.ResponseWriter, flusher http.Flusher, event string, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(indexHTML); err != nil {
		log.Printf("Error writing index HTML: %v", err)
	}
}

func handleChat(w http.ResponseWriter, r *http.Request, sm *SessionManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.FormValue("session_id")
	tabID := r.FormValue("tab_id")
	message := r.FormValue("message")

	if sessionID == "" || tabID == "" || message == "" {
		http.Error(w, "Missing session_id, tab_id, or message", http.StatusBadRequest)
		return
	}

	// Get chat for this tab — check before writing SSE headers
	chat, err := sm.GetChat(sessionID, tabID)
	if err != nil {
		status := http.StatusBadRequest
		msg := err.Error()
		if strings.Contains(err.Error(), "session not found") {
			status = http.StatusGone
			msg = "Session expired. Please refresh the page to start a new session."
			log.Printf("[410] session=%s tab=%s: %s", sessionID, tabID, err.Error())
		} else {
			log.Printf("[400] session=%s tab=%s: %s", sessionID, tabID, err.Error())
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": "not_found", "message": msg})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	var fullContent strings.Builder
	var fullReasoning strings.Builder

	var doneOnce sync.Once
	sendDone := func(contentHTML, reasoningHTML string) {
		doneOnce.Do(func() {
			doneData, _ := json.Marshal(map[string]string{
				"content_html":   contentHTML,
				"reasoning_html": reasoningHTML,
			})
			sseEvent(w, flusher, "done", string(doneData))
		})
	}

	defer func() {
		if r := recover(); r != nil {
			errHTML := renderMarkdown(fmt.Sprintf("⚠️ **Internal Error:**\n\n```\n%v\n```", r))
			sendDone(errHTML, "")
		}
	}()

	chat.SetCallback(&creative.ChatEventCallback{
		OnStreamChunk: func(chunk string) {
			fullContent.WriteString(chunk)
			data, _ := json.Marshal(map[string]string{"content": chunk})
			sseEvent(w, flusher, "content_chunk", string(data))
		},
		OnReasoning: func(text string) {
			fullReasoning.WriteString(text)
			data, _ := json.Marshal(map[string]string{"content": text})
			sseEvent(w, flusher, "reasoning_chunk", string(data))
		},
		OnToolCall: func(name, args string) {
			data, _ := json.Marshal(map[string]string{"name": name, "args": args})
			sseEvent(w, flusher, "tool_call", string(data))
		},
		OnToolResult: func(name, result string) {
			preview := result
			if len(preview) > 400 {
				preview = preview[:400] + "..."
			}
			data, _ := json.Marshal(map[string]string{"name": name, "result": preview})
			sseEvent(w, flusher, "tool_result", string(data))
		},
		OnRetry: func(attempt int, err error) {
			data, _ := json.Marshal(map[string]interface{}{
				"attempt": attempt,
				"error":   err.Error(),
			})
			sseEvent(w, flusher, "retry", string(data))
		},
		OnInfo: func(eventType, message string) {
			data, _ := json.Marshal(map[string]string{
				"type":    eventType,
				"message": message,
			})
			sseEvent(w, flusher, "info", string(data))
		},
	})

	_, err = chat.SendStream(message, true)
	if err != nil {
		partialContent := strings.TrimSpace(fullContent.String())
		partialReasoning := strings.TrimSpace(fullReasoning.String())
		if partialContent != "" || partialReasoning != "" {
			errNote := fmt.Sprintf("\n\n---\n🔌 **Connection lost:** `%s`", err.Error())
			fullContent.WriteString(errNote)
			sendDone(renderMarkdown(fullContent.String()), renderMarkdown(partialReasoning))
			return
		}

		errHTML := renderMarkdown(fmt.Sprintf("⚠️ **Error:**\n\n```\n%s\n```", err.Error()))
		sendDone(errHTML, "")
		return
	}

	contentHTML := renderMarkdown(fullContent.String())
	reasoningHTML := renderMarkdown(fullReasoning.String())
	sendDone(contentHTML, reasoningHTML)
}

func handleConfig(w http.ResponseWriter, r *http.Request, cfg *creative.Config) {
	w.Header().Set("Content-Type", "application/json")

	defaultMode := ""
	if len(cfg.Modes) > 0 {
		defaultMode = cfg.Modes[0].Name
	}

	type modeInfo struct {
		Name  string `json:"name"`
		Label string `json:"label"`
	}

	modes := make([]modeInfo, len(cfg.Modes))
	for i, m := range cfg.Modes {
		modes[i] = modeInfo{Name: m.Name, Label: m.Label}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"default_mode": defaultMode,
		"modes":        modes,
	})
}

func handleTabsCreate(w http.ResponseWriter, r *http.Request, sm *SessionManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.FormValue("session_id")
	modeName := r.FormValue("mode")
	if sessionID == "" || modeName == "" {
		http.Error(w, "Missing session_id or mode", http.StatusBadRequest)
		return
	}

	tabID, err := sm.CreateTab(sessionID, modeName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"tab_id": tabID})
}

func handleTabsList(w http.ResponseWriter, r *http.Request, sm *SessionManager) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "Missing session_id", http.StatusBadRequest)
		return
	}

	tabs, err := sm.ListTabs(sessionID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGone)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "session_expired",
			"message": "Session expired. Please refresh the page to start a new session.",
		})
		return
	}

	defaultMode := ""
	if len(sm.cfg.Modes) > 0 {
		defaultMode = sm.cfg.Modes[0].Name
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tabs":         tabs,
		"default_mode": defaultMode,
	})
}

func handleTabsClose(w http.ResponseWriter, r *http.Request, sm *SessionManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.FormValue("session_id")
	tabID := r.FormValue("tab_id")
	if sessionID == "" || tabID == "" {
		http.Error(w, "Missing session_id or tab_id", http.StatusBadRequest)
		return
	}

	if err := sm.CloseTab(sessionID, tabID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGone)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "session_expired",
			"message": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{})
}

func handleHeartbeat(w http.ResponseWriter, r *http.Request, sm *SessionManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.FormValue("session_id")
	if sessionID == "" {
		http.Error(w, "Missing session_id", http.StatusBadRequest)
		return
	}

	sm.Heartbeat(sessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{})
}

func handleSessionClose(w http.ResponseWriter, r *http.Request, sm *SessionManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.FormValue("session_id")
	if sessionID != "" {
		sm.CloseSession(sessionID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{})
}

func renderMarkdown(text string) string {
	if text == "" {
		return ""
	}
	output := blackfriday.Run([]byte(text))
	return string(output)
}
