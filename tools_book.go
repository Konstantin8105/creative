package creative

import (
	"bufio"
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
func BookTools(folder string) []Tool {
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
				return listBooksTool(folder, params)
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
				return readBookLinesTool(folder, params)
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
				return searchInBookTool(folder, params)
			},
		},
	}
}

// resolveFile проверяет, что файл существует, имеет расширение .txt/.md,
// находится внутри folder, и возвращает полный путь.
func resolveFile(folder, filename string) (fullPath string, errMsg string) {
	files, err := getFiles(folder)
	if err != nil {
		return "", err.Error()
	}
	for _, file := range files {
		if file == filename {
			return filepath.Clean(filepath.Join(folder, filename)), ""
		}
	}
	return "", fmt.Sprintf("Ошибка: файл %q не найден. Используйте list_books для просмотра доступных книг.", filename)
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

// getFiles return all acceptable files
func getFiles(folder string) (files []string, err error) {
	if folder == "" {
		err = fmt.Errorf("Ошибка: не указана папка с книгами. Установите переменную folder")
		return
	}
	info, err := os.Stat(folder)
	if err != nil {
		err = fmt.Errorf("Ошибка: папка %q не найдена.", folder)
		return
	}
	if !info.IsDir() {
		err = fmt.Errorf("Ошибка: %q не является папкой.", folder)
		return
	}

	entries, err := os.ReadDir(folder)
	if err != nil {
		err = fmt.Errorf("Ошибка при чтении папки: %v", err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".txt" && ext != ".md" {
			continue
		}
		files = append(files, entry.Name())
	}
	if len(files) == 0 {
		err = fmt.Errorf("В папке не найдено книг.")
		return
	}
	return
}

func listBooksTool(folder, params string) string {
	files, err := getFiles(folder)
	if err != nil {
		return err.Error()
	}

	countLines := func(path string) int {
		f, err := os.Open(path)
		if err != nil {
			return 0
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		lines := 0
		for scanner.Scan() {
			lines++
		}
		return lines
	}

	sort.Strings(files)
	var b strings.Builder
	fmt.Fprintf(&b, "Доступные книги:\n")
	for _, name := range files {
		fullPath := filepath.Join(folder, name)
		fi, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		lines := countLines(fullPath)
		fmt.Fprintf(&b, "Файл: \"%s\"\n", name)
		fmt.Fprintf(&b, "Размер: %s (%d байт)\n", formatFileSize(fi.Size()), fi.Size())
		fmt.Fprintf(&b, "Строк: %d\n", lines)
		fmt.Fprintf(&b, "Время последнего изменения файла: %s\n", fi.ModTime().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func readBookLinesTool(folder string, params string) string {
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
	if data.Start > data.End {
		return "Ошибка: начальная строка больше конечной."
	}
	if data.End-data.Start > 1000 {
		return "Ошибка: слишком большой диапазон (максимум 1000 строк за вызов)."
	}

	fullPath, errMsg := resolveFile(folder, data.Filename)
	if errMsg != "" {
		return errMsg
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return fmt.Sprintf("Ошибка: не удалось открыть файл %q.", data.Filename)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	currentLine := 1
	var b strings.Builder
	fmt.Fprintf(&b, "--- Строки %d-%d из %q ---\n", data.Start, data.End, data.Filename)

	for scanner.Scan() {
		if currentLine > data.End {
			break
		}
		if currentLine >= data.Start {
			fmt.Fprintf(&b, "%d: %s\n", currentLine, scanner.Text())
		}
		currentLine++
	}

	if currentLine <= data.Start {
		totalLines := currentLine - 1
		return fmt.Sprintf("Файл %q содержит только %d строк. Запрошен диапазон %d-%d.",
			data.Filename, totalLines, data.Start, data.End)
	}

	return b.String()
}

func searchInBookTool(folder, params string) string {
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

	data.Pattern = strings.TrimSpace(data.Pattern)
	if data.Pattern == "" {
		return "Ошибка: не указан паттерн. Используйте: search_in_book \"имя_файла\" \"паттерн\" [режим]"
	}

	files, err := getFiles(folder)
	if err != nil {
		return err.Error()
	}

	// single search
	for _, file := range files {
		if file != data.Filename {
			continue
		}
		fullPath, errMsg := resolveFile(folder, file)
		if errMsg != "" {
			return errMsg
		}
		return runSearch(fullPath, data.Filename, data.Pattern, data.Mode)
	}
	// search in all files
	var buf strings.Builder
	for _, file := range files {
		fullPath, errMsg := resolveFile(folder, file)
		if errMsg != "" {
			continue
		}
		result := runSearch(fullPath, file, data.Pattern, data.Mode)
		fmt.Fprintf(&buf, "%s\n", result)
	}
	return buf.String()
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

func runSearch(fullPath, filename, pattern, mode string) string {
	if pattern == "" {
		return "Ошибка: не указан паттерн для поиска. Используйте search_in_book имя_файла \"текст для поиска\""
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "", "keyword":
		if looksLikeRegex(pattern) {
			if _, err := regexp.Compile(pattern); err == nil {
				return searchByRegex(fullPath, filename, pattern)
			}
		}
		return searchByKeyword(fullPath, filename, pattern)
	case "regex":
		return searchByRegex(fullPath, filename, pattern)
	default:
		return fmt.Sprintf("Ошибка: неизвестный режим %q. Используйте 'keyword' или 'regex'.", mode)
	}
}

func searchByKeyword(fullPath, filename, pattern string) string {
	f, err := os.Open(fullPath)
	if err != nil {
		return fmt.Sprintf("Ошибка: не удалось открыть файл %q.", filename)
	}
	defer f.Close()

	orParts := splitOR(pattern)
	scanner := bufio.NewScanner(f)
	var b strings.Builder
	lineNum := 0
	count := 0
	maxResults := 50

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lineLower := strings.ToLower(line)
		if matchesAnyOR(lineLower, orParts) {
			if count < maxResults {
				fmt.Fprintf(&b, "Строка %d: %s\n", lineNum, strings.TrimSpace(line))
			}
			count++
		}
	}

	if count == 0 {
		return fmt.Sprintf("В файле %q не найдено совпадений с %q.", filename, pattern)
	}

	var result strings.Builder
	fmt.Fprintf(&result, "Поиск в %q (режим: keyword, паттерн: %q)\n", filename, pattern)
	fmt.Fprintf(&result, "Найдено %d совпадений", count)
	if count > maxResults {
		fmt.Fprintf(&result, " (показаны первые %d)", maxResults)
	}
	result.WriteString(":\n\n")
	result.WriteString(b.String())
	return result.String()
}

func splitOR(pattern string) []string {
	raw := strings.Split(pattern, "|")
	var parts []string
	for _, p := range raw {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			parts = append(parts, strings.ToLower(trimmed))
		}
	}
	if len(parts) == 0 {
		return []string{strings.ToLower(pattern)}
	}
	return parts
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

func searchByRegex(fullPath, filename, pattern string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Sprintf("Ошибка: не удалось разобрать регулярное выражение: %v.\nИспользуйте mode=keyword для поиска по ключевым словам.", err)
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return fmt.Sprintf("Ошибка: не удалось открыть файл %q.", filename)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var b strings.Builder
	lineNum := 0
	count := 0
	maxResults := 50

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			if count < maxResults {
				fmt.Fprintf(&b, "Строка %d: %s\n", lineNum, strings.TrimSpace(line))
			}
			count++
		}
	}

	if count == 0 {
		return fmt.Sprintf("В файле %q не найдено совпадений с регулярным выражением %q.", filename, pattern)
	}

	var result strings.Builder
	fmt.Fprintf(&result, "Поиск в %q (режим: regex, паттерн: %q)\n", filename, pattern)
	fmt.Fprintf(&result, "Найдено %d совпадений", count)
	if count > maxResults {
		fmt.Fprintf(&result, " (показаны первые %d)", maxResults)
	}
	result.WriteString(":\n\n")
	result.WriteString(b.String())
	return result.String()
}
