package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Konstantin8105/creative"
	"github.com/Konstantin8105/creative/internal/webserver"
)

const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorGray   = "\033[90m"
)

func main() {
	log.SetOutput(os.Stdout)
	creative.LoggingEnabled = true

	var (
		configPath = flag.String("config", "", "Path to configuration JSON file (required)")
		webMode    = flag.Bool("web", false, "Run as web server instead of console chat")
		port       = flag.String("port", "2345", "Web server port (used with -web)")
		help       = flag.Bool("help", false, "Show help")
	)

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, configHelpText())
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *configPath == "" {
		fmt.Println(configHelpText())
		os.Exit(1)
	}

	cfg, err := creative.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Web mode: start HTTP server
	if *webMode {
		webserver.Start(cfg, *port)
		return
	}

	// CLI mode: select mode and start interactive chat
	modeNames := make([]string, len(cfg.Modes))
	for i, m := range cfg.Modes {
		modeNames[i] = m.Name
	}
	sort.Strings(modeNames)

	fmt.Println("Доступные режимы:")
	for i, name := range modeNames {
		mc := findMode(cfg, name)
		if mc == nil {
			continue
		}
		fmt.Printf("  %d: %s\n", i+1, mc.Label)
	}
	fmt.Printf("Выберите режим (1-%d) [по умолчанию 1]: ", len(modeNames))

	var choice int
	fmt.Scanf("%d", &choice)
	if choice < 1 || choice > len(modeNames) {
		choice = 1
	}
	selectedName := modeNames[choice-1]
	selectedMode := findMode(cfg, selectedName)
	if selectedMode == nil {
		log.Fatalf("Mode %q not found", selectedName)
	}

	// Resolve prompt (panics on error)
	configDir := filepath.Dir(*configPath)
	prompt := selectedMode.ResolvePrompt(configDir)

	// Create AI provider
	prvAI := creative.NewRouterAI(cfg.Provider)
	ch := creative.NewChat(prvAI)
	ch.AddSystem(prompt)
	creative.BooksFolder = selectedMode.BooksFolder

	if selectedMode.BooksFolder != "" {
		ch.SetTools(append(creative.DefaultTools(), creative.BookTools()...))
	} else {
		ch.SetTools(creative.DefaultTools())
	}

	// Interactive chat loop
	fmt.Printf("\n")
	fmt.Printf("  %s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorBold+colorBlue, colorReset)
	fmt.Printf("  %s 📚  IZYseek%s\n", colorBold+colorCyan, colorReset)
	fmt.Printf("  %s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorBold+colorBlue, colorReset)
	fmt.Printf("  %sMode:%s %s%s%s\n", colorBold, colorReset, colorGray, selectedMode.Label, colorReset)
	fmt.Printf("  %s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorBold+colorBlue, colorReset)
	fmt.Printf("  %sType '%sexit%s' or '%squit%s' to stop.%s\n\n", colorDim, colorBold+colorRed, colorDim, colorBold+colorRed, colorDim, colorReset)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("%s>%s ", colorBold+colorGreen, colorReset)
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		lower := strings.ToLower(input)
		if lower == "exit" || lower == "quit" {
			fmt.Printf("\n  %sBye! 👋%s\n\n", colorBold+colorCyan, colorReset)
			break
		}

		if lower == "/new" {
			ch = creative.NewChat(prvAI)
			ch.AddSystem(prompt)
			if selectedMode.BooksFolder != "" {
				ch.SetTools(append(creative.DefaultTools(), creative.BookTools()...))
			} else {
				ch.SetTools(creative.DefaultTools())
			}
			fmt.Printf("%s--- Новый диалог ---%s\n\n", colorBold+colorCyan, colorReset)
			continue
		}

		fmt.Printf("\n%s  🎯 %s%s%s\n\n", colorBold+colorGreen, colorReset, input, colorReset)

		_, err := ch.SendStream(input, true)
		if err != nil {
			fmt.Printf("\n%s  ⚠️  Error:%s %v%s\n\n", colorBold+colorRed, colorReset, err, colorReset)
			continue
		}

		fmt.Println()
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Input error: %v", err)
	}
}

func configHelpText() string {
	return `Usage: chat -config <path>

Example config.json:

{
    "provider": {
        "endpoint": "http://localhost:11434/v1/",
        "model": "gpt-oss:20b",
        "key": "",
        "context_size": 62000,
        "timeout": "4h",
        "thinking_mode": false,
        "reasoning_effort": "high",
        "user_id": ""
    },
    "modes": [
        {
            "name": "engineer",
            "label": "Инженерные нормативы",
            "prompt_file": "./prompts/engineer.promt",
            "books_folder": "./books/engineer"
        },
        {
            "name": "simple",
            "label": "Простой режим",
            "books_folder": "./books/simple"
        }
    ]
}

Prompt resolution rules:
  1. If prompt_file is set → read that file
  2. If prompt_file is not set → look for *.promt in books_folder
  3. If no .promt found → panic
  4. If multiple .promt found → panic
  5. If neither prompt_file nor books_folder → panic

Flags:
  -config string
        Path to configuration JSON file (required)
  -web
        Run as web server instead of console chat
  -port string
        Web server port (default "2345")
`
}

func findMode(cfg *creative.Config, name string) *creative.ModeConfig {
	for i := range cfg.Modes {
		if cfg.Modes[i].Name == name {
			return &cfg.Modes[i]
		}
	}
	return nil
}
