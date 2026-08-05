package creative

import (
	"strings"
	"sync"
)

// MockAI is a scripted AIrunner for tests. It supports two modes:
//
//   - Scripted mode: Responses are consumed in order, one ChatMessage per
//     SendStream call (see NewMockAI). This mode takes precedence.
//   - Legacy mode (former TestAi): Resp/Rs return canned text, and
//     ToolCallsOnToolRequest/ToolCallsFinalResponse simulate an AI that keeps
//     requesting tool calls until iteration exhaustion forces a final answer.
//
// Every received message history and tool set is recorded for assertions.
type MockAI struct {
	mu sync.Mutex

	// Err, when non-nil, is returned by every SendStream call.
	Err error

	// Scripted mode: responses consumed in order, one per SendStream call.
	Responses []ChatMessage

	// Legacy mode: single canned response returned by every call.
	Resp string

	// Legacy mode: response assembled by concatenating these chunks.
	Rs []string

	// Legacy mode: values returned by GetModels.
	Models    string
	ModelsErr error

	// Legacy mode: value returned by GetContextSize.
	Context int

	// Legacy mode: when tools are present, the mock returns these tool calls
	// instead of text, simulating an AI that keeps requesting tool calls.
	ToolCallsOnToolRequest []ToolCall

	// Legacy mode: when tools are removed (nil), the mock returns this text as
	// the forced final response after iteration exhaustion.
	ToolCallsFinalResponse string

	// Calls records snapshots of the messages passed to each SendStream call.
	Calls [][]ChatMessage

	// Tools records snapshots of the tool definitions per SendStream call.
	Tools [][]Tool
}

// NewMockAI creates a MockAI that returns the given responses in order.
func NewMockAI(responses ...ChatMessage) *MockAI {
	return &MockAI{Responses: responses}
}

// GetContextSize implements AIrunner.
func (m *MockAI) GetContextSize() int { return m.Context }

// GetModels implements AIrunner.
func (m *MockAI) GetModels() (string, error) { return m.Models, m.ModelsErr }

// Stop implements AIrunner.
func (m *MockAI) Stop() error { return nil }

// SendStream implements AIrunner. It records the call, then either consumes
// the next scripted response or falls back to the legacy canned behavior.
func (m *MockAI) SendStream(chs []ChatMessage, isChat bool, callback func(chunkType, chunk string), tools []Tool) (ChatMessage, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, append([]ChatMessage(nil), chs...))
	m.Tools = append(m.Tools, append([]Tool(nil), tools...))
	m.mu.Unlock()

	if m.Err != nil {
		return ChatMessage{Role: "assistant"}, m.Err
	}
	if len(m.Responses) > 0 {
		resp := m.Responses[0]
		m.Responses = m.Responses[1:]
		if callback != nil && resp.Content != "" {
			callback("content", resp.Content)
		}
		return resp, nil
	}
	// Tools disabled (nil) and a forced final response prepared — this is the
	// call triggered after iteration exhaustion.
	if tools == nil && m.ToolCallsFinalResponse != "" {
		if callback != nil {
			callback("content", m.ToolCallsFinalResponse)
		}
		return ChatMessage{Role: "assistant", Content: m.ToolCallsFinalResponse}, nil
	}
	// Tools present and tool calls to return — simulate an AI that keeps
	// requesting tool calls without producing a final answer.
	if tools != nil && len(m.ToolCallsOnToolRequest) > 0 {
		return ChatMessage{Role: "assistant", Content: "", ToolCalls: m.ToolCallsOnToolRequest}, nil
	}
	if len(m.Rs) > 0 {
		var full strings.Builder
		for _, r := range m.Rs {
			full.WriteString(r)
			if callback != nil {
				callback("content", r)
			}
		}
		return ChatMessage{Role: "assistant", Content: full.String()}, nil
	}
	if callback != nil && m.Resp != "" {
		callback("content", m.Resp)
	}
	return ChatMessage{Role: "assistant", Content: m.Resp}, nil
}

// SystemPrompt returns the rendered system prompt of the first SendStream
// call, or an empty string when no call was made.
func (m *MockAI) SystemPrompt() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Calls) == 0 || len(m.Calls[0]) == 0 {
		return ""
	}
	return m.Calls[0][0].Content
}

// ToolNames returns the tool names of the nth SendStream call (0-based).
func (m *MockAI) ToolNames(call int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if call < 0 || call >= len(m.Tools) {
		return nil
	}
	names := make([]string, 0, len(m.Tools[call]))
	for _, t := range m.Tools[call] {
		names = append(names, t.Name)
	}
	return names
}
