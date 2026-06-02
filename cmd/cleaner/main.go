package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Konstantin8105/creative"
)

func main() {
	configPath := flag.String("config", "", "Path to configuration JSON file (required)")
	help := flag.Bool("help", false, "Show help")
	flag.Parse()

	if *help {
		fmt.Print(`Usage: cleaner -config <path>

Cleans .txt and .md files in all books_folder directories from config:
  - Removes empty lines
  - Trims whitespace from each line (TrimSpace)
  - Removes carriage return characters (\r)
`)
		return
	}

	if *configPath == "" {
		log.Fatal("Error: -config flag is required")
	}

	cfg, err := creative.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	for _, mode := range cfg.Modes {
		for _, folder := range mode.Folders {
			if folder == "" {
				continue
			}
			folderInfo, err := os.Stat(folder)
			if err != nil {
				log.Printf("Warning: mode %q: books_folder %q not found: %v", mode.Name, folder, err)
				continue
			}
			if !folderInfo.IsDir() {
				log.Printf("Warning: mode %q: books_folder %q is not a directory", mode.Name, folder)
				continue
			}
			processFolder(mode.Name, folder)
		}
	}
}

func processFolder(modeName, folder string) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		log.Printf("Warning: mode %q: cannot read folder %q: %v", modeName, folder, err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".txt" && ext != ".md" {
			continue
		}

		filePath := filepath.Join(folder, entry.Name())
		cleanFile(modeName, filePath)
	}
}

func cleanFile(modeName, filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Warning: mode %q: cannot read %q: %v", modeName, filePath, err)
		return
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	var cleaned []string
	for _, line := range lines {
		// line = strings.ReplaceAll(line, "\r", "")
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	result := strings.Join(cleaned, "\n")
	for range 100 {
		if !strings.Contains(result, "\n\n") {
			break
		}
		result = strings.ReplaceAll(result, "\n\n", "\n")
	}

	if err := os.WriteFile(filePath, []byte(result), 0644); err != nil {
		log.Printf("Error: mode %q: cannot write %q: %v", modeName, filePath, err)
		return
	}

	fmt.Printf("  %-40s %d lines → %d lines\n", entryName(filePath), len(lines), len(cleaned))
}

func entryName(path string) string {
	base := filepath.Base(path)
	if len(base) > 40 {
		return "..." + base[len(base)-37:]
	}
	return base
}
