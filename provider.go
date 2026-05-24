package creative

import (
	"time"
)

// Provider represents configuration for AI model provider
// Valid ranges:
//   - Model: non-empty string
//   - Endpoint: valid URL format, non-empty
//   - Key: optional API key, can be empty for local providers
//   - ContextSize: positive integer, typically 1000-200000
//   - RequestTimeout: positive duration, typically 1m-24h
//   - ThinkingMode: enables/disables DeepSeek thinking mode
//   - ReasoningEffort: "high" or "max" (thinking mode effort level)
//   - UserID: user identifier for rate limit isolation ([a-zA-Z0-9\-_]+, max 512)
type Provider struct {
	Model string // AI model name, e.g., "llama3.1", "gpt-4"

	Endpoint string // API endpoint URL, e.g., "http://localhost:11434/api/"
	Key      string // API key for external providers (optional)

	ContextSize int // Maximum context window size in tokens

	RequestTimeout time.Duration // Timeout for HTTP requests

	// Thinking mode (DeepSeek-specific)
	ThinkingMode    bool   // enable/disable thinking mode
	ReasoningEffort string // "high" or "max" (default: "high")

	// User isolation for rate limiting
	UserID string // user_id parameter, format: [a-zA-Z0-9\-_]+, max 512 chars
}
