package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Konstantin8105/creative"
)

func main() {
	log.SetOutput(os.Stdout)

	var (
		booksDir = flag.String("books", "", "Path to the books directory (required)")
		help     = flag.Bool("help", false, "Show help")

		// AI provider configuration flags
		endpoint    = flag.String("endpoint", "http://localhost:11434/v1/", "AI API endpoint (OpenAI-compatible)")
		model       = flag.String("model", "gpt-oss:20b", "Model name for AI generation")
		key         = flag.String("key", "", "API key for external provider (optional)")
		timeout     = flag.Duration("timeout", 4*time.Hour, "Request timeout duration")
		contextSize = flag.Int("context", 62000, "AI context window size in tokens")

		// DeepSeek-specific flags
		thinkingMode    = flag.Bool("thinking", false, "Enable DeepSeek thinking mode")
		reasoningEffort = flag.String("reasoning-effort", "high", "Thinking mode effort level (high or max)")
		userID          = flag.String("user-id", "", "User ID for rate limit isolation")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nInteractive chat for book analysis.\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -books ./books -model llama3.1\n", os.Args[0])
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *booksDir == "" {
		log.Fatal("Error: Books directory is required. Use -books flag.")
	}

	// Validate books directory
	info, err := os.Stat(*booksDir)
	if err != nil || !info.IsDir() {
		log.Fatalf("Error: books directory %q is not accessible or not a directory.", *booksDir)
	}

	// Set books folder for tools
	creative.BooksFolder = *booksDir

	// Initialize AI provider
	prv := creative.Provider{
		Endpoint:        *endpoint,
		Model:           *model,
		Key:             *key,
		RequestTimeout:  *timeout,
		ContextSize:     *contextSize,
		ThinkingMode:    *thinkingMode,
		ReasoningEffort: *reasoningEffort,
		UserID:          *userID,
	}

	// Create chat with provider
	prvAI := creative.NewRouterAI(prv)
	ch := creative.NewChat(prvAI)

	// Add system prompt for book analysis
	ch.AddSystem(creative.BookSystemPrompt())

	// Combine tools: default + book tools
	tools := append(creative.DefaultTools(), creative.BookTools()...)
	ch.SetTools(tools)

	// Add tools description to system prompt
	ch.AddSystem(creative.ToolsPrompt(tools))

	// Interactive chat loop
	fmt.Printf("📚 Book Analysis Chat\n")
	fmt.Printf("Books directory: %s\n", *booksDir)
	fmt.Printf("Model: %s\n", *model)
	fmt.Printf("Type 'exit', 'quit' or Ctrl+C to stop.\n\n")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		lower := strings.ToLower(input)
		if lower == "exit" || lower == "quit" {
			fmt.Println("Bye!")
			break
		}

		// Send to AI
		resp, err := ch.Send("chat", input, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		fmt.Println(resp)
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Input error: %v", err)
	}
}
