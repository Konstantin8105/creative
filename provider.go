package creative

import (
	"time"
)

var AdditionMailChatText = "Какие ещё желаешь написать письма или служебные команды? Подумай, какие вопросы могут возникнуть у других ролей после твоего письма. Напиши ответы на них заранее."

// Provider represents configuration for AI model provider
// Valid ranges:
//   - Model: non-empty string
//   - Endpoint: valid URL format, non-empty
//   - Key: optional API key, can be empty for local providers
//   - ContextSize: positive integer, typically 1000-200000
//   - RequestTimeout: positive duration, typically 1m-24h
//   - KeepAlive: duration string like "5m", "1h", "24h", or "-1" for infinite
type Provider struct {
	Model string // AI model name, e.g., "llama3.1", "gpt-4"

	Endpoint string // API endpoint URL, e.g., "http://localhost:11434/api/"
	Key      string // API key for external providers (optional)

	ContextSize int // Maximum context window size in tokens

	RequestTimeout time.Duration // Timeout for HTTP requests
}
