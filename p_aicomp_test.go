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
	t.Run("SendStream", func(t *testing.T) {
		ai := TestAi{rs: []string{"Hello", " ", "World", "!"}}
		out, err := ai.SendStream(nil, true, func(chunkType, chunk string) {
			t.Logf("chunk [%s]: %s", chunkType, chunk)
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
		out, err := ai.SendStream(nil, true, func(chunkType, chunk string) {
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
	t.Run("NewRouterAI", func(t *testing.T) {
		prv := creative.Provider{Model: "test-model"}
		ai := creative.NewRouterAI(prv)
		if ai == nil {
			t.Fatal("NewRouterAI returned nil")
		}
	})
	t.Run("GetContextSize", func(t *testing.T) {
		ai := creative.NewRouterAI(creative.Provider{ContextSize: 8192})
		if sz := ai.GetContextSize(); sz != 8192 {
			t.Errorf("GetContextSize = %d, want 8192", sz)
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
			// Only send a few chunks — Stop() is a no-op so the stream
			// will complete normally. 100 chunks × 10ms = 1s is unnecessary.
			for i := 0; i < 5; i++ {
				fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"chunk-%d \"}}]}\n\n", i)
				flusher.Flush()
				time.Sleep(10 * time.Millisecond)
			}
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer srv.Close()

		prv := creative.ProviderConfig{
			Endpoint:       srv.URL + "/v1",
			Model:          "test-model",
			RequestTimeout: creative.DurationString(30 * time.Second),
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

		// Let it stream a couple chunks, then stop (no-op)
		time.Sleep(50 * time.Millisecond)
		err := ai.Stop()
		if err != nil {
			t.Fatal(err)
		}

		// RouterAI.Stop() is a documented no-op (cancellation is per-request
		// via context.WithTimeout), so the stream should complete normally.
		r := <-ch
		if r.err != nil {
			t.Fatal(r.err)
		}
		t.Logf("Full response: %q", r.resp.Content)
		if r.resp.Content == "" {
			t.Error("expected non-empty content")
		} else {
			t.Logf("Content after Stop(): %q", r.resp.Content)
		}
	})
}

// ---------------------------------------------------------------------------
// RouterAI SendStream error tests with httptest
// ---------------------------------------------------------------------------

func TestRouterAI_SendStream_EmptyEndpoint(t *testing.T) {
	ai := creative.NewRouterAI(creative.Provider{Endpoint: "", Model: "test"})
	_, err := ai.SendStream(nil, true, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty endpoint")
	}
	if !strings.Contains(err.Error(), "empty endpoint") {
		t.Errorf("expected 'empty endpoint' in error, got: %v", err)
	}
}

func TestRouterAI_SendStream_EmptyModel(t *testing.T) {
	ai := creative.NewRouterAI(creative.Provider{Endpoint: "http://localhost:9999", Model: ""})
	_, err := ai.SendStream(nil, true, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty model")
	}
	if !strings.Contains(err.Error(), "empty model") {
		t.Errorf("expected 'empty model' in error, got: %v", err)
	}
}

func TestRouterAI_SendStream_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error occurred"))
	}))
	defer srv.Close()

	ai := creative.NewRouterAI(creative.ProviderConfig{
		Endpoint:       srv.URL + "/v1",
		Model:          "test-model",
		RequestTimeout: creative.DurationString(5 * time.Second),
	})
	_, err := ai.SendStream(nil, true, nil, nil)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected 'status 500' in error, got: %v", err)
	}
}

func TestRouterAI_SendStream_APIErrorInStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		fmt.Fprintf(w, "data: {\"error\":{\"message\":\"rate limit exceeded\",\"type\":\"rate_limit_error\"}}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	ai := creative.NewRouterAI(creative.ProviderConfig{
		Endpoint:       srv.URL + "/v1",
		Model:          "test-model",
		RequestTimeout: creative.DurationString(5 * time.Second),
	})
	_, err := ai.SendStream(nil, true, nil, nil)
	if err == nil {
		t.Fatal("expected API error")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("expected 'rate limit exceeded' in error, got: %v", err)
	}
}

func TestRouterAI_SendStream_DoneMarker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	ai := creative.NewRouterAI(creative.ProviderConfig{
		Endpoint:       srv.URL + "/v1",
		Model:          "test-model",
		RequestTimeout: creative.DurationString(5 * time.Second),
	})
	resp, err := ai.SendStream(nil, true, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" {
		t.Errorf("Content=%q, want %q", resp.Content, "hello")
	}
}

func TestRouterAI_SendStream_GenerateMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		fmt.Fprintf(w, "data: {\"choices\":[{\"text\":\"hello world\"}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	ai := creative.NewRouterAI(creative.ProviderConfig{
		Endpoint:       srv.URL + "/v1",
		Model:          "test-model",
		RequestTimeout: creative.DurationString(5 * time.Second),
	})
	resp, err := ai.SendStream(nil, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello world" {
		t.Errorf("Content=%q, want %q", resp.Content, "hello world")
	}
}

func TestRouterAI_SendStream_ToolCallAccumulation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		// Tool call in parts across multiple chunks
		fmt.Fprintf(w, `data: {"choices":[{"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"mock_tool","arguments":""}}]}}]}`+"\n\n")
		flusher.Flush()
		fmt.Fprintf(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{}"}}]}}]}`+"\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	ai := creative.NewRouterAI(creative.ProviderConfig{
		Endpoint:       srv.URL + "/v1",
		Model:          "test-model",
		RequestTimeout: creative.DurationString(5 * time.Second),
	})
	resp, err := ai.SendStream(
		[]creative.ChatMessage{{Role: "user", Content: "time please"}},
		true, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "mock_tool" {
		t.Errorf("tool name=%q, want %q", resp.ToolCalls[0].Function.Name, "mock_tool")
	}
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
	if testing.Short() {
		return
	}
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

	prv := creative.ProviderConfig{
		Endpoint:       endpoint,
		Model:          "",
		Key:            apiKey,
		RequestTimeout: creative.DurationString(5 * time.Minute),
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

	t.Run("SendStream_chat", func(t *testing.T) {
		var chunks []string
		resp, err := ai.SendStream([]creative.ChatMessage{
			{Role: "system", Content: "Отвечай коротко, одним словом"},
			{Role: "user", Content: "Назови столицу Франции"},
		}, true, func(chunkType, chunk string) {
			chunks = append(chunks, chunk)
			t.Logf("chunk [%s]: %s", chunkType, chunk)
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
		}, false, func(chunkType, chunk string) {
			chunks = append(chunks, chunk)
			t.Logf("chunk [%s]: %s", chunkType, chunk)
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
		oldFolder := creative.BooksFolder
		creative.BooksFolder = "testdata"
		defer func() { creative.BooksFolder = oldFolder }()

		ch := creative.NewChat(ai)
		ch.SetTools(creative.BookTools())

		resp, err := ch.SendStream("Перечисли все доступные книги, используя инструмент list_books.", true)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("ToolCall final response: %s", resp)
		if resp == "" {
			t.Error("empty response")
		}
	})

	t.Run("BookTools", func(t *testing.T) {
		oldFolder := creative.BooksFolder
		creative.BooksFolder = "testdata"
		defer func() { creative.BooksFolder = oldFolder }()

		ch := creative.NewChat(ai)
		allTools := append(creative.DefaultTools(), creative.BookTools()...)
		ch.SetTools(allTools)

		resp, err := ch.SendStream(`Выполни следующие действия:
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
