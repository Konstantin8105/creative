package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Konstantin8105/creative"
)

//go:embed prompts/writer.promt
var writerPrompt string

// maxContinueMessages prevents infinite loop in runUntilDone.
var maxContinueMessages = 3

// maxBranchDepth limits subtask branching: at depth >= maxBranchDepth
// the "subtask" tool is not provided, so the AI does the work itself.
const maxBranchDepth = 2

type WriterConfig struct {
	Query       string   `json:"query"`
	Filename    string   `json:"filename"`
	BookFolders []string `json:"book_folders,omitempty"`
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

	flag.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: writer -configs book1.json,book2.json,...\n")
		fmt.Fprintf(os.Stdout, "Example of config:\n%s\n", func() string {
			example := Config{
				Provider: creative.ProviderConfig{
					Endpoint:    "http://192.168.56.1:1234/v1",
					Model:       "openai/gpt-oss-20b",
					Key:         "lmstudio",
					ContextSize: 24000,
				},
			}
			example.Queries = append(example.Queries,
				WriterConfig{
					Query:       "example of query",
					Filename:    "filename of result",
					BookFolders: []string{"c:\folder", "."},
				},
				WriterConfig{
					Query:    "second example",
					Filename: "second filename",
				},
			)
			dat, _ := json.MarshalIndent(example, " ", "   ")
			return string(dat)
		}())
		flag.PrintDefaults()
	}
	if *configs == "" {
		flag.Usage()
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

	log.Printf("[writer] задача: %s", q.Query)
	write(q.Filename, "%s %s\n", strings.Repeat("#", q.depth+1), q.Query)

	tmpl := struct {
		Query       string
		AddBookTool bool
		AddSubTask  bool
	}{
		Query: q.Query,
	}

	var tools []creative.Tool
	if 0 < len(q.BookFolders) { // BookTools panics on an empty list
		tools = append(tools, creative.BookTools(q.BookFolders...)...)
		tmpl.AddBookTool = true
	}
	var subtasks []string
	if q.depth < maxBranchDepth {
		tools = append(tools, subtaskTool(&subtasks))
		tmpl.AddSubTask = true
	}

	var prompt string
	{
		t, err := template.New("todos").Parse(writerPrompt)
		if err != nil {
			panic(err)
		}
		var buf bytes.Buffer
		err = t.Execute(&buf, tmpl)
		if err != nil {
			panic(err)
		}
		prompt = buf.String()
	}
	// log.Printf("prompt: %s", prompt)

	chat := creative.NewChat(prvAI)
	chat.AddSystem(prompt)
	chat.SetTools(tools)

	content, err := runUntilDone(chat, "Выполни задачу.")
	if err != nil {
		return fmt.Errorf("run: %v", err)
	}

	write(q.Filename, "\n%s\n", content)
	for is, subtask := range subtasks {
		log.Printf("Depth%d.%d %s\n", q.depth, is, subtask)
	}

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

func write(filename string, format string, a ...any) {
	dat, _ := os.ReadFile(filename)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, format, a...)
	dat = append(dat, buf.Bytes()...)
	err := os.WriteFile(filename, dat, 0644)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
}

// subtaskTool is a single uniform tool: it behaves identically at every level
// and does not know whether it is used for the main task or a subtask.
func subtaskTool(subtasks *[]string) creative.Tool {
	return creative.Tool{
		Name:        "subtask",
		Description: "Запустить подзадачу. В параметре description передай ПОЛНОЕ самодостаточное описание подзадачи со всем необходимым контекстом.",
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

			// log.Printf("add subtask: %s", desc)
			*subtasks = append(*subtasks, desc)
			return "Подзадачи поставлены в очередь, не дожидайся их окончания."
		},
	}
}

func runUntilDone(chat *creative.Chat, firstMsg string) (string, error) {
	var acc strings.Builder
	msg := firstMsg

	chat.SetCallback(&creative.ChatEventCallback{
		OnStreamChunk: func(chunk string) {
			fmt.Print(chunk)
		},
	})

	for i := 0; i <= maxContinueMessages; i++ {
		// log.Printf("iter:%d", i)
		resp, err := chat.SendStream(msg, true)
		if err != nil {
			return "", err
		}
		// log.Printf("iter:%d %s", i, resp)

		acc.WriteString(resp)
		if i == maxContinueMessages {
			log.Printf("[writer] достигнут лимит продолжений (%d)", maxContinueMessages)
			break
		}
		log.Printf("[writer] продолжаю...")
		msg = "Продолжи"

		if strings.Contains(resp, "Я закончил") {
			break
		}
	}
	return strings.TrimSpace(acc.String()), nil
}
