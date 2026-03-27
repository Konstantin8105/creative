package creative_test

import (
	"testing"

	"github.com/Konstantin8105/compare"
	"github.com/Konstantin8105/creative"
)

func TestAgent(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		prv := TestAi{rs: []string{
			"one",
			"two",
			"three",
			string(creative.FinishDisscussion),
		}}
		agent := creative.NewAgent(&prv, "lucky", "do some stuff")
		agent.Init()

		_, err := agent.Send("Hello")
		if err != nil {
			t.Fatal(err)
		}
		compare.Test(t, td("agent.single"), []byte(agent.String()))
	})
	t.Run("two", func(t *testing.T) {
		prv := TestAi{rs: []string{
			"one",
			"two",
			"three",
			string(creative.FinishDisscussion),
		}}
		agent := creative.NewAgent(&prv, "lucky", "do some stuff")
		agent.Init()

		_, err := agent.Send("Hello")
		if err != nil {
			t.Fatal(err)
		}

		prv.counter = 0
		for i := range prv.rs {
			prv.rs[i] += "2"
		}
		_, err = agent.Send("Hello 2")
		if err != nil {
			t.Fatal(err)
		}

		compare.Test(t, td("agent.two"), []byte(agent.String()))
	})
}
