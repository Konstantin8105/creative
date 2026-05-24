package creative

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// MaxToolIterations is the maximum number of tool call iterations per send.
var MaxToolIterations = 5

// DebugAgentOutput controls whether agent chat state is written to .out files.
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

// processToolCalls processes tool calls in a loop.
// Each iteration: extracts all {{tool:...}} from the last assistant response,
// executes them, replaces markers with results, and sends back to AI.
// Continues until no more tool calls are found or MaxToolIterations is reached.
func (ch *Chat) processToolCalls(isChat bool) (string, error) {
	for iteration := 0; iteration < MaxToolIterations; iteration++ {
		last := ch.msgs[len(ch.msgs)-1]
		if last.Role != "assistant" {
			return last.Content, nil
		}

		// Extract ALL tool calls from the response
		calls := ExtractAllToolCalls(last.Content, MaxToolIterations)
		if len(calls) == 0 {
			return last.Content, nil
		}

		// Replace markers and execute all tools
		assistantIdx := len(ch.msgs) - 1
		content := last.Content
		for _, call := range calls {
			result, err := ExecuteTool(call.Name, call.Params, ch.Tools)
			if err != nil {
				return "", fmt.Errorf("tool execution error: %w", err)
			}
			content = strings.ReplaceAll(content, call.Raw, result)
			ch.msgs = append(ch.msgs, ChatMessage{
				Role:    "system",
				Content: fmt.Sprintf("Результат выполнения инструмента `%s`: %s", call.Name, result),
			})
		}
		ch.msgs[assistantIdx].Content = content

		// Single AI call with all tool results in context
		response, err := ch.prv.Send(ch.msgs, isChat)
		if err != nil {
			return "", err
		}
		response = strings.TrimSpace(response)
		if response == "" {
			return ch.msgs[len(ch.msgs)-2].Content, nil
		}
		ch.msgs = append(ch.msgs, ChatMessage{Role: "assistant", Content: response})

		// If no more tool calls in the response — we're done
		if !strings.Contains(response, "{{tool:") {
			return response, nil
		}
		// Otherwise, loop and process the next batch
	}

	// Max iterations reached — return last response
	last := ch.msgs[len(ch.msgs)-1]
	return last.Content, nil
}
