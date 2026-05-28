// Конвертер PDF -> Markdown с помощью Xpdf pdftotext.
//
// На вход подаётся папка с PDF-файлами. Программа находит все .pdf,
// конвертирует каждый в .md с тем же именем, используя утилиту
// pdftotext.exe из состава Xpdf Tools.
//
// Поддерживает русский (кириллица) и английский языки через UTF-8.
//
// Использование:
//
//	go run ./cmd/pdftotext -folder <путь_к_папке> -j 4
//	go run ./cmd/pdftotext -j 10               # конвертировать PDF в текущей папке
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// ========== ПУТИ К ИНСТРУМЕНТАМ (при необходимости отредактируйте) ==========

// Путь к папке с Xpdf Tools (версия 4.06)
const xpdfDir = `C:\Users\e19700019\Downloads\xpdf-tools-win-4.06`

// Используем 64-битную версию pdftotext.exe
const pdftotextExe = xpdfDir + `\bin64\pdftotext.exe`

// Путь к файлам поддержки кириллицы (русские шрифты/кодировки)
const cyrillicDir = `C:\Users\e19700019\Downloads\xpdf-cyrillic.tar\xpdf-cyrillic`

// Файл сопоставления имён символов с Unicode (для болгарской/русской кириллицы)
const nameToUnicodeFile = cyrillicDir + `\Bulgarian.nameToUnicode`

// Unicode-карта для кодировки KOI8-R (нужна для корректного вывода русского текста)
const unicodeMapFile = cyrillicDir + `\KOI8-R.unicodeMap`

// Максимальное количество параллельных потоков
const maxWorkersLimit = 10

// Глобальная блокировка для синхронизации вывода из разных goroutine
var printMu sync.Mutex

// Счётчики результатов (защищены мьютексом)
var (
	mu         sync.Mutex
	totalOK    int
	totalEmpty int
	totalFail  int
)

func checkPrerequisites() {
	// Проверяем, есть ли сам pdftotext.exe
	if _, err := os.Stat(pdftotextExe); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "ОШИБКА: pdftotext.exe не найден: %s\n", pdftotextExe)
		os.Exit(1)
	}

	// Проверяем папку с кириллическими файлами
	if info, err := os.Stat(cyrillicDir); os.IsNotExist(err) || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "ОШИБКА: папка кириллической поддержки не найдена: %s\n", cyrillicDir)
		os.Exit(1)
	}

	// Проверяем файл сопоставления имён символов
	if _, err := os.Stat(nameToUnicodeFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "ОШИБКА: файл не найден: %s\n", nameToUnicodeFile)
		os.Exit(1)
	}

	// Проверяем файл Unicode-карты KOI8-R
	if _, err := os.Stat(unicodeMapFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "ОШИБКА: файл не найден: %s\n", unicodeMapFile)
		os.Exit(1)
	}
}

func createXpdfrc(xpdfrcPath string) error {
	// Создаёт временный конфигурационный файл xpdfrc.
	//
	// Зачем это нужно:
	// - По умолчанию pdftotext не знает о кириллических шрифтах.
	// - Мы подключаем Bulgarian.nameToUnicode — он помогает правильно
	//   распознавать имена символов в PDF (кириллица).
	// - Подключаем KOI8-R.unicodeMap — карта для преобразования KOI8-R в Unicode.
	// - Устанавливаем textEncoding=UTF-8, чтобы на выходе был читаемый
	//   текст в UTF-8 (поддерживает и русский, и английский).
	content := fmt.Sprintf(`# xpdfrc - сгенерировано pdftotext
nameToUnicode		%s
unicodeMap	KOI8-R		%s
textEncoding	UTF-8
`, nameToUnicodeFile, unicodeMapFile)

	return os.WriteFile(xpdfrcPath, []byte(content), 0644)
}

func convertPDF(pdfPath, outputPath, xpdfrcPath string) bool {
	// Формируем команду для запуска pdftotext.
	// Параметры:
	//   -cfg <файл>   - указываем наш конфиг с кириллицей и UTF-8
	//   -enc UTF-8    - выходная кодировка: Unicode (для русского + английского)
	//   -layout       - сохраняет физическую структуру страницы (колонки, таблицы)
	//   -nopgbrk      - не вставлять символы разрыва страницы (чтобы .md был чистым)
	//   -eol dos      - концы строк в формате Windows (CR+LF)
	cmd := exec.Command(
		pdftotextExe,
		"-cfg", xpdfrcPath,
		"-enc", "UTF-8",
		"-layout",
		"-nopgbrk",
		"-eol", "dos",
		pdfPath,
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Проверяем, завершился ли pdftotext с ненулевым кодом
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(output))
			if stderr != "" {
				fmt.Fprintf(os.Stderr, "  ОШИБКА: pdftotext завершился с кодом %d (%s)\n",
					exitErr.ExitCode(), stderr)
			} else {
				fmt.Fprintf(os.Stderr, "  ОШИБКА: pdftotext завершился с кодом %d\n",
					exitErr.ExitCode())
			}
		} else {
			fmt.Fprintf(os.Stderr, "  ОШИБКА: не удалось запустить pdftotext: %v\n", err)
		}
		return false
	}

	return true
}

func trimOutputFile(outputPath string) bool {
	// Обрезает лишние пробелы в начале и конце каждой строки файла.
	//
	// Затем, если после обработки файл оказался пустым (нет ни одного
	// непустого символа) — удаляет его и возвращает false.
	//
	// Это нужно для двух целей:
	//   1. Убрать "мусорные" пробелы, которые pdftotext иногда оставляет.
	//   2. Не создавать .md файлы, если PDF оказался пустым или
	//      нечитаемым (pdftotext создаёт пустой файл, но он нам не нужен).

	file, err := os.Open(outputPath)
	if err != nil {
		// Если файл не читается — не трогаем его
		return true
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Обрезаем пробелы в начале и конце каждой строки
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		return true
	}

	// Склеиваем обратно
	trimmed := strings.Join(lines, "\n")
	// Убираем лишние пробелы в начале и конце всего текста,
	// добавляем финальный перевод строки
	trimmed = strings.TrimSpace(trimmed) + "\n"

	// Если после обработки текст пуст — удаляем файл
	if strings.TrimSpace(trimmed) == "" {
		os.Remove(outputPath)
		return false
	}

	// Записываем обработанный текст обратно
	if err := os.WriteFile(outputPath, []byte(trimmed), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "  ОШИБКА: не удалось записать %s: %v\n", outputPath, err)
		return true
	}

	return true
}

func convertWorker(pdfPath string, idx, total int, xpdfrcPath string, wg *sync.WaitGroup, sem chan struct{}) {
	defer wg.Done()
	defer func() { <-sem }() // освобождаем слот в семафоре

	// Формируем путь к выходному файлу (.md вместо .pdf)
	outputPath := strings.TrimSuffix(pdfPath, filepath.Ext(pdfPath)) + ".md"

	// Печатаем статус начала конвертации (синхронизированно)
	printMu.Lock()
	fmt.Printf("[%d/%d] Конвертация: %s\n", idx, total, filepath.Base(pdfPath))
	printMu.Unlock()

	success := convertPDF(pdfPath, outputPath, xpdfrcPath)

	if success {
		hasContent := trimOutputFile(outputPath)
		printMu.Lock()
		if hasContent {
			// Получаем размер файла
			if info, err := os.Stat(outputPath); err == nil {
				sizeKB := float64(info.Size()) / 1024.0
				fmt.Printf("[%d/%d] -> ГОТОВО: %s (%.1f КБ)\n",
					idx, total, filepath.Base(outputPath), sizeKB)
			} else {
				fmt.Printf("[%d/%d] -> ГОТОВО: %s\n",
					idx, total, filepath.Base(outputPath))
			}
			mu.Lock()
			totalOK++
			mu.Unlock()
		} else {
			fmt.Printf("[%d/%d] -> ПУСТОЙ (файл удалён)\n", idx, total)
			mu.Lock()
			totalEmpty++
			mu.Unlock()
		}
		printMu.Unlock()
	} else {
		printMu.Lock()
		fmt.Printf("[%d/%d] -> ОШИБКА\n", idx, total)
		printMu.Unlock()
		mu.Lock()
		totalFail++
		mu.Unlock()
	}
}

func convertFolder(folder string, xpdfrcPath string, maxWorkers int) {
	// Находим все PDF-файлы в указанной папке
	pattern := filepath.Join(folder, "*.pdf")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: не удалось прочитать папку %s: %v\n", folder, err)
		return
	}

	// Если PDF-файлов нет — сообщаем и выходим
	if len(matches) == 0 {
		fmt.Printf("PDF-файлы не найдены в папке: %s\n", folder)
		return
	}

	total := len(matches)

	fmt.Printf("Найдено %d PDF-файл(ов) в: %s\n", total, folder)
	fmt.Printf("Результаты сохраняются в: %s\n", folder)
	if maxWorkers > 1 {
		fmt.Printf("Параллельных потоков: %d\n", maxWorkers)
	} else {
		fmt.Printf("Режим: последовательный (1 поток)\n")
	}
	fmt.Println(strings.Repeat("-", 60))

	// Семафор для ограничения количества параллельных задач
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for idx, pdfPath := range matches {
		wg.Add(1)
		sem <- struct{}{} // захватываем слот (блокируется, если все заняты)
		go convertWorker(pdfPath, idx+1, total, xpdfrcPath, &wg, sem)
	}

	// Ждём завершения всех задач
	wg.Wait()

	fmt.Println(strings.Repeat("-", 60))

	// Формируем строку статистики
	var parts []string
	if totalOK > 0 {
		parts = append(parts, fmt.Sprintf("%d успешно", totalOK))
	}
	if totalEmpty > 0 {
		parts = append(parts, fmt.Sprintf("%d пустых", totalEmpty))
	}
	if totalFail > 0 {
		parts = append(parts, fmt.Sprintf("%d с ошибками", totalFail))
	}
	fmt.Printf("Завершено: %s из %d\n", strings.Join(parts, ", "), total)
}

func main() {
	// Разбираем аргументы командной строки
	var (
		folder   = flag.String("folder", ".", "Путь к папке с PDF-файлами (по умолчанию: текущая папка)")
		jobs     = flag.Int("j", 4, "Количество параллельных потоков (1-10, по умолчанию: 4)")
		showHelp = flag.Bool("help", false, "Показать справку")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Конвертер PDF -> Markdown с помощью Xpdf pdftotext\n")
		fmt.Fprintf(os.Stderr, "\nИспользование:\n")
		fmt.Fprintf(os.Stderr, "  pdftotext -folder <путь_к_папке> -j 4\n")
		fmt.Fprintf(os.Stderr, "  pdftotext -j 10\n")
		fmt.Fprintf(os.Stderr, "\nПараметры:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}

	// Проверяем количество потоков
	maxWorkers := *jobs
	if maxWorkers < 1 {
		fmt.Fprintln(os.Stderr, "ОШИБКА: число потоков (-j) должно быть >= 1")
		os.Exit(1)
	}
	if maxWorkers > maxWorkersLimit {
		fmt.Fprintf(os.Stderr, "ОШИБКА: число потоков (-j) не может превышать %d\n", maxWorkersLimit)
		os.Exit(1)
	}

	// Преобразуем в абсолютный путь
	absFolder, err := filepath.Abs(*folder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: не удалось получить абсолютный путь: %v\n", err)
		os.Exit(1)
	}

	// Проверяем, что папка существует
	if info, err := os.Stat(absFolder); os.IsNotExist(err) || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "ОШИБКА: папка не найдена: %s\n", absFolder)
		os.Exit(1)
	}

	fmt.Printf("Конвертер PDF -> Markdown\n")
	fmt.Printf("Используется pdftotext: %s\n", pdftotextExe)
	fmt.Printf("Папка с PDF:           %s\n", absFolder)
	fmt.Println()

	// Проверяем, что все инструменты на месте
	checkPrerequisites()

	// Создаём временный файл конфигурации xpdfrc
	tmpFile, err := os.CreateTemp("", "xpdfrc-*.tmp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: не удалось создать временный файл: %v\n", err)
		os.Exit(1)
	}
	xpdfrcPath := tmpFile.Name()
	tmpFile.Close()

	// Удаляем временный файл при выходе
	defer os.Remove(xpdfrcPath)

	// Записываем конфиг с кириллической поддержкой
	if err := createXpdfrc(xpdfrcPath); err != nil {
		fmt.Fprintf(os.Stderr, "ОШИБКА: не удалось записать конфиг xpdfrc: %v\n", err)
		os.Exit(1)
	}

	// Запускаем конвертацию всех PDF (с указанным числом потоков)
	convertFolder(absFolder, xpdfrcPath, maxWorkers)
}
