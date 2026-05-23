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

func (ai *TestAi) Send(chs []creative.ChatMessage, isChat bool) (repsonce string, err error) {
	if 0 < len(ai.rs) {
		defer func() {
			ai.counter++
		}()
		return ai.rs[ai.counter], nil
	}
	return ai.resp, ai.err
}

func (ai *TestAi) SendStream(chs []creative.ChatMessage, isChat bool, callback func(chunk string)) (repsonce string, err error) {
	if 0 < len(ai.rs) {
		var full strings.Builder
		for i, r := range ai.rs {
			full.WriteString(r)
			if callback != nil {
				callback(r)
			}
			// Simulate continue for agent iterations
			_ = i
		}
		return full.String(), nil
	}
	if callback != nil {
		callback(ai.resp)
	}
	return ai.resp, ai.err
}
