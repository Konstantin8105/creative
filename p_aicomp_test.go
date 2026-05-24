package creative_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Konstantin8105/creative"
)

// ---------------------------------------------------------------------------
// Unit tests with mock
// ---------------------------------------------------------------------------

func TestAiComp(t *testing.T) {
	t.Run("models", func(t *testing.T) {
		ai := TestAi{models: "gpt-4, gpt-3.5"}
		out, err := ai.GetModels()
		if err != nil {
			t.Error(err)
		}
		t.Logf("%s", out)
	})
	t.Run("chat", func(t *testing.T) {
		for _, isChat := range []bool{false, true} {
			t.Run(func() string {
				if isChat {
					return "chat"
				}
				return "generate"
			}(), func(t *testing.T) {
				ai := TestAi{resp: "42"}
				out, err := ai.Send([]creative.ChatMessage{
					{Role: "system", Content: "You the best math teacher"},
					{Role: "assistant", Content: "1+1 = ?"},
				}, isChat, nil)
				if err != nil {
					t.Error(err)
				}
				t.Logf("%s", out.Content)
			})
		}
	})
	t.Run("SendStream", func(t *testing.T) {
		ai := TestAi{rs: []string{"Hello", " ", "World", "!"}}
		out, err := ai.SendStream(nil, true, func(chunk string) {
			t.Logf("chunk: %s", chunk)
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := "Hello World!"
		if out.Content != expected {
			t.Errorf("got %q, want %q", out.Content, expected)
		}
	})
	t.Run("SendStream_empty", func(t *testing.T) {
		ai := TestAi{resp: "single response"}
		var chunks []string
		out, err := ai.SendStream(nil, true, func(chunk string) {
			chunks = append(chunks, chunk)
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if out.Content != "single response" {
			t.Errorf("got %q, want %q", out.Content, "single response")
		}
		if len(chunks) != 1 || chunks[0] != "single response" {
			t.Errorf("chunks: got %v, want [single response]", chunks)
		}
	})
	t.Run("RouterAI_type", func(t *testing.T) {
		var _ creative.AIrunner = (*creative.RouterAI)(nil)
	})
	t.Run("Stop", func(t *testing.T) {
		// Create a test HTTP server that streams slowly
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("expected http.Flusher")
			}
			for i := 0; i < 100; i++ {
				fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"chunk-%d \"}}]}\n\n", i)
				flusher.Flush()
				time.Sleep(10 * time.Millisecond)
			}
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer srv.Close()

		prv := creative.Provider{
			Endpoint:       srv.URL + "/v1",
			Model:          "test-model",
			RequestTimeout: 30 * time.Second,
			ContextSize:    4096,
		}
		ai := creative.NewRouterAI(prv)

		type result struct {
			resp creative.ChatMessage
			err  error
		}
		ch := make(chan result, 1)

		go func() {
			resp, err := ai.SendStream([]creative.ChatMessage{
				{Role: "user", Content: "test"},
			}, true, nil, nil)
			ch <- result{resp, err}
		}()

		// Let it stream a few chunks, then stop
		time.Sleep(50 * time.Millisecond)
		err := ai.Stop()
		if err != nil {
			t.Fatal(err)
		}

		r := <-ch
		if r.err == nil {
			t.Error("expected error after Stop(), got nil")
		} else {
			t.Logf("Got expected error after Stop(): %v", r.err)
		}
		if r.resp.Content == "" {
			t.Error("expected partial content after Stop()")
		} else {
			t.Logf("Partial content after Stop(): %q", r.resp.Content)
		}
	})
}

// ---------------------------------------------------------------------------
// Integration tests with LM Studio / OpenAI-compatible server
//
// Default model: openai/gpt-oss-20b
// Override with CREATIVE_MODEL environment variable.
// If the model is not found in the server model list, test is skipped.
//
// Usage:
//   go test -v -run TestLMStudio                                # uses openai/gpt-oss-20b
//   CREATIVE_MODEL="qwen/qwen2.5-coder-14b" go test -v -run TestLMStudio
//
// Environment variables:
//   CREATIVE_ENDPOINT - API endpoint (default: http://127.0.0.1:1234/v1)
//   CREATIVE_MODEL    - model name (default: openai/gpt-oss-20b)
//   CREATIVE_KEY      - API key (optional)
// ---------------------------------------------------------------------------

func TestLMStudio(t *testing.T) {
	// Configuration from environment or defaults
	endpoint := os.Getenv("CREATIVE_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:1234/v1"
	}
	modelName := os.Getenv("CREATIVE_MODEL")
	if modelName == "" {
		modelName = "openai/gpt-oss-20b"
	}
	apiKey := os.Getenv("CREATIVE_KEY")

	t.Logf("Endpoint: %s", endpoint)
	t.Logf("Model:   %s", modelName)

	prv := creative.Provider{
		Endpoint:       endpoint,
		Model:          "",
		Key:            apiKey,
		RequestTimeout: 5 * time.Minute,
		ContextSize:    4096,
	}

	// Check if server is reachable and model is available
	ai := creative.NewRouterAI(prv)
	modelsOut, err := ai.GetModels()
	if err != nil {
		t.Skipf("Server not reachable at %s: %v", endpoint, err)
	}
	t.Logf("Available models: %s", modelsOut)

	if !modelInList(modelsOut, modelName) {
		t.Skipf("Model %q not found on server", modelName)
	}

	// Set the model and run tests
	prv.Model = modelName
	ai = creative.NewRouterAI(prv)

	t.Run("Send_chat", func(t *testing.T) {
		resp, err := ai.Send([]creative.ChatMessage{
			{Role: "system", Content: "Отвечай только одним числом, без пояснений"},
			{Role: "user", Content: "Сколько будет 2+2?"},
		}, true, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Chat response: %s", resp.Content)
		if resp.Content == "" {
			t.Error("empty response")
		}
	})

	t.Run("Send_generate", func(t *testing.T) {
		resp, err := ai.Send([]creative.ChatMessage{
			{Role: "system", Content: "Отвечай только одним числом"},
			{Role: "user", Content: "Сколько будет 3*4?"},
		}, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Generate response: %s", resp.Content)
		if resp.Content == "" {
			t.Error("empty response")
		}
	})

	t.Run("SendStream_chat", func(t *testing.T) {
		var chunks []string
		resp, err := ai.SendStream([]creative.ChatMessage{
			{Role: "system", Content: "Отвечай коротко, одним словом"},
			{Role: "user", Content: "Назови столицу Франции"},
		}, true, func(chunk string) {
			chunks = append(chunks, chunk)
			t.Logf("chunk: %s", chunk)
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Full streaming response: %s", resp.Content)
		if resp.Content == "" {
			t.Error("empty streaming response")
		}
		if len(chunks) == 0 {
			t.Error("no chunks received")
		}
		assembled := strings.Join(chunks, "")
		if assembled != resp.Content {
			t.Errorf("assembled chunks != full response:\n  chunks: %q\n  resp:   %q", assembled, resp.Content)
		}
	})

	t.Run("SendStream_generate", func(t *testing.T) {
		var chunks []string
		resp, err := ai.SendStream([]creative.ChatMessage{
			{Role: "user", Content: "Напиши одно слово: привет"},
		}, false, func(chunk string) {
			chunks = append(chunks, chunk)
			t.Logf("chunk: %s", chunk)
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Full streaming (generate) response: %s", resp.Content)
		if resp.Content == "" {
			t.Error("empty streaming response")
		}
		if len(chunks) == 0 {
			t.Error("no chunks received")
		}
		assembled := strings.Join(chunks, "")
		if assembled != resp.Content {
			t.Errorf("assembled chunks != full response:\n  chunks: %q\n  resp:   %q", assembled, resp.Content)
		}
	})

	t.Run("ToolCall", func(t *testing.T) {
		ch := creative.NewChat(ai)
		ch.SetTools(creative.DefaultTools())
		ch.AddSystem(creative.ToolsPrompt(creative.DefaultTools()))

		resp, err := ch.Send("", "Который сейчас час? Используй инструмент get_current_time.", true)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("ToolCall final response: %s", resp)
		if resp == "" {
			t.Error("empty response")
		}

		str := ch.String()
		t.Logf("Chat messages: %s", str)
		if strings.Contains(str, "\"role\":\"tool\"") {
			t.Log("OK: tool get_current_time was called via native tool_calls")
		} else {
			t.Log("Note: tool was not called via native tool_calls (model may not support it)")
		}
	})

	t.Run("BookTools", func(t *testing.T) {
		// Проверяет, что AI может вызывать книжные инструменты через нативные tool_calls.

		// Устанавливаем папку с книгами на testdata
		oldFolder := creative.BooksFolder
		creative.BooksFolder = "testdata"
		defer func() { creative.BooksFolder = oldFolder }()

		ch := creative.NewChat(ai)
		allTools := append(creative.DefaultTools(), creative.BookTools()...)
		ch.SetTools(allTools)
		ch.AddSystem(creative.ToolsPrompt(allTools))

		resp, err := ch.Send("", `Выполни следующие действия:
1. Посмотри список книг.
2. Получи информацию о book_sample.txt.
3. Найди в book_sample.txt слово "Париж".`, true)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("BookTools final response: %s", resp)
		if resp == "" {
			t.Error("empty response")
		}

		str := ch.String()
		t.Logf("Chat messages: %s", str)

		if strings.Contains(str, "\"role\":\"tool\"") {
			t.Log("OK: book tools were called via native tool_calls")
		} else {
			t.Log("Note: book tools were not called via native tool_calls (model may not support tool_calls)")
		}
	})

}

// modelInList checks if the given model name exists in the JSON model list
// returned by an OpenAI-compatible /models endpoint.
func modelInList(modelsJSON, modelID string) bool {
	// Build the exact pattern: "id":"<modelID>"
	pattern := `"id":"` + modelID + `"`
	if strings.Contains(modelsJSON, pattern) {
		return true
	}
	// Also try with space: "id": "<modelID>"
	pattern2 := `"id": "` + modelID + `"`
	return strings.Contains(modelsJSON, pattern2)
}
