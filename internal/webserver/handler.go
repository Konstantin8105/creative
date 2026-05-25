package webserver

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	w.Write(indexHTML)
}

func handleChat(w http.ResponseWriter, r *http.Request, sm *SessionManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.FormValue("session_id")
	message := r.FormValue("message")
	if sessionID == "" || message == "" {
		http.Error(w, "Missing session_id or message", http.StatusBadRequest)
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

	chat := sm.GetOrCreate(sessionID)
	var fullContent strings.Builder
	var fullReasoning strings.Builder

	chat.SetCallback(&creative.ChatEventCallback{
		OnStreamChunk: func(chunk string) {
			fullContent.WriteString(chunk)
			// Don't send content chunks during streaming — buffer for final done event
		},
		OnReasoning: func(text string) {
			fullReasoning.WriteString(text)
			// Don't send reasoning chunks during streaming — buffer for final done event
		},
		OnToolCall: func(name, args string) {
			data, _ := json.Marshal(map[string]string{"name": name, "args": args})
			sseEvent(w, flusher, "tool_call", string(data))
		},
		OnToolResult: func(name, result string) {
			preview := result
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			data, _ := json.Marshal(map[string]string{"name": name, "result": preview})
			sseEvent(w, flusher, "tool_result", string(data))
		},
	})

	_, err := chat.SendStream(message, true)
	if err != nil {
		errData, _ := json.Marshal(map[string]string{"message": err.Error()})
		sseEvent(w, flusher, "error", string(errData))
		sseEvent(w, flusher, "done", "{}")
		return
	}

	contentHTML := renderMarkdown(fullContent.String())
	reasoningHTML := renderMarkdown(fullReasoning.String())

	doneData, _ := json.Marshal(map[string]string{
		"content_html":   contentHTML,
		"reasoning_html": reasoningHTML,
	})
	sseEvent(w, flusher, "done", string(doneData))
}

func renderMarkdown(text string) string {
	if text == "" {
		return ""
	}
	output := blackfriday.Run([]byte(text))
	return string(output)
}
