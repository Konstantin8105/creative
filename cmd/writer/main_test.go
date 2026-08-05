package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Konstantin8105/creative"
)

// newTestQuery returns a WriterConfig writing into a fresh temp file.
func newTestQuery(t *testing.T) WriterConfig {
	t.Helper()
	return WriterConfig{
		Query:    "Тема книги",
		Filename: filepath.Join(t.TempDir(), "book.md"),
	}
}

func TestRunQueryWritesFile(t *testing.T) {
	mock := creative.NewMockAI(creative.ChatMessage{Role: "assistant", Content: "Содержание.\nЯ закончил"})
	q := newTestQuery(t)

	if err := runQuery(mock, q, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	dat, err := os.ReadFile(q.Filename)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	out := string(dat)
	if !strings.Contains(out, "# Тема книги") {
		t.Errorf("file missing title header: %q", out)
	}
	if !strings.Contains(out, "Содержание.") {
		t.Errorf("file missing content: %q", out)
	}
}

func TestRunQuerySubtask(t *testing.T) {
	mock := creative.NewMockAI(
		creative.ChatMessage{
			Role: "assistant",
			ToolCalls: []creative.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: creative.ToolCallFunction{
					Name:      "subtask",
					Arguments: `{"description": "Опиши главу 1"}`,
				},
			}},
		},
		creative.ChatMessage{
			Role: "assistant",
			ToolCalls: []creative.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: creative.ToolCallFunction{
					Name:      "subtask",
					Arguments: `{"description": "Опиши главу 2"}`,
				},
			}},
		},
		creative.ChatMessage{Role: "assistant", Content: "Основная часть.\nЯ закончил"},
		creative.ChatMessage{Role: "assistant", Content: "Глава 1.\nЯ закончил"},
		creative.ChatMessage{Role: "assistant", Content: "Глава 2.\nЯ закончил"},
	)
	q := newTestQuery(t)

	if err := runQuery(mock, q, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	dat, err := os.ReadFile(q.Filename)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	want := "# Тема книги\n" +
		"\nОсновная часть.\nЯ закончил\n" +
		"## Тема книги\nОпиши главу 1\n" +
		"\nГлава 1.\nЯ закончил\n" +
		"## Тема книги\nОпиши главу 2\n" +
		"\nГлава 2.\nЯ закончил\n"
	if got := string(dat); got != want {
		t.Errorf("file content mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestRunQueryMultipleSubtasks(t *testing.T) {
	mock := creative.NewMockAI(
		creative.ChatMessage{
			Role: "assistant",
			ToolCalls: []creative.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: creative.ToolCallFunction{
						Name:      "subtask",
						Arguments: `{"description": "Опиши главу 1"}`,
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: creative.ToolCallFunction{
						Name:      "subtask",
						Arguments: `{"description": "Опиши главу 2"}`,
					},
				},
			},
		},
		creative.ChatMessage{Role: "assistant", Content: "Основная часть.\nЯ закончил"},
		creative.ChatMessage{Role: "assistant", Content: "Глава 1.\nЯ закончил"},
		creative.ChatMessage{Role: "assistant", Content: "Глава 2.\nЯ закончил"},
	)
	q := newTestQuery(t)

	if err := runQuery(mock, q, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	dat, err := os.ReadFile(q.Filename)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	want := "# Тема книги\n" +
		"\nОсновная часть.\nЯ закончил\n" +
		"## Тема книги\nОпиши главу 1\n" +
		"\nГлава 1.\nЯ закончил\n" +
		"## Тема книги\nОпиши главу 2\n" +
		"\nГлава 2.\nЯ закончил\n"
	if got := string(dat); got != want {
		t.Errorf("file content mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestRunQuerySubtaskToolNotProvidedAtDepthLimit(t *testing.T) {
	mock := creative.NewMockAI(creative.ChatMessage{Role: "assistant", Content: "Готово.\nЯ закончил"})
	q := newTestQuery(t)
	q.depth = maxBranchDepth

	if err := runQuery(mock, q, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	for _, name := range mock.ToolNames(0) {
		if name == "subtask" {
			t.Errorf("subtask tool provided at depth %d", maxBranchDepth)
		}
	}
}

func TestRunQuerySubtaskToolProvidedBelowDepthLimit(t *testing.T) {
	mock := creative.NewMockAI(creative.ChatMessage{Role: "assistant", Content: "Готово.\nЯ закончил"})
	q := newTestQuery(t)

	if err := runQuery(mock, q, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	found := false
	for _, name := range mock.ToolNames(0) {
		if name == "subtask" {
			found = true
		}
	}
	if !found {
		t.Errorf("subtask tool not provided at depth %d", q.depth)
	}
}

func TestRunQueryValidation(t *testing.T) {
	mock := creative.NewMockAI()

	if err := runQuery(mock, WriterConfig{Filename: "x.md"}, ""); err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Errorf("empty query error = %v", err)
	}
	if err := runQuery(mock, WriterConfig{Query: "q"}, ""); err == nil || !strings.Contains(err.Error(), "filename is required") {
		t.Errorf("empty filename error = %v", err)
	}

	q := newTestQuery(t)
	if err := os.WriteFile(q.Filename, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runQuery(mock, q, ""); err == nil || !strings.Contains(err.Error(), "файл уже существует") {
		t.Errorf("existing file error = %v", err)
	}
}

func TestRunQueryProviderError(t *testing.T) {
	mock := creative.NewMockAI()
	mock.Err = errors.New("boom")
	q := newTestQuery(t)

	if err := runQuery(mock, q, ""); err == nil {
		t.Fatal("expected error from provider")
	}
}

func TestSystemPromptRendering(t *testing.T) {
	mock := creative.NewMockAI(creative.ChatMessage{Role: "assistant", Content: "ok.\nЯ закончил"})
	q := newTestQuery(t)

	if err := runQuery(mock, q, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	if p := mock.SystemPrompt(); !strings.Contains(p, q.Query) {
		t.Errorf("system prompt missing query: %q", p)
	} else if !strings.Contains(p, "subtask") {
		t.Errorf("system prompt missing subtask section: %q", p)
	} else if strings.Contains(p, "BookTools") {
		t.Errorf("system prompt should not mention BookTools: %q", p)
	}

	mock = creative.NewMockAI(creative.ChatMessage{Role: "assistant", Content: "ok.\nЯ закончил"})
	q = newTestQuery(t)
	q.BookFolders = []string{"."}
	if err := runQuery(mock, q, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	if p := mock.SystemPrompt(); !strings.Contains(p, "BookTools") {
		t.Errorf("system prompt missing BookTools section: %q", p)
	}
}

func TestSubtaskTool(t *testing.T) {
	var subtasks []string
	tool := subtaskTool(&subtasks)

	if res := tool.Execute("not json"); !strings.HasPrefix(res, "Ошибка: неверный JSON") {
		t.Errorf("invalid JSON result = %q", res)
	}
	if res := tool.Execute(`{"description": "   "}`); res != "Ошибка: поле description не должно быть пустым" {
		t.Errorf("empty description result = %q", res)
	}
	if res := tool.Execute(`{"description": "Опиши главу"}`); !strings.Contains(res, "очередь") {
		t.Errorf("queued result = %q", res)
	}
	if len(subtasks) != 1 || subtasks[0] != "Опиши главу" {
		t.Errorf("subtasks = %v, want [Опиши главу]", subtasks)
	}
}

func TestRunUntilDone(t *testing.T) {
	// Stops at "Я закончил".
	mock := creative.NewMockAI(creative.ChatMessage{Role: "assistant", Content: "Текст.\nЯ закончил"})
	chat := creative.NewChat(mock)
	chat.AddSystem("system")
	got, err := runUntilDone(chat, "Выполни задачу.")
	if err != nil {
		t.Fatalf("runUntilDone: %v", err)
	}
	if !strings.Contains(got, "Текст.") {
		t.Errorf("result = %q", got)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("SendStream calls = %d, want 1", len(mock.Calls))
	}

	// Stops after maxContinueMessages.
	mock = creative.NewMockAI(
		creative.ChatMessage{Role: "assistant", Content: "a"},
		creative.ChatMessage{Role: "assistant", Content: "b"},
		creative.ChatMessage{Role: "assistant", Content: "c"},
		creative.ChatMessage{Role: "assistant", Content: "d"},
	)
	chat = creative.NewChat(mock)
	chat.AddSystem("system")
	got, err = runUntilDone(chat, "Выполни задачу.")
	if err != nil {
		t.Fatalf("runUntilDone: %v", err)
	}
	if got != "abcd" {
		t.Errorf("result = %q, want abcd", got)
	}
	if len(mock.Calls) != maxContinueMessages+1 {
		t.Errorf("SendStream calls = %d, want %d", len(mock.Calls), maxContinueMessages+1)
	}
}
