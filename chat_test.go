package creative_test

import (
	"testing"

	"github.com/Konstantin8105/compare"
	"github.com/Konstantin8105/creative"
)

func TestChat(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		prv := TestAi{}
		ch := creative.NewChat(&prv)
		_, err := ch.Send("Hello", false)
		if err != nil {
			t.Fatal(err)
		}
		compare.Test(t, td("chat.empty"), []byte(ch.String()))
	})
	t.Run("responce", func(t *testing.T) {
		prv := TestAi{resp: "My name"}
		ch := creative.NewChat(&prv)
		_, err := ch.Send("Hello", false)
		if err != nil {
			t.Fatal(err)
		}
		compare.Test(t, td("chat.responce"), []byte(ch.String()))
	})
	t.Run("system", func(t *testing.T) {
		prv := TestAi{resp: "My name"}
		ch := creative.NewChat(&prv)
		ch.AddSystem("info of agent system")
		_, err := ch.Send("Hello", false)
		if err != nil {
			t.Fatal(err)
		}
		compare.Test(t, td("chat.system"), []byte(ch.String()))
	})
	t.Run("dialog", func(t *testing.T) {
		prv := TestAi{resp: "My name"}
		ch := creative.NewChat(&prv)
		ch.AddSystem("info of agent system")
		for _, s := range []string{"QWE", "WER", "ERT"} {
			prv.resp = "OUT:" + s
			_, err := ch.Send("IN:"+s, false)
			if err != nil {
				t.Fatal(err)
			}
		}
		compare.Test(t, td("chat.dialog"), []byte(ch.String()))
	})
	// ---------------------------------------------------------------------------
	// Iteration exhaustion tests — verify graceful termination when AI keeps
	// calling tools without producing a final answer.
	// ---------------------------------------------------------------------------

	t.Run("iteration_exhaustion_force_final_response", func(t *testing.T) {
		// Simulate an AI that always returns tool_calls but never text content.
		// After MaxToolIterations, the system should force a final response
		// by temporarily removing tools and re-sending the conversation.
		const maxIter = 8
		creative.MaxToolIterations = maxIter

		prv := &TestAi{
			toolCallsOnToolRequest: []creative.ToolCall{
				{
					ID:   "call_test_1",
					Type: "function",
					Function: creative.ToolCallFunction{
						Name:      "get_current_time",
						Arguments: "{}",
					},
				},
			},
			toolCallsFinalResponse: "Финальный ответ после исчерпания итераций",
		}
		ch := creative.NewChat(prv)
		ch.SetTools(creative.DefaultTools())

		resp, err := ch.Send("test", true)
		if err != nil {
			t.Fatal(err)
		}
		if resp != "Финальный ответ после исчерпания итераций" {
			t.Errorf("got %q, want %q", resp, "Финальный ответ после исчерпания итераций")
		}
	})

	t.Run("iteration_exhaustion_tools_restored", func(t *testing.T) {
		// Verify that after exhaustion, ch.Tools is restored to the original set.
		creative.MaxToolIterations = 1 // exhaust quickly

		prv := &TestAi{
			toolCallsOnToolRequest: []creative.ToolCall{
				{ID: "call_1", Type: "function", Function: creative.ToolCallFunction{Name: "get_current_time", Arguments: "{}"}},
			},
			toolCallsFinalResponse: "done",
		}
		ch := creative.NewChat(prv)
		tools := creative.DefaultTools()
		ch.SetTools(tools)

		_, err := ch.Send("test", true)
		if err != nil {
			t.Fatal(err)
		}

		// Tools must be restored after processToolCalls returns
		if len(ch.Tools) == 0 {
			t.Error("ch.Tools is empty after exhaustion — tools were not restored")
		}
		if ch.Tools[0].Name != "get_current_time" {
			t.Errorf("ch.Tools[0].Name = %q, want %q", ch.Tools[0].Name, "get_current_time")
		}
	})

	t.Run("iteration_not_exhausted_no_force", func(t *testing.T) {
		// When the AI stops calling tools before exhausting iterations,
		// the normal response should be returned without forcing.
		creative.MaxToolIterations = 8

		prv := &TestAi{
			resp: "Обычный ответ без вызова инструментов",
		}
		ch := creative.NewChat(prv)
		ch.SetTools(creative.DefaultTools())

		resp, err := ch.Send("test", true)
		if err != nil {
			t.Fatal(err)
		}
		if resp != "Обычный ответ без вызова инструментов" {
			t.Errorf("got %q, want %q", resp, "Обычный ответ без вызова инструментов")
		}
	})
}
