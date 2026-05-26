package creative

// Prompt represents a text prompt for AI agents
type Prompt string

// AIrunner defines the interface for AI providers
// Implementations must handle AI model interactions
type AIrunner interface {
	// GetContextSize return context size
	GetContextSize() int

	GetModels() (string, error)

	// SendStream sends messages with streaming support.
	// callback is called for each chunk of generated text.
	// chunkType is "content" for regular text or "reasoning" for thinking mode output.
	// Returns the complete assembled response ChatMessage.
	// tools parameter contains available tool definitions.
	SendStream(chs []ChatMessage, isChat bool, callback func(chunkType, chunk string), tools []Tool) (msg ChatMessage, err error)

	// Stop cancels an ongoing Send or SendStream operation.
	// Returns nil if no operation is in progress.
	Stop() error
}
