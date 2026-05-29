package creative_test

import (
	"strings"
	"testing"

	"github.com/Konstantin8105/creative"
)

func TestToolLookupAndExecute(t *testing.T) {
	t.Run("not_found", func(t *testing.T) {
		tools := creative.DefaultTools()
		for _, tool := range tools {
			if tool.Name == "nonexistent_tool" {
				t.Fatal("unexpected match")
			}
		}
	})

	t.Run("found_and_execute", func(t *testing.T) {
		tools := creative.DefaultTools()
		for _, tool := range tools {
			if tool.Name == "get_current_time" {
				result := tool.Execute("")
				if result == "" {
					t.Error("expected non-empty result from get_current_time")
				}
				return
			}
		}
		t.Fatal("get_current_time tool not found in DefaultTools")
	})
}

func TestToolParamsToString(t *testing.T) {
	t.Run("nil_parameters_json_object", func(t *testing.T) {
		// Tool with no Parameters schema — should extract values from JSON
		tool := creative.Tool{Name: "test", Parameters: nil}
		result := creative.ToolParamsToString(tool, `{"a":"hello","b":"world"}`)
		// Order is non-deterministic for map, but both values should appear
		if !strings.Contains(result, "hello") || !strings.Contains(result, "world") {
			t.Errorf("expected both values, got %q", result)
		}
	})

	t.Run("nil_parameters_non_json", func(t *testing.T) {
		// Tool with no Parameters, non-JSON input — should return as-is
		tool := creative.Tool{Name: "test", Parameters: nil}
		result := creative.ToolParamsToString(tool, "simple string")
		if result != "simple string" {
			t.Errorf("got %q, want %q", result, "simple string")
		}
	})

	t.Run("nil_parameters_invalid_json", func(t *testing.T) {
		// Invalid JSON should be returned as-is
		tool := creative.Tool{Name: "test", Parameters: nil}
		result := creative.ToolParamsToString(tool, "{invalid}")
		if result != "{invalid}" {
			t.Errorf("got %q, want %q", result, "{invalid}")
		}
	})

	t.Run("with_parameters_ordered", func(t *testing.T) {
		tool := creative.Tool{
			Name: "test",
			Parameters: &creative.ToolParameters{
				Type:       "object",
				Properties: map[string]creative.ToolProperty{},
				Required:   []string{"first", "second"},
			},
		}
		result := creative.ToolParamsToString(tool, `{"second":"world","first":"hello"}`)
		expected := "hello world"
		if result != expected {
			t.Errorf("got %q, want %q (ordered by Required)", result, expected)
		}
	})

	t.Run("with_parameters_order_missing", func(t *testing.T) {
		tool := creative.Tool{
			Name: "test",
			Parameters: &creative.ToolParameters{
				Type:       "object",
				Properties: map[string]creative.ToolProperty{},
				Required:   []string{"first", "second"},
			},
		}
		result := creative.ToolParamsToString(tool, `{"first":"hello"}`)
		expected := "hello"
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("with_parameters_value_containing_spaces", func(t *testing.T) {
		tool := creative.Tool{
			Name: "test",
			Parameters: &creative.ToolParameters{
				Type:       "object",
				Properties: map[string]creative.ToolProperty{},
				Required:   []string{"filename"},
			},
		}
		result := creative.ToolParamsToString(tool, `{"filename":"СП 16.13330.2017.txt"}`)
		// Should be quoted because it doesn't contain spaces
		expected := `"СП 16.13330.2017.txt"`
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("with_parameters_invalid_json", func(t *testing.T) {
		tool := creative.Tool{
			Name: "test",
			Parameters: &creative.ToolParameters{
				Type:       "object",
				Properties: map[string]creative.ToolProperty{},
				Required:   []string{"first"},
			},
		}
		result := creative.ToolParamsToString(tool, "{invalid}")
		if result != "{invalid}" {
			t.Errorf("got %q, want %q (fallback)", result, "{invalid}")
		}
	})

	t.Run("with_parameters_no_required", func(t *testing.T) {
		tool := creative.Tool{
			Name: "test",
			Parameters: &creative.ToolParameters{
				Type:       "object",
				Properties: map[string]creative.ToolProperty{},
				Required:   []string{},
			},
		}
		result := creative.ToolParamsToString(tool, `{"key":"val"}`)
		// No required fields — should return raw JSON
		if result != `{"key":"val"}` {
			t.Errorf("got %q, want %q", result, `{"key":"val"}`)
		}
	})
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
