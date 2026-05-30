package creative

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_Valid(t *testing.T) {
	cfg, err := LoadConfig("testdata/valid.config")
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
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
