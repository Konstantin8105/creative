package creative

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Данные инструменты только для временного тестирования
func TestToolPatterns(t *testing.T) {
	testdata := "testdata"
	t.Run("search_in_book", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(testdata, "tool.search_in_book"))
		if err != nil {
			return
		}
		data = bytes.ReplaceAll(data, []byte("\r"), []byte{})
		lines := strings.Split(string(data), "\n")
		for il, line := range lines {
			if line == "" {
				continue
			}
			t.Run(fmt.Sprintf("%02d", il), func(t *testing.T) {
				result := searchInBookTool([]string{testdata}, line)
				t.Logf("%s", result)
			})
		}
	})
	t.Run("read_book_lines", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(testdata, "tool.read_book_lines"))
		if err != nil {
			return
		}
		data = bytes.ReplaceAll(data, []byte("\r"), []byte{})
		lines := strings.Split(string(data), "\n")
		for il, line := range lines {
			if line == "" {
				continue
			}
			t.Run(fmt.Sprintf("%02d", il), func(t *testing.T) {
				result := readBookLinesTool([]string{testdata}, line)
				t.Logf("%s", result)
			})
		}
	})
}
