package creative

// Prompt represents a text prompt for AI agents
type Prompt string

// AIrunner defines the interface for AI providers
// Implementations must handle AI model interactions
type AIrunner interface {
	// GetContextSize return context size
	GetContextSize() int

	GetModels() (string, error)

	Send(chs []ChatMessage, isChat bool) (repsonce string, err error)

	// SendStream sends messages with streaming support.
	// callback is called for each chunk of generated text.
	// Returns the complete assembled response.
	SendStream(chs []ChatMessage, isChat bool, callback func(chunk string)) (repsonce string, err error)
}
