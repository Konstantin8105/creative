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
//   - KeepAlive: duration string like "5m", "1h", "24h", or "-1" for infinite
type Provider struct {
	Model string // AI model name, e.g., "llama3.1", "gpt-4"

	Endpoint string // API endpoint URL, e.g., "http://localhost:11434/api/"
	Key      string // API key for external providers (optional)

	ContextSize int // Maximum context window size in tokens

	RequestTimeout time.Duration // Timeout for HTTP requests
	KeepAlive      string        // Keep-alive duration for model in memory
}

// defaultOptions returns default generation parameters for AI models
// context: context window size in tokens, must be positive (typically 1000-200000)
// Returns: map with default generation options
func defaultOptions(context int) map[string]interface{} {
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
