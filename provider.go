package creative

import (
	"time"
)

// DurationString is a time.Duration that marshals as human-readable strings
// like "4h", "30m", "60s" in JSON, while remaining a time.Duration internally.
type DurationString time.Duration

func (d DurationString) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func (d *DurationString) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = DurationString(dur)
	return nil
}

// ProviderConfig represents configuration for AI model provider
type ProviderConfig struct {
	Model           string         `json:"model"`
	Endpoint        string         `json:"endpoint"`
	Key             string         `json:"key,omitempty"`
	ContextSize     int            `json:"context_size"`
	RequestTimeout  DurationString `json:"timeout"`
	ThinkingMode    bool           `json:"thinking_mode"`
	ReasoningEffort string         `json:"reasoning_effort,omitempty"`
	UserID          string         `json:"user_id,omitempty"`
}

// Provider is a deprecated alias for ProviderConfig.
// Deprecated: use ProviderConfig instead.
type Provider = ProviderConfig
