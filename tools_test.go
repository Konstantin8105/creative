package creative_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Konstantin8105/creative"
)

func TestExtractToolCall(t *testing.T) {
	t.Run("no_call", func(t *testing.T) {
		name, params, found := creative.ExtractToolCall("Hello world")
		if found {
			t.Fatal("unexpected tool call")
		}
		if name != "" || params != "" {
			t.Fatal("name or params should be empty")
		}
	})
	t.Run("simple", func(t *testing.T) {
		name, params, found := creative.ExtractToolCall("Check time {{tool:get_current_time}} now")
		if !found {
			t.Fatal("tool call not found")
		}
		if name != "get_current_time" {
			t.Fatalf("got name %q, want get_current_time", name)
		}
		if params != "" {
			t.Fatalf("got params %q, want empty", params)
		}
	})
	t.Run("with_params", func(t *testing.T) {
		name, params, found := creative.ExtractToolCall("{{tool:some_tool value1}}")
		if !found {
			t.Fatal("tool call not found")
		}
		if name != "some_tool" {
			t.Fatalf("got name %q, want some_tool", name)
		}
		if params != "value1" {
			t.Fatalf("got params %q, want value1", params)
		}
	})
	t.Run("incomplete", func(t *testing.T) {
		_, _, found := creative.ExtractToolCall("{{tool:get_current_time")
		if found {
			t.Fatal("should not match incomplete call")
		}
	})
	t.Run("empty_tool_name", func(t *testing.T) {
		name, params, found := creative.ExtractToolCall("{{tool: }}")
		if !found {
			t.Fatal("tool call not found")
		}
		if name != "" {
			t.Fatalf("got name %q, want empty", name)
		}
		if params != "" {
			t.Fatalf("got params %q, want empty", params)
		}
	})
}

func TestExecuteTool(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		result, err := creative.ExecuteTool("get_current_time", "", creative.DefaultTools())
		if err != nil {
			t.Fatal(err)
		}
		if result == "" {
			t.Fatal("empty result")
		}
		// Verify it's a valid RFC 3339 time
		_, err = time.Parse(time.RFC3339, result)
		if err != nil {
			t.Fatalf("invalid time format: %v", err)
		}
	})
	t.Run("not_found", func(t *testing.T) {
		_, err := creative.ExecuteTool("nonexistent", "", creative.DefaultTools())
		if err == nil {
			t.Fatal("expected error for unknown tool")
		}
	})
}

func TestBuildToolCall(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		got := creative.BuildToolCall("get_current_time")
		want := "{{tool:get_current_time}}"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("with_params", func(t *testing.T) {
		got := creative.BuildToolCallWithParams("get_current_time", "UTC")
		want := "{{tool:get_current_time UTC}}"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestToolsPrompt(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got := creative.ToolsPrompt(nil)
		if got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})
	t.Run("with_tools", func(t *testing.T) {
		tools := creative.DefaultTools()
		got := creative.ToolsPrompt(tools)
		if !strings.Contains(got, "get_current_time") {
			t.Fatal("prompt should contain tool name")
		}
		if !strings.Contains(got, "{{tool:") {
			t.Fatal("prompt should contain tool call format")
		}
	})
}

func TestChatWithToolCall(t *testing.T) {
	t.Run("tool_call_processed", func(t *testing.T) {
		// First response contains a tool call, second response is the final answer
		ai := TestAi{rs: []string{
			"Current time is {{tool:get_current_time}}",
			"The time is now known.",
		}}
		ch := creative.NewChat(&ai)
		ch.SetTools(creative.DefaultTools())

		resp, err := ch.Send("test", "What time is it?", true)
		if err != nil {
			t.Fatal(err)
		}
		if resp != "The time is now known." {
			t.Fatalf("got %q, want %q", resp, "The time is now known.")
		}
		// Verify the tool call was replaced with time
		str := ch.String()
		if strings.Contains(str, "{{tool:") {
			t.Fatal("tool call marker should have been replaced")
		}
		if !strings.Contains(str, "Результат выполнения инструмента") {
			t.Fatal("should contain tool result message")
		}
	})
	t.Run("no_tool_call", func(t *testing.T) {
		ai := TestAi{resp: "Just a normal response"}
		ch := creative.NewChat(&ai)
		ch.SetTools(creative.DefaultTools())

		resp, err := ch.Send("test", "Hello", true)
		if err != nil {
			t.Fatal(err)
		}
		if resp != "Just a normal response" {
			t.Fatalf("got %q, want %q", resp, "Just a normal response")
		}
	})
}
