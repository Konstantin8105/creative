package creative

import (
	"encoding/json"
	"os"
	"strings"
)

// TODO
var (
	DebugAgentOutput = true
)

func NewChat(prv AIrunner) *Chat {
	return &Chat{prv: prv}
}

type Chat struct {
	system []string
	msgs   []ChatMessage
	prv    AIrunner
}

func (ch Chat) String() string {
	data, err := json.MarshalIndent(ch.msgs, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(data)
}

func (ch *Chat) AddSystem(system ...string) {
	ch.system = append(ch.system, system...)
}

func (ch *Chat) Send(agentName, input string, isChat bool) (responce string, err error) {
	if len(ch.msgs) == 0 && 0 < len(ch.system) {
		s := strings.Join(ch.system, "\n\n")
		ch.msgs = append(ch.msgs, ChatMessage{Role: "system", Content: s})
	}
	ch.msgs = append(ch.msgs,
		ChatMessage{Role: "user", Content: input},
	)
	if DebugAgentOutput && agentName != "" {
		// ignore error
		data, err := json.MarshalIndent(ch.msgs, "", "  ")
		if err == nil {
			_ = os.WriteFile(agentName+".out", data, 0777)
		}
	}
	responce, err = ch.prv.Send(ch.msgs, isChat)
	if err != nil {
		return
	}
	responce = strings.TrimSpace(responce)
	ch.msgs = append(ch.msgs,
		ChatMessage{Role: "assistant", Content: responce},
	)
	return
}
