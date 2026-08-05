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

//go:embed prompts/writer.promt
var writerPrompt string

// maxContinueMessages prevents infinite loop in runUntilDone.
var maxContinueMessages = 20

// maxBranchDepth limits subtask branching: at depth >= maxBranchDepth
// the "subtask" tool is not provided, so the AI does the work itself.
const maxBranchDepth = 5

type WriterConfig struct {
	Query       string   `json:"query"`
	Filename    string   `json:"filename"`
	BookFolders []string `json:"book_folders"`
	depth       int
}

type Config struct {
	Provider creative.ProviderConfig `json:"provider"`
	Queries  []WriterConfig          `json:"queries"`
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.SetOutput(os.Stdout)

	configs := flag.String("configs", "", "Comma-separated list of book config JSONs")
	flag.Parse()

	if *configs == "" {
		fmt.Println("Usage: writer -configs book1.json,book2.json,...")
		flag.PrintDefaults()
		os.Exit(1)
	}

	for _, path := range strings.Split(*configs, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		log.Printf("[writer] конфиг: %s", path)
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stdout, "Cannot read config: %v", err)
			continue
		}

		var cfg Config
		if err := json.Unmarshal(raw, &cfg); err != nil {
			fmt.Fprintf(os.Stdout, "Cannot parse config: %v", err)
			continue
		}
		if len(cfg.Queries) == 0 {
			fmt.Fprintf(os.Stdout, "queries is required")
			continue
		}
		prvAI := creative.NewRouterAI(cfg.Provider)
		for _, q := range cfg.Queries {
			if err := runQuery(prvAI, q); err != nil {
				fmt.Fprintf(os.Stdout, "[writer] ошибка: %v", err)
				continue
			}
		}
	}
}

func runQuery(prvAI creative.AIrunner, q WriterConfig) error {
	if q.Query == "" {
		return fmt.Errorf("writer.query is required")
	}
	if q.Filename == "" {
		return fmt.Errorf("writer.filename is required")
	}
	if _, err := os.Stat(q.Filename); q.depth == 0 && !os.IsNotExist(err) {
		return fmt.Errorf("[writer] файл уже существует: %s — пропускаем", q.Filename)
	}
	if err := os.MkdirAll(filepath.Dir(q.Filename), 0755); err != nil {
		return fmt.Errorf("Cannot create output directory: %v", err)
	}

	log.Printf("[writer] задача: %s", preview(q.Query))

	prompt := strings.Replace(writerPrompt, "__TASK__", q.Query, 1)

	chat := creative.NewChat(prvAI)
	chat.AddSystem(prompt)

	var tools []creative.Tool
	if len(q.BookFolders) > 0 { // BookTools panics on an empty list
		tools = append(tools, creative.BookTools(q.BookFolders...)...)
	}
	var subtasks []string
	if q.depth < maxBranchDepth {
		tools = append(tools, subtaskTool(&subtasks))
	}
	chat.SetTools(tools)

	content, err := runUntilDone(chat, "Выполни задачу.")
	if err != nil {
		return fmt.Errorf("run: %v", err)
	}

	outFile, err := os.OpenFile(q.Filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("Cannot create output file: %v", err)
	}
	defer outFile.Close()

	fmt.Fprintf(outFile, "# %s\n\n%s\n", preview(q.Query), content)
	log.Printf("[writer] готово: %s", q.Filename)

	for _, subtask := range subtasks {
		sub := q
		sub.Query += "\n" + subtask
		sub.depth += 1
		if err := runQuery(prvAI, sub); err != nil {
			fmt.Fprintf(os.Stdout, "runQuery error: %v", err)
			continue
		}
	}
	return nil
}

// subtaskTool is a single uniform tool: it behaves identically at every level
// and does not know whether it is used for the main task or a subtask.
func subtaskTool(subtasks *[]string) creative.Tool {
	return creative.Tool{
		Name:        "subtask",
		Description: "Запустить подзадачу. В параметре description передай ПОЛНОЕ самодостаточное описание подзадачи со всем необходимым контекстом. Результат вернётся текстом.",
		Parameters: &creative.ToolParameters{
			Type: "object",
			Properties: map[string]creative.ToolProperty{
				"description": {
					Type:        "string",
					Description: "Полное самодостаточное описание подзадачи со всем необходимым контекстом",
				},
			},
			Required: []string{"description"},
		},
		Execute: func(params string) string {
			var p struct {
				Description string `json:"description"`
			}
			if err := json.Unmarshal([]byte(params), &p); err != nil {
				return fmt.Sprintf("Ошибка: неверный JSON параметров: %v", err)
			}
			desc := strings.TrimSpace(p.Description)
			if desc == "" {
				return "Ошибка: поле description не должно быть пустым"
			}

			*subtasks = append(*subtasks, desc)
			return "Будет выполнено позже."
		},
	}
}

func preview(s string) string {
	const max = 60
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
