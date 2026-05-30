package creative

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ModeConfig represents a single chat mode with its label, prompt source, and books folder.
type ModeConfig struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	PromptFile  string `json:"prompt_file,omitempty"`
	BooksFolder string `json:"books_folder,omitempty"`
}

// Config is the top-level configuration loaded from a JSON file.
type Config struct {
	Provider ProviderConfig `json:"provider"`
	Modes    []ModeConfig   `json:"modes"`
}

// LoadConfig reads and parses a JSON configuration file from the given path.
// It validates that at least one mode is defined and sets ModeConfig.Name
// from the JSON "name" field. The configDir is stored for relative path resolution.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if len(cfg.Modes) == 0 {
		return nil, fmt.Errorf("config: at least one mode must be defined")
	}

	// Validate each mode
	for i, m := range cfg.Modes {
		if m.Name == "" {
			return nil, fmt.Errorf("config: modes[%d].name is required", i)
		}
		if m.Label == "" {
			return nil, fmt.Errorf("config: mode %q: label is required", m.Name)
		}
		if m.BooksFolder == "" {
			return nil, fmt.Errorf("config: mode %q: empty folder", m.Name)
		}
		if _, err := os.Stat(m.BooksFolder); os.IsNotExist(err) {
			return nil, fmt.Errorf("config: mode %q: folder is not exist", m.Name)
		}
	}

	// find prompt
	for i, m := range cfg.Modes {
		if m.PromptFile != "" {
			path := m.PromptFile
			_, err := os.ReadFile(path)
			if err != nil {
				panic(fmt.Errorf("mode `%s`: not found promt", m.Name))
			}
			continue
		}
		// search prompt
		files, err := filepath.Glob(filepath.Join(m.BooksFolder, "*.promt"))
		if err != nil {
			panic(fmt.Errorf("find prompt: %v", err))
		}
		if len(files) != 1 {
			panic(fmt.Errorf("find prompt: not valid amount prompts %d", len(files)))
		}
		path := files[0]
		_, err = os.ReadFile(path)
		if err != nil {
			panic(fmt.Errorf("mode `%s`: not found promt", m.Name))
		}
		cfg.Modes[i].PromptFile = path
	}

	return &cfg, nil
}

func (m ModeConfig) GetPrompt() string {
	data, err := os.ReadFile(m.PromptFile)
	if err != nil {
		panic(fmt.Errorf("mode `%s`: not found promt", m.Name))
	}
	return string(data)
}
