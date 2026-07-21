package creative

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListAdd_AddLines(t *testing.T) {
	tempDir := t.TempDir()
	tools := ListAddTool(tempDir)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	tool := tools[0]
	if tool.Name != "list_add" {
		t.Errorf("expected name 'list_add', got %q", tool.Name)
	}

	json := `{"name": "mylist", "items": ["first line", "second line"]}`
	result := tool.Execute(json)
	if !strings.Contains(result, "Added 2 lines") {
		t.Errorf("unexpected result: %s", result)
	}

	data, err := os.ReadFile(filepath.Join(tempDir, "_list_mylist.txt"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), lines)
	}
	if lines[0] != "first line" {
		t.Errorf("line 0: got %q", lines[0])
	}
	if lines[1] != "second line" {
		t.Errorf("line 1: got %q", lines[1])
	}
}

func TestListAdd_Append(t *testing.T) {
	tempDir := t.TempDir()
	tool := ListAddTool(tempDir)[0]

	tool.Execute(`{"name": "tasks", "items": ["line1"]}`)
	result := tool.Execute(`{"name": "tasks", "items": ["line2"]}`)
	if !strings.Contains(result, "Added 1 line") {
		t.Errorf("unexpected result: %s", result)
	}

	data, _ := os.ReadFile(filepath.Join(tempDir, "_list_tasks.txt"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[1] != "line2" {
		t.Errorf("expected 'line2', got %q", lines[1])
	}
}

func TestListAdd_EmptyItems(t *testing.T) {
	tempDir := t.TempDir()
	tool := ListAddTool(tempDir)[0]

	result := tool.Execute(`{"name": "empty", "items": []}`)
	if !strings.Contains(result, "Added 0 lines") {
		t.Errorf("unexpected result: %s", result)
	}

	info, err := os.Stat(filepath.Join(tempDir, "_list_empty.txt"))
	if err != nil {
		t.Fatal("file should exist")
	}
	if info.Size() != 0 {
		t.Errorf("expected empty file, size=%d", info.Size())
	}
}

func TestListAdd_SearchViaBookTools(t *testing.T) {
	tempDir := t.TempDir()
	tool := ListAddTool(tempDir)[0]

	tool.Execute(`{"name": "fruits", "items": ["apple", "banana", "cherry"]}`)

	files, err := getFiles([]string{tempDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one file from getFiles")
	}

	path := files[0][0]
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "apple") {
		t.Errorf("expected 'apple' in file content, got: %s", content)
	}
}

func TestListAdd_NameWithSpaces(t *testing.T) {
	tempDir := t.TempDir()
	tool := ListAddTool(tempDir)[0]

	tool.Execute(`{"name": "my tasks", "items": ["item1"]}`)

	info, err := os.Stat(filepath.Join(tempDir, "_list_my_tasks.txt"))
	if err != nil {
		t.Fatal("expected file 'my_tasks.txt'")
	}
	_ = info
}

func TestListAdd_NameWithPathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	tool := ListAddTool(tempDir)[0]

	result := tool.Execute(`{"name": "../escape", "items": ["x"]}`)
	if !strings.Contains(result, "Error") {
		t.Errorf("expected error for path traversal, got: %s", result)
	}
}

func TestListAdd_EmptyName(t *testing.T) {
	tempDir := t.TempDir()
	tool := ListAddTool(tempDir)[0]

	result := tool.Execute(`{"name": "", "items": ["x"]}`)
	if !strings.Contains(result, "Error") {
		t.Errorf("expected error for empty name, got: %s", result)
	}
}

func TestListAdd_NewlineInItem(t *testing.T) {
	tempDir := t.TempDir()
	tool := ListAddTool(tempDir)[0]

	result := tool.Execute(`{"name": "test", "items": ["line with\nnewline"]}`)
	if !strings.Contains(result, "Added 1 line") {
		t.Errorf("unexpected result: %s", result)
	}

	data, _ := os.ReadFile(filepath.Join(tempDir, "_list_test.txt"))
	if strings.Contains(string(data), "\nnewline") {
		t.Errorf("newline should have been replaced, got: %q", string(data))
	}
	if !strings.Contains(string(data), "with newline") {
		t.Errorf("expected 'with newline', got: %q", string(data))
	}
}

func TestSanitizeListName(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
		want    string
	}{
		{"hello", false, "hello"},
		{"Hello World", false, "hello_world"},
		{"../bad", true, ""},
		{"a/b", true, ""},
		{"a\\b", true, ""},
		{"a:b", true, ""},
		{"a*b", true, ""},
		{"a?b", true, ""},
		{"a\"b", true, ""},
		{"a<b", true, ""},
		{"a>b", true, ""},
		{"a|b", true, ""},
		{"a\x00b", true, ""},
		{"  spaces  ", false, "__spaces__"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := sanitizeListName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
