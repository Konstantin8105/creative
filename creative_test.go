package creative_test

import (
	"path/filepath"
	"strings"

	"github.com/Konstantin8105/creative"
)

func td(filename string) string {
	return filepath.Join("testdata", filename)
}

// TestAi is a mock implementation of creative.AIrunner for testing.
type TestAi struct {
	context int

	counter int
	rs      []string

	resp string
	err  error

	models       string
	error_models error

	// ToolCall simulation: when tools are present, the mock returns these
	// tool calls instead of text content, simulating an AI that keeps
	// requesting tool calls without producing a final answer.
	toolCallsOnToolRequest []creative.ToolCall
	// When tools are removed (nil), the mock returns this text as the
	// forced final response after iteration exhaustion.
	toolCallsFinalResponse string
}

func (ai TestAi) GetContextSize() int {
	return ai.context
}

func (ai TestAi) GetModels() (string, error) {
	return ai.models, ai.error_models
}

func (ai *TestAi) Stop() error {
	return nil
}

func (ai *TestAi) SendStream(chs []creative.ChatMessage, isChat bool, callback func(chunkType, chunk string), tools []creative.Tool) (creative.ChatMessage, error) {
	// Tools disabled (nil) and we have a forced final response prepared —
	// this is the call triggered after iteration exhaustion.
	if tools == nil && ai.toolCallsFinalResponse != "" {
		if callback != nil {
			callback("content", ai.toolCallsFinalResponse)
		}
		return creative.ChatMessage{Role: "assistant", Content: ai.toolCallsFinalResponse}, nil
	}
	// Tools present and we have tool calls to return — simulate an AI that
	// keeps requesting tool calls without producing a final answer.
	if tools != nil && len(ai.toolCallsOnToolRequest) > 0 {
		return creative.ChatMessage{
			Role:      "assistant",
			Content:   "",
			ToolCalls: ai.toolCallsOnToolRequest,
		}, nil
	}
	if 0 < len(ai.rs) {
		var full strings.Builder
		for _, r := range ai.rs {
			full.WriteString(r)
			if callback != nil {
				callback("content", r)
			}
		}
		return creative.ChatMessage{Role: "assistant", Content: full.String()}, nil
	}
	if callback != nil {
		callback("content", ai.resp)
	}
	return creative.ChatMessage{Role: "assistant", Content: ai.resp}, ai.err
}
