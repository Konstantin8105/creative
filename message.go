package creative

// ToolCallFunction represents the function details in a tool call
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string of arguments
}

// ToolCall represents a native tool call from the AI
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // "function"
	Function ToolCallFunction `json:"function"`
}

// ChatMessage represents a single message in chat conversation
type ChatMessage struct {
	Role             string     `json:"role"`                        // "system", "user", "assistant", "tool"
	Content          string     `json:"content"`                     // Message content
	ReasoningContent string     `json:"reasoning_content,omitempty"` // DeepSeek chain-of-thought (thinking mode)
	ToolCallID       string     `json:"tool_call_id,omitempty"`      // Tool call ID for role "tool" responses
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`        // Native tool calls (AI -> tool)
}
