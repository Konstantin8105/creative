package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Konstantin8105/creative"
	"github.com/Konstantin8105/creative/internal/webserver"
)

func main() {
	log.SetOutput(os.Stdout)
	creative.LoggingEnabled = true

	var (
		configPath = flag.String("config", "", "Path to configuration JSON file (required)")
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
	webserver.Start(cfg, *port)
}

func configHelpText() string {
	cfg := creative.Config{
		Provider: creative.ProviderConfig{
			Model:           "gpt-oss:20b",
			Endpoint:        "http://localhost:1234/v1/",
			Key:             "lmstudio",
			ContextSize:     62000,
			RequestTimeout:  creative.DurationString(4 * time.Hour),
			ThinkingMode:    false,
			ReasoningEffort: "high",
			UserID:          "",
		},
		Modes: []creative.ModeConfig{
			{
				Name:       "engineer",
				Label:      "Инженерные нормативы",
				PromptFile: "./prompts/engineer.promt",
				Folders:    []string{"./books/engineer"},
			},
			{
				Name:    "simple",
				Label:   "Простой режим",
				Folders: []string{"./books/simple"},
			},
		},
	}

	data, err := json.MarshalIndent(cfg, " ", "  ")
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf(`Usage: chat.exe -config <path>

Example config.json:
%s

Flags:
  -config string
        Path to configuration JSON file (required)
  -port string
        Web server port (default "2345")
`, string(data))
}
