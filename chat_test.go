package creative_test

import (
	"testing"

	"github.com/Konstantin8105/creative"
)

func TestChat(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		prv := TestAi{}
		ch := creative.NewChat(&prv)
		_, err := ch.SendStream("Hello", false)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("response", func(t *testing.T) {
		prv := TestAi{resp: "My name"}
		ch := creative.NewChat(&prv)
		_, err := ch.SendStream("Hello", false)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("system", func(t *testing.T) {
		prv := TestAi{resp: "My name"}
		ch := creative.NewChat(&prv)
		ch.AddSystem("info of agent system")
		_, err := ch.SendStream("Hello", false)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("dialog", func(t *testing.T) {
		prv := TestAi{resp: "My name"}
		ch := creative.NewChat(&prv)
		ch.AddSystem("info of agent system")
		for _, s := range []string{"QWE", "WER", "ERT"} {
			prv.resp = "OUT:" + s
			_, err := ch.SendStream("IN:"+s, false)
			if err != nil {
				t.Fatal(err)
			}
		}
	})
	// ---------------------------------------------------------------------------
	// Iteration exhaustion tests — verify graceful termination when AI keeps
	// calling tools without producing a final answer.
	// ---------------------------------------------------------------------------

	t.Run("iteration_exhaustion_force_final_response", func(t *testing.T) {
		const maxIter = 8
		creative.MaxToolIterations = maxIter

		prv := &TestAi{
			toolCallsOnToolRequest: []creative.ToolCall{
				{
					ID:   "call_test_1",
					Type: "function",
					Function: creative.ToolCallFunction{
						Name:      "mock_tool",
						Arguments: "{}",
					},
				},
			},
			toolCallsFinalResponse: "Финальный ответ после исчерпания итераций",
		}
		ch := creative.NewChat(prv)
		ch.SetTools([]creative.Tool{
			{
				Name:        "mock_tool",
				Description: "Mock tool for testing",
				Parameters: &creative.ToolParameters{
					Type:       "object",
					Properties: map[string]creative.ToolProperty{},
					Required:   []string{},
				},
				Execute: func(params string) string { return "mock result" },
			},
		})

		resp, err := ch.SendStream("test", true)
		if err != nil {
			t.Fatal(err)
		}
		if resp != "Финальный ответ после исчерпания итераций" {
			t.Errorf("got %q, want %q", resp, "Финальный ответ после исчерпания итераций")
		}
	})

	t.Run("iteration_exhaustion_tools_restored", func(t *testing.T) {
		creative.MaxToolIterations = 1

		prv := &TestAi{
			toolCallsOnToolRequest: []creative.ToolCall{
				{ID: "call_1", Type: "function", Function: creative.ToolCallFunction{Name: "mock_tool", Arguments: "{}"}},
			},
			toolCallsFinalResponse: "done",
		}
		ch := creative.NewChat(prv)
		tools := []creative.Tool{
			{
				Name:        "mock_tool",
				Description: "Mock tool for testing",
				Parameters: &creative.ToolParameters{
					Type:       "object",
					Properties: map[string]creative.ToolProperty{},
					Required:   []string{},
				},
				Execute: func(params string) string { return "mock result" },
			},
		}
		ch.SetTools(tools)

		_, err := ch.SendStream("test", true)
		if err != nil {
			t.Fatal(err)
		}

		if len(ch.Tools) == 0 {
			t.Error("ch.Tools is empty after exhaustion — tools were not restored")
		}
		if ch.Tools[0].Name != "mock_tool" {
			t.Errorf("ch.Tools[0].Name = %q, want %q", ch.Tools[0].Name, "mock_tool")
		}
	})

	t.Run("iteration_not_exhausted_no_force", func(t *testing.T) {
		creative.MaxToolIterations = 8

		prv := &TestAi{
			resp: "Обычный ответ без вызова инструментов",
		}
		ch := creative.NewChat(prv)
		ch.SetTools(creative.DefaultTools())

		resp, err := ch.SendStream("test", true)
		if err != nil {
			t.Fatal(err)
		}
		if resp != "Обычный ответ без вызова инструментов" {
			t.Errorf("got %q, want %q", resp, "Обычный ответ без вызова инструментов")
		}
	})
}
