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

type Ollama Provider

type OllamaRequest struct {
	Model     string                 `json:"model"`
	Prompt    string                 `json:"prompt"`
	Messages  []OllamaChatMessage    `json:"messages,omitempty"`
	Stream    bool                   `json:"stream"`
	KeepAlive string                 `json:"keep_alive,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
}

type OllamaResponse struct {
	Response string            `json:"response"`
	Message  OllamaChatMessage `json:"message"`
	Done     bool              `json:"done"`
}

type OllamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// doRequest — универсальный метод для отправки запросов
func (o Ollama) doRequest(endpoint string, body OllamaRequest) (string, error) {
	if o.RequestTimeout == 0 {
		o.RequestTimeout = 40 * time.Minute
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}
	/*
		client := &http.Client{Timeout: o.RequestTimeout}
		resp, err := client.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return "", fmt.Errorf("http error: %w", err)
		}
		defer resp.Body.Close()
	*/

	client := &http.Client{Timeout: o.RequestTimeout}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request error: %w", err)
	}
	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")
	if o.Key != "" {
		// Для OpenAI-совместимых API обычно используется Bearer токен
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

// Run — для обратной совместимости (generate)
// func (o OllamaRep) RunSeq(request string) (string, error) {
// 	return o.doRequest(o.Endpoint, ollamaRequest{
// 		Model:   o.Model,
// 		Prompt:  request,
// 		Stream:  false,
// 		Options: defaultOptions,
// 	})
// }

// chatURL формирует URL для чата
// func (o OllamaRep) chatURL() string {
// 	return strings.Replace(o.Endpoint, "/generate", "/chat", 1)
// }

// Chat отправляет историю сообщений
func (o Ollama) send(endpoint string, isChat bool, messages []OllamaChatMessage) (string, error) {
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

// amount times of running chat
var steps = -1

// Run — многошаговый диалог с "Ещё"
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
func (o Ollama) Run(request string) (response string, err error) {
	var messages []OllamaChatMessage
	messages = append(messages, OllamaChatMessage{Role: "user", Content: request})

	endpoint := o.Endpoint
	if endpoint[len(endpoint)-1] != '/' {
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
		return "", err
	}
	messages = append(messages, OllamaChatMessage{Role: "assistant", Content: resp})
	response += resp
	log.Printf("Ollama first responce: %s", resp)

	for i := range steps - 1 {
		messages = append(messages, OllamaChatMessage{Role: "user", Content: "Ещё"})
		resp, err = o.send(endpoint, isChat, messages)
		if err != nil {
			return "", err
		}
		resp = strings.TrimSpace(resp)
		if resp == "" {
			break
		}
		log.Printf("Ollama chat step %d responce: %s", i, resp)
		messages = append(messages, OllamaChatMessage{Role: "assistant", Content: resp})
		response += "\n" + resp
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
