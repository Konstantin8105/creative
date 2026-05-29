package creative

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

// ToolParamsToString converts JSON arguments from native tool_calls
// to a space-separated string for legacy Execute functions.
// Preserves the Required fields order from Parameters schema.
func ToolParamsToString(tool Tool, jsonArgs string) string {
	if tool.Parameters == nil {
		// For tools with no schema, pass the JSON as-is (simple tools like get_current_time)
		// or try to strip JSON wrapping if it looks like {"key":"val"}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(jsonArgs), &m); err != nil {
			return jsonArgs
		}
		// No schema, but we got a JSON object — extract values in key order
		var parts []string
		for k, v := range m {
			_ = k
			parts = append(parts, fmt.Sprintf("%v", v))
		}
		return strings.Join(parts, " ")
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(jsonArgs), &args); err != nil {
		return jsonArgs // fallback
	}

	var parts []string
	// Use Required order for deterministic output
	for _, key := range tool.Parameters.Required {
		if val, ok := args[key]; ok {
			strVal := fmt.Sprintf("%v", val)
			// Wrap values containing spaces in quotes so downstream parsing
			// (strings.Fields) preserves them as a single token.
			if strings.Contains(strVal, " ") {
				strVal = `"` + strVal + `"`
			}
			parts = append(parts, strVal)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	return jsonArgs
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


