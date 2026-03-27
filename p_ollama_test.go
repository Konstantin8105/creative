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
	prv.ContextSize = 20000
	t.Run("agentmailbox", func(t *testing.T) {
		var mb creative.MailBox
		agent := creative.NewAgentMailBox(creative.Ollama(prv), "math", "Ты хорошо знаешь математику и на твоя задача ответить только результатом, оформленным в виде письма даже если не знаешь отправителя. К примеру, если надо решить 12 + 3, то в ответном письме надо написать 15. А если надо решить 1 + 2, то в ответном письме надо написать 3.", &mb, creative.DefaultMailPermission())
		agent.Init()
		// answers
		{
			old := creative.MaxAgentIterations
			creative.MaxAgentIterations = 20
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
		t.Logf("%s", mb)
	})
}
