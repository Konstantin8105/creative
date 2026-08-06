package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Konstantin8105/creative"
)

// newTestQuery returns a WriterConfig writing into a fresh temp file.
func newTestQuery(t *testing.T) WriterConfig {
	t.Helper()
	return WriterConfig{
		Query:    QueryData{Name: "Тема книги"},
		Filename: filepath.Join(t.TempDir(), "book.md"),
	}
}

func TestRunQueryWritesFile(t *testing.T) {
	mock := creative.NewMockAI(creative.ChatMessage{Role: "assistant", Content: "Содержание.\nЯ закончил"})
	q := newTestQuery(t)

	if err := runQuery(mock, q, nil, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	dat, err := os.ReadFile(q.Filename)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	out := string(dat)
	if !strings.Contains(out, "# .0 Тема книги") {
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
					Arguments: `{"name": "Глава 1", "description": "Опиши главу 1"}`,
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
					Arguments: `{"name": "Глава 2", "description": "Опиши главу 2"}`,
				},
			}},
		},
		creative.ChatMessage{Role: "assistant", Content: "Основная часть.\nЯ закончил"},
		creative.ChatMessage{Role: "assistant", Content: "Глава 1.\nЯ закончил"},
		creative.ChatMessage{Role: "assistant", Content: "Глава 2.\nЯ закончил"},
	)
	q := newTestQuery(t)

	if err := runQuery(mock, q, nil, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	dat, err := os.ReadFile(q.Filename)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got := string(dat); got != wantBookContent() {
		t.Errorf("file content mismatch:\ngot:\n%q\nwant:\n%q", got, wantBookContent())
	}
}

func wantBookContent() string {
	return "# .0 Тема книги\n" +
		"\nОсновная часть.\nЯ закончил\n" +
		"## .0.1 Глава 1\n" +
		"Общая задача: Тема книги\n" +
		"Список всех подзадач:\n" +
		"=> Глава 1: Опиши главу 1\n" +
		"   Глава 2: Опиши главу 2\n" +
		"Реши только эту подзадачу: Глава 1\n" +
		"Опиши главу 1\n" +
		"\nГлава 1.\nЯ закончил\n" +
		"## .1.1 Глава 2\n" +
		"Общая задача: Тема книги\n" +
		"Список всех подзадач:\n" +
		"   Глава 1: Опиши главу 1\n" +
		"=> Глава 2: Опиши главу 2\n" +
		"Реши только эту подзадачу: Глава 2\n" +
		"Опиши главу 2\n" +
		"\nГлава 2.\nЯ закончил\n"
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
						Arguments: `{"name": "Глава 1", "description": "Опиши главу 1"}`,
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: creative.ToolCallFunction{
						Name:      "subtask",
						Arguments: `{"name": "Глава 2", "description": "Опиши главу 2"}`,
					},
				},
			},
		},
		creative.ChatMessage{Role: "assistant", Content: "Основная часть.\nЯ закончил"},
		creative.ChatMessage{Role: "assistant", Content: "Глава 1.\nЯ закончил"},
		creative.ChatMessage{Role: "assistant", Content: "Глава 2.\nЯ закончил"},
	)
	q := newTestQuery(t)

	if err := runQuery(mock, q, nil, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	dat, err := os.ReadFile(q.Filename)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got := string(dat); got != wantBookContent() {
		t.Errorf("file content mismatch:\ngot:\n%q\nwant:\n%q", got, wantBookContent())
	}
}

func TestRunQuerySubtaskToolNotProvidedAtDepthLimit(t *testing.T) {
	mock := creative.NewMockAI(creative.ChatMessage{Role: "assistant", Content: "Готово.\nЯ закончил"})
	q := newTestQuery(t)
	q.depth = maxBranchDepth

	if err := runQuery(mock, q, nil, ""); err != nil {
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

	if err := runQuery(mock, q, nil, ""); err != nil {
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

	if err := runQuery(mock, WriterConfig{Filename: "x.md"}, nil, ""); err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Errorf("empty query error = %v", err)
	}
	if err := runQuery(mock, WriterConfig{Query: QueryData{Name: "q"}}, nil, ""); err == nil || !strings.Contains(err.Error(), "filename is required") {
		t.Errorf("empty filename error = %v", err)
	}

	q := newTestQuery(t)
	if err := os.WriteFile(q.Filename, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runQuery(mock, q, nil, ""); err == nil || !strings.Contains(err.Error(), "файл уже существует") {
		t.Errorf("existing file error = %v", err)
	}
}

func TestRunQueryProviderError(t *testing.T) {
	mock := creative.NewMockAI()
	mock.Err = errors.New("boom")
	q := newTestQuery(t)

	if err := runQuery(mock, q, nil, ""); err == nil {
		t.Fatal("expected error from provider")
	}
}

func TestSystemPromptRendering(t *testing.T) {
	mock := creative.NewMockAI(creative.ChatMessage{Role: "assistant", Content: "ok.\nЯ закончил"})
	q := newTestQuery(t)

	if err := runQuery(mock, q, nil, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	if p := mock.SystemPrompt(); !strings.Contains(p, q.Query.Name) {
		t.Errorf("system prompt missing query: %q", p)
	} else if !strings.Contains(p, "subtask") {
		t.Errorf("system prompt missing subtask section: %q", p)
	} else if strings.Contains(p, "BookTools") {
		t.Errorf("system prompt should not mention BookTools: %q", p)
	}

	mock = creative.NewMockAI(creative.ChatMessage{Role: "assistant", Content: "ok.\nЯ закончил"})
	q = newTestQuery(t)
	q.BookFolders = []string{"."}
	if err := runQuery(mock, q, nil, ""); err != nil {
		t.Fatalf("runQuery: %v", err)
	}
	if p := mock.SystemPrompt(); !strings.Contains(p, "BookTools") {
		t.Errorf("system prompt missing BookTools section: %q", p)
	}
}

func TestSubtaskTool(t *testing.T) {
	var subtasks []QueryData
	tool := subtaskTool(&subtasks)

	if res := tool.Execute("not json"); !strings.HasPrefix(res, "Ошибка: неверный JSON") {
		t.Errorf("invalid JSON result = %q", res)
	}
	if res := tool.Execute(`{"name": "   "}`); res != "Ошибка: поле name не должно быть пустым" {
		t.Errorf("empty name result = %q", res)
	}
	if res := tool.Execute(`{"name": "Глава 1"}`); !strings.Contains(res, "очередь") {
		t.Errorf("queued result = %q", res)
	}
	if res := tool.Execute(`{"name": "Глава 2", "description": "Опиши главу 2"}`); !strings.Contains(res, "очередь") {
		t.Errorf("queued result = %q", res)
	}
	want := []QueryData{
		{Name: "Глава 1"},
		{Name: "Глава 2", Description: "Опиши главу 2"},
	}
	if !reflect.DeepEqual(subtasks, want) {
		t.Errorf("subtasks = %v, want %v", subtasks, want)
	}
}
