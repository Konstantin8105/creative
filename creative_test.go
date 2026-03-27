package creative_test

import (
	"path/filepath"

	"github.com/Konstantin8105/creative"
)

func td(filename string) string {
	return filepath.Join("testdata", filename)
}

type TestAi struct {
	context int

	counter int
	rs      []string

	resp string
	err  error
}

func (ai TestAi) GetContextSize() int {
	return ai.context
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
