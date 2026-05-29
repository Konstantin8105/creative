package creative

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{
		"provider": {
			"endpoint": "http://localhost:11434/v1/",
			"model": "test-model",
			"context_size": 4096,
			"timeout": "30s"
		},
		"modes": [
			{
				"name": "engineer",
				"label": "Engineer",
				"prompt_file": "./prompts/engineer.promt",
				"books_folder": "./books"
			}
		]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}
	if cfg.configDir != dir {
		t.Errorf("configDir = %q, want %q", cfg.configDir, dir)
	}
	if cfg.Provider.Model != "test-model" {
		t.Errorf("Model = %q, want %q", cfg.Provider.Model, "test-model")
	}
	if len(cfg.Modes) != 1 {
		t.Fatalf("len(Modes) = %d, want 1", len(cfg.Modes))
	}
	if cfg.Modes[0].Name != "engineer" {
		t.Errorf("Mode.Name = %q, want %q", cfg.Modes[0].Name, "engineer")
	}
	if cfg.Modes[0].Label != "Engineer" {
		t.Errorf("Mode.Label = %q, want %q", cfg.Modes[0].Label, "Engineer")
	}
}

func TestLoadConfig_EmptyModes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{
		"provider": {"endpoint": "http://localhost:11434/v1/", "model": "test", "context_size": 4096, "timeout": "30s"},
		"modes": []
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for empty modes")
	}
}

func TestLoadConfig_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{
		"provider": {"endpoint": "x", "model": "x", "context_size": 4096, "timeout": "30s"},
		"modes": [{"label": "No Name"}]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoadConfig_MissingLabel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{
		"provider": {"endpoint": "x", "model": "x", "context_size": 4096, "timeout": "30s"},
		"modes": [{"name": "test"}]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing label")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{bad json}"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestResolvePrompt_WithPromptFile(t *testing.T) {
	dir := t.TempDir()
	promptContent := "You are a helpful assistant."
	promtPath := filepath.Join(dir, "test.promt")
	if err := os.WriteFile(promtPath, []byte(promptContent), 0644); err != nil {
		t.Fatal(err)
	}

	mc := ModeConfig{Name: "test", PromptFile: "test.promt"}
	result := mc.ResolvePrompt(dir)
	if result != promptContent {
		t.Errorf("ResolvePrompt = %q, want %q", result, promptContent)
	}
}

func TestResolvePrompt_WithPromptFile_Absolute(t *testing.T) {
	dir := t.TempDir()
	promptContent := "Absolute path prompt."
	promtPath := filepath.Join(dir, "abs.promt")
	if err := os.WriteFile(promtPath, []byte(promptContent), 0644); err != nil {
		t.Fatal(err)
	}

	mc := ModeConfig{Name: "test", PromptFile: promtPath}
	result := mc.ResolvePrompt("/some/other/dir")
	if result != promptContent {
		t.Errorf("ResolvePrompt = %q, want %q", result, promptContent)
	}
}

func TestResolvePrompt_PanicNoPromptFileNoFolder(t *testing.T) {
	mc := ModeConfig{Name: "test", Label: "Test"}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	mc.ResolvePrompt(t.TempDir())
}

func TestResolvePrompt_PanicPromptFileNotFound(t *testing.T) {
	mc := ModeConfig{Name: "test", PromptFile: "./nonexistent.promt"}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	mc.ResolvePrompt(t.TempDir())
}

func TestFindPromptInFolder_None(t *testing.T) {
	dir := t.TempDir()
	path, err := findPromptInFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("expected empty, got %q", path)
	}
}

func TestFindPromptInFolder_Single(t *testing.T) {
	dir := t.TempDir()
	promtPath := filepath.Join(dir, "test.promt")
	if err := os.WriteFile(promtPath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	path, err := findPromptInFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path != promtPath {
		t.Errorf("path = %q, want %q", path, promtPath)
	}
}

func TestFindPromptInFolder_Multiple(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.promt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.promt"), []byte("b"), 0644)
	_, err := findPromptInFolder(dir)
	if err == nil {
		t.Fatal("expected error for multiple .promt files")
	}
}

func TestFindPromptInFolder_IgnoresOtherExtensions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "doc.md"), []byte("x"), 0644)
	path, err := findPromptInFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("expected empty, got %q", path)
	}
}

func TestResolvePrompt_FromBooksFolder(t *testing.T) {
	dir := t.TempDir()
	booksDir := filepath.Join(dir, "books")
	os.MkdirAll(booksDir, 0755)
	promptContent := "You are a book assistant."
	os.WriteFile(filepath.Join(booksDir, "mode.promt"), []byte(promptContent), 0644)

	// Also put a txt file to ensure it's ignored
	os.WriteFile(filepath.Join(booksDir, "book.txt"), []byte("book content"), 0644)

	mc := ModeConfig{Name: "test", BooksFolder: "books"}
	result := mc.ResolvePrompt(dir)
	if result != promptContent {
		t.Errorf("ResolvePrompt = %q, want %q", result, promptContent)
	}
}

func TestDurationString_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		input string
		want  DurationString
	}{
		{"30s", DurationString(30 * 1000000000)},
		{"5m", DurationString(5 * 60 * 1000000000)},
		{"4h", DurationString(4 * 3600 * 1000000000)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var d DurationString
			if err := d.UnmarshalText([]byte(tt.input)); err != nil {
				t.Fatal(err)
			}
			if d != tt.want {
				t.Errorf("UnmarshalText(%q) = %v, want %v", tt.input, time.Duration(d), time.Duration(tt.want))
			}
			text, err := d.MarshalText()
			if err != nil {
				t.Fatal(err)
			}
			// MarshalText returns Go duration string like "5m0s"
			if string(text) == "" {
				t.Error("empty MarshalText result")
			}
		})
	}
}

func TestDurationString_UnmarshalInvalid(t *testing.T) {
	var d DurationString
	err := d.UnmarshalText([]byte("not-a-duration"))
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}
