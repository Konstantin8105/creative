//go:build ignore

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Konstantin8105/creative"
)

func main() {
	log.SetOutput(os.Stdout)
	// Гарантируем восстановление переменной окружения
	// defer creative.KeepAliveGuard()()
	// creative.SetupSignalHandler()
	// // Устанавливаем бесконечное удержание модели
	// if err := creative.SetGlobalKeepAlive("-1"); err != nil {
	// 	log.Fatal(err)
	// }
	var (
		inputFile     = flag.String("input", "", "Input file with the task (required)")
		reloadMailbox = flag.Bool("reload", true, "Reload mailbox if exist")
		help          = flag.Bool("help", false, "Show help")

		// AI provider flags
		endpoint    = flag.String("endpoint", "http://localhost:11434/api/", "AI API endpoint (by default Ollama)")
		model       = flag.String("model", "gpt-oss:20b", "Model name")
		key         = flag.String("key", "", "API key for external provider")
		timeout     = flag.Duration("timeout", 4*time.Hour, "Request timeout")
		contextSize = flag.Int("context", 62000, "size AI context")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}
	if *inputFile == "" {
		log.Fatal("Input file required")
	}

	creative.AI = new(creative.Ollama{
		Endpoint:       *endpoint,
		Model:          *model,
		Key:            *key,
		RequestTimeout: *timeout,
		KeepAlive:      "48h",
		ContextSize:    *contextSize,
	})
	// create agents
	var ntw creative.AgentNetwork
	// add agents
	ntw.AddAgent(filepath.Join("agent", "dreamer.md"))
	ntw.AddAgent(filepath.Join("agent", "realist.md"))
	ntw.AddAgent(filepath.Join("agent", "critic.md"))
	// ntw.AddAgent(filepath.Join("agent", "arxiv.md"))
	// ntw.AddAgent(filepath.Join("agent", "solver.md"))
	// add links
	ntw.Links = [][]string{{"dreamer", "realist", "critic"}}
	// Чтение задания из файла
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("Error reading input file: %v", err)
	}
	input := strings.TrimSpace(string(data))
	if input == "" {
		log.Fatal("Input file is empty")
	}
	// run
	creative.MaxIterations = 2000
	creative.ReloadMailbox = *reloadMailbox
	output, err := ntw.Run(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
	}
	fmt.Fprintf(os.Stdout, "%s\n", output)
}
