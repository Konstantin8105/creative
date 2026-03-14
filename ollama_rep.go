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

var _ AIrunner = new(OllamaRep)

type OllamaRep ollama

// ChatMessage — сообщение в чате
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// responseBody — универсальная структура ответа
type responseBody struct {
	Response string      `json:"response"`
	Message  ChatMessage `json:"message"`
	Done     bool        `json:"done"`
}

// doRequest — универсальный метод для отправки запросов
func (o OllamaRep) doRequest(url string, body ollamaRequest) (string, error) {
	if o.RequestTimeout == 0 {
		o.RequestTimeout = 40 * time.Minute
	}
	body.Stream = false
	body.KeepAlive = o.KeepAlive
	body.Options = defaultOllamaOptions

	jsonData, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}

	client := &http.Client{Timeout: o.RequestTimeout}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
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

	var rb responseBody
	if err := json.Unmarshal(data, &rb); err != nil {
		return "", fmt.Errorf("unmarshal error: %w", err)
	}
	if rb.Response != "" {
		return rb.Response, nil
	}
	return rb.Message.Content, nil
}

// Run — для обратной совместимости (generate)
func (o OllamaRep) RunSeq(request string) (string, error) {
	return o.doRequest(o.Endpoint, ollamaRequest{
		Model:   o.Model,
		Prompt:  request,
		Stream:  false,
		Options: defaultOllamaOptions,
	})
}

// chatURL формирует URL для чата
func (o OllamaRep) chatURL() string {
	return strings.Replace(o.Endpoint, "/generate", "/chat", 1)
}

// Chat отправляет историю сообщений
func (o OllamaRep) Chat(messages []ChatMessage) (string, error) {
	return o.doRequest(o.chatURL(), ollamaRequest{
		Model:     o.Model,
		Messages:  messages,
		Stream:    false,
		KeepAlive: o.KeepAlive,
		Options:   defaultOllamaOptions,
	})
}

// amount times of running chat
var steps = 5

// Run — многошаговый диалог с "Ещё"
func (o OllamaRep) Run(request string) (response string, err error) {
	var messages []ChatMessage
	messages = append(messages, ChatMessage{Role: "user", Content: request})

	resp, err := o.Chat(messages)
	if err != nil {
		return "", err
	}
	messages = append(messages, ChatMessage{Role: "assistant", Content: resp})
	response += resp

	for step := range steps {
		messages = append(messages, ChatMessage{Role: "user", Content: "Ещё"})
		resp, err = o.Chat(messages)
		if err != nil {
			return "", err
		}
		resp = strings.TrimSpace(resp)
		if resp == "" {
			break
		}
		log.Printf("OllamaRep step %d responce: %s", step, resp)
		messages = append(messages, ChatMessage{Role: "assistant", Content: resp})
		response += "\n" + resp
	}
	return
}
