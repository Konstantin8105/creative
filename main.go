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
		ollama      = flag.Bool("ollama", true, "If true, then use Ollama API. If false, then use OpenAI comparable API")
		endpoint    = flag.String("endpoint", "http://localhost:11434/api/", "AI API endpoint (default: Ollama)")
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
	pr := creative.Provider{
		Endpoint:       *endpoint,
		Model:          *model,
		Key:            *key,
		RequestTimeout: *timeout,
		KeepAlive:      "48h",
		ContextSize:    *contextSize,
	}
	if *ollama {
		creative.AI = new(creative.Ollama(pr))
	} else {
		creative.AI = new(creative.RouterAI(pr))
	}

	// Create agent network
	var ntw creative.AgentNetwork

	// Add agents from definition files
	ntw.AddAgent(filepath.Join("agent", "dreamer.md"))
	ntw.AddAgent(filepath.Join("agent", "realist.md"))
	ntw.AddAgent(filepath.Join("agent", "critic.md"))
	// Optional agents (commented out):
	// ntw.AddAgent(filepath.Join("agent", "arxiv.md"))
	// ntw.AddAgent(filepath.Join("agent", "solver.md"))

	// Define communication links between agents
	// Each inner array represents a fully connected group
	ntw.Links = [][]string{{"dreamer", "realist", "critic"}}

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
	creative.MaxIterations = 2000
	creative.ReloadMailbox = *reloadMailbox

	// Run the agent network
	output, err := ntw.Run(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	// Output results
	fmt.Fprintf(os.Stdout, "%s\n", output)
}
