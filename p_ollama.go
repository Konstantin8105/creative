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

var _ AIrunner = new(Ollama)

// Ollama is an AI provider implementation for Ollama API
// It embeds Provider configuration and implements AIrunner interface
type Ollama Provider

// OllamaRequest represents request structure for Ollama API
// Valid ranges:
//   - Model: non-empty string
//   - Prompt: string (can be empty if Messages provided)
//   - Messages: array of chat messages
//   - Stream: boolean
//   - KeepAlive: duration string like "5m", "1h", "24h", or "-1" for infinite
//   - Options: map of generation parameters
type OllamaRequest struct {
	Model     string                 `json:"model"`
	Prompt    string                 `json:"prompt"`
	Messages  []ChatMessage          `json:"messages,omitempty"`
	Stream    bool                   `json:"stream"`
	KeepAlive string                 `json:"keep_alive,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
}

// OllamaResponse represents response structure from Ollama API
type OllamaResponse struct {
	Response string      `json:"response"`
	Message  ChatMessage `json:"message"`
	Done     bool        `json:"done"`
}

// ChatMessage represents a single message in chat conversation
type ChatMessage struct {
	Role    string `json:"role"`    // "user", "assistant", or "system"
	Content string `json:"content"` // Message content
}

// doRequest sends HTTP request to Ollama API endpoint
// endpoint: API endpoint URL, must be non-empty and valid
// body: request payload
// Returns: response string or error
func (o Ollama) doRequest(endpoint string, body OllamaRequest) (string, error) {
	// Validate endpoint
	if endpoint == "" {
		return "", fmt.Errorf("empty endpoint URL")
	}

	// Set default timeout if not specified
	if o.RequestTimeout == 0 {
		o.RequestTimeout = 40 * time.Minute
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}

	client := &http.Client{Timeout: o.RequestTimeout}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request error: %w", err)
	}
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if o.Key != "" {
		// For OpenAI-compatible APIs, Bearer token is typically used
		req.Header.Set("Authorization", "Bearer "+o.Key)
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

	var rb OllamaResponse
	if err := json.Unmarshal(data, &rb); err != nil {
		return "", fmt.Errorf("unmarshal error: %w", err)
	}
	if rb.Response != "" {
		return rb.Response, nil
	}
	return rb.Message.Content, nil
}

// send sends messages to Ollama API endpoint
// endpoint: API endpoint URL
// isChat: true for chat endpoint, false for generate endpoint
// messages: array of chat messages
// Returns: response string or error
func (o Ollama) send(endpoint string, isChat bool, messages []ChatMessage) (string, error) {
	// Validate model name
	if o.Model == "" {
		return "", fmt.Errorf("empty model name")
	}

	pr := OllamaRequest{
		Model:     o.Model,
		Stream:    false,
		KeepAlive: o.KeepAlive,
		Options:   defaultOptions(o.ContextSize),
	}
	if isChat {
		pr.Messages = messages
	} else {
		for _, m := range messages {
			pr.Prompt += m.Content + "\n"
		}
	}
	return o.doRequest(endpoint, pr)
}

// steps defines number of additional chat iterations after initial response
// Valid range: -1 (no additional steps) or positive integer
// Default: -1 (single response only)
var steps = -1

// Run executes multi-step dialogue with AI model
// request: user input string, must be non-empty
// Returns: concatenated response string or error
// Note: Uses chat endpoint if steps > 0, otherwise uses generate endpoint
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
func (o Ollama) Run(request string) (response []Mail, err error) {
	// Validate input
	if request == "" {
		return nil, fmt.Errorf("empty request")
	}

	// Validate endpoint
	if o.Endpoint == "" {
		return nil, fmt.Errorf("empty endpoint")
	}

	var messages []ChatMessage
	messages = append(messages, ChatMessage{Role: "user", Content: request})

	endpoint := o.Endpoint
	// Ensure endpoint ends with slash for path concatenation
	if len(endpoint) > 0 && endpoint[len(endpoint)-1] != '/' {
		endpoint += "/"
	}

	isChat := false
	if 0 < steps {
		isChat = true
		endpoint += "chat"
	} else {
		endpoint += "generate"
	}

	log.Printf("Ollama endpoint: %s", endpoint)
	resp, err := o.send(endpoint, isChat, messages)
	if err != nil {
		return nil, err
	}
	messages = append(messages, ChatMessage{Role: "assistant", Content: resp})
	{
		ms, _ := ParseMails(resp) // ignore error
		response = append(response, ms...)
	}
	log.Printf("Ollama first response: %s", resp)

	// Execute additional steps if configured
	// steps-1 because first response already obtained
	for i := 0; i < steps-1; i++ {
		messages = append(messages, ChatMessage{Role: "user", Content: "Ещё"})
		resp, err = o.send(endpoint, isChat, messages)
		if err != nil {
			return response, err // Return partial response on error
		}
		resp = strings.TrimSpace(resp)
		if resp == "" {
			break // Stop if empty response
		}
		log.Printf("Ollama chat step %d response: %s", i, resp)
		messages = append(messages, ChatMessage{Role: "assistant", Content: resp})
		{
			ms, _ := ParseMails(resp) // ignore error
			response = append(response, ms...)
		}
	}
	return
}

/*
var (
	origKeepAlive string
	KeepAliveSet  bool = true
)

func SetGlobalKeepAlive(val string) error {
	origKeepAlive = os.Getenv("OLLAMA_KEEP_ALIVE")
	KeepAliveSet = true
	return os.Setenv("OLLAMA_KEEP_ALIVE", val)
}

func RestoreGlobalKeepAlive() error {
	if !KeepAliveSet {
		return nil
	}
	if origKeepAlive == "" {
		return os.Unsetenv("OLLAMA_KEEP_ALIVE")
	}
	return os.Setenv("OLLAMA_KEEP_ALIVE", origKeepAlive)
}

func KeepAliveGuard() func() {
	old := os.Getenv("OLLAMA_KEEP_ALIVE")
	return func() {
		if old == "" {
			os.Unsetenv("OLLAMA_KEEP_ALIVE")
		} else {
			os.Setenv("OLLAMA_KEEP_ALIVE", old)
		}
	}
}

func SetupSignalHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		RestoreGlobalKeepAlive()
		os.Exit(1)
	}()
}
*/
