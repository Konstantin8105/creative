package creative

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// BookTools возвращает набор инструментов для работы с книгами.
// Перед использованием установите BooksFolder.
func BookTools(folders ...string) []Tool {
	if len(folders) == 0 {
		panic("BookTools folders is empty")
	}
	return []Tool{
		{
			Name:        "list_books",
			Description: "List all .txt and .md files in the books folder. Returns file names, sizes, and line counts.",
			Parameters: &ToolParameters{
				Type:       "object",
				Properties: map[string]ToolProperty{},
				Required:   []string{},
			},
			Execute: func(params string) string {
				return listBooksTool(folders, params)
			},
		},
		{
			Name:        "read_book_lines",
			Description: "Read lines from a book. Parameters: filename, start_line, end_line. Line numbering starts at 1. Maximum 1000 lines per call. Example: read_book_lines book.txt 1 50",
			Parameters: &ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"filename": {
						Type:        "string",
						Description: "Name of the book file (e.g., book.txt or book.md)",
					},
					"start_line": {
						Type:        "integer",
						Description: "Starting line number (1-based)",
					},
					"end_line": {
						Type:        "integer",
						Description: "Ending line number (inclusive, max 1000 lines from start)",
					},
				},
				Required: []string{"filename", "start_line", "end_line"},
			},
			Execute: func(params string) string {
				return readBookLinesTool(folders, params)
			},
		},
		{
			Name:        "search_in_book",
			Description: "Search in books by keywords or regular expression. If filename is specified, search only in that book. If filename is omitted, search across all books. Parameters: filename (optional), pattern, mode (optional). Modes: keyword (substring search with | as OR, default) or regex. Example: search_in_book book.txt \"Napoleon\" or search_in_book pattern keyword",
			Parameters: &ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"filename": {
						Type:        "string",
						Description: "Optional. Name of the book file (e.g., book.txt or book.md). If omitted, searches all books.",
					},
					"pattern": {
						Type:        "string",
						Description: "Search pattern: keyword or regular expression. Use | for OR in keyword mode (e.g., 'anchor|belief|state')",
					},
					"mode": {
						Type:        "string",
						Description: "Search mode: 'keyword' (default, case-insensitive, use | for OR) or 'regex'",
						Enum:        []string{"keyword", "regex"},
					},
				},
				Required: []string{"pattern"},
			},
			Execute: func(params string) string {
				return searchInBookTool(folders, params, false)
			},
		},
		{
			Name:        "search_stats",
			Description: "Search in all books by keywords or regular expression. Parameters: pattern, mode (optional). Modes: keyword (substring search with | as OR, default) or regex. Try to use this tool before search_in_book for avoid many noise information. Example: search_stats \"Napoleon\" or search_stats pattern keyword",
			Parameters: &ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"pattern": {
						Type:        "string",
						Description: "Search pattern: keyword or regular expression. Use | for OR in keyword mode (e.g., 'anchor|belief|state')",
					},
					"mode": {
						Type:        "string",
						Description: "Search mode: 'keyword' (default, case-insensitive, use | for OR) or 'regex'",
						Enum:        []string{"keyword", "regex"},
					},
				},
				Required: []string{"pattern"},
			},
			Execute: func(params string) string {
				return searchInBookTool(folders, params, true)
			},
		},
	}
}

func formatFileSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// getFiles return all acceptable files ( ["C:\\Rer\\1.txt", "1.txt"],  ...)
func getFiles(folders []string) (files [][2]string, err error) {
	if len(folders) == 0 {
		err = fmt.Errorf("getFiles folders is empty")
	}
	for _, folder := range folders {
		if folder == "" {
			err = fmt.Errorf("Ошибка: не указана папка с книгами. Установите переменную folder")
			return
		}
		var info os.FileInfo
		info, err = os.Stat(folder)
		if err != nil {
			err = fmt.Errorf("Ошибка: папка %q не найдена.", folder)
			return
		}
		if !info.IsDir() {
			err = fmt.Errorf("Ошибка: %q не является папкой.", folder)
			return
		}

		folder, err = filepath.Abs(folder)
		if !info.IsDir() {
			err = fmt.Errorf("Ошибка: %q не могу получить аблолютный путь", folder)
			return
		}

		var entries []os.DirEntry
		entries, err = os.ReadDir(folder)
		if err != nil {
			err = fmt.Errorf("Ошибка при чтении папки: %v", err)
			return
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".txt" && ext != ".md" && ext != ".go" {
				continue
			}
			files = append(files, [2]string{
				filepath.Join(folder, entry.Name()),
				entry.Name(),
			})
		}
	}
	if len(files) == 0 {
		err = fmt.Errorf("В папке не найдено книг.")
		return
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i][1] < files[j][1]
	})
	return
}

func listBooksTool(folders []string, _ string) string {
	files, err := getFiles(folders)
	if err != nil {
		return err.Error()
	}

	countLines := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			return err.Error()
		}
		lines := strings.Split(string(data), "\n")
		return fmt.Sprintf("%d", len(lines))
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "Доступные книги:\n")
	for _, file := range files {
		fmt.Fprintf(&buf, "Файл: \"%s\"\n", file[1])
		fi, err := os.Stat(file[0])
		if err != nil {
			fmt.Fprintf(&buf, "Не могу получить данные по файлу")
			continue
		}
		fmt.Fprintf(&buf, "Размер: %s (%d байт)\n", formatFileSize(fi.Size()), fi.Size())
		lines := countLines(file[0])
		fmt.Fprintf(&buf, "Строк: %s\n", lines)
		fmt.Fprintf(&buf, "Время последнего изменения файла: %s\n", fi.ModTime().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&buf, "\n")
	}
	return buf.String()
}

func readBookLinesTool(folders []string, params string) string {
	params = strings.TrimSpace(params)
	if params == "" {
		return "Ошибка: недостаточно параметров. Используйте: read_book_lines имя_файла начальная_строка конечная_строка"
	}

	data := struct {
		Filename string `json:"filename"`
		Start    int    `json:"start_line"`
		End      int    `json:"end_line"`
	}{}

	if err := json.Unmarshal([]byte(params), &data); err != nil {
		log.Printf("readBookLinesTool: not valid JSON: `%s`", params)
		return fmt.Sprintf("Ошибка: не корректной формат JSON для выходных данных: %v", err)
	}

	data.Filename = strings.TrimSpace(data.Filename)

	if data.Filename == "" {
		return "Ошибка: неверный формат. Используйте: read_book_lines \"имя файла.txt\" 1 50"
	}
	if data.Start < 1 {
		return "Ошибка: номер строки должен быть >= 1."
	}
	if data.End < data.Start {
		return "Ошибка: начальная строка больше конечной."
	}
	if data.End-data.Start > 1000 {
		return "Ошибка: слишком большой диапазон (максимум 1000 строк за вызов)."
	}

	files, err := getFiles(folders)
	if err != nil {
		return err.Error()
	}

	var buf strings.Builder
	found := false
	for _, file := range files {
		var name string = file[1]
		if name != data.Filename {
			continue
		}

		dataFile, err := os.ReadFile(file[0])
		if err != nil {
			return err.Error()
		}
		lines := strings.Split(string(dataFile), "\n")
		fmt.Fprintf(&buf, "--- Строки %d-%d из %q ---\n", data.Start, data.End, data.Filename)
		for pos := data.Start - 1; pos <= data.End-1 && pos < len(lines); pos++ {
			fmt.Fprintf(&buf, "%d: %s\n", pos+1, lines[pos])
		}
		found = true
		break
	}
	if !found {
		return fmt.Sprintf("Не найдет файл %s\n", data.Filename)
	}
	return buf.String()
}

func searchInBookTool(folders []string, params string, statistic bool) string {
	params = strings.TrimSpace(params)
	if params == "" {
		return "Ошибка: не указаны параметры. Используйте: search_in_book \"имя_файла\" \"паттерн\" [режим]"
	}

	data := struct {
		Filename string `json:"filename"`
		Pattern  string `json:"pattern"`
		Mode     string `json:"mode"`
	}{}

	if err := json.Unmarshal([]byte(params), &data); err != nil {
		log.Printf("searchInBookTool: not valid JSON: `%s`", params)
		return fmt.Sprintf("Ошибка: не корректной формат JSON для выходных данных: %v", err)
	}

	data.Filename = strings.TrimSpace(data.Filename)
	data.Pattern = strings.TrimSpace(data.Pattern)
	if data.Pattern == "" {
		return "Ошибка: не указан паттерн. Используйте: search_in_book \"имя_файла\" \"паттерн\" [режим]"
	}

	files, err := getFiles(folders)
	if err != nil {
		return err.Error()
	}
	if data.Filename == "" || statistic {
		// search all files
		var buf strings.Builder
		for _, file := range files {
			result := runSearch(file, data.Pattern, data.Mode, statistic)
			fmt.Fprintf(&buf, "%s\n", result)
		}
		return buf.String()
	}
	// search in single file
	for _, file := range files {
		name := file[1]
		if name != data.Filename {
			continue
		}
		result := runSearch(file, data.Pattern, data.Mode, false)
		return result
	}
	// error
	return "Ничего не найдено\n"
}

func splitLastMode(s string) (mode, pattern string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	fields := strings.Fields(s)
	if len(fields) == 1 {
		return "", s
	}
	last := strings.ToLower(fields[len(fields)-1])
	if last == "keyword" || last == "regex" {
		return last, strings.Join(fields[:len(fields)-1], " ")
	}
	return "", s
}

var regexMetacharacters = []string{
	".", "*", "+", "[", "]", "(", ")", "^", "$", "\\", "?", "{", "}",
}

func looksLikeRegex(pattern string) bool {
	for _, mc := range regexMetacharacters {
		if strings.Contains(pattern, mc) {
			return true
		}
	}
	return false
}

func runSearch(file [2]string, pattern, mode string, statistic bool) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "Ошибка: не указан паттерн для поиска. Используйте search_in_book имя_файла \"текст для поиска\""
	}
	mode = strings.TrimSpace(mode)
	mode = strings.ToLower(strings.TrimSpace(mode))

	var isRegex bool
	switch mode {
	case "", "keyword":
		if looksLikeRegex(pattern) {
			if _, err := regexp.Compile(pattern); err == nil {
				isRegex = true
				break
			}
		}
		isRegex = false
	case "regex":
		isRegex = true
	default:
		return fmt.Sprintf("Ошибка: неизвестный режим %q. Используйте 'keyword' или 'regex'.", mode)
	}
	// naming
	var modeName string
	if isRegex {
		modeName = "regex"
	} else {
		modeName = "keyword"
	}
	// prepare regexp
	var re *regexp.Regexp
	if isRegex {
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			return fmt.Sprintf("Ошибка: не удалось разобрать регулярное выражение: %v.\nИспользуйте mode=keyword для поиска по ключевым словам.", err)
		}
	}
	// read file
	data, err := os.ReadFile(file[0])
	if err != nil {
		return fmt.Sprintf("Ошибка: не могу прочесть файл %s\n", file[1])
	}
	lines := strings.Split(string(data), "\n")

	// prepare patterns
	orPatterns := func() []string {
		raw := strings.Split(strings.ToLower(pattern), "|")
		var parts []string
		for _, p := range raw {
			trimmed := strings.TrimSpace(p)
			if trimmed == "" {
				continue
			}
			parts = append(parts, strings.ToLower(trimmed))
		}
		if len(parts) == 0 {
			return []string{strings.ToLower(pattern)}
		}
		return parts
	}()

	var buf strings.Builder
	count := 0
	maxResults := 200
	for pos, line := range lines {
		if isRegex {
			if !re.MatchString(line) {
				continue
			}
		} else {
			if !matchesAnyOR(strings.ToLower(line), orPatterns) {
				continue
			}
		}
		if count < maxResults && !statistic {
			fmt.Fprintf(&buf, "Строка %d: %s\n", pos+1, strings.TrimSpace(line))
		}
		count++
	}
	if count == 0 {
		if statistic {
			return "" // не выводить ничего
		}
		return fmt.Sprintf("В файле %q не найдено совпадений с паттерном %q в режиме %s.", file[1], pattern, modeName)
	}
	if statistic {
		return fmt.Sprintf("Совпадений в файле %s (паттерн: %q): %d\n",
			file[1], pattern, count)
	}

	var result strings.Builder
	fmt.Fprintf(&result, "Поиск в %q (паттерн: %q)\n", file[1], pattern)
	fmt.Fprintf(&result, "Режим: \"%s\"\n", modeName)
	fmt.Fprintf(&result, "Найдено %d совпадений", count)
	if count > maxResults {
		fmt.Fprintf(&result, " (показаны первые %d)", maxResults)
	}
	result.WriteString(":\n\n")
	result.WriteString(buf.String())
	return result.String()
}

func matchesAnyOR(lineLower string, orParts []string) bool {
	if len(orParts) == 1 {
		return strings.Contains(lineLower, orParts[0])
	}
	for _, part := range orParts {
		if strings.Contains(lineLower, part) {
			return true
		}
	}
	return false
}
