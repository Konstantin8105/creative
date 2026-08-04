package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Konstantin8105/creative"
)

//go:embed prompts/decompose.promt
var decomposePrompt string

//go:embed prompts/writer.promt
var writerPromptTemplate string

// maxContinueMessages prevents infinite loop in runUntilDone.
var maxContinueMessages = 20

type WriterConfig struct {
	Query    string `json:"query"`
	Filename string `json:"filename"`
}

type Config struct {
	Provider    creative.ProviderConfig `json:"provider"`
	BookFolders []string                `json:"book_folders"`
	Writer      WriterConfig            `json:"writer"`
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.SetOutput(os.Stdout)

	configPath := flag.String("config", "", "Path to book config JSON (required)")
	flag.Parse()

	if *configPath == "" {
		fmt.Println("Usage: writer -config <book.json>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	raw, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("Cannot read config: %v", err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		log.Fatalf("Cannot parse config: %v", err)
	}
	if cfg.Writer.Query == "" {
		log.Fatalf("writer.query is required")
	}
	if cfg.Writer.Filename == "" {
		log.Fatalf("writer.filename is required")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Writer.Filename), 0755); err != nil {
		log.Fatalf("Cannot create output directory: %v", err)
	}

	prvAI := creative.NewRouterAI(cfg.Provider)

	log.Printf("[writer] декомпозиция темы...")
	decomposeChat := creative.NewChat(prvAI)
	decomposeChat.AddSystem(decomposePrompt)
	decomposeChat.SetTools(creative.BookTools(cfg.BookFolders...))

	rawChapters, err := runUntilDone(decomposeChat, cfg.Writer.Query)
	if err != nil {
		log.Fatalf("Decompose failed: %v", err)
	}
	if rawChapters == "" {
		log.Fatalf("Decompose returned empty response")
	}

	chapters := func() []string {
		marker := "==="
		lines := strings.Split(rawChapters, "\n")
		var chs []string
		var cur strings.Builder
		for _, line := range lines {
			if strings.Contains(line, marker) {
				if 0 < cur.Len() {
					chs = append(chs, cur.String())
					cur.Reset()
				}
				continue
			}
			if cur.Len() <= 0 {
				cur.WriteString(line)
			} else {
				cur.WriteString("\n")
				cur.WriteString(line)
			}
		}
		if 0 < cur.Len() {
			chs = append(chs, cur.String())
		}
		return chs
	}()
	if len(chapters) == 0 {
		log.Fatalf("Decompose did not produce any chapters. Raw response:\n%s", rawChapters)
	}
	log.Printf("[writer] найдено %d глав", len(chapters))

	outFile, err := os.OpenFile(cfg.Writer.Filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Cannot create output file: %v", err)
	}
	defer outFile.Close()

	r := []rune(strings.TrimSpace(cfg.Writer.Query))
	if 60 < len(r) {
		r = r[:60]
	}
	title := string(r)
	fmt.Fprintf(outFile, "# %s\n\n", title)

	for i, rawBlock := range chapters {
		log.Printf("[writer] глава %d/%d", i+1, len(chapters))
		fmt.Printf("\n=== Глава %d ===\n%s\n\n", i+1, preview(rawBlock, 60))

		chat := creative.NewChat(prvAI)
		chat.AddSystem(fmt.Sprintf(writerPromptTemplate, rawBlock))
		chat.SetTools(creative.BookTools(cfg.BookFolders...))

		content, err := runUntilDone(chat, "Напиши главу.")
		if err != nil {
			log.Fatalf("Chapter %d failed: %v", i+1, err)
		}

		fmt.Fprintf(outFile, "## Глава %d\n\n%s\n\n---\n\n", i+1, content)
		outFile.Sync()
		log.Printf("[writer] глава %d завершена", i+1)
	}

	log.Printf("[writer] готово: %s", cfg.Writer.Filename)
}

func preview(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "..."
}

func runUntilDone(chat *creative.Chat, firstMsg string) (string, error) {
	var acc strings.Builder
	msg := firstMsg
	done := false

	chat.SetCallback(&creative.ChatEventCallback{
		OnStreamChunk: func(chunk string) {
			if done {
				return
			}
			idx := strings.Index(chunk, "Я закончил")
			if idx < 0 {
				fmt.Print(chunk)
			} else {
				fmt.Print(chunk[:idx])
				done = true
			}
		},
	})

	for i := 0; i <= maxContinueMessages; i++ {
		resp, err := chat.SendStream(msg, true)
		if err != nil {
			return "", err
		}
		idx := strings.Index(resp, "Я закончил")
		if idx < 0 {
			acc.WriteString(resp)
			if i == maxContinueMessages {
				log.Printf("[writer] достигнут лимит продолжений (%d)", maxContinueMessages)
				break
			}
			log.Printf("[writer] продолжаю...")
			msg = "Продолжи"
			time.Sleep(time.Second)
		} else {
			acc.WriteString(resp[:idx])
			break
		}
	}
	return strings.TrimSpace(acc.String()), nil
}
