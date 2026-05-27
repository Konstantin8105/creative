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

	// sendDoneOnce guarantees the 'done' SSE event is sent exactly once,
	// even on panic or error. This way the frontend always removes the
	// typing indicator and shows *something* to the user.
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

	// Catch any panic (e.g. nil pointer in callbacks) and show error to user
	defer func() {
		if r := recover(); r != nil {
			errHTML := renderMarkdown(fmt.Sprintf("?? **Internal Error:**\n\n```\n%v\n```", r))
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
			if len(preview) > 200 {
				preview = preview[:200] + "..."
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

	_, err := chat.SendStream(message, true)
	if err != nil {
		// Defensive fallback: if chat.SendStream returned an error but the
		// callback already buffered partial content (e.g. from stream errors),
		// show the partial content together with the error message.
		partialContent := strings.TrimSpace(fullContent.String())
		partialReasoning := strings.TrimSpace(fullReasoning.String())
		if partialContent != "" || partialReasoning != "" {
			errNote := fmt.Sprintf("\n\n---\n?? **Connection lost:** `%s`", err.Error())
			fullContent.WriteString(errNote)
			sendDone(renderMarkdown(fullContent.String()), renderMarkdown(partialReasoning))
			return
		}

		// No partial content � show error as a regular assistant message
		errHTML := renderMarkdown(fmt.Sprintf("?? **Error:**\n\n```\n%s\n```", err.Error()))
		sendDone(errHTML, "")
		return
	}

	contentHTML := renderMarkdown(fullContent.String())
	reasoningHTML := renderMarkdown(fullReasoning.String())
	sendDone(contentHTML, reasoningHTML)
}

func renderMarkdown(text string) string {
	if text == "" {
		return ""
	}
	output := blackfriday.Run([]byte(text))
	return string(output)
}
