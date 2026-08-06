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
var maxContinueMessages = 5

// maxBranchDepth limits subtask branching: at depth >= maxBranchDepth
// the "subtask" tool is not provided, so the AI does the work itself.
const maxBranchDepth = 1

type WriterConfig struct {
	Query       QueryData `json:"query"`
	Filename    string    `json:"filename"`
	BookFolders []string  `json:"book_folders,omitempty"`
	depth       int
}

type QueryData struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
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
					Endpoint:    "http://127.0.0.1:1234/v1",
					Model:       "openai/gpt-oss-20b",
					Key:         "lmstudio",
					ContextSize: 24000,
				},
			}
			example.Queries = append(example.Queries,
				WriterConfig{
					Query: QueryData{
						Name: "example of query",
					},
					Filename:    "filename of result",
					BookFolders: []string{"c:\folder", "."},
				},
				WriterConfig{
					Query: QueryData{
						Name:        "second example",
						Description: "write shortly",
					},
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
			file := make(chan string, 1)
			go func() {
				for str := range file {
					var dat []byte
					if _, err := os.Stat(q.Filename); !os.IsNotExist(err) {
						dat, err = os.ReadFile(q.Filename)
						if err != nil {
							log.Printf("Error: %v", err)
							continue
						}
					}
					dat = append(dat, []byte("\n"+str)...)
					err = os.WriteFile(q.Filename, dat, 0644)
					if err != nil {
						log.Printf("Error: %v", err)
						continue
					}
				}
			}()
			defer func() {
				close(file)
			}()
			err := runQuery(prvAI, q, file, "")
			if err != nil {
				fmt.Fprintf(os.Stdout, "[writer] ошибка: %v", err)
				continue
			}
		}
	}
}

func runQuery(prvAI creative.AIrunner, q WriterConfig, file chan<- string, prefix string) error {
	if q.Query.Name == "" {
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

	log.Printf("[writer] задача: %#v\n", q.Query)
	if file != nil {
		file <- fmt.Sprintf("%s %s.%d %s\n", strings.Repeat("#", q.depth+1), prefix, q.depth, q.Query.Name)
	}
	//if q.Query.Description != "" {
	//	file <- fmt.Sprintf("%s\n", q.Query.Description)
	//}

	tmpl := struct {
		Query       string
		AddBookTool bool
		AddSubTask  bool
	}{
		Query: fmt.Sprintf("Имя задачи: %s\nОписание для задачи: %s\n", q.Query.Name, q.Query.Description),
	}
	// prepare tools
	var tools []creative.Tool
	if 0 < len(q.BookFolders) {
		tools = append(tools, creative.BookTools(q.BookFolders...)...)
		tmpl.AddBookTool = true
	}
	var subtasks []QueryData
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
			fmt.Fprintf(os.Stdout, "Tool result: %s\n%s\n\n", name, string(short[:60]))
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
	for is, subtask := range subtasks {
		log.Printf("Depth %s.%d.%d %s\n", prefix, q.depth, is, subtask.Name)
	}
	for is := range subtasks {
		if err := runQuery(prvAI, WriterConfig{
			Query:       subtaskQuery(q.Query, subtasks, is),
			Filename:    q.Filename,
			BookFolders: q.BookFolders,
			depth:       q.depth + 1,
		}, file, fmt.Sprintf("%s.%d", prefix, is)); err != nil {
			fmt.Fprintf(os.Stdout, "runQuery error: %v", err)
			continue
		}
	}
	return nil
}

// subtaskQuery строит QueryData для is-й подзадачи: общая задача, список всех
// подзадач и только одна помеченная «реши её», чтобы каждый рекурсивный
// запрос решал ровно одну конкретную задачу.
func subtaskQuery(parent QueryData, subtasks []QueryData, is int) QueryData {
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
	return QueryData{Name: subtasks[is].Name, Description: strings.TrimSpace(b.String())}
}

// subtaskTool is a single uniform tool: it behaves identically at every level
// and does not know whether it is used for the main task or a subtask.
func subtaskTool(subtasks *[]QueryData) creative.Tool {
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
			q := QueryData{Name: name, Description: strings.TrimSpace(p.Description)}
			log.Printf("add subtask: %#v\n\n", q)
			*subtasks = append(*subtasks, q)
			return "Подзадача поставлена в очередь. Не пиши её текст сам — она будет выполнена отдельным запросом."
		},
	}
}
