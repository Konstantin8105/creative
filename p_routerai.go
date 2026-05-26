package creative

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var _ AIrunner = new(RouterAI)

// RouterAI is an AI provider implementation for OpenAI-compatible API (DeepSeek, etc.).
// It embeds Provider configuration and implements AIrunner interface.
type RouterAI struct {
	Provider
}

// NewRouterAI creates a new RouterAI from a Provider configuration.
func NewRouterAI(prv Provider) *RouterAI {
	return &RouterAI{Provider: prv}
}

// Stop is a no-op. Cancellation is handled per-request via context.WithTimeout.
// RouterAI is shared across sessions, so there is no single operation to cancel.
func (o *RouterAI) Stop() error {
	return nil
}

func (o *RouterAI) GetContextSize() int {
	return o.ContextSize
}

func (o *RouterAI) GetModels() (out string, err error) {
	endpoint := o.Endpoint + "/models"
	resp, err := http.Get(endpoint)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	out = string(body)
	return
}

// buildEndpoint constructs the API endpoint URL based on isChat flag.
func (o *RouterAI) buildEndpoint(isChat bool) string {
	endpoint := o.Endpoint
	if len(endpoint) > 0 && endpoint[len(endpoint)-1] != '/' {
		endpoint += "/"
	}
	if isChat {
		endpoint += "chat/completions"
	} else {
		endpoint += "completions"
	}
	return endpoint
}

// openAIRequest is the full request body for the OpenAI Chat Completions API.
type openAIRequest struct {
	Model           string                   `json:"model"`
	Messages        []ChatMessage            `json:"messages,omitempty"`
	Prompt          string                   `json:"prompt,omitempty"`
	Stream          bool                     `json:"stream"`
	Temperature     float64                  `json:"temperature,omitempty"`
	MaxTokens       int                      `json:"max_tokens,omitempty"`
	TopP            float64                  `json:"top_p,omitempty"`
	Thinking        *thinkingParam           `json:"thinking,omitempty"`
	ReasoningEffort string                   `json:"reasoning_effort,omitempty"`
	Tools           []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice      interface{}              `json:"tool_choice,omitempty"`
	UserID          string                   `json:"user_id,omitempty"`
}

type thinkingParam struct {
	Type string `json:"type"` // "enabled" or "disabled"
}

// requestBody builds the JSON request body for the RouterAI API.
func (o *RouterAI) requestBody(messages []ChatMessage, isChat bool, stream bool, tools []Tool) interface{} {
	body := openAIRequest{
		Model:       o.Model,
		Stream:      stream,
		Temperature: 0.7,
		MaxTokens:   o.ContextSize,
		TopP:        0.9,
	}

	// Thinking mode (DeepSeek-specific)
	if o.ThinkingMode {
		body.Thinking = &thinkingParam{Type: "enabled"}
		if o.ReasoningEffort != "" {
			body.ReasoningEffort = o.ReasoningEffort
		} else {
			body.ReasoningEffort = "high"
		}
	}

	// User ID for rate limit isolation
	if o.UserID != "" {
		body.UserID = o.UserID
	}

	// Native tools format (only tools with Parameters defined)
	nativeTools := ToolsToOpenAI(tools)
	if len(nativeTools) > 0 {
		body.Tools = nativeTools
		body.ToolChoice = "auto"
	}

	if isChat {
		body.Messages = messages
	} else {
		var prompt strings.Builder
		for _, m := range messages {
			prompt.WriteString(m.Content)
			prompt.WriteString("\n")
		}
		body.Prompt = strings.TrimSpace(prompt.String())
	}

	return body
}

// openAIResponse is the response structure from the OpenAI Chat Completions API.
type openAIResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role             string     `json:"role"`
			Content          string     `json:"content"`
			ReasoningContent string     `json:"reasoning_content"`
			ToolCalls        []ToolCall `json:"tool_calls"`
		} `json:"message"`
		Text string `json:"text"`
	} `json:"choices"`
	Usage struct {
		PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
		PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
	} `json:"usage"`
}

// streamChunk is the SSE stream chunk structure for OpenAI Chat Completions.
type streamChunk struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role             string           `json:"role"`
			Content          string           `json:"content"`
			ReasoningContent string           `json:"reasoning_content"`
			ToolCalls        []streamToolCall `json:"tool_calls"`
		} `json:"delta"`
		Text string `json:"text"`
	} `json:"choices"`
}

// streamToolCall represents a tool call delta in an SSE stream chunk.
type streamToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// defaults sets default values for unset fields.
func (o *RouterAI) defaults() {
	if o.Endpoint == "" {
		o.Endpoint = "http://localhost:11434/v1/"
	}
	if o.Model == "" {
		o.Model = "gpt-3.5-turbo"
	}
	if o.RequestTimeout == 0 {
		o.RequestTimeout = 4 * time.Hour
	}
	if o.ContextSize <= 0 {
		o.ContextSize = 4096
	}
}

// Send sends messages to the AI and returns the full assistant response message.
func (o *RouterAI) Send(messages []ChatMessage, isChat bool, tools []Tool) (response ChatMessage, err error) {
	if o.Endpoint == "" {
		err = fmt.Errorf("empty endpoint")
		return
	}
	if o.Model == "" {
		err = fmt.Errorf("empty model name")
		return
	}
	if o.RequestTimeout == 0 {
		o.RequestTimeout = 4 * time.Hour
	}
	if o.ContextSize <= 0 {
		o.ContextSize = 4096
	}

	endpoint := o.buildEndpoint(isChat)
	body := o.requestBody(messages, isChat, false, tools)

	jsonData, err := json.Marshal(body)
	if err != nil {
		err = fmt.Errorf("marshal error: %w", err)
		return
	}

	client := &http.Client{Timeout: o.RequestTimeout}
	req, err := http.NewRequestWithContext(context.Background(), "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		err = fmt.Errorf("create request error: %w", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if o.Key != "" {
		req.Header.Set("Authorization", "Bearer "+o.Key)
	}

	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("http error: %w", err)
		return
	}
	defer func() {
		errC := resp.Body.Close()
		if errC != nil && err == nil {
			err = errC
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("read error: %w", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
		return
	}

	var rb openAIResponse
	if err = json.Unmarshal(data, &rb); err != nil {
		err = fmt.Errorf("unmarshal error: %w", err)
		return
	}
	if len(rb.Choices) == 0 {
		err = fmt.Errorf("no choices in response")
		return
	}

	response.Role = "assistant"
	if isChat {
		response.Content = rb.Choices[0].Message.Content
		response.ReasoningContent = rb.Choices[0].Message.ReasoningContent
		response.ToolCalls = rb.Choices[0].Message.ToolCalls
	} else {
		response.Content = rb.Choices[0].Text
	}

	return
}

// SendStream sends messages with streaming support.
// callback is called for each chunk of generated text.
// chunkType is "content" for regular text or "reasoning" for thinking mode output.
// Returns the complete assembled response message.
func (o *RouterAI) SendStream(messages []ChatMessage, isChat bool, callback func(chunkType, chunk string), tools []Tool) (response ChatMessage, err error) {
	if o.Endpoint == "" {
		err = fmt.Errorf("empty endpoint")
		return
	}
	if o.Model == "" {
		err = fmt.Errorf("empty model name")
		return
	}
	if o.RequestTimeout == 0 {
		o.RequestTimeout = 4 * time.Hour
	}
	if o.ContextSize <= 0 {
		o.ContextSize = 4096
	}

	endpoint := o.buildEndpoint(isChat)
	body := o.requestBody(messages, isChat, true, tools)

	jsonData, err := json.Marshal(body)
	if err != nil {
		err = fmt.Errorf("marshal error: %w", err)
		return
	}

	client := &http.Client{Timeout: o.RequestTimeout}
	req, err := http.NewRequestWithContext(context.Background(), "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		err = fmt.Errorf("create request error: %w", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if o.Key != "" {
		req.Header.Set("Authorization", "Bearer "+o.Key)
	}

	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("http error: %w", err)
		return
	}
	defer func() {
		errC := resp.Body.Close()
		if errC != nil && err == nil {
			err = errC
		}
	}()

	if resp.StatusCode != http.StatusOK {
		data, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			err = fmt.Errorf("status %d (body read error: %v)", resp.StatusCode, readErr)
			return
		}
		err = fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
		return
	}

	// Parse SSE (Server-Sent Events) stream
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line length

	var fullContent strings.Builder
	var fullReasoning strings.Builder

	// Tool call accumulation: index -> ToolCall
	toolCallAcc := make(map[int]*ToolCall)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines (keep-alive mechanism)
		if line == "" {
			continue
		}

		// Skip lines that don't start with "data: " (keep-alive comments like ": keep-alive")
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		// Extract the JSON payload after "data: "
		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimSpace(data)

		// Check for stream end marker
		if data == "[DONE]" {
			break
		}

		// Check for API error in the stream data before parsing as a chunk.
		// DeepSeek may return error objects in stream format (e.g. rate limit, server errors).
		var apiErr struct {
			Error *struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &apiErr); err == nil && apiErr.Error != nil {
			response.Role = "assistant"
			response.Content = fullContent.String()
			response.ReasoningContent = fullReasoning.String()
			response.ToolCalls = finalizeToolCalls(toolCallAcc)
			return response, fmt.Errorf("API error in stream: %s (type: %s)", apiErr.Error.Message, apiErr.Error.Type)
		}

		// Parse the chunk
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Skip malformed chunks
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		if isChat {
			// Process content delta
			content := chunk.Choices[0].Delta.Content
			if content != "" {
				fullContent.WriteString(content)
				if callback != nil {
					callback("content", content)
				}
			}

			// Process reasoning_content delta (DeepSeek thinking mode)
			reasoning := chunk.Choices[0].Delta.ReasoningContent
			if reasoning != "" {
				fullReasoning.WriteString(reasoning)
				if callback != nil {
					callback("reasoning", reasoning)
				}
			}

			// Process tool_calls delta (accumulate across chunks)
			toolCalls := chunk.Choices[0].Delta.ToolCalls
			for _, tc := range toolCalls {
				existing, ok := toolCallAcc[tc.Index]
				if !ok {
					// First chunk for this tool call index — create a new entry
					toolCallAcc[tc.Index] = &ToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: ToolCallFunction{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				} else {
					// Subsequent chunk — accumulate arguments
					if tc.ID != "" {
						existing.ID = tc.ID
					}
					if tc.Function.Name != "" {
						existing.Function.Name = tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						existing.Function.Arguments += tc.Function.Arguments
					}
				}
			}
		} else {
			content := chunk.Choices[0].Text
			if content != "" {
				fullContent.WriteString(content)
				if callback != nil {
					callback("content", content)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		response.Role = "assistant"
		response.Content = fullContent.String()
		response.ReasoningContent = fullReasoning.String()
		response.ToolCalls = finalizeToolCalls(toolCallAcc)
		return response, fmt.Errorf("stream read error: %w", err)
	}

	response.Role = "assistant"
	response.Content = fullContent.String()
	response.ReasoningContent = fullReasoning.String()
	response.ToolCalls = finalizeToolCalls(toolCallAcc)
	return
}

// finalizeToolCalls converts the accumulated tool calls map to a sorted slice.
func finalizeToolCalls(acc map[int]*ToolCall) []ToolCall {
	if len(acc) == 0 {
		return nil
	}
	// Collect indices and sort them
	indices := make([]int, 0, len(acc))
	for idx := range acc {
		indices = append(indices, idx)
	}
	// Sort indices (ascending)
	for i := 0; i < len(indices); i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[j] < indices[i] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}
	result := make([]ToolCall, len(indices))
	for i, idx := range indices {
		result[i] = *acc[idx]
	}
	return result
}
