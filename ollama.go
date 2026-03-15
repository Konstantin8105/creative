package creative

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Ollama — клиент для работы с Ollama API
type ollama struct {
	Endpoint       string
	Model          string
	RequestTimeout time.Duration
	KeepAlive      string
}

type ollamaRequest struct {
	Model     string                 `json:"model"`
	Prompt    string                 `json:"prompt"`
	Messages  []ChatMessage          `json:"messages,omitempty"`
	Stream    bool                   `json:"stream"`
	KeepAlive string                 `json:"keep_alive,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
}

// DefaultOptions — параметры генерации
var defaultOllamaOptions = map[string]interface{}{
	"temperature": 0.7,
	"top_p":       0.9,
	"top_k":       40,
	"num_predict": 3048,
	"num_ctx":     32000,
}

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
