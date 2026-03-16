package creative

import (
	"time"
)

type Provider struct {
	Model string

	Endpoint string
	Key      string

	ContextSize int

	RequestTimeout time.Duration
	KeepAlive      string
}

func defaultOptions(context int) map[string]interface{} {
	// DefaultOptions — параметры генерации
	return map[string]interface{}{
		"temperature": 0.7,
		"top_p":       0.9,
		"top_k":       40,
		"num_predict": 3048,
		"num_ctx":     context,
	}
}
