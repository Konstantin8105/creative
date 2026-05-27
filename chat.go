package creative

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// MaxToolIterations is the maximum number of tool call iterations per send.
var MaxToolIterations = 8

// ToolResultMaxPreview is the maximum length of a tool result preview in callbacks.
// Set to 0 for full (untruncated) output.
// Default is 200 characters.
var ToolResultMaxPreview = 200

// MaxSendRetries is the maximum number of retries on transient server errors
// (INTERNAL_ERROR, HTTP 500/503, connection errors).
// Default is 2 retries (3 total attempts). Set to 0 to disable retry.
var MaxSendRetries = 5

// ChatEventCallback receives live updates during Chat.SendStream().
// All callbacks are optional (nil = no callback).
type ChatEventCallback struct {
	// OnStreamChunk is called for each chunk of streamed text from the AI.
	OnStreamChunk func(chunk string)
	// OnToolCall is called when a tool is invoked.
	OnToolCall func(name string, args string)
	// OnToolResult is called when a tool returns its result.
	OnToolResult func(name string, result string)
	// OnReasoning is called for reasoning_content chunks (DeepSeek thinking mode).
	OnReasoning func(text string)
	// OnRetry is called before each retry attempt (not called on first attempt).
	// attempt is the retry number (1-based), err is the error that triggered retry.
	OnRetry func(attempt int, err error)
	// OnInfo is a universal informational message block.
	// eventType is a discriminator string (e.g. "iteration", "message_size").
	OnInfo func(eventType string, message string)
}

func NewChat(prv AIrunner) *Chat {
	return &Chat{prv: prv}
}

// ensurePrv panics if the AIrunner provider is nil.
func (ch *Chat) ensurePrv() {
	if ch.prv == nil {
		panic("creative: AIrunner provider is nil; use NewChat with a non-nil provider")
	}
}

type Chat struct {
	system   []string
	msgs     []ChatMessage
	prv      AIrunner
	Tools    []Tool
	callback *ChatEventCallback // optional callback for live events
}

// SetTools configures available tools. When tools are set, tool call
// processing is enabled in Send().
func (ch *Chat) SetTools(tools []Tool) {
	ch.Tools = tools
}

// SetCallback sets an optional ChatEventCallback for live event reporting
// during SendStream(). All callback functions are optional.
// Pass nil to disable callbacks.
func (ch *Chat) SetCallback(cb *ChatEventCallback) {
	ch.callback = cb
}

func (ch Chat) String() string {
	data, err := json.MarshalIndent(ch.msgs, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(data)
}

func (ch *Chat) AddSystem(system ...string) {
	ch.system = append(ch.system, system...)
}

// Send sends a message to the AI, processes any tool calls
// (native tool_calls or legacy {{tool:...}} markers),
// and returns the final response text.
func (ch *Chat) Send(input string, isChat bool) (responce string, err error) {
	ch.ensurePrv()
	if len(ch.msgs) == 0 && 0 < len(ch.system) {
		s := strings.Join(ch.system, "\n\n")
		ch.msgs = append(ch.msgs, ChatMessage{Role: "system", Content: s})
	}
	// Save checkpoint before adding user message; rollback on error to prevent
	// invalid message sequences (e.g. user without assistant) that cause
	// DeepSeek API errors on subsequent requests.
	checkpoint := len(ch.msgs)
	defer func() {
		if err != nil {
			ch.msgs = ch.msgs[:checkpoint]
		}
	}()
	ch.msgs = append(ch.msgs,
		ChatMessage{Role: "user", Content: input},
	)

	assistantMsg, err := ch.prv.SendStream(ch.msgs, isChat, nil, ch.Tools)
	if err != nil {
		return
	}
	if assistantMsg.Role == "" {
		assistantMsg.Role = "assistant"
	}

	ch.msgs = append(ch.msgs, assistantMsg)

	// Process tool calls — if we got native tool_calls
	if len(ch.Tools) > 0 && len(assistantMsg.ToolCalls) > 0 {
		responce, err = ch.processToolCalls(isChat)
		if err != nil {
			return
		}
	} else {
		responce = strings.TrimSpace(assistantMsg.Content)
	}
	return
}

// isTransientError returns true if the error is likely a transient server issue
// that can be safely retried without modifying the message history.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// HTTP/2 INTERNAL_ERROR from server peer
	if strings.Contains(msg, "INTERNAL_ERROR") {
		return true
	}
	// HTTP 500 Server Error, 503 Server Overloaded, 429 Rate Limited
	if strings.Contains(msg, "status 500") || strings.Contains(msg, "status 503") || strings.Contains(msg, "status 429") {
		return true
	}
	// Stream read/connection errors
	if strings.Contains(msg, "stream read error") || strings.Contains(msg, "stream error") {
		return true
	}
	// Generic connection/timeout errors
	if strings.Contains(msg, "connection") || strings.Contains(msg, "timeout") || strings.Contains(msg, "EOF") {
		return true
	}
	return false
}

// validateMessages checks that the message sequence is valid before sending to the AI API.
// It logs a warning if consecutive user messages are found (indicates a bug or edge case).
// This is a safety net to prevent sending invalid message sequences that cause API errors.
func validateMessages(msgs []ChatMessage) {
	for i := 1; i < len(msgs); i++ {
		if msgs[i].Role == "user" && msgs[i-1].Role == "user" {
			log.Printf("WARN: consecutive user messages at indices %d and %d — message history may be corrupted",
				i-1, i)
		}
	}
}

// retrySendStream calls prv.SendStream with retry logic for transient errors.
// It never modifies ch.msgs on error.
func (ch *Chat) retrySendStream(isChat bool, streamCB func(chunkType, chunk string)) (assistantMsg ChatMessage, err error) {
	maxAttempts := 1 + MaxSendRetries // first attempt + retries
	if MaxSendRetries < 0 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			// Fire OnRetry callback before retrying
			if ch.callback != nil && ch.callback.OnRetry != nil {
				ch.callback.OnRetry(attempt-1, lastErr)
			}
			// Sleep with backoff: 1s, 2s, 3s...
			time.Sleep(time.Duration(attempt-1) * time.Second)
		}

		// Validate message sequence before sending; warn on duplicate user messages.
		validateMessages(ch.msgs)

		assistantMsg, err = ch.prv.SendStream(ch.msgs, isChat, streamCB, ch.Tools)
		if err == nil {
			return assistantMsg, nil // success
		}

		lastErr = err
		if !isTransientError(err) {
			// Permanent error — don't retry
			return assistantMsg, err
		}
		// Transient error — will retry
	}

	// All retries exhausted — dump messages to JSON for debugging
	if false {
		data, _ := json.MarshalIndent(ch.msgs, "", "  ")
		_ = os.WriteFile("chat_error_dump.json", data, 0644)
		log.Printf("ERROR: chat messages dumped to chat_error_dump.json (%d bytes)", len(data))
	}
	if assistantMsg.Content != "" || len(assistantMsg.ToolCalls) > 0 {
		return assistantMsg, fmt.Errorf("stream error after %d retries: %w", maxAttempts, lastErr)
	}
	return assistantMsg, fmt.Errorf("stream error after %d retries: %w", maxAttempts, lastErr)
}

// SendStream sends a message to the AI with streaming support.
// The assistant's response text is streamed via the callback set by SetCallback().
// Processes any tool calls and streams subsequent responses.
// Returns the final response text.
func (ch *Chat) SendStream(input string, isChat bool) (response string, err error) {
	ch.ensurePrv()
	if len(ch.msgs) == 0 && 0 < len(ch.system) {
		s := strings.Join(ch.system, "\n\n")
		ch.msgs = append(ch.msgs, ChatMessage{Role: "system", Content: s})
	}
	// Save checkpoint before adding user message; rollback on error to prevent
	// invalid message sequences that cause DeepSeek API errors on subsequent requests.
	checkpoint := len(ch.msgs)
	defer func() {
		if err != nil {
			ch.msgs = ch.msgs[:checkpoint]
		}
	}()
	ch.msgs = append(ch.msgs,
		ChatMessage{Role: "user", Content: input},
	)

	// Build the streaming callback that routes to ChatEventCallback
	streamCB := func(chunkType, chunk string) {
		if ch.callback == nil {
			return
		}
		switch chunkType {
		case "content":
			if ch.callback.OnStreamChunk != nil {
				ch.callback.OnStreamChunk(chunk)
			}
		case "reasoning":
			if ch.callback.OnReasoning != nil {
				ch.callback.OnReasoning(chunk)
			}
		}
	}

	assistantMsg, err := ch.retrySendStream(isChat, streamCB)
	if err != nil {
		return "", err
	}
	if assistantMsg.Role == "" {
		assistantMsg.Role = "assistant"
	}

	ch.msgs = append(ch.msgs, assistantMsg)

	// Process tool calls — if we got native tool_calls
	if len(ch.Tools) > 0 && len(assistantMsg.ToolCalls) > 0 {
		response, err = ch.processToolCalls(isChat)
		if err != nil {
			return "", err
		}
	} else {
		response = strings.TrimSpace(assistantMsg.Content)
	}
	return response, nil
}

// processToolCalls processes tool calls with streaming support.
// Each iteration executes all tool calls in batch, streams the AI's response
// via SendStream, and continues until no more tool calls are found
// or MaxToolIterations is reached.
func (ch *Chat) processToolCalls(isChat bool) (_ string, err error) {
	defer func() {
		// Fire message size info
		if ch.callback != nil && ch.callback.OnInfo != nil {
			if err != nil {
				ch.callback.OnInfo("message_size",
					fmt.Sprintf("Error: %v", err))
			}
			if data, err := json.Marshal(ch.msgs); err == nil {
				ch.callback.OnInfo("message_size",
					fmt.Sprintf("Messages: %s (%d msgs)",
						formatFileSize(int64(len(data))), len(ch.msgs)))
			}
		}
	}()
	for iteration := 0; iteration < MaxToolIterations; iteration++ {
		// Fire iteration info
		if ch.callback != nil && ch.callback.OnInfo != nil {
			ch.callback.OnInfo("iteration",
				fmt.Sprintf("Iteration %d/%d", iteration+1, MaxToolIterations))
		}

		last := ch.msgs[len(ch.msgs)-1]
		if last.Role != "assistant" {
			return last.Content, nil
		}

		// No tool_calls — we're done
		if len(last.ToolCalls) == 0 {
			return last.Content, nil
		}

		// Save checkpoint before adding tool result messages; rollback on error
		// to prevent orphaned tool results without a corresponding assistant response.
		checkpoint := len(ch.msgs)

		// Execute all tool calls in batch
		for _, tc := range last.ToolCalls {
			tool, found := findTool(tc.Function.Name, ch.Tools)
			if !found {
				// Send error as tool result — AI sees the mistake and can retry
				ch.msgs = append(ch.msgs, ChatMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("Error: tool '%s' not found. Available tools: %s", tc.Function.Name, ch.listToolNames()),
				})
				continue
			}
			params := ToolParamsToString(tool, tc.Function.Arguments)

			// Fire OnToolCall callback
			if ch.callback != nil && ch.callback.OnToolCall != nil {
				ch.callback.OnToolCall(tool.Name, tc.Function.Arguments)
			}

			result := tool.Execute(params)

			// Fire OnToolResult callback
			if ch.callback != nil && ch.callback.OnToolResult != nil {
				ch.callback.OnToolResult(tool.Name, result)
			}

			ch.msgs = append(ch.msgs, ChatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}

		// Send back to AI with tool results using streaming (with retry)
		streamCB := func(chunkType, chunk string) {
			if ch.callback == nil {
				return
			}
			switch chunkType {
			case "content":
				if ch.callback.OnStreamChunk != nil {
					ch.callback.OnStreamChunk(chunk)
				}
			case "reasoning":
				if ch.callback.OnReasoning != nil {
					ch.callback.OnReasoning(chunk)
				}
			}
		}

		response, err := ch.retrySendStream(isChat, streamCB)
		if err != nil {
			ch.msgs = ch.msgs[:checkpoint]
			return "", err
		}
		if response.Role == "" {
			response.Role = "assistant"
		}
		response.Content = strings.TrimSpace(response.Content)
		if response.Content == "" && len(response.ToolCalls) == 0 {
			// No content and no tool calls — return previous assistant response.
			// Search backwards to find the last assistant with non-empty content,
			// skipping tool result messages that may have been appended.
			for i := len(ch.msgs) - 1; i >= 0; i-- {
				if ch.msgs[i].Role == "assistant" && ch.msgs[i].Content != "" {
					return ch.msgs[i].Content, nil
				}
			}
			return "", nil
		}
		ch.msgs = append(ch.msgs, response)
	}

	// Max iterations reached — return last response
	last := ch.msgs[len(ch.msgs)-1]

	return last.Content, nil
}

// Stop cancels an ongoing AI operation for this chat.
// RouterAI is shared across all sessions, so Stop is a no-op.
// Each HTTP request has its own context and timeout.
func (ch *Chat) Stop() error {
	return nil
}

// findTool looks up a tool by name in the tools slice.
func findTool(name string, tools []Tool) (Tool, bool) {
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// listToolNames returns a comma-separated list of available tool names.
func (ch *Chat) listToolNames() string {
	names := make([]string, len(ch.Tools))
	for i, t := range ch.Tools {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}
