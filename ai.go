package creative

// AI is the global AI runner instance used by all agents
// Must be initialized before running any agents
var AI AIrunner

// AIrunner defines the interface for AI providers
// Implementations must handle AI model interactions
type AIrunner interface {
	// Run executes an AI request and returns the response
	// request: input prompt string, must be non-empty
	// Returns: response string and any error encountered
	Run(request string) (response []Mail, err error)

	Ask(request string, action func(response string)) (err error)
}
