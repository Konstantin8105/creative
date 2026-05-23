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

	// Command line flags with descriptions and default values
	var (
		inputFile     = flag.String("input", "", "Input file with the task (required)")
		reloadMailbox = flag.Bool("reload", true, "Reload mailbox if exists")
		help          = flag.Bool("help", false, "Show help")

		// AI provider configuration flags
		endpoint    = flag.String("endpoint", "http://localhost:11434/v1/", "AI API endpoint (OpenAI-compatible)")
		model       = flag.String("model", "gpt-oss:20b", "Model name for AI generation")
		key         = flag.String("key", "", "API key for external provider (optional)")
		timeout     = flag.Duration("timeout", 4*time.Hour, "Request timeout duration")
		contextSize = flag.Int("context", 62000, "AI context window size in tokens")
	)

	// Custom usage function
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -input task.txt -model llama3.1\n", os.Args[0])
	}

	flag.Parse()

	// Show help if requested
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Validate required input file
	if *inputFile == "" {
		log.Fatal("Error: Input file is required. Use -input flag to specify task file.")
	}

	// Initialize AI provider with configuration
	prv := creative.Provider{
		Endpoint:       *endpoint,
		Model:          *model,
		Key:            *key,
		RequestTimeout: *timeout,
		ContextSize:    *contextSize,
	}

	// Create agent network with OpenAI-compatible provider
	ntw := creative.NewMailNetwork(creative.RouterAI(prv))

	// Add agents from definition files
	ntw.AddAgent(filepath.Join("agent", "operator.md"), creative.DefaultMailPermission())
	mp := creative.DefaultMailPermission()
	mp.Solved.Other = true
	ntw.AddAgent(filepath.Join("agent", "tester.md"), mp)
	ntw.AddLinks([]string{"operator", "tester"})

	// Read task from input file
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("Error reading input file: %v", err)
	}

	input := strings.TrimSpace(string(data))
	if input == "" {
		log.Fatal("Error: Input file is empty")
	}

	// Configure global parameters
	MaxIterations := 2000
	creative.ReloadMailbox = *reloadMailbox

	// Run the agent network
	ntw.AddSystem(input)
	err = ntw.Run(MaxIterations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
