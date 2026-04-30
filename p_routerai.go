package creative

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var _ AIrunner = new(RouterAI)

// RouterAI is an AI provider implementation for RouterAI API
// It embeds Provider configuration and implements AIrunner interface
// RouterAI is a unified API gateway for accessing OpenAI, Anthropic, Google and other providers
type RouterAI Provider

func (pr RouterAI) GetContextSize() int {
	return pr.ContextSize
}

func (o RouterAI) GetModels() (out string, err error) {
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

func (o RouterAI) Send(messages []ChatMessage, isChat bool) (repsonce string, err error) {
	// Validate endpoint
	if o.Endpoint == "" {
		err = fmt.Errorf("empty endpoint")
		return
	}
	// Validate model name
	if o.Model == "" {
		err = fmt.Errorf("empty model name")
		return
	}
	// Set default timeout if not specified
	if o.RequestTimeout == 0 {
		o.RequestTimeout = 4 * time.Hour
	}
	// Validate context size
	if o.ContextSize <= 0 {
		o.ContextSize = 4096 // Default fallback
	}

	endpoint := o.Endpoint
	// Ensure endpoint ends with slash for path concatenation
	if len(endpoint) > 0 && endpoint[len(endpoint)-1] != '/' {
		endpoint += "/"
	}
	if isChat {
		// endpoint += "chat"
		endpoint += "chat/completions"
	} else {
		// endpoint += "generate"
		endpoint += "completions"
	}

	// log.Printf("RouterAI endpoint: %s", endpoint)

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
	pr := struct {
		Model       string        `json:"model"`
		Messages    []ChatMessage `json:"messages,omitempty"`
		Prompt      string        `json:"prompt,omitempty"`
		Stream      bool          `json:"stream,omitempty"`
		Temperature float64       `json:"temperature,omitempty"`
		MaxTokens   int           `json:"max_tokens,omitempty"`
		TopP        float64       `json:"top_p,omitempty"`
		// FrequencyPenalty float64       `json:"frequency_penalty,omitempty"`
		// PresencePenalty  float64       `json:"presence_penalty,omitempty"`
	}{
		Model:       o.Model,
		Stream:      false,
		Temperature: 0.7,
		MaxTokens:   o.ContextSize,
		TopP:        0.9,
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

	// defaultOllamaOptions returns default generation parameters for AI models
	// context: context window size in tokens, must be positive (typically 1000-200000)
	// Returns: map with default generation options
	// defaultOllamaOptions := func(context int) map[string]interface{} {

	// 	return map[string]interface{}{
	// 		"temperature": 0.7,     // Range: 0.0-2.0, controls randomness
	// 		"top_p":       0.9,     // Range: 0.0-1.0, nucleus sampling parameter
	// 		"top_k":       40,      // Range: 1-100, top-k sampling
	// 		"num_predict": 3048,    // Maximum tokens to generate, positive integer
	// 		"num_ctx":     context, // Context window size
	// 	}
	// }

	// ka := time.Duration(60 * time.Minute)
	// pr := struct { // OllamaRequest
	// 	Model     string                 `json:"model"`
	// 	Prompt    string                 `json:"prompt"`
	// 	Messages  []ChatMessage          `json:"messages,omitempty"`
	// 	Stream    bool                   `json:"stream"`
	// 	KeepAlive *time.Duration         `json:"keep_alive,omitempty"`
	// 	Options   map[string]interface{} `json:"options,omitempty"`
	// }{
	// 	Model:     o.Model,
	// 	Stream:    false,
	// 	KeepAlive: &ka, // Avoid cold start
	// 	Options:   defaultOllamaOptions(o.ContextSize),
	// }

	// if isChat {
	// 	pr.Messages = messages
	// } else {
	// 	for _, m := range messages {
	// 		pr.Prompt += m.Content + "\n"
	// 	}
	// }

	jsonData, err := json.Marshal(pr)
	if err != nil {
		err = fmt.Errorf("marshal error: %w", err)
		return
	}

	client := &http.Client{Timeout: o.RequestTimeout}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		err = fmt.Errorf("create request error: %w", err)
		return
	}
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if o.Key != "" {
		// For OpenAI-compatible APIs, Bearer token is typically used
		req.Header.Set("Authorization", "Bearer "+o.Key)
	}
	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("http error: %w", err)
		return
	}
	// log.Printf("RouterAI response: %v", resp)
	defer func() {
		errC := resp.Body.Close()
		if errC != nil {
			if err != nil {
				err = errors.Join(err, errC)
			} else {
				err = errC
			}
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("read error: %w", err)
		return
	}
	// log.Printf("RouterAI response data: %s", string(data))
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
		return
	}

	// RouterAIResponse represents response structure from RouterAI API
	// Follows OpenAI-compatible response format
	rb := struct {
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
	}{}
	if err = json.Unmarshal(data, &rb); err != nil {
		err = fmt.Errorf("unmarshal error: %w", err)
		return
	}
	if len(rb.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return rb.Choices[0].Message.Content, nil
}
