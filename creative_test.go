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
}

func (ai TestAi) GetContextSize() int {
	return ai.context
}

func (ai TestAi) GetModels() (string, error) {
	return ai.models, ai.error_models
}

func (ai *TestAi) Send(chs []creative.ChatMessage, isChat bool, tools []creative.Tool) (creative.ChatMessage, error) {
	if 0 < len(ai.rs) {
		defer func() {
			ai.counter++
		}()
		return creative.ChatMessage{Role: "assistant", Content: ai.rs[ai.counter]}, nil
	}
	return creative.ChatMessage{Role: "assistant", Content: ai.resp}, ai.err
}

func (ai *TestAi) Stop() error {
	return nil
}

func (ai *TestAi) SendStream(chs []creative.ChatMessage, isChat bool, callback func(chunk string), tools []creative.Tool) (creative.ChatMessage, error) {
	if 0 < len(ai.rs) {
		var full strings.Builder
		for _, r := range ai.rs {
			full.WriteString(r)
			if callback != nil {
				callback(r)
			}
		}
		return creative.ChatMessage{Role: "assistant", Content: full.String()}, nil
	}
	if callback != nil {
		callback(ai.resp)
	}
	return creative.ChatMessage{Role: "assistant", Content: ai.resp}, ai.err
}
