package main

import (
	"bytes"
	"crypto/md5"
	_ "embed"
	"encoding/hex"
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
const maxContinueMessages = 5

// maxBranchDepth limits subtask branching: at depth >= maxBranchDepth
// the "subtask" tool is not provided, so the AI does the work itself.
const maxBranchDepth = 1

type Config struct {
	Provider creative.ProviderConfig `json:"provider"`
	Queries  []Query                 `json:"queries"`
}

type Query struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	BookFolders []string `json:"book_folders,omitempty"`
	depth       int
}

func flowToFile(q Query, path string) (file chan string, c func()) {
	var filename string
	// generate filename
	for iter := 0; ; iter++ {
		hasher := md5.New()
		hasher.Write([]byte(q.Name))
		hash := hex.EncodeToString(hasher.Sum(nil))[:8]
		filename = fmt.Sprintf("%s.%s.%03d.md",
			strings.TrimSuffix(path, filepath.Ext(path)), hash, iter)
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			break
		}
	}
	// create folders
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		panic(fmt.Errorf("Cannot create output directory: %v", err))
	}
	//
	file = make(chan string, 1)
	go func() {
		for str := range file {
			var dat []byte
			if _, err := os.Stat(filename); !os.IsNotExist(err) {
				dat, err = os.ReadFile(filename)
				if err != nil {
					log.Printf("Error: %v", err)
					continue
				}
			}
			dat = append(dat, []byte("\n"+str)...)
			err := os.WriteFile(filename, dat, 0644)
			if err != nil {
				log.Printf("Error: %v", err)
				continue
			}
		}
	}()
	return file, func() { close(file) }
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.SetOutput(os.Stdout)

	config := flag.String("config", "", "Config JSON")
	flag.Parse()
	*config = strings.TrimSpace(*config)

	flag.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: writer -configs book1.json\n")
		fmt.Fprintf(os.Stdout, "Example of config:\n%s\n", func() string {
			example := Config{
				Provider: creative.ProviderConfig{
					Endpoint:    "http://127.0.0.1:1234/v1",
					Model:       "openai/gpt-oss-20b",
					Key:         "lmstudio",
					ContextSize: 24000,
				},
				Queries: []Query{
					{
						Name:        "example of query",
						BookFolders: []string{"c:\folder", "."},
					}, {
						Name:        "second example",
						Description: "write shortly",
						BookFolders: []string{"c:\folder", "."},
					},
				},
			}
			dat, _ := json.MarshalIndent(example, " ", "   ")
			return string(dat)
		}())
		flag.PrintDefaults()
	}
	if *config == "" {
		flag.Usage()
		os.Exit(1)
	}

	path := strings.TrimSpace(*config)
	if path == "" {
		fmt.Fprintf(os.Stdout, "Empty config")
		return
	}
	log.Printf("[writer] конфиг: %s", path)
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Cannot read config: %v", err)
		return
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		fmt.Fprintf(os.Stdout, "Cannot parse config: %v", err)
		return
	}
	if len(cfg.Queries) == 0 {
		fmt.Fprintf(os.Stdout, "queries is required")
		return
	}
	prvAI := creative.NewRouterAI(cfg.Provider)
	for _, q := range cfg.Queries {
		file, closeFile := flowToFile(q, path)
		err := runQuery(prvAI, q, file)
		closeFile()
		if err != nil {
			fmt.Fprintf(os.Stdout, "[writer] ошибка: %v", err)
			continue
		}
	}
}

func runQuery(prvAI creative.AIrunner, q Query, file chan<- string) error {
	log.Printf("Task: %#v\n", q)
	if q.Name == "" {
		return fmt.Errorf("writer.query is required")
	}
	// prepare prompt template
	tmpl := struct {
		Query       string
		AddBookTool bool
		AddSubTask  bool
	}{
		Query: fmt.Sprintf("Имя задачи: %s\nОписание для задачи: %s\n", q.Name, q.Description),
	}
	// prepare tools
	var tools []creative.Tool
	if 0 < len(q.BookFolders) {
		tools = append(tools, creative.BookTools(q.BookFolders...)...)
		tmpl.AddBookTool = true
	}
	var subtasks []Query
	if q.depth < maxBranchDepth {
		tools = append(tools, subtaskTool(&subtasks))
		tmpl.AddSubTask = true
	}
	// prepare prompt
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
	// prepare chat
	chat := creative.NewChat(prvAI)
	chat.AddSystem(prompt)
	chat.SetTools(tools)
	chat.SetCallback(&creative.ChatEventCallback{
		OnStreamChunk: func(chunk string) {
			fmt.Fprint(os.Stdout, chunk)
		},
		OnToolResult: func(name string, result string) {
			short := []rune(result)
			if 60 < len(short) {
				short = short[:60]
			}
			fmt.Fprintf(os.Stdout, "Tool result: %s\n%s\n\n", name, string(short))
		},
	})
	// run until done
	for i, msg := 0, ""; i <= maxContinueMessages; i++ {
		if maxContinueMessages-i == 0 {
			msg = "Это последнее сообщение. Пора окончивать."
		} else {
			msg = fmt.Sprintf("Продолжи выполнять свою задачу. У тебя есть возможность написать ещё %d сообщений.", maxContinueMessages-i)
		}
		log.Printf("[writer] msg: %s\n", msg)
		resp, err := chat.SendStream(msg, true)
		if err != nil {
			return err
		}
		const endWord = "Я закончил"
		if file != nil {
			file <- fmt.Sprintf("\n%s\n", strings.TrimSpace(strings.ReplaceAll(resp, endWord, "")))
		}
		if strings.Contains(resp, endWord) {
			break
		}
	}
	// run sub tasks
	for is := range subtasks {
		if err := runQuery(prvAI, Query{
			Name:        subtasks[is].Name,
			Description: subtaskQuery(q, subtasks, is),
			BookFolders: q.BookFolders,
			depth:       q.depth + 1,
		}, file); err != nil {
			fmt.Fprintf(os.Stdout, "runQuery error: %v", err)
			continue
		}
	}
	return nil
}

func subtaskQuery(parent Query, subtasks []Query, is int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Общая задача: %s\n", parent.Name)
	if parent.Description != "" {
		fmt.Fprintf(&b, "%s\n", parent.Description)
	}
	b.WriteString("Список всех подзадач:\n")
	for i, s := range subtasks {
		if i == is {
			b.WriteString("=> ")
		} else {
			b.WriteString("   ")
		}
		b.WriteString(s.Name)
		//if s.Description != "" {
		//	fmt.Fprintf(&b, ": %s", s.Description)
		//}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Реши только эту подзадачу: %s\n", subtasks[is].Name)
	if subtasks[is].Description != "" {
		fmt.Fprintf(&b, "%s\n", subtasks[is].Description)
	}
	return strings.TrimSpace(b.String())
}

// subtaskTool is a single uniform tool: it behaves identically at every level
// and does not know whether it is used for the main task or a subtask.
func subtaskTool(subtasks *[]Query) creative.Tool {
	return creative.Tool{
		Name:        "subtask",
		Description: "Поставить подзадачу в очередь. Она будет выполнена отдельным запросом.",
		Parameters: &creative.ToolParameters{
			Type: "object",
			Properties: map[string]creative.ToolProperty{
				"name": {
					Type:        "string",
					Description: "Краткое название подзадачи, например «Разработка ...»",
				},
				"description": {
					Type:        "string",
					Description: "Полное самодостаточное описание подзадачи со всем необходимым контекстом",
				},
			},
			Required: []string{"name"},
		},
		Execute: func(params string) string {
			var p struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal([]byte(params), &p); err != nil {
				return fmt.Sprintf("Ошибка: неверный JSON параметров: %v", err)
			}
			name := strings.TrimSpace(p.Name)
			if name == "" {
				return "Ошибка: поле name не должно быть пустым"
			}
			q := Query{Name: name, Description: strings.TrimSpace(p.Description)}
			log.Printf("add subtask: %#v\n\n", q)
			*subtasks = append(*subtasks, q)
			return "Подзадача поставлена в очередь. Не пиши её текст сам — она будет выполнена отдельным запросом."
		},
	}
}
