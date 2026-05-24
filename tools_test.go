package creative_test

import (
	"strings"
	"testing"

	"github.com/Konstantin8105/creative"
)

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
	})
}
