package creative_test

import (
	"testing"

	"github.com/Konstantin8105/creative"
)

func TestDefaultToolsAreEmpty(t *testing.T) {
	tools := creative.DefaultTools()
	if len(tools) != 0 {
		t.Errorf("DefaultTools should be empty, got %d tools", len(tools))
	}
}

func TestToOpenAITool(t *testing.T) {
	t.Run("nil_parameters", func(t *testing.T) {
		tool := creative.Tool{Name: "test", Parameters: nil}
		result := tool.ToOpenAITool()
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("with_parameters", func(t *testing.T) {
		tool := creative.Tool{
			Name:        "my_tool",
			Description: "My test tool",
			Parameters: &creative.ToolParameters{
				Type:       "object",
				Properties: map[string]creative.ToolProperty{},
				Required:   []string{},
			},
		}
		result := tool.ToOpenAITool()
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if typ, ok := result["type"]; !ok || typ != "function" {
			t.Errorf("type = %v, want 'function'", typ)
		}
		fn, ok := result["function"].(map[string]interface{})
		if !ok {
			t.Fatal("function key missing or wrong type")
		}
		if fn["name"] != "my_tool" {
			t.Errorf("name = %v, want 'my_tool'", fn["name"])
		}
		if fn["description"] != "My test tool" {
			t.Errorf("description = %v, want 'My test tool'", fn["description"])
		}
	})
}

func TestToolsToOpenAI(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result := creative.ToolsToOpenAI(nil)
		if len(result) != 0 {
			t.Errorf("expected empty, got %d", len(result))
		}
	})

	t.Run("empty", func(t *testing.T) {
		result := creative.ToolsToOpenAI([]creative.Tool{})
		if len(result) != 0 {
			t.Errorf("expected empty, got %d", len(result))
		}
	})

	t.Run("mixed", func(t *testing.T) {
		tools := []creative.Tool{
			{Name: "no_params", Parameters: nil},
			{
				Name:        "with_params",
				Description: "Has params",
				Parameters: &creative.ToolParameters{
					Type:       "object",
					Properties: map[string]creative.ToolProperty{},
					Required:   []string{},
				},
			},
			{Name: "no_params2", Parameters: nil},
		}
		result := creative.ToolsToOpenAI(tools)
		if len(result) != 1 {
			t.Fatalf("expected 1 tool with parameters, got %d", len(result))
		}
		fn, ok := result[0]["function"].(map[string]interface{})
		if !ok {
			t.Fatal("function key missing")
		}
		if fn["name"] != "with_params" {
			t.Errorf("name = %v, want 'with_params'", fn["name"])
		}
	})
}
