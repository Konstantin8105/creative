package creative_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Konstantin8105/creative"
)

func TestAiComp(t *testing.T) {
	prv := creative.Provider{
		Endpoint:       "http://127.0.0.1:1234/v1",
		Model:          "qwen3:0.6b",
		Key:            "",
		RequestTimeout: 10 * time.Minute,
		ContextSize:    2000,
	}
	t.Run("models", func(t *testing.T) {
		aic := creative.RouterAI(prv)
		out, err := aic.GetModels()
		if err != nil {
			t.Error(err)
		}
		t.Logf("%s", out)
	})
	t.Run("chat", func(t *testing.T) {
		for _, isChat := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v", isChat), func(t *testing.T) {
				aic := creative.RouterAI(prv)
				out, err := aic.Send([]creative.ChatMessage{
					{Role: "system", Content: "You the best math teacher and return only result of math opertions"},
					{Role: "assistant", Content: "1+1 = ?"},
				}, isChat)
				if err != nil {
					t.Error(err)
				}
				t.Logf("%s", out)
			})
		}
	})
	t.Run("agent", func(t *testing.T) {
		agent := creative.NewAgent(creative.RouterAI(prv), "math", "Ты хорошо знаешь математику и на мои задачи отвечаешь только результат")
		agent.Init()
		// answers
		for is, s := range []string{"4 + 3 =", "10 + 12 ="} {
			r, err := agent.Send(s)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%d: %s", is, r)
		}
		t.Logf("%s", agent.String())
	})
	return
}
