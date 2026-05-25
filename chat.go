package creative

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MaxToolIterations is the maximum number of tool call iterations per send.
var MaxToolIterations = 20

// ToolResultMaxPreview is the maximum length of a tool result preview in callbacks.
// Set to 0 for full (untruncated) output.
// Default is 200 characters.
var ToolResultMaxPreview = 200

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
	ch.msgs = append(ch.msgs,
		ChatMessage{Role: "user", Content: input},
	)

	assistantMsg, err := ch.prv.Send(ch.msgs, isChat, ch.Tools)
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

	assistantMsg, err := ch.prv.SendStream(ch.msgs, isChat, streamCB, ch.Tools)
	if err != nil {
		return "", err
	}
	if assistantMsg.Role == "" {
		assistantMsg.Role = "assistant"
	}

	ch.msgs = append(ch.msgs, assistantMsg)

	// Process tool calls — if we got native tool_calls
	if len(ch.Tools) > 0 && len(assistantMsg.ToolCalls) > 0 {
		response, err = ch.processToolCallsStream(isChat)
		if err != nil {
			return "", err
		}
	} else {
		response = strings.TrimSpace(assistantMsg.Content)
	}
	return response, nil
}

// processToolCalls processes both native tool_calls and legacy {{tool:...}} markers.
// Each iteration processes one batch of tool calls (native or legacy),
// then sends results back to AI.
// Continues until no more tool calls are found or MaxToolIterations is reached.
func (ch *Chat) processToolCalls(isChat bool) (string, error) {
	for iteration := 0; iteration < MaxToolIterations; iteration++ {
		last := ch.msgs[len(ch.msgs)-1]
		if last.Role != "assistant" {
			return last.Content, nil
		}

		// No tool_calls — we're done
		if len(last.ToolCalls) == 0 {
			return last.Content, nil
		}

		// Execute all tool calls in batch
		for _, tc := range last.ToolCalls {
			tool, found := findTool(tc.Function.Name, ch.Tools)
			if !found {
				return "", fmt.Errorf("tool not found: %s", tc.Function.Name)
			}
			// Convert JSON arguments to space-separated for Execute functions
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

		// Send back to AI with tool results (using streaming)
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

		response, err := ch.prv.SendStream(ch.msgs, isChat, streamCB, ch.Tools)
		if err != nil {
			return "", err
		}
		if response.Role == "" {
			response.Role = "assistant"
		}
		response.Content = strings.TrimSpace(response.Content)
		if response.Content == "" && len(response.ToolCalls) == 0 {
			// No content and no tool calls — return previous response
			return ch.msgs[len(ch.msgs)-2].Content, nil
		}
		ch.msgs = append(ch.msgs, response)

		// Continue loop if AI made more tool calls
	}

	// Max iterations reached — return last response
	last := ch.msgs[len(ch.msgs)-1]
	return last.Content, nil
}

// processToolCallsStream processes tool calls with streaming support.
// Each iteration streams the AI's response after tool results via SendStream.
func (ch *Chat) processToolCallsStream(isChat bool) (string, error) {
	for iteration := 0; iteration < MaxToolIterations; iteration++ {
		last := ch.msgs[len(ch.msgs)-1]
		if last.Role != "assistant" {
			return last.Content, nil
		}

		// No tool_calls — we're done
		if len(last.ToolCalls) == 0 {
			return last.Content, nil
		}

		// Execute all tool calls in batch
		for _, tc := range last.ToolCalls {
			tool, found := findTool(tc.Function.Name, ch.Tools)
			if !found {
				return "", fmt.Errorf("tool not found: %s", tc.Function.Name)
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

		// Send back to AI with tool results using streaming
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

		response, err := ch.prv.SendStream(ch.msgs, isChat, streamCB, ch.Tools)
		if err != nil {
			return "", err
		}
		if response.Role == "" {
			response.Role = "assistant"
		}
		response.Content = strings.TrimSpace(response.Content)
		if response.Content == "" && len(response.ToolCalls) == 0 {
			return ch.msgs[len(ch.msgs)-2].Content, nil
		}
		ch.msgs = append(ch.msgs, response)
	}

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
