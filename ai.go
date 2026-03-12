package creative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var AI AIrunner

type AIrunner interface {
	Run(request string) (responce string, err error)
}

type Ollama struct {
	Endpoint string
	Model    string
	RequestTimeout time.Duration
}


func (o Ollama) Run(request string) (responce string, err error) {
	if o.RequestTimeout == 0 {
		o.RequestTimeout = 40 * time.Minute
	}
	type ollamaRequest struct {
		Model   string                 `json:"model"`
		Prompt  string                 `json:"prompt"`
		Stream  bool                   `json:"stream"`
		Options map[string]interface{} `json:"options,omitempty"`
	}
	options := map[string]interface{}{
		"temperature": 0.7,
		"top_p":       0.9,
		"top_k":       40,
		"num_predict": 2048,
	}
	reqBody := ollamaRequest{
		Model:   o.Model,
		Prompt:  request,
		Stream:  false,
		Options: options,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}
	client := &http.Client{Timeout: o.RequestTimeout}
	resp, err := client.Post(o.Endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read error: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	type ollamaResponse struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}
	var or ollamaResponse
	if err := json.Unmarshal(body, &or); err != nil {
		return "", fmt.Errorf("unmarshal error: %w", err)
	}
	return or.Response, nil
}
