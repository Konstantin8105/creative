package creative

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	configDir string         // set by LoadConfig; used for relative path resolution
	Provider   ProviderConfig `json:"provider"`
	Modes      []ModeConfig   `json:"modes"`
}

// ConfigDir returns the directory containing the config file.
func (c *Config) ConfigDir() string {
	return c.configDir
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

	cfg.configDir = filepath.Dir(path)

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
	}

	return &cfg, nil
}

// ResolvePrompt returns the system prompt content for this mode.
// Panics on error — if a mode cannot resolve its prompt the program must not start.
func (mc ModeConfig) ResolvePrompt(configDir string) string {
	if mc.PromptFile != "" {
		path := mc.PromptFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(configDir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			panic(fmt.Sprintf("mode %q: reading prompt_file %q: %v", mc.Name, mc.PromptFile, err))
		}
		return string(data)
	}

	// No prompt_file — look for *.promt in books_folder
	if mc.BooksFolder != "" {
		return mc.resolvePromptFromFolder(configDir)
	}

	panic(fmt.Sprintf("mode %q: no prompt source — set prompt_file or books_folder", mc.Name))
}

// resolvePromptFromFolder looks for a single *.promt file in the books folder.
func (mc ModeConfig) resolvePromptFromFolder(configDir string) string {
	folder := mc.BooksFolder
	if !filepath.IsAbs(folder) {
		folder = filepath.Join(configDir, folder)
	}

	path, err := findPromptInFolder(folder)
	if err != nil {
		panic(fmt.Sprintf("mode %q: %v", mc.Name, err))
	}
	if path == "" {
		panic(fmt.Sprintf("mode %q: no *.promt file found in books_folder %q", mc.Name, mc.BooksFolder))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("mode %q: reading %q: %v", mc.Name, path, err))
	}
	return string(data)
}

// findPromptInFolder searches for *.promt files in the given folder.
// Returns the single matching file, or an error if zero or multiple are found.
func findPromptInFolder(folder string) (string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return "", fmt.Errorf("reading books_folder %q: %w", folder, err)
	}

	var promtFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".promt") {
			promtFiles = append(promtFiles, filepath.Join(folder, e.Name()))
		}
	}

	if len(promtFiles) == 0 {
		return "", nil
	}
	if len(promtFiles) > 1 {
		return "", fmt.Errorf("multiple *.promt files found in %q: %s", folder, strings.Join(promtFiles, ", "))
	}
	return promtFiles[0], nil
}


