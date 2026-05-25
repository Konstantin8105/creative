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
	"github.com/Konstantin8105/creative/internal/webserver"
)

// ANSI color codes for terminal output
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

	var (
		booksDir = flag.String("books", "", "Path to the books directory (required)")
		help     = flag.Bool("help", false, "Show help")

		// Mode selection
		webMode = flag.Bool("web", false, "Run as web server instead of console chat")
		port    = flag.String("port", "2345", "Web server port (used with -web)")

		// AI provider configuration flags
		endpoint    = flag.String("endpoint", "http://localhost:11434/v1/", "AI API endpoint (OpenAI-compatible)")
		model       = flag.String("model", "gpt-oss:20b", "Model name for AI generation")
		key         = flag.String("key", "", "API key for external provider (optional)")
		timeout     = flag.Duration("timeout", 4*time.Hour, "Request timeout duration")
		contextSize = flag.Int("context", 62000, "AI context window size in tokens")

		// Tool result display
		fullResult = flag.Bool("full-result", false, "Show full tool results without truncation")

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

	// Set tool result preview length (0 = full output)
	if *fullResult {
		creative.ToolResultMaxPreview = 0
	}

	// Web mode: start HTTP server instead of console chat
	if *webMode {
		log.Printf("Starting web server on :%s", *port)
		webserver.Start(prvAI, tools, *port)
		return
	}

	// Set up beautiful streaming callbacks
	ch.SetCallback(&creative.ChatEventCallback{
		OnStreamChunk: func(chunk string) {
			fmt.Print(colorCyan + chunk + colorReset)
		},
		OnReasoning: func(text string) {
			fmt.Print(colorDim + colorGray + text + colorReset)
		},
		OnToolCall: func(name, args string) {
			// Pretty-print the JSON args
			prettyArgs := args
			if strings.HasPrefix(args, "{") {
				prettyArgs = strings.ReplaceAll(args, "\"", "")
				prettyArgs = strings.ReplaceAll(prettyArgs, "{", "")
				prettyArgs = strings.ReplaceAll(prettyArgs, "}", "")
				prettyArgs = strings.ReplaceAll(prettyArgs, ",", ", ")
			}
			fmt.Printf("\n%s🔧 %sTool: %s%s(%s%s%s)%s\n",
				colorReset,
				colorBold,
				colorBlue, name,
				colorYellow, prettyArgs,
				colorReset,
				colorReset,
			)
		},
		OnToolResult: func(name, result string) {
			preview := result
			maxPreview := creative.ToolResultMaxPreview
			if maxPreview < 0 {
				maxPreview = 0
			}
			isTruncated := maxPreview > 0 && len(result) > maxPreview
			if isTruncated {
				preview = result[:maxPreview] + "... " + colorGray + "[truncated]" + colorReset
				// Replace newlines for compact single-line display
				preview = strings.ReplaceAll(preview, "\n", " ↵ ")
				fmt.Printf("%s  %s✅ %s%s → %s%s\n",
					colorReset,
					colorBold,
					colorGreen, name,
					colorReset, preview,
				)
			} else {
				fmt.Printf("%s  %s✅ %s%s →%s\n%s\n",
					colorReset,
					colorBold,
					colorGreen, name,
					colorReset,
					preview,
				)
			}
		},
	})

	// Interactive chat loop
	fmt.Printf("\n")
	fmt.Printf("  %s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorBold+colorBlue, colorReset)
	fmt.Printf("  %s 📚  IZYseek%s\n", colorBold+colorCyan, colorReset)
	fmt.Printf("  %s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorBold+colorBlue, colorReset)
	fmt.Printf("  %sBooks:%s %s%s%s\n", colorBold, colorReset, colorGray, *booksDir, colorReset)
	fmt.Printf("  %sModel:%s %s%s%s\n", colorBold, colorReset, colorGray, *model, colorReset)
	if *thinkingMode {
		fmt.Printf("  %sThinking:%s %senabled (effort: %s)%s\n", colorBold, colorReset, colorGray, *reasoningEffort, colorReset)
	}
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

		// Print user message
		fmt.Printf("\n%s  🎯 %s%s%s\n\n", colorBold+colorGreen, colorReset, input, colorReset)

		// Send to AI with streaming
		_, err := ch.SendStream(input, true)
		if err != nil {
			fmt.Printf("\n%s  ⚠️  Error:%s %v%s\n\n", colorBold+colorRed, colorReset, err, colorReset)
			continue
		}

		// Print final blank line after response
		fmt.Println()
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Input error: %v", err)
	}
}
