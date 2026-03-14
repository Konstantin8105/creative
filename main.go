//go:build ignore

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
	log.SetOutput(os.Stdout)
	// Гарантируем восстановление переменной окружения
	defer creative.KeepAliveGuard()()
	creative.SetupSignalHandler()

	// Устанавливаем бесконечное удержание модели
	if err := creative.SetGlobalKeepAlive("-1"); err != nil {
		log.Fatal(err)
	}

	inputFile := flag.String("input", "", "Input file with the task (required)")
	model := flag.String("model", "gpt-oss:20b", "Ollama model name")
	help := flag.Bool("help", false, "Show help")

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

	creative.AI = new(creative.OllamaRep{
		Endpoint: "http://localhost:11434/api/generate",
		Model:    *model,
	})
	// create agents
	agents := []creative.Agent{
		creative.AgentFile(filepath.Join("agent", "dreamer.md")),
		creative.AgentFile(filepath.Join("agent", "realist.md")),
		creative.AgentFile(filepath.Join("agent", "critic.md")),
		// creative.AgentFile(filepath.Join("agent", "arxiv.md")),
		// creative.AgentFile(filepath.Join("agent", "solver.md")),
	}
	for i := range agents {
		agents[i].Other = agents
	}
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
	output := creative.Run(agents, input)
	fmt.Fprintf(os.Stdout, "%s\n", output)
}
