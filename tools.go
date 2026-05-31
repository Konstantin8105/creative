package creative

// ToolParameters defines JSON Schema for native tool parameters.
// When non-nil, the tool definition is sent to AI in native OpenAI format.
type ToolParameters struct {
	Type       string                  `json:"type"` // "object"
	Properties map[string]ToolProperty `json:"properties"`
	Required   []string                `json:"required"`
}

// ToolProperty defines a single parameter property in JSON Schema.
type ToolProperty struct {
	Type        string   `json:"type"` // string, number, integer, boolean, array
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// Tool represents a callable function that the AI can invoke.
type Tool struct {
	Name        string
	Description string
	Parameters  *ToolParameters // nil = no params; non-nil enables native tool format
	Execute     func(params string) string
}

// DefaultTools returns the default set of tools available to agents.
func DefaultTools() []Tool {
	return []Tool{}
}

// ToOpenAITool converts a Tool to the OpenAI tools API format.
// Returns nil if the tool has no Parameters (legacy-only tool).
func (t Tool) ToOpenAITool() map[string]interface{} {
	if t.Parameters == nil {
		return nil
	}
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.Parameters,
		},
	}
}

// ToolsToOpenAI converts all tools with Parameters to OpenAI tools API format.
// Tools without Parameters are excluded from the native format.
func ToolsToOpenAI(tools []Tool) []map[string]interface{} {
	var result []map[string]interface{}
	for _, t := range tools {
		if native := t.ToOpenAITool(); native != nil {
			result = append(result, native)
		}
	}
	return result
}
