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
			Description: "Показать список всех .txt и .md файлов в папке книг. Возвращает имена файлов, размер и количество строк.",
			Execute:     listBooksTool,
		},
		{
			Name:        "read_book_lines",
			Description: "Прочитать строки из книги. Параметры: имя_файла начальная_строка конечная_строка. Нумерация строк с 1. Максимум 1000 строк за вызов. Пример: read_book_lines война_и_мир.txt 1 50",
			Execute:     readBookLinesTool,
		},
		{
			Name:        "search_in_book",
			Description: "Поиск в книге по ключевым словам или регулярному выражению. Параметры: имя_файла \"паттерн\" [режим]. Режимы: keyword (поиск подстроки, по умолчанию) или regex. Пример: search_in_book война_и_мир.txt \"Наполеон\" или search_in_book file.md \"\\bGo\\b\" regex",
			Execute:     searchInBookTool,
		},
		{
			Name:        "book_info",
			Description: "Показать мета-информацию о книге: размер, количество строк, количество символов, дата изменения. Параметры: имя_файла. Пример: book_info война_и_мир.txt",
			Execute:     bookInfoTool,
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
	// Проверка расширения
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".txt" && ext != ".md" {
		return "", fmt.Sprintf("Ошибка: файл %q имеет расширение %s. Поддерживаются только файлы .txt и .md.", filename, ext)
	}
	// Проверка, что файл не выходит за пределы BooksFolder
	fullPath = filepath.Clean(filepath.Join(BooksFolder, filename))
	booksFolderClean := filepath.Clean(BooksFolder)
	if !strings.HasPrefix(fullPath, booksFolderClean+string(os.PathSeparator)) &&
		fullPath != booksFolderClean {
		return "", fmt.Sprintf("Ошибка: файл %q должен находиться в папке книг.", filename)
	}
	// Проверка существования
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

// countLines возвращает количество строк в файле.
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

// formatFileSize форматирует размер файла в человекочитаемый вид.
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

// listBooksTool возвращает список всех .txt и .md файлов в BooksFolder.
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
			return nil // skip inaccessible files
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

// readBookLinesTool читает строки из файла в заданном диапазоне.
func readBookLinesTool(params string) string {
	// Парсинг параметров: filename start_line end_line
	parts := strings.Fields(params)
	if len(parts) < 3 {
		return "Ошибка: недостаточно параметров. Используйте: read_book_lines имя_файла начальная_строка конечная_строка"
	}

	// Имя файла может содержать пробелы, если оно заключено в кавычки
	var filename string
	if strings.HasPrefix(params, "\"") {
		// Имя файла в кавычках
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
		// Простой случай — имя файла без пробелов
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

	// Если запрошенный диапазон выходит за конец файла
	if currentLine <= startLine {
		totalLines := currentLine - 1
		return fmt.Sprintf("Файл %q содержит только %d строк. Запрошен диапазон %d-%d.", filename, totalLines, startLine, endLine)
	}

	return b.String()
}

// searchInBookTool ищет по ключевым словам или regex в файле.
func searchInBookTool(params string) string {
	// Парсинг: filename "pattern" [mode]
	// Имя файла может быть в кавычках или без
	var filename, pattern, mode string

	// Убираем лишние пробелы
	params = strings.TrimSpace(params)

	if params == "" {
		return "Ошибка: не указаны параметры. Используйте: search_in_book имя_файла \"паттерн\" [режим]"
	}

	// Пытаемся найти имя файла (может быть в кавычках или без)
	if strings.HasPrefix(params, "\"") {
		endQuote := strings.Index(params[1:], "\"")
		if endQuote < 0 {
			return "Ошибка: неверный формат. Используйте: search_in_book \"имя файла.txt\" \"паттерн\""
		}
		filename = params[1 : endQuote+1]
		remaining := strings.TrimSpace(params[endQuote+2:])
		if remaining == "" {
			return "Ошибка: не указан паттерн для поиска."
		}
		// Теперь оставшаяся часть: "pattern" [mode] или pattern [mode]
		if strings.HasPrefix(remaining, "\"") {
			endQuote2 := strings.Index(remaining[1:], "\"")
			if endQuote2 < 0 {
				return "Ошибка: неверный формат паттерна."
			}
			pattern = remaining[1 : endQuote2+1]
			mode = strings.TrimSpace(remaining[endQuote2+2:])
		} else {
			parts := strings.Fields(remaining)
			pattern = parts[0]
			if len(parts) > 1 {
				mode = parts[1]
			}
		}
	} else {
		parts := strings.Fields(params)
		if len(parts) < 2 {
			return "Ошибка: недостаточно параметров. Используйте: search_in_book имя_файла \"паттерн\""
		}
		filename = parts[0]
		// Паттерн — второй параметр, остальное опционально
		if len(parts) >= 3 {
			pattern = parts[1]
			mode = parts[2]
		} else {
			pattern = parts[1]
		}
	}

	if pattern == "" {
		return "Ошибка: не указан паттерн для поиска. Используйте search_in_book имя_файла \"текст для поиска\""
	}

	fullPath, errMsg := resolveFile(filename)
	if errMsg != "" {
		return errMsg
	}

	// Определяем режим поиска
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" || mode == "keyword" {
		// Поиск по ключевым словам (регистронезависимый)
		return searchByKeyword(fullPath, filename, pattern)
	} else if mode == "regex" {
		return searchByRegex(fullPath, filename, pattern)
	} else {
		return fmt.Sprintf("Ошибка: неизвестный режим %q. Используйте 'keyword' или 'regex'.", mode)
	}
}

// searchByKeyword ищет регистронезависимые вхождения подстроки.
func searchByKeyword(fullPath, filename, pattern string) string {
	f, err := os.Open(fullPath)
	if err != nil {
		return fmt.Sprintf("Ошибка: не удалось открыть файл %q.", filename)
	}
	defer f.Close()

	patternLower := strings.ToLower(pattern)
	scanner := bufio.NewScanner(f)
	var b strings.Builder
	lineNum := 0
	count := 0
	maxResults := 50

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), patternLower) {
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

// searchByRegex ищет по регулярному выражению.
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

// bookInfoTool возвращает мета-информацию о файле.
func bookInfoTool(params string) string {
	params = strings.TrimSpace(params)
	if params == "" {
		return "Ошибка: не указано имя файла. Используйте: book_info имя_файла"
	}

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
