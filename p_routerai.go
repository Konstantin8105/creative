package creative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

var _ AIrunner = new(RouterAI)

// RouterAI is an AI provider implementation for RouterAI API
// It embeds Provider configuration and implements AIrunner interface
// RouterAI is a unified API gateway for accessing OpenAI, Anthropic, Google and other providers
type RouterAI Provider

// RouterAIRequest represents request structure for RouterAI API chat completions
// Valid ranges:
//   - Model: non-empty string, identifier of the model (see https://routerai.ru/models)
//   - Messages: array of chat messages, required for chat completions
//   - Prompt: string, required for completions endpoint
//   - Stream: boolean, default false
//   - Temperature: float value from 0 to 2, controls randomness
//   - MaxTokens: positive integer, maximum tokens to generate (optional)
//   - TopP: float value from 0 to 1, nucleus sampling parameter (optional)
//   - FrequencyPenalty: float value from -2 to 2, penalty for frequent tokens (optional)
//   - PresencePenalty: float value from -2 to 2, penalty for new tokens (optional)
type RouterAIRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages,omitempty"`
	Prompt      string        `json:"prompt,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
	// FrequencyPenalty float64       `json:"frequency_penalty,omitempty"`
	// PresencePenalty  float64       `json:"presence_penalty,omitempty"`
}

// RouterAIResponse represents response structure from RouterAI API
// Follows OpenAI-compatible response format
type RouterAIResponse struct {
	ID string `json:"id"`
	// Object  string `json:"object"`
	// Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		// FinishReason string `json:"finish_reason"`
		// Index        int    `json:"index"`
	} `json:"choices"`
	// Usage struct {
	// 	PromptTokens     int `json:"prompt_tokens"`
	// 	CompletionTokens int `json:"completion_tokens"`
	// 	TotalTokens      int `json:"total_tokens"`
	// } `json:"usage,omitempty"`
}

// doRequest sends HTTP request to RouterAI API endpoint
// endpoint: API endpoint URL, must be non-empty and valid
// body: request payload
// Returns: response string or error
func (r RouterAI) doRequest(endpoint string, body RouterAIRequest) (string, error) {
	// Validate endpoint
	if endpoint == "" {
		return "", fmt.Errorf("empty endpoint URL")
	}

	// Set default timeout if not specified
	if r.RequestTimeout == 0 {
		r.RequestTimeout = 40 * time.Minute
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}

	client := &http.Client{Timeout: r.RequestTimeout}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request error: %w", err)
	}
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if r.Key != "" {
		// RouterAI uses Bearer token authentication
		req.Header.Set("Authorization", "Bearer "+r.Key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read error: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
	}

	var rb RouterAIResponse
	if err := json.Unmarshal(data, &rb); err != nil {
		return "", fmt.Errorf("unmarshal error: %w", err)
	}
	if len(rb.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return rb.Choices[0].Message.Content, nil
}

// send sends messages to RouterAI API endpoint
// endpoint: API endpoint URL
// isChat: true for chat completions endpoint, false for completions endpoint
// messages: array of chat messages
// Returns: response string or error
func (r RouterAI) send(endpoint string, isChat bool, messages []ChatMessage) (string, error) {
	// Validate model name
	if r.Model == "" {
		return "", fmt.Errorf("empty model name")
	}

	// Prepare request with default parameters
	pr := RouterAIRequest{
		Model:       r.Model,
		Stream:      false,
		Temperature: 0.7,
		MaxTokens:   2048,
		TopP:        0.9,
	}

	// Add context size to max tokens if available
	if 0 < r.ContextSize && r.ContextSize < pr.MaxTokens {
		pr.MaxTokens = r.ContextSize
	}

	if isChat {
		// Chat completions endpoint uses messages array
		pr.Messages = messages
	} else {
		// Completions endpoint uses prompt string
		for _, m := range messages {
			pr.Prompt += m.Content + "\n"
		}
		pr.Prompt = strings.TrimSpace(pr.Prompt)
	}

	return r.doRequest(endpoint, pr)
}

// Run executes multi-step dialogue with AI model
// request: user input string, must be non-empty
// Returns: concatenated response string or error
// Note: Uses chat completions endpoint if steps > 0, otherwise uses completions endpoint
// In documentation:
// For a chat-based interaction using the /v1/chat/completions endpoint:
// ```bash
//
//	curl https://routerai.ru/api/v1/chat/completions \
//	  --request POST \
//	  --header 'Content-Type: application/json' \
//	  --header 'Authorization: Bearer YOUR_SECRET_TOKEN' \
//	  --data '{
//	  "model": "deepseek/deepseek-chat-v3.1",
//	  "messages": [
//	    {
//	      "role": "user",
//	      "content": "Привет, как дела?"
//	    }
//	  ],
//	  "stream": false,
//	  "temperature": 1
//	}'
//
// ```
//
// For prompt-based text generation using the /v1/completions endpoint:
// ```bash
//
//	curl https://routerai.ru/api/v1/completions \
//	  --request POST \
//	  --header 'Content-Type: application/json' \
//	  --header 'Authorization: Bearer YOUR_SECRET_TOKEN' \
//	  --data '{
//	  "model": "deepseek/deepseek-chat-v3.1",
//	  "prompt": "Напиши короткий рассказ о космосе",
//	  "stream": false,
//	  "temperature": 0.7
//	}'
//
// ```
func (r RouterAI) Ask(request string) (response []string, err error) {
	// Validate input
	if request == "" {
		return nil, fmt.Errorf("empty request")
	}

	// Validate endpoint
	if r.Endpoint == "" {
		return nil, fmt.Errorf("empty endpoint")
	}

	var messages []ChatMessage
	messages = append(messages, ChatMessage{Role: "user", Content: request})

	endpoint := r.Endpoint
	// Ensure endpoint ends with slash for path concatenation
	if len(endpoint) > 0 && endpoint[len(endpoint)-1] != '/' {
		endpoint += "/"
	}

	isChat := false
	if 0 < AmountMessages {
		isChat = true
		endpoint += "chat/completions"
	} else {
		endpoint += "completions"
	}

	log.Printf("RouterAI endpoint: %s", endpoint)
	resp, err := r.send(endpoint, isChat, messages)
	response = append(response, resp)
	if err != nil {
		return
	}
	messages = append(messages, ChatMessage{Role: "assistant", Content: resp})
	log.Printf("RouterAI first response: %s", resp)

	// Execute additional steps if configured
	// steps-1 because first response already obtained
	for i := 0; i < AmountMessages-1; i++ {
		messages = append(messages, ChatMessage{Role: "user", Content: AdditionMailChatText})
		resp, err = r.send(endpoint, isChat, messages)
		resp = strings.TrimSpace(resp)
		response = append(response, resp)
		if err != nil {
			return
		}
		if resp == "" {
			break // Stop if empty response
		}
		log.Printf("RouterAI chat step %d response: %s", i, resp)
		messages = append(messages, ChatMessage{Role: "assistant", Content: resp})
	}
	return
}

func (r RouterAI) Run(request string) (response []Mail, err error) {
	parts, err := r.Ask(request)
	for _, p := range parts {
		ms, _ := ParseMails(p)
		response = append(response, ms...)
	}
	return
}

/*
// Example usage in main.go:
// creative.AI = new(creative.RouterAI{
//     Endpoint:       "https://routerai.ru/api/",
//     Model:          "deepseek/deepseek-chat-v3.1",
//     Key:            "your_api_key_here",
//     RequestTimeout: 4 * time.Hour,
//     KeepAlive:      "48h",
//     ContextSize:    62000,
// })
*/
