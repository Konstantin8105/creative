package creative

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// MaxToolIterations is the maximum number of tool call iterations per send.
var MaxToolIterations = 5

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
	Tools  []Tool
}

// SetTools configures available tools. When tools are set, tool call
// processing is enabled in Send().
func (ch *Chat) SetTools(tools []Tool) {
	ch.Tools = tools
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
	// Process tool calls
	if len(ch.Tools) > 0 {
		responce, err = ch.processToolCalls(isChat)
		if err != nil {
			return
		}
	}
	return
}

func (ch *Chat) processToolCalls(isChat bool) (string, error) {
	last := ch.msgs[len(ch.msgs)-1]
	if last.Role != "assistant" {
		return last.Content, nil
	}
	name, params, found := ExtractToolCall(last.Content)
	if !found {
		return last.Content, nil
	}
	result, err := ExecuteTool(name, params, ch.Tools)
	if err != nil {
		return "", fmt.Errorf("tool execution error: %w", err)
	}
	callStr := BuildToolCallWithParams(name, params)
	if params == "" {
		callStr = BuildToolCall(name)
	}
	ch.msgs[len(ch.msgs)-1].Content = strings.ReplaceAll(last.Content, callStr, result)
	ch.msgs = append(ch.msgs, ChatMessage{
		Role:    "system",
		Content: fmt.Sprintf("Результат выполнения инструмента `%s`: %s", name, result),
	})
	// Let the AI continue once with the tool result in context
	response, err := ch.prv.Send(ch.msgs, isChat)
	if err != nil {
		return "", err
	}
	response = strings.TrimSpace(response)
	if response == "" {
		// AI had nothing more to say; return the modified original response
		return ch.msgs[len(ch.msgs)-2].Content, nil
	}
	ch.msgs = append(ch.msgs, ChatMessage{Role: "assistant", Content: response})
	return response, nil
}
