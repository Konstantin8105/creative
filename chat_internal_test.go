package creative

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Mock for internal (white-box) tests
// ---------------------------------------------------------------------------

// mockAI is a flexible mock for white-box testing of unexported functions.
type mockAI struct {
	context   int
	responses []ChatMessage // returned in sequence per call
	errs      []error       // returned in sequence per call
	callCount int
}

func (m *mockAI) GetContextSize() int        { return m.context }
func (m *mockAI) GetModels() (string, error) { return "", nil }
func (m *mockAI) Stop() error                { return nil }
func (m *mockAI) SendStream(_ []ChatMessage, _ bool, cb func(string, string), _ []Tool) (ChatMessage, error) {
	idx := m.callCount
	m.callCount++
	var resp ChatMessage
	if idx < len(m.responses) {
		resp = m.responses[idx]
	}
	var err error
	if idx < len(m.errs) {
		err = m.errs[idx]
	}
	if cb != nil && resp.Content != "" {
		cb("content", resp.Content)
	}
	return resp, err
}

// ---------------------------------------------------------------------------
// ensurePrv
// ---------------------------------------------------------------------------

func TestEnsurePrv_PanicsOnNil(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "provider is nil") {
			t.Fatalf("expected 'provider is nil' in panic message, got: %s", msg)
		}
	}()
	ch := &Chat{prv: nil}
	ch.ensurePrv()
}

func TestEnsurePrv_NonNil(t *testing.T) {
	ch := &Chat{prv: &mockAI{}}
	// Should NOT panic
	ch.ensurePrv()
}

// ---------------------------------------------------------------------------
// isTransientError
// ---------------------------------------------------------------------------

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"INTERNAL_ERROR", fmt.Errorf("INTERNAL_ERROR"), true},
		{"status 500", fmt.Errorf("status 500"), true},
		{"status 503", fmt.Errorf("status 503"), true},
		{"status 429", fmt.Errorf("status 429"), true},
		{"stream read error", fmt.Errorf("stream read error: connection reset"), true},
		{"stream error", fmt.Errorf("stream error: broken pipe"), true},
		{"connection refused", fmt.Errorf("connection refused"), true},
		{"timeout", fmt.Errorf("request timeout"), true},
		{"EOF", fmt.Errorf("unexpected EOF"), true},
		{"random error", fmt.Errorf("random error"), false},
		{"status 400", fmt.Errorf("status 400"), false},
		{"status 401", fmt.Errorf("status 401"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientError(tt.err)
			if got != tt.want {
				t.Errorf("isTransientError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateMessages
// ---------------------------------------------------------------------------

func TestValidateMessages(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		// Should not panic
		validateMessages(nil)
	})
	t.Run("empty", func(t *testing.T) {
		validateMessages([]ChatMessage{})
	})
	t.Run("valid_sequence", func(t *testing.T) {
		validateMessages([]ChatMessage{
			{Role: "system"},
			{Role: "user"},
			{Role: "assistant"},
			{Role: "user"},
		})
	})
	t.Run("consecutive_users", func(t *testing.T) {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer log.SetOutput(nil)

		validateMessages([]ChatMessage{
			{Role: "user"},
			{Role: "user"},
		})
		out := buf.String()
		if !strings.Contains(out, "WARN") || !strings.Contains(out, "consecutive user") {
			t.Errorf("expected warning about consecutive users, got: %s", out)
		}
	})
}

// ---------------------------------------------------------------------------
// findTool
// ---------------------------------------------------------------------------

func TestFindTool(t *testing.T) {
	tools := []Tool{
		{Name: "tool_a"},
		{Name: "tool_b"},
	}

	t.Run("found", func(t *testing.T) {
		tool, ok := findTool("tool_a", tools)
		if !ok {
			t.Fatal("expected to find tool_a")
		}
		if tool.Name != "tool_a" {
			t.Errorf("got Name=%q, want %q", tool.Name, "tool_a")
		}
	})
	t.Run("not_found", func(t *testing.T) {
		_, ok := findTool("nonexistent", tools)
		if ok {
			t.Fatal("expected not found")
		}
	})
	t.Run("empty_slice", func(t *testing.T) {
		_, ok := findTool("x", nil)
		if ok {
			t.Fatal("expected not found in nil slice")
		}
	})
}

// ---------------------------------------------------------------------------
// listToolNames
// ---------------------------------------------------------------------------

func TestListToolNames(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ch := &Chat{}
		got := ch.listToolNames()
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
	t.Run("multiple", func(t *testing.T) {
		ch := &Chat{
			Tools: []Tool{
				{Name: "alpha"},
				{Name: "beta"},
				{Name: "gamma"},
			},
		}
		got := ch.listToolNames()
		if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") || !strings.Contains(got, "gamma") {
			t.Errorf("expected all tool names, got %q", got)
		}
		if !strings.Contains(got, ", ") {
			t.Errorf("expected comma-separated, got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// buildEndpoint
// ---------------------------------------------------------------------------

func TestBuildEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		isChat   bool
		want     string
	}{
		{"chat_trailing_slash", "http://localhost:11434/api/", true, "http://localhost:11434/api/chat/completions"},
		{"chat_no_trailing_slash", "http://localhost:11434/api", true, "http://localhost:11434/api/chat/completions"},
		{"generate_trailing_slash", "http://localhost:11434/api/", false, "http://localhost:11434/api/completions"},
		{"generate_no_trailing_slash", "http://localhost:11434/api", false, "http://localhost:11434/api/completions"},
		{"chat_empty", "", true, "chat/completions"},
		{"generate_empty", "", false, "completions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &RouterAI{Provider: Provider{Endpoint: tt.endpoint}}
			got := o.buildEndpoint(tt.isChat)
			if got != tt.want {
				t.Errorf("buildEndpoint(%v) = %q, want %q", tt.isChat, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// requestBody
// ---------------------------------------------------------------------------

func TestRequestBody(t *testing.T) {
	t.Run("basic_chat", func(t *testing.T) {
		o := &RouterAI{Provider: Provider{Model: "test-model", ContextSize: 4096}}
		body := o.requestBody(
			[]ChatMessage{{Role: "user", Content: "hello"}},
			true, false, nil,
		)
		req, ok := body.(openAIRequest)
		if !ok {
			t.Fatalf("expected openAIRequest, got %T", body)
		}
		if req.Model != "test-model" {
			t.Errorf("Model=%q, want %q", req.Model, "test-model")
		}
		if req.Stream != false {
			t.Errorf("Stream=%v, want false", req.Stream)
		}
		if req.Thinking != nil {
			t.Errorf("expected nil Thinking, got %+v", req.Thinking)
		}
		if len(req.Messages) != 1 || req.Messages[0].Content != "hello" {
			t.Errorf("Messages unexpected: %+v", req.Messages)
		}
		if req.Prompt != "" {
			t.Errorf("expected empty Prompt for chat, got %q", req.Prompt)
		}
	})

	t.Run("basic_generate", func(t *testing.T) {
		o := &RouterAI{Provider: Provider{Model: "test-model", ContextSize: 4096}}
		body := o.requestBody(
			[]ChatMessage{{Role: "user", Content: "hello"}},
			false, false, nil,
		)
		req, ok := body.(openAIRequest)
		if !ok {
			t.Fatalf("expected openAIRequest, got %T", body)
		}
		if req.Prompt != "hello" {
			t.Errorf("Prompt=%q, want %q", req.Prompt, "hello")
		}
		if len(req.Messages) != 0 {
			t.Errorf("expected empty Messages for generate, got %d", len(req.Messages))
		}
	})

	t.Run("thinking_mode", func(t *testing.T) {
		o := &RouterAI{Provider: Provider{
			Model:           "test-model",
			ContextSize:     4096,
			ThinkingMode:    true,
			ReasoningEffort: "max",
		}}
		body := o.requestBody(nil, true, true, nil)
		req, ok := body.(openAIRequest)
		if !ok {
			t.Fatalf("expected openAIRequest, got %T", body)
		}
		if req.Thinking == nil || req.Thinking.Type != "enabled" {
			t.Errorf("expected Thinking enabled, got %+v", req.Thinking)
		}
		if req.ReasoningEffort != "max" {
			t.Errorf("ReasoningEffort=%q, want %q", req.ReasoningEffort, "max")
		}
	})

	t.Run("thinking_mode_default_effort", func(t *testing.T) {
		o := &RouterAI{Provider: Provider{
			Model:        "test-model",
			ContextSize:  4096,
			ThinkingMode: true,
			// ReasoningEffort is empty — should default to "high"
		}}
		body := o.requestBody(nil, true, true, nil)
		req, ok := body.(openAIRequest)
		if !ok {
			t.Fatalf("expected openAIRequest, got %T", body)
		}
		if req.ReasoningEffort != "high" {
			t.Errorf("ReasoningEffort=%q, want %q", req.ReasoningEffort, "high")
		}
	})

	t.Run("user_id", func(t *testing.T) {
		o := &RouterAI{Provider: Provider{
			Model:  "test-model",
			UserID: "user-abc",
		}}
		body := o.requestBody(nil, true, true, nil)
		req, ok := body.(openAIRequest)
		if !ok {
			t.Fatalf("expected openAIRequest, got %T", body)
		}
		if req.UserID != "user-abc" {
			t.Errorf("UserID=%q, want %q", req.UserID, "user-abc")
		}
	})

	t.Run("with_tools", func(t *testing.T) {
		o := &RouterAI{Provider: Provider{Model: "test-model"}}
		tools := []Tool{
			{
				Name:        "my_tool",
				Description: "A test tool",
				Parameters: &ToolParameters{
					Type:       "object",
					Properties: map[string]ToolProperty{},
					Required:   []string{},
				},
			},
		}
		body := o.requestBody(nil, true, true, tools)
		req, ok := body.(openAIRequest)
		if !ok {
			t.Fatalf("expected openAIRequest, got %T", body)
		}
		if len(req.Tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(req.Tools))
		}
		if req.ToolChoice != "auto" {
			t.Errorf("ToolChoice=%v, want 'auto'", req.ToolChoice)
		}
	})

	t.Run("stream_default", func(t *testing.T) {
		o := &RouterAI{Provider: Provider{Model: "test-model"}}
		body := o.requestBody(nil, true, true, nil)
		req, ok := body.(openAIRequest)
		if !ok {
			t.Fatalf("expected openAIRequest, got %T", body)
		}
		if req.Stream != true {
			t.Errorf("Stream=%v, want true", req.Stream)
		}
	})
}

// ---------------------------------------------------------------------------
// finalizeToolCalls
// ---------------------------------------------------------------------------

func TestFinalizeToolCalls(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result := finalizeToolCalls(nil)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})
	t.Run("empty", func(t *testing.T) {
		result := finalizeToolCalls(map[int]*ToolCall{})
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})
	t.Run("single", func(t *testing.T) {
		acc := map[int]*ToolCall{
			0: {ID: "call_1", Function: ToolCallFunction{Name: "foo"}},
		}
		result := finalizeToolCalls(acc)
		if len(result) != 1 {
			t.Fatalf("expected 1, got %d", len(result))
		}
		if result[0].ID != "call_1" {
			t.Errorf("ID=%q, want %q", result[0].ID, "call_1")
		}
	})
	t.Run("out_of_order", func(t *testing.T) {
		acc := map[int]*ToolCall{
			2: {ID: "call_3"},
			0: {ID: "call_1"},
			1: {ID: "call_2"},
		}
		result := finalizeToolCalls(acc)
		if len(result) != 3 {
			t.Fatalf("expected 3, got %d", len(result))
		}
		if result[0].ID != "call_1" || result[1].ID != "call_2" || result[2].ID != "call_3" {
			t.Errorf("expected sorted order [call_1, call_2, call_3], got %+v", result)
		}
	})
}

// ---------------------------------------------------------------------------
// formatFileSize
// ---------------------------------------------------------------------------

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"kilobytes", 2048, "2.0 KB"},
		{"kilobytes_exact", 1024, "1.0 KB"},
		{"megabytes", 1048576, "1.0 MB"},
		{"megabytes_fraction", 1572864, "1.5 MB"}, // 1.5 MB
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFileSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatFileSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// splitLastMode
// ---------------------------------------------------------------------------

func TestSplitLastMode(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantMode    string
		wantPattern string
	}{
		{"empty", "", "", ""},
		{"single_word", "hello", "", "hello"},
		{"two_words", "hello world", "", "hello world"},
		{"keyword_mode", "hello keyword", "keyword", "hello"},
		{"regex_mode", "hello regex", "regex", "hello"},
		{"multi_word_regex", "hello world regex", "regex", "hello world"},
		{"case_insensitive_keyword", "hello KEYWORD", "keyword", "hello"},
		{"case_insensitive_regex", "hello REGEX", "regex", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, pattern := splitLastMode(tt.input)
			if mode != tt.wantMode {
				t.Errorf("mode=%q, want %q", mode, tt.wantMode)
			}
			if pattern != tt.wantPattern {
				t.Errorf("pattern=%q, want %q", pattern, tt.wantPattern)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// looksLikeRegex
// ---------------------------------------------------------------------------

func TestLooksLikeRegex(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		{"simple_word", "hello", false},
		{"dot", ".", true},
		{"star", ".*", true},
		{"plus", "a+", true},
		{"brackets", "[abc]", true},
		{"parens", "(abc)", true},
		{"caret", "^Глава", true},
		{"dollar", "end$", true},
		{"backslash", "\\d+", true},
		{"pipe_only", "hello|world", false},
		{"pipe_and_dot", "hello|wor.d", true},
		{"hyphen", "Серое-небо", false},
		{"question_mark", "colou?r", true},
		{"curly_braces", "a{1,2}", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeRegex(tt.pattern)
			if got != tt.want {
				t.Errorf("looksLikeRegex(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// splitOR
// ---------------------------------------------------------------------------

func TestSplitOR(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    []string
	}{
		{"empty", "", []string{""}},
		{"single", "hello", []string{"hello"}},
		{"two_parts", "hello|world", []string{"hello", "world"}},
		{"three_parts", "a|b|c", []string{"a", "b", "c"}},
		{"double_pipe", "a||b", []string{"a", "b"}},
		{"leading_pipe", "|hello", []string{"hello"}},
		{"trailing_pipe", "hello|", []string{"hello"}},
		{"with_spaces", "hello world|foo bar", []string{"hello world", "foo bar"}},
		{"trim_spaces", "  a  |  b  ", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitOR(tt.pattern)
			if len(got) != len(tt.want) {
				t.Fatalf("len=%d, want %d; got %v, want %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d]=%q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// matchesAnyOR
// ---------------------------------------------------------------------------

func TestMatchesAnyOR(t *testing.T) {
	t.Run("single_match", func(t *testing.T) {
		if !matchesAnyOR("hello world", []string{"hello"}) {
			t.Error("expected match")
		}
	})
	t.Run("single_no_match", func(t *testing.T) {
		if matchesAnyOR("hello world", []string{"xyz"}) {
			t.Error("expected no match")
		}
	})
	t.Run("multiple_match_first", func(t *testing.T) {
		if !matchesAnyOR("hello world", []string{"hello", "world"}) {
			t.Error("expected match on first")
		}
	})
	t.Run("multiple_match_second", func(t *testing.T) {
		if !matchesAnyOR("hello world", []string{"xyz", "world"}) {
			t.Error("expected match on second")
		}
	})
	t.Run("multiple_no_match", func(t *testing.T) {
		if matchesAnyOR("hello world", []string{"xyz", "abc"}) {
			t.Error("expected no match")
		}
	})
}

// ---------------------------------------------------------------------------
// retrySendStream
// ---------------------------------------------------------------------------

func TestRetrySendStream_SuccessFirstAttempt(t *testing.T) {
	mock := &mockAI{
		responses: []ChatMessage{{Role: "assistant", Content: "ok"}},
	}
	ch := &Chat{prv: mock}
	MaxSendRetries = 2

	msg, err := ch.retrySendStream(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "ok" {
		t.Errorf("Content=%q, want %q", msg.Content, "ok")
	}
}

func TestRetrySendStream_TransientThenSuccess(t *testing.T) {
	mock := &mockAI{
		responses: []ChatMessage{
			{Role: "assistant", Content: ""},            // first call fails
			{Role: "assistant", Content: "after retry"}, // retry succeeds
		},
		errs: []error{fmt.Errorf("INTERNAL_ERROR"), nil},
	}
	ch := &Chat{prv: mock}
	MaxSendRetries = 5

	msg, err := ch.retrySendStream(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "after retry" {
		t.Errorf("Content=%q, want %q", msg.Content, "after retry")
	}
	if mock.callCount != 2 {
		t.Errorf("expected 2 calls, got %d", mock.callCount)
	}
}

func TestRetrySendStream_AllRetriesExhausted(t *testing.T) {
	mock := &mockAI{
		errs: []error{fmt.Errorf("INTERNAL_ERROR"), fmt.Errorf("INTERNAL_ERROR"), fmt.Errorf("INTERNAL_ERROR")},
	}
	ch := &Chat{prv: mock}
	MaxSendRetries = 2 // 3 total attempts

	_, err := ch.retrySendStream(true, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "retries") {
		t.Errorf("expected 'retries' in error, got: %v", err)
	}
}

func TestRetrySendStream_PermanentError(t *testing.T) {
	mock := &mockAI{
		errs: []error{fmt.Errorf("status 400")},
	}
	ch := &Chat{prv: mock}
	MaxSendRetries = 5

	_, err := ch.retrySendStream(true, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "retries") {
		t.Errorf("should NOT retry permanent error, got: %v", err)
	}
}

func TestRetrySendStream_OnRetryCallback(t *testing.T) {
	mock := &mockAI{
		responses: []ChatMessage{{Content: "final"}},
		errs:      []error{fmt.Errorf("INTERNAL_ERROR"), nil},
	}
	ch := &Chat{prv: mock, callback: &ChatEventCallback{}}
	MaxSendRetries = 5

	var retryCalled bool
	ch.callback.OnRetry = func(attempt int, err error) {
		retryCalled = true
		if attempt != 1 {
			t.Errorf("attempt=%d, want 1", attempt)
		}
		if err == nil || err.Error() != "INTERNAL_ERROR" {
			t.Errorf("unexpected err: %v", err)
		}
	}

	_, err := ch.retrySendStream(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !retryCalled {
		t.Error("OnRetry callback was not called")
	}
}

func TestRetrySendStream_NegativeRetries(t *testing.T) {
	mock := &mockAI{
		responses: []ChatMessage{{Role: "assistant", Content: "ok"}},
	}
	ch := &Chat{prv: mock}
	MaxSendRetries = -1 // should clamp to 1 attempt

	msg, err := ch.retrySendStream(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "ok" {
		t.Errorf("Content=%q, want %q", msg.Content, "ok")
	}
	if mock.callCount != 1 {
		t.Errorf("expected 1 call with negative retries, got %d", mock.callCount)
	}
}

// ---------------------------------------------------------------------------
// processToolCalls
// ---------------------------------------------------------------------------

func TestProcessToolCalls_NoToolCalls(t *testing.T) {
	ch := &Chat{
		prv:  &mockAI{},
		msgs: []ChatMessage{{Role: "assistant", Content: "hello"}},
	}
	result, err := ch.processToolCalls(true)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Errorf("got %q, want %q", result, "hello")
	}
}

func TestProcessToolCalls_ExecutesToolAndReturns(t *testing.T) {
	toolExecuted := false
	tools := []Tool{
		{
			Name: "test_tool",
			Execute: func(params string) string {
				toolExecuted = true
				return "tool_result_ok"
			},
		},
	}
	mock := &mockAI{
		responses: []ChatMessage{
			{Role: "assistant", Content: "final answer"},
		},
	}
	ch := &Chat{prv: mock, Tools: tools,
		msgs: []ChatMessage{
			{Role: "user", Content: "use tool"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "test_tool", Arguments: "{}"}},
				},
			},
		},
	}

	result, err := ch.processToolCalls(true)
	if err != nil {
		t.Fatal(err)
	}
	if result != "final answer" {
		t.Errorf("got %q, want %q", result, "final answer")
	}
	if !toolExecuted {
		t.Error("tool was not executed")
	}
}

func TestProcessToolCalls_UnknownTool(t *testing.T) {
	mock := &mockAI{
		responses: []ChatMessage{
			{Role: "assistant", Content: "sorry, unknown tool"},
		},
	}
	ch := &Chat{prv: mock, Tools: []Tool{},
		msgs: []ChatMessage{
			{Role: "user", Content: "use tool"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "nonexistent", Arguments: "{}"}},
				},
			},
		},
	}

	result, err := ch.processToolCalls(true)
	if err != nil {
		t.Fatal(err)
	}
	if result != "sorry, unknown tool" {
		t.Errorf("got %q, want %q", result, "sorry, unknown tool")
	}
	// Check that error was sent as tool result
	// After processToolCalls, msgs should be: user, assistant(tool_calls), tool, assistant(response)
	lastMsg := ch.msgs[len(ch.msgs)-2] // second-to-last before final assistant
	if lastMsg.Role != "tool" {
		t.Errorf("expected tool role, got %q", lastMsg.Role)
	}
	if !strings.Contains(lastMsg.Content, "not found") {
		t.Errorf("expected 'not found' in tool result, got: %s", lastMsg.Content)
	}
}

func TestProcessToolCalls_CallbackFired(t *testing.T) {
	var toolCallFired, toolResultFired bool
	tools := []Tool{
		{
			Name: "test_tool",
			Execute: func(params string) string {
				return "result_data"
			},
		},
	}
	mock := &mockAI{
		responses: []ChatMessage{
			{Role: "assistant", Content: "done"},
		},
	}
	ch := &Chat{prv: mock, Tools: tools,
		msgs: []ChatMessage{
			{Role: "user", Content: "hello"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "test_tool", Arguments: `{"key":"val"}`}},
				},
			},
		},
		callback: &ChatEventCallback{
			OnToolCall: func(name, args string) {
				toolCallFired = true
			},
			OnToolResult: func(name, result string) {
				toolResultFired = true
			},
		},
	}

	_, err := ch.processToolCalls(true)
	if err != nil {
		t.Fatal(err)
	}
	if !toolCallFired {
		t.Error("OnToolCall was not fired")
	}
	if !toolResultFired {
		t.Error("OnToolResult was not fired")
	}
}

func TestProcessToolCalls_ErrorRollback(t *testing.T) {
	tools := []Tool{
		{
			Name: "test_tool",
			Execute: func(params string) string {
				return "result"
			},
		},
	}
	mock := &mockAI{
		responses: []ChatMessage{
			{}, // second call will fail (response to tool results)
		},
		errs: []error{fmt.Errorf("stream error: connection lost")},
	}
	beforeMsgs := []ChatMessage{
		{Role: "user", Content: "hello"},
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "test_tool", Arguments: "{}"}},
			},
		},
	}
	ch := &Chat{prv: mock, Tools: tools,
		msgs: append([]ChatMessage(nil), beforeMsgs...),
	}
	beforeLen := len(ch.msgs)

	_, err := ch.processToolCalls(true)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Rollback: msgs should be back to initial state (before tool results were added)
	if len(ch.msgs) != beforeLen {
		t.Errorf("expected rollback to %d messages, got %d", beforeLen, len(ch.msgs))
	}
}

func TestProcessToolCalls_EmptyContentNoToolCalls(t *testing.T) {
	// When AI returns empty content AND no tool calls, fall back to the
	// previous assistant message with content.
	mock := &mockAI{
		responses: []ChatMessage{
			{Role: "assistant", Content: "", ToolCalls: nil}, // empty response after tool result
		},
	}
	tools := []Tool{
		{
			Name:    "test_tool",
			Execute: func(params string) string { return "res" },
		},
	}
	ch := &Chat{prv: mock, Tools: tools,
		msgs: []ChatMessage{
			{Role: "user", Content: "use tool"},
			{Role: "assistant", Content: "previous content", ToolCalls: []ToolCall{
				{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "test_tool", Arguments: "{}"}},
			}},
		},
	}

	result, err := ch.processToolCalls(true)
	if err != nil {
		t.Fatal(err)
	}
	if result != "previous content" {
		t.Errorf("expected fallback to 'previous content', got %q", result)
	}
}

func TestProcessToolCalls_MaxIterations(t *testing.T) {
	oldMax := MaxToolIterations
	MaxToolIterations = 1
	defer func() { MaxToolIterations = oldMax }()

	mock := &mockAI{
		responses: []ChatMessage{
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "test_tool", Arguments: "{}"}},
				},
			},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_2", Type: "function", Function: ToolCallFunction{Name: "test_tool", Arguments: "{}"}},
				},
			},
		},
	}
	tools := []Tool{
		{
			Name:    "test_tool",
			Execute: func(params string) string { return "res" },
		},
	}
	ch := &Chat{prv: mock, Tools: tools,
		msgs: []ChatMessage{{Role: "user", Content: "loop"}},
	}

	result, err := ch.processToolCalls(true)
	if err != nil {
		t.Fatal(err)
	}
	// MaxToolIterations=1 means one loop, returns last content (from last assistant msg which has no content since it has tool_calls)
	_ = result
}

// ---------------------------------------------------------------------------
// SendStream (indirectly via processToolCalls)
// ---------------------------------------------------------------------------

func TestSendStream_Basic(t *testing.T) {
	mock := &mockAI{
		responses: []ChatMessage{{Role: "assistant", Content: "response text"}},
	}
	ch := NewChat(mock)

	resp, err := ch.SendStream("hello", true)
	if err != nil {
		t.Fatal(err)
	}
	if resp != "response text" {
		t.Errorf("got %q, want %q", resp, "response text")
	}
	// Check message history
	if len(ch.msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(ch.msgs))
	}
	if ch.msgs[0].Role != "user" || ch.msgs[0].Content != "hello" {
		t.Errorf("msg[0] unexpected: %+v", ch.msgs[0])
	}
	if ch.msgs[1].Role != "assistant" || ch.msgs[1].Content != "response text" {
		t.Errorf("msg[1] unexpected: %+v", ch.msgs[1])
	}
}

func TestSendStream_WithSystem(t *testing.T) {
	mock := &mockAI{
		responses: []ChatMessage{{Role: "assistant", Content: "ok"}},
	}
	ch := NewChat(mock)
	ch.AddSystem("You are a helpful assistant.")

	resp, err := ch.SendStream("hello", true)
	if err != nil {
		t.Fatal(err)
	}
	if resp != "ok" {
		t.Errorf("got %q, want %q", resp, "ok")
	}
	// First message should be system
	if len(ch.msgs) != 3 {
		t.Fatalf("expected 3 messages (system+user+assistant), got %d", len(ch.msgs))
	}
	if ch.msgs[0].Role != "system" || !strings.Contains(ch.msgs[0].Content, "helpful") {
		t.Errorf("msg[0] unexpected: %+v", ch.msgs[0])
	}
}

func TestSendStream_ErrorRollback(t *testing.T) {
	mock := &mockAI{
		errs: []error{fmt.Errorf("status 500")},
	}
	ch := NewChat(mock)

	_, err := ch.SendStream("hello", true)
	if err == nil {
		t.Fatal("expected error")
	}
	// On error, msgs should roll back to before user message (to empty)
	if len(ch.msgs) != 0 {
		t.Fatalf("expected 0 messages after rollback, got %d: %+v", len(ch.msgs), ch.msgs)
	}
}

func TestSendStream_CallbackRouting(t *testing.T) {
	mock := &mockAI{
		responses: []ChatMessage{{Role: "assistant", Content: "chunked response"}},
	}
	ch := NewChat(mock)

	var chunks []string
	var reasoning []string
	ch.SetCallback(&ChatEventCallback{
		OnStreamChunk: func(s string) { chunks = append(chunks, s) },
		OnReasoning:   func(s string) { reasoning = append(reasoning, s) },
	})

	resp, err := ch.SendStream("hello", true)
	if err != nil {
		t.Fatal(err)
	}
	if resp != "chunked response" {
		t.Errorf("got %q, want %q", resp, "chunked response")
	}
	if len(chunks) == 0 {
		t.Error("expected stream chunks")
	}
}

// ---------------------------------------------------------------------------
// SetCallback
// ---------------------------------------------------------------------------

func TestSetCallback(t *testing.T) {
	ch := NewChat(&mockAI{})
	if ch.callback != nil {
		t.Fatal("expected nil callback initially")
	}
	cb := &ChatEventCallback{OnStreamChunk: func(s string) {}}
	ch.SetCallback(cb)
	if ch.callback != cb {
		t.Fatal("SetCallback did not set the callback")
	}
	ch.SetCallback(nil)
	if ch.callback != nil {
		t.Fatal("SetCallback(nil) did not clear the callback")
	}
}

// ---------------------------------------------------------------------------
// Stop
// ---------------------------------------------------------------------------

func TestChatStop(t *testing.T) {
	ch := NewChat(&mockAI{})
	if err := ch.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}
