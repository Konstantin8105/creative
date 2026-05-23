package creative

import (
	"bufio"
	"bytes"
	"encoding/json"
	
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var _ AIrunner = new(RouterAI)

// RouterAI is an AI provider implementation for RouterAI (OpenAI-compatible) API.
// It embeds Provider configuration and implements AIrunner interface.
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

// buildEndpoint constructs the API endpoint URL based on isChat flag.
func (o RouterAI) buildEndpoint(isChat bool) string {
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

// requestBody builds the JSON request body for the RouterAI API.
func (o RouterAI) requestBody(messages []ChatMessage, isChat bool, stream bool) interface{} {
	type request struct {
		Model       string        `json:"model"`
		Messages    []ChatMessage `json:"messages,omitempty"`
		Prompt      string        `json:"prompt,omitempty"`
		Stream      bool          `json:"stream"`
		Temperature float64       `json:"temperature,omitempty"`
		MaxTokens   int           `json:"max_tokens,omitempty"`
		TopP        float64       `json:"top_p,omitempty"`
	}

	body := request{
		Model:       o.Model,
		Stream:      stream,
		Temperature: 0.7,
		MaxTokens:   o.ContextSize,
		TopP:        0.9,
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

func (o RouterAI) Send(messages []ChatMessage, isChat bool) (response string, err error) {
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
	body := o.requestBody(messages, isChat, false)

	jsonData, err := json.Marshal(body)
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

	rb := struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Text    string `json:"text"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}{}
	if err = json.Unmarshal(data, &rb); err != nil {
		err = fmt.Errorf("unmarshal error: %w", err)
		return
	}
	if len(rb.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	if isChat {
		return rb.Choices[0].Message.Content, nil
	}
	return rb.Choices[0].Text, nil
}

// SendStream sends messages with streaming support.
// callback is called for each chunk of generated text.
// Returns the complete assembled response.
func (o RouterAI) SendStream(messages []ChatMessage, isChat bool, callback func(chunk string)) (response string, err error) {
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
	body := o.requestBody(messages, isChat, true)

	jsonData, err := json.Marshal(body)
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

	var full strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip lines that don't start with "data: "
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

		// Parse the chunk
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Text string `json:"text"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Skip malformed chunks
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		var content string
		if isChat {
			content = chunk.Choices[0].Delta.Content
		} else {
			content = chunk.Choices[0].Text
		}

		if content == "" {
			continue
		}

		full.WriteString(content)
		if callback != nil {
			callback(content)
		}
	}

	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("stream read error: %w", err)
	}

	return full.String(), nil
}
