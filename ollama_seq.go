package creative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var _ AIrunner = new(OllamaSeq)

type OllamaSeq ollama

func (o OllamaSeq) Run(request string) (responce string, err error) {
	reqBody := ollamaRequest{
		Model:   o.Model,
		Prompt:  request,
		Stream:  false,
		Options: defaultOllamaOptions,
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
