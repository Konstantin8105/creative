package creative

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

var _ AIrunner = new(Ollama)

// Ollama is an AI provider implementation for Ollama API
// It embeds Provider configuration and implements AIrunner interface
type Ollama Provider

func (pr Ollama) GetContextSize() int {
	return pr.ContextSize
}

// ChatMessage represents a single message in chat conversation
type ChatMessage struct {
	Role    string `json:"role"`    // "user", "assistant", or "system"
	Content string `json:"content"` // Message content
}

// In documentation:
// To generate a response using the generate endpoint, send a POST request with a JSON body specifying the model and prompt:
// ```bash
//
//	curl http://localhost:11434/api/generate -d '{
//	  "model": "llama3.1",
//	  "prompt": "Why is the sky blue?"
//	}'
//
// ```
//
// For a chat-based interaction using the /api/chat endpoint:
// ```bash
//
//	curl http://localhost:11434/api/chat -d '{
//	  "model": "llama3.1",
//	  "messages": [
//	    { "role": "user", "content": "Why is the sky blue?" }
//	  ]
//	}'
//
// ```
func (o Ollama) Send(messages []ChatMessage, isChat bool) (repsonce string, err error) {
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

	endpoint := o.Endpoint
	// Ensure endpoint ends with slash for path concatenation
	if len(endpoint) > 0 && endpoint[len(endpoint)-1] != '/' {
		endpoint += "/"
	}
	if isChat {
		endpoint += "chat"
	} else {
		endpoint += "generate"
	}

	// log.Printf("Ollama endpoint: %s", endpoint)

	// defaultOllamaOptions returns default generation parameters for AI models
	// context: context window size in tokens, must be positive (typically 1000-200000)
	// Returns: map with default generation options
	defaultOllamaOptions := func(context int) map[string]interface{} {
		// Validate context size
		if context <= 0 {
			context = 4096 // Default fallback
		}

		return map[string]interface{}{
			"temperature": 0.7,     // Range: 0.0-2.0, controls randomness
			"top_p":       0.9,     // Range: 0.0-1.0, nucleus sampling parameter
			"top_k":       40,      // Range: 1-100, top-k sampling
			"num_predict": 3048,    // Maximum tokens to generate, positive integer
			"num_ctx":     context, // Context window size
		}
	}

	ka := time.Duration(60 * time.Minute)
	pr := struct { // OllamaRequest
		Model     string                 `json:"model"`
		Prompt    string                 `json:"prompt"`
		Messages  []ChatMessage          `json:"messages,omitempty"`
		Stream    bool                   `json:"stream"`
		KeepAlive *time.Duration         `json:"keep_alive,omitempty"`
		Options   map[string]interface{} `json:"options,omitempty"`
	}{
		Model:     o.Model,
		Stream:    false,
		KeepAlive: &ka, // Avoid cold start
		Options:   defaultOllamaOptions(o.ContextSize),
	}

	if isChat {
		pr.Messages = messages
	} else {
		for _, m := range messages {
			pr.Prompt += m.Content + "\n"
		}
	}

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
	// log.Printf("Ollama response: %v", resp)
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
	// log.Printf("Ollama response body: %s", string(data))
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
		return
	}
	rb := struct { // OllamaResponse
		Response string      `json:"response"`
		Message  ChatMessage `json:"message"`
		Done     bool        `json:"done"`
	}{}
	if err = json.Unmarshal(data, &rb); err != nil {
		err = fmt.Errorf("unmarshal error: %w", err)
		return
	}
	repsonce = rb.Message.Content
	return
}
