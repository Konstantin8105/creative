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
}
