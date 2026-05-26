package creative_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Konstantin8105/creative"
)

func TestBookTools(t *testing.T) {
	// Устанавливаем BooksFolder на testdata
	testdata := func(name string) string {
		return filepath.Join("testdata", name)
	}
	_ = testdata // used in read_book_lines for params

	// Сохраняем и восстанавливаем BooksFolder
	oldFolder := creative.BooksFolder
	creative.BooksFolder = "testdata"
	defer func() { creative.BooksFolder = oldFolder }()

	// Получаем инструменты
	tools := creative.BookTools()

	// executeTool выполняет инструмент по имени с параметрами
	executeTool := func(t *testing.T, name, params string) string {
		t.Helper()
		result, err := creative.ExecuteTool(name, params, tools)
		if err != nil {
			t.Fatalf("ExecuteTool(%q, %q): %v", name, params, err)
		}
		return result
	}

	t.Run("list_books", func(t *testing.T) {
		// В testdata есть файлы .txt и .md
		result := executeTool(t, "list_books", "")
		if !strings.Contains(result, "book_sample.txt") {
			t.Errorf("list_books should contain book_sample.txt, got:\n%s", result)
		}
		if !strings.Contains(result, "book_sample.md") {
			t.Errorf("list_books should contain book_sample.md, got:\n%s", result)
		}
		if !strings.Contains(result, "empty.txt") {
			t.Errorf("list_books should contain empty.txt, got:\n%s", result)
		}
		if !strings.Contains(result, "строк") {
			t.Errorf("list_books should show line count, got:\n%s", result)
		}
	})

	t.Run("list_books_nonexistent_folder", func(t *testing.T) {
		creative.BooksFolder = "nonexistent_folder_xyz"
		defer func() { creative.BooksFolder = "testdata" }()
		result := executeTool(t, "list_books", "")
		if !strings.Contains(result, "Ошибка") {
			t.Errorf("expected error for non-existent folder, got:\n%s", result)
		}
	})

	t.Run("list_books_empty_booksfolder", func(t *testing.T) {
		creative.BooksFolder = ""
		defer func() { creative.BooksFolder = "testdata" }()
		result := executeTool(t, "list_books", "")
		if !strings.Contains(result, "Ошибка") {
			t.Errorf("expected error for empty BooksFolder, got:\n%s", result)
		}
	})

	t.Run("read_book_lines_valid_range", func(t *testing.T) {
		result := executeTool(t, "read_book_lines", "book_sample.txt 1 5")
		if !strings.Contains(result, "Строки 1-5") {
			t.Errorf("should show range, got:\n%s", result)
		}
		if !strings.Contains(result, "Глава 1") {
			t.Errorf("should contain first chapter, got:\n%s", result)
		}
		if !strings.Contains(result, "1:") {
			t.Errorf("should contain line numbers, got:\n%s", result)
		}
	})

	t.Run("read_book_lines_with_quoted_filename", func(t *testing.T) {
		result := executeTool(t, "read_book_lines", "\"book_sample.txt\" 1 3")
		if !strings.Contains(result, "1:") {
			t.Errorf("should read lines with quoted filename, got:\n%s", result)
		}
	})

	t.Run("read_book_lines_out_of_bounds", func(t *testing.T) {
		result := executeTool(t, "read_book_lines", "book_sample.txt 1 999999")
		if !strings.Contains(result, "слишком большой диапазон") {
			t.Errorf("should reject >1000 lines, got:\n%s", result)
		}
	})

	t.Run("read_book_lines_beyond_file", func(t *testing.T) {
		result := executeTool(t, "read_book_lines", "book_sample.txt 200 210")
		if !strings.Contains(result, "содержит только") {
			t.Errorf("should report file is shorter, got:\n%s", result)
		}
	})

	t.Run("read_book_lines_negative_start", func(t *testing.T) {
		result := executeTool(t, "read_book_lines", "book_sample.txt -5 10")
		if !strings.Contains(result, "Ошибка") {
			t.Errorf("should reject negative line, got:\n%s", result)
		}
	})

	t.Run("read_book_lines_start_gt_end", func(t *testing.T) {
		result := executeTool(t, "read_book_lines", "book_sample.txt 10 5")
		if !strings.Contains(result, "Ошибка") {
			t.Errorf("should reject start > end, got:\n%s", result)
		}
	})

	t.Run("read_book_lines_nonexistent_file", func(t *testing.T) {
		result := executeTool(t, "read_book_lines", "nonexistent.txt 1 10")
		if !strings.Contains(result, "не найден") {
			t.Errorf("should report file not found, got:\n%s", result)
		}
	})

	t.Run("read_book_lines_empty_filename", func(t *testing.T) {
		result := executeTool(t, "read_book_lines", "")
		if !strings.Contains(result, "Ошибка") {
			t.Errorf("should reject empty params, got:\n%s", result)
		}
	})

	t.Run("read_book_lines_bad_extension", func(t *testing.T) {
		result := executeTool(t, "read_book_lines", "book_sample.go 1 5")
		if !strings.Contains(result, "Поддерживаются только файлы") {
			t.Errorf("should reject .go extension, got:\n%s", result)
		}
	})

	t.Run("search_in_book_keyword", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.txt Париж")
		if !strings.Contains(result, "keyword") {
			t.Errorf("should show keyword mode, got:\n%s", result)
		}
		if !strings.Contains(result, "Париж") {
			t.Errorf("should find 'Париж', got:\n%s", result)
		}
		if !strings.Contains(result, "Строка") {
			t.Errorf("should show line numbers, got:\n%s", result)
		}
	})

	t.Run("search_in_book_regex", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.md Go regex")
		if !strings.Contains(result, "regex") {
			t.Errorf("should show regex mode, got:\n%s", result)
		}
		if !strings.Contains(result, "Строка") {
			t.Errorf("should find something with regex, got:\n%s", result)
		}
	})

	t.Run("search_in_book_no_match", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.txt ZZZZZnotfound")
		if !strings.Contains(result, "не найдено") {
			t.Errorf("should report no matches, got:\n%s", result)
		}
	})

	t.Run("search_in_book_invalid_regex", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.txt \"[\" regex")
		if !strings.Contains(result, "Ошибка") {
			t.Errorf("should reject invalid regex, got:\n%s", result)
		}
	})

	t.Run("search_in_book_empty_pattern", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.txt")
		if !strings.Contains(result, "Ошибка") {
			t.Errorf("should reject empty pattern, got:\n%s", result)
		}
	})

	t.Run("search_in_book_quoted_filename", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "\"book_sample.txt\" Москва")
		if !strings.Contains(result, "Москва") {
			t.Errorf("should find with quoted filename, got:\n%s", result)
		}
	})

	t.Run("search_in_book_pattern_with_spaces", func(t *testing.T) {
		// Сложный паттерн с пробелами не должен выдавать "неизвестный режим"
		result := executeTool(t, "search_in_book", "book_sample.txt румяное яблоко")
		if !strings.Contains(result, "румяное яблоко") {
			t.Errorf("should find the multi-word pattern, got:\n%s", result)
		}
	})

	t.Run("book_info", func(t *testing.T) {
		result := executeTool(t, "book_info", "book_sample.txt")
		if !strings.Contains(result, "Файл:") {
			t.Errorf("should show file info, got:\n%s", result)
		}
		if !strings.Contains(result, "Строк:") {
			t.Errorf("should show line count, got:\n%s", result)
		}
		if !strings.Contains(result, "Размер:") {
			t.Errorf("should show file size, got:\n%s", result)
		}
	})

	t.Run("book_info_empty_file", func(t *testing.T) {
		result := executeTool(t, "book_info", "empty.txt")
		if !strings.Contains(result, "Строк: 0") {
			t.Errorf("empty file should have 0 lines, got:\n%s", result)
		}
	})

	t.Run("book_info_nonexistent_file", func(t *testing.T) {
		result := executeTool(t, "book_info", "nonexistent.txt")
		if !strings.Contains(result, "Ошибка") {
			t.Errorf("should report error for nonexistent file, got:\n%s", result)
		}
	})

	t.Run("book_info_empty_params", func(t *testing.T) {
		result := executeTool(t, "book_info", "")
		if !strings.Contains(result, "Ошибка") {
			t.Errorf("should reject empty params, got:\n%s", result)
		}
	})

	t.Run("search_in_book_markdown", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.md интерфейс keyword")
		if !strings.Contains(result, "интерфейс") || !strings.Contains(result, "Строка") {
			t.Errorf("should find 'интерфейс' in markdown, got:\n%s", result)
		}
	})

	t.Run("search_in_book_keyword_pipe_or", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.txt Москва|Париж")
		if !strings.Contains(result, "keyword") {
			t.Errorf("should show keyword mode, got:\n%s", result)
		}
		if !strings.Contains(result, "Москва") {
			t.Errorf("pipe OR should find 'Москва', got:\n%s", result)
		}
		if !strings.Contains(result, "Париж") {
			t.Errorf("pipe OR should find 'Париж', got:\n%s", result)
		}
		if !strings.Contains(result, "Найдено") {
			t.Errorf("pipe OR should find matches, got:\n%s", result)
		}
	})

	t.Run("search_in_book_keyword_pipe_or_single_match", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.txt Москва|ZZZZZ")
		if !strings.Contains(result, "Москва") {
			t.Errorf("pipe OR single match should find 'Москва', got:\n%s", result)
		}
	})

	t.Run("search_in_book_keyword_pipe_leading", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.txt |Москва")
		if !strings.Contains(result, "Москва") {
			t.Errorf("pipe OR with leading pipe should find 'Москва', got:\n%s", result)
		}
	})

	t.Run("search_in_book_keyword_pipe_trailing", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.txt Москва|")
		if !strings.Contains(result, "Москва") {
			t.Errorf("pipe OR with trailing pipe should find 'Москва', got:\n%s", result)
		}
	})

	t.Run("search_in_book_keyword_pipe_double", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.txt Москва||Париж")
		if !strings.Contains(result, "Москва") {
			t.Errorf("pipe OR with double pipe should find 'Москва', got:\n%s", result)
		}
		if !strings.Contains(result, "Париж") {
			t.Errorf("pipe OR with double pipe should find 'Париж', got:\n%s", result)
		}
	})

	t.Run("search_in_book_single_term_still_works", func(t *testing.T) {
		// Regression test: single term without pipe should still work
		result := executeTool(t, "search_in_book", "book_sample.txt Париж")
		if !strings.Contains(result, "Париж") {
			t.Errorf("single term should find 'Париж', got:\n%s", result)
		}
	})

	// ─── Auto-detect regex tests ───
	// When a pattern contains regex metacharacters (like . * + [ ] ( ) etc.),
	// the tool should auto-detect it as regex and use regex search instead of keyword.

	t.Run("search_in_book_auto_regex_dot_star", func(t *testing.T) {
		// Pattern "нач.*путеш" should auto-detect and find "Так началось моё путешествие"
		result := executeTool(t, "search_in_book", "book_sample.txt нач.*путеш")
		if !strings.Contains(result, "regex") {
			t.Errorf("auto-detect should show regex mode, got:\n%s", result)
		}
		if !strings.Contains(result, "началось моё путешествие") {
			t.Errorf("auto-detect regex should find 'началось моё путешествие', got:\n%s", result)
		}
	})

	t.Run("search_in_book_auto_regex_start_anchor", func(t *testing.T) {
		// Pattern "^Глава" should auto-detect and find chapter headers
		result := executeTool(t, "search_in_book", "book_sample.txt ^Глава")
		if !strings.Contains(result, "regex") {
			t.Errorf("auto-detect should show regex mode, got:\n%s", result)
		}
		if !strings.Contains(result, "Глава") {
			t.Errorf("auto-detect regex '^Глава' should find chapters, got:\n%s", result)
		}
	})

	t.Run("search_in_book_auto_regex_alternation", func(t *testing.T) {
		// Pattern "Москв[уа]|Пет[е]рбург" should auto-detect and find Moscow or Petersburg
		result := executeTool(t, "search_in_book", "book_sample.txt Москв[уа]")
		if !strings.Contains(result, "regex") {
			t.Errorf("auto-detect should show regex mode, got:\n%s", result)
		}
		if !strings.Contains(result, "Москва") {
			t.Errorf("auto-detect regex should find 'Москва', got:\n%s", result)
		}
	})

	t.Run("search_in_book_auto_regex_plus_quantifier", func(t *testing.T) {
		// Pattern "\\d+" should auto-detect as regex
		result := executeTool(t, "search_in_book", "book_sample.md \\d+")
		if !strings.Contains(result, "regex") {
			t.Errorf("auto-detect should show regex mode, got:\n%s", result)
		}
		if !strings.Contains(result, "Строка") {
			t.Errorf("auto-detect regex '\\d+' should match lines, got:\n%s", result)
		}
	})

	t.Run("search_in_book_keyword_not_affected_by_dash", func(t *testing.T) {
		// Simple keyword with hyphens should NOT be auto-detected as regex.
		// "Серое небо" (with space) is in the file, but "Серое-небо" (with hyphen) is not.
		// The search should stay in keyword mode and report no matches — not switch to regex.
		result := executeTool(t, "search_in_book", "book_sample.txt Серое-небо")
		// Should NOT contain "regex" — the hyphen is not a regex metacharacter
		if strings.Contains(result, "regex") {
			t.Errorf("simple keyword with dash should NOT switch to regex mode, got:\n%s", result)
		}
		// Should say "не найдено" because there's no literal "Серое-небо" in the file
		if !strings.Contains(result, "не найдено") {
			t.Errorf("should report no matches for 'Серое-небо', got:\n%s", result)
		}
	})

	t.Run("search_in_book_explicit_regex_still_works", func(t *testing.T) {
		// Explicit regex mode should still work as before
		result := executeTool(t, "search_in_book", "book_sample.md Go regex")
		if !strings.Contains(result, "regex") {
			t.Errorf("should show regex mode, got:\n%s", result)
		}
		if !strings.Contains(result, "Строка") {
			t.Errorf("should find something with regex, got:\n%s", result)
		}
	})

	// Tests for filenames containing spaces (regression for native tool call bug).
	// These simulate what happens after ToolParamsToString converts JSON args
	// like {"filename":"СП 16.13330.2017.txt"} → "СП 16.13330.2017.txt" (quoted).

	t.Run("book_info_spaced_filename_quoted", func(t *testing.T) {
		// Simulate native tool call: ToolParamsToString produces '"file with spaces.txt"'
		result := executeTool(t, "book_info", "\"book_sample.txt\"")
		if !strings.Contains(result, "Файл:") {
			t.Errorf("book_info with quoted spaced filename should work, got:\n%s", result)
		}
	})

	t.Run("search_in_book_spaced_filename_quoted", func(t *testing.T) {
		// Simulate native tool call with filename containing spaces
		result := executeTool(t, "search_in_book", "\"book_sample.txt\" Париж")
		if !strings.Contains(result, "Париж") {
			t.Errorf("search_in_book with quoted spaced filename should work, got:\n%s", result)
		}
	})

	t.Run("read_book_lines_spaced_filename_quoted", func(t *testing.T) {
		// Simulate native tool call with filename containing spaces
		result := executeTool(t, "read_book_lines", "\"book_sample.txt\" 1 3")
		if !strings.Contains(result, "1:") {
			t.Errorf("read_book_lines with quoted spaced filename should work, got:\n%s", result)
		}
	})

	t.Run("book_info_temp_file_with_spaces", func(t *testing.T) {
		// End-to-end test: create a temp file with spaces in name, verify tool works
		tmpFile := filepath.Join("testdata", "СП 16.13330.2017 Тестовый файл.txt")
		err := os.WriteFile(tmpFile, []byte("Тестовое содержимое\nСтрока 2\n"), 0644)
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile)

		result, err := creative.ExecuteTool("book_info", "\"СП 16.13330.2017 Тестовый файл.txt\"", tools)
		if err != nil {
			t.Fatalf("book_info with spaced filename: %v", err)
		}
		if !strings.Contains(result, "Файл:") || !strings.Contains(result, "Тестовый файл") {
			t.Errorf("book_info should work with spaced filename, got:\n%s", result)
		}
	})

	t.Run("search_in_book_temp_file_with_spaces", func(t *testing.T) {
		// End-to-end test: create a temp file with spaces in name, search in it
		tmpFile := filepath.Join("testdata", "ГОСТ 32569-2013 Трубопроводы.txt")
		err := os.WriteFile(tmpFile, []byte("Тестовое содержимое\nболт М20\nгайка\n"), 0644)
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile)

		result, err := creative.ExecuteTool("search_in_book", "\"ГОСТ 32569-2013 Трубопроводы.txt\" болт", tools)
		if err != nil {
			t.Fatalf("search_in_book with spaced filename: %v", err)
		}
		if !strings.Contains(result, "болт") {
			t.Errorf("search_in_book should find 'болт' in spaced filename, got:\n%s", result)
		}
	})
}
