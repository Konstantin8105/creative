package creative

import (
	"fmt"
	"strings"
	"time"
)

// Tool represents a callable function that the AI can invoke.
type Tool struct {
	Name        string
	Description string
	Execute     func(params string) string
}

// DefaultTools returns the default set of tools available to agents.
func DefaultTools() []Tool {
	return []Tool{
		{
			Name:        "get_current_time",
			Description: "Получить текущие дату и время. Возвращает текущее системное время в формате RFC 3339.",
			Execute: func(params string) string {
				return time.Now().Format(time.RFC3339)
			},
		},
	}
}

// ExtractToolCall finds the first tool call in text using format {{tool:name}} or {{tool:name params}}.
// Returns the tool name, parameters, and whether a tool call was found.
func ExtractToolCall(text string) (name string, params string, found bool) {
	prefix := "{{tool:"
	start := strings.Index(text, prefix)
	if start < 0 {
		return "", "", false
	}
	// After "{{tool:", find the closing "}}"
	rest := text[start+len(prefix):]
	end := strings.Index(rest, "}}")
	if end < 0 {
		return "", "", false
	}
	content := strings.TrimSpace(rest[:end])
	space := strings.Index(content, " ")
	if space < 0 {
		return content, "", true
	}
	return content[:space], strings.TrimSpace(content[space+1:]), true
}

// ExecuteTool looks up a tool by name and executes it with the given params.
func ExecuteTool(name, params string, tools []Tool) (string, error) {
	for _, t := range tools {
		if t.Name == name {
			return t.Execute(params), nil
		}
	}
	return "", fmt.Errorf("tool not found: %s", name)
}

// BuildToolCall builds a tool call string in the expected format.
func BuildToolCall(name string) string {
	return "{{tool:" + name + "}}"
}

// ExtractAllToolCalls finds ALL tool calls in text using format {{tool:name}} or {{tool:name params}}.
// Returns them in order of appearance, limited to maxCount (0 = no limit).
// Each call includes the full raw string for direct replacement.
func ExtractAllToolCalls(text string, maxCount int) []struct {
	Name   string
	Params string
	Raw    string
} {
	var calls []struct {
		Name   string
		Params string
		Raw    string
	}
	remaining := text
	prefix := "{{tool:"
	for {
		if maxCount > 0 && len(calls) >= maxCount {
			break
		}
		name, params, found := ExtractToolCall(remaining)
		if !found {
			break
		}
		// Find the raw string for this call
		start := strings.Index(remaining, prefix)
		rest := remaining[start+len(prefix):]
		end := strings.Index(rest, "}}")
		raw := remaining[start : start+len(prefix)+end+2]
		calls = append(calls, struct {
			Name   string
			Params string
			Raw    string
		}{Name: name, Params: params, Raw: raw})
		remaining = remaining[start+len(prefix)+end+2:]
	}
	return calls
}

// BuildToolCallWithParams builds a tool call string with parameters.
func BuildToolCallWithParams(name, params string) string {
	return "{{tool:" + name + " " + params + "}}"
}

// ToolsPrompt returns a system prompt describing available tools.
func ToolsPrompt(tools []Tool) string {
	if len(tools) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Доступные инструменты:\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "- %s: %s\n", t.Name, t.Description)
	}
	b.WriteString("\nФормат вызова: {{tool:название_инструмента}}\n")
	b.WriteString("Если нужно передать параметры: {{tool:название_инструмента параметры}}\n")
	return b.String()
}
