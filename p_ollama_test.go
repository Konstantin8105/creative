package creative_test

import (
	"testing"
	"time"

	"github.com/Konstantin8105/creative"
)

func TestOllama(t *testing.T) {
	prv := creative.Provider{
		Endpoint:       "http://localhost:11434/api/",
		Model:          "qwen3:0.6b",
		Key:            "",
		RequestTimeout: 10 * time.Minute,
		ContextSize:    2000,
	}
	t.Run("agent", func(t *testing.T) {
		agent := creative.NewAgent(creative.Ollama(prv), "math", "Ты хорошо знаешь математику и на мои задачи отвечаешь только результат")
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

	// Для следующего теста нужен большая модель

	t.Run("agentmailbox", func(t *testing.T) {
		var mb creative.MailBox
		agent := creative.NewAgentMailBox(creative.Ollama(prv), "math", "Ты хорошо знаешь математику и на мои задачи отвечаешь только результат", &mb, creative.DefaultMailPermission())
		agent.Init()
		// answers
		{
			old := creative.MaxAgentIterations
			creative.MaxAgentIterations = 2
			defer func() {
				creative.MaxAgentIterations = old
			}()
		}
		mb.Add([]creative.Mail{
			{
				From: "user",
				To:   "math",
				Body: "Реши и верни результат: 4 + 3 =",
			}, {
				From: "user",
				To:   "math",
				Body: "Реши и верни результат: 10 + 12 = ",
			},
		}, true)
		err := agent.Run()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s", agent.String())
	})
}
