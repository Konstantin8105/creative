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
		_, err := ch.Send("", "Hello", false)
		if err != nil {
			t.Fatal(err)
		}
		compare.Test(t, td("chat.empty"), []byte(ch.String()))
	})
	t.Run("responce", func(t *testing.T) {
		prv := TestAi{resp: "My name"}
		ch := creative.NewChat(&prv)
		_, err := ch.Send("", "Hello", false)
		if err != nil {
			t.Fatal(err)
		}
		compare.Test(t, td("chat.responce"), []byte(ch.String()))
	})
	t.Run("system", func(t *testing.T) {
		prv := TestAi{resp: "My name"}
		ch := creative.NewChat(&prv)
		ch.AddSystem("info of agent system")
		_, err := ch.Send("", "Hello", false)
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
			_, err := ch.Send("", "IN:"+s, false)
			if err != nil {
				t.Fatal(err)
			}
		}
		compare.Test(t, td("chat.dialog"), []byte(ch.String()))
	})
}
