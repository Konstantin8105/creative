package creative_test

import (
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

	t.Run("search_in_book_unknown_mode", func(t *testing.T) {
		result := executeTool(t, "search_in_book", "book_sample.txt текст invalid_mode")
		if !strings.Contains(result, "Ошибка") {
			t.Errorf("should reject unknown mode, got:\n%s", result)
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
}
