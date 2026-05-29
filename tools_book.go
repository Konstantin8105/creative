package creative

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// BooksFolder — путь к папке с книгами.
// Устанавливается один раз перед использованием BookTools.
// Все инструменты работают только с файлами внутри этой папки.
var BooksFolder string

// BookTools возвращает набор инструментов для работы с книгами.
// Перед использованием установите BooksFolder.
func BookTools() []Tool {
	return []Tool{
		{
			Name:        "list_books",
			Description: "List all .txt and .md files in the books folder. Returns file names, sizes, and line counts.",
			Parameters: &ToolParameters{
				Type:       "object",
				Properties: map[string]ToolProperty{},
				Required:   []string{},
			},
			Execute: listBooksTool,
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
			Execute: readBookLinesTool,
		},
		{
			Name:        "search_in_book",
			Description: "Search in a book by keywords or regular expression. Parameters: filename, pattern, mode (optional). Modes: keyword (substring search with | as OR, default) or regex. Example: search_in_book book.txt \"Napoleon\" or search_in_book file.md \"\\bGo\\b\" regex",
			Parameters: &ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"filename": {
						Type:        "string",
						Description: "Name of the book file (e.g., book.txt or book.md)",
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
				Required: []string{"filename", "pattern"},
			},
			Execute: searchInBookTool,
		},
		{
			Name:        "book_info",
			Description: "Show meta-information about a book: size, line count, character count, modification date. Parameters: filename. Example: book_info book.txt",
			Parameters: &ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"filename": {
						Type:        "string",
						Description: "Name of the book file (e.g., book.txt or book.md)",
					},
				},
				Required: []string{"filename"},
			},
			Execute: bookInfoTool,
		},
	}
}

// resolveFile проверяет, что файл существует, имеет расширение .txt/.md,
// находится внутри BooksFolder, и возвращает полный путь.
func resolveFile(filename string) (fullPath string, errMsg string) {
	if BooksFolder == "" {
		return "", "Ошибка: не указана папка с книгами. Установите переменную BooksFolder."
	}
	if filename == "" {
		return "", "Ошибка: не указано имя файла. Используйте list_books для просмотра доступных книг."
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".txt" && ext != ".md" {
		return "", fmt.Sprintf("Ошибка: файл %q имеет расширение %s. Поддерживаются только файлы .txt и .md.", filename, ext)
	}
	fullPath = filepath.Clean(filepath.Join(BooksFolder, filename))
	booksFolderClean := filepath.Clean(BooksFolder)
	if !strings.HasPrefix(fullPath, booksFolderClean+string(os.PathSeparator)) &&
		fullPath != booksFolderClean {
		return "", fmt.Sprintf("Ошибка: файл %q должен находиться в папке книг.", filename)
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Sprintf("Ошибка: файл %q не найден. Используйте list_books для просмотра доступных книг.", filename)
		}
		return "", fmt.Sprintf("Ошибка: не удалось получить информацию о файле %q: %v", filename, err)
	}
	if info.IsDir() {
		return "", fmt.Sprintf("Ошибка: %q является папкой, а не файлом.", filename)
	}
	return fullPath, ""
}

func countLines(path string) int {
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

func listBooksTool(params string) string {
	if BooksFolder == "" {
		return "Ошибка: не указана папка с книгами. Установите переменную BooksFolder."
	}
	info, err := os.Stat(BooksFolder)
	if err != nil {
		return fmt.Sprintf("Ошибка: папка %q не найдена.", BooksFolder)
	}
	if !info.IsDir() {
		return fmt.Sprintf("Ошибка: %q не является папкой.", BooksFolder)
	}

	var files []string
	err = filepath.Walk(BooksFolder, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".txt" && ext != ".md" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return fmt.Sprintf("Ошибка при обходе папки: %v", err)
	}

	if len(files) == 0 {
		return fmt.Sprintf("В папке %q не найдено книг в формате .txt или .md.", BooksFolder)
	}

	sort.Strings(files)
	var b strings.Builder
	fmt.Fprintf(&b, "Доступные книги в %q:\n", BooksFolder)
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(BooksFolder, f)
		lines := countLines(f)
		fmt.Fprintf(&b, "  %s (%s, %d строк)\n", rel, formatFileSize(fi.Size()), lines)
	}
	return b.String()
}

func readBookLinesTool(params string) string {
	parts := strings.Fields(params)
	if len(parts) < 3 {
		return "Ошибка: недостаточно параметров. Используйте: read_book_lines имя_файла начальная_строка конечная_строка"
	}

	var filename string
	if strings.HasPrefix(params, "\"") {
		endQuote := strings.Index(params[1:], "\"")
		if endQuote < 0 {
			return "Ошибка: неверный формат. Используйте: read_book_lines \"имя файла.txt\" 1 50"
		}
		filename = params[1 : endQuote+1]
		remaining := strings.TrimSpace(params[endQuote+2:])
		parts = strings.Fields(remaining)
		if len(parts) < 2 {
			return "Ошибка: недостаточно параметров. Укажите начальную и конечную строку."
		}
	} else {
		filename = parts[0]
		parts = parts[1:]
	}

	if len(parts) < 2 {
		return "Ошибка: недостаточно параметров. Укажите начальную и конечную строку."
	}

	var startLine, endLine int
	_, err := fmt.Sscanf(parts[0], "%d", &startLine)
	if err != nil {
		return fmt.Sprintf("Ошибка: начальная строка должна быть числом, получено %q.", parts[0])
	}
	_, err = fmt.Sscanf(parts[1], "%d", &endLine)
	if err != nil {
		return fmt.Sprintf("Ошибка: конечная строка должна быть числом, получено %q.", parts[1])
	}

	fullPath, errMsg := resolveFile(filename)
	if errMsg != "" {
		return errMsg
	}

	if startLine < 1 {
		return "Ошибка: номер строки должен быть >= 1."
	}
	if startLine > endLine {
		return "Ошибка: начальная строка больше конечной."
	}
	if endLine-startLine > 1000 {
		return "Ошибка: слишком большой диапазон (максимум 1000 строк за вызов)."
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return fmt.Sprintf("Ошибка: не удалось открыть файл %q.", filename)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	currentLine := 1
	var b strings.Builder
	fmt.Fprintf(&b, "--- Строки %d-%d из %q ---\n", startLine, endLine, filename)

	for scanner.Scan() {
		if currentLine > endLine {
			break
		}
		if currentLine >= startLine {
			fmt.Fprintf(&b, "%d: %s\n", currentLine, scanner.Text())
		}
		currentLine++
	}

	if currentLine <= startLine {
		totalLines := currentLine - 1
		return fmt.Sprintf("Файл %q содержит только %d строк. Запрошен диапазон %d-%d.", filename, totalLines, startLine, endLine)
	}

	return b.String()
}

func searchInBookTool(params string) string {
	params = strings.TrimSpace(params)

	if params == "" {
		return "Ошибка: не указаны параметры. Используйте: search_in_book имя_файла \"паттерн\" [режим]"
	}

	if strings.HasPrefix(params, "\"") {
		endQuote := strings.Index(params[1:], "\"")
		if endQuote < 0 {
			return "Ошибка: неверный формат. Используйте: search_in_book \"имя файла.txt\" \"паттерн\""
		}
		filename := params[1 : endQuote+1]
		remaining := strings.TrimSpace(params[endQuote+2:])
		if remaining == "" {
			return "Ошибка: не указан паттерн для поиска."
		}
		if strings.HasPrefix(remaining, "\"") {
			endQuote2 := strings.Index(remaining[1:], "\"")
			if endQuote2 < 0 {
				return "Ошибка: неверный формат паттерна."
			}
			pattern := remaining[1 : endQuote2+1]
			fullPath, errMsg := resolveFile(filename)
			if errMsg != "" {
				return errMsg
			}
			mode := strings.TrimSpace(remaining[endQuote2+2:])
			return runSearch(fullPath, filename, pattern, mode)
		}
		mode, pattern := splitLastMode(remaining)
		fullPath, errMsg := resolveFile(filename)
		if errMsg != "" {
			return errMsg
		}
		return runSearch(fullPath, filename, pattern, mode)
	}

	parts := strings.Fields(params)
	if len(parts) < 2 {
		return "Ошибка: недостаточно параметров. Используйте: search_in_book имя_файла \"паттерн\""
	}
	filename := parts[0]
	remaining := strings.Join(parts[1:], " ")
	mode, pattern := splitLastMode(remaining)
	fullPath, errMsg := resolveFile(filename)
	if errMsg != "" {
		return errMsg
	}
	return runSearch(fullPath, filename, pattern, mode)
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

func bookInfoTool(params string) string {
	params = strings.TrimSpace(params)
	if params == "" {
		return "Ошибка: не указано имя файла. Используйте: book_info имя_файла"
	}

	params = strings.Trim(params, "\"")

	fullPath, errMsg := resolveFile(params)
	if errMsg != "" {
		return errMsg
	}

	fi, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Sprintf("Ошибка: не удалось получить информацию о файле %q.", params)
	}

	lines := countLines(fullPath)
	modTime := fi.ModTime().Format("2006-01-02 15:04:05")

	var b strings.Builder
	fmt.Fprintf(&b, "Файл: %s\n", params)
	fmt.Fprintf(&b, "Размер: %s (%d байт)\n", formatFileSize(fi.Size()), fi.Size())
	fmt.Fprintf(&b, "Строк: %d\n", lines)
	fmt.Fprintf(&b, "Символов: %d\n", fi.Size())
	fmt.Fprintf(&b, "Последнее изменение: %s\n", modTime)
	return b.String()
}
