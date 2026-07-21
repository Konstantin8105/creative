package creative

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// listFilePrefix is prepended to every list filename to guarantee
// no collision with book/document files in shared folders.
const listFilePrefix = "_list_"

// ListAddTool returns a tool that appends lines to list files in tempDir.
func ListAddTool(tempDir string) []Tool {
	itemsSchema := &ToolProperty{
		Type:        "string",
		Description: "A single line to add to the list",
	}
	return []Tool{
		{
			Name:        "list_add",
			Description: "Add lines to a named list. Lists are stored as \"_list_<name>.txt\" files and can be browsed/searched via list_books, read_book_lines, search_in_book, search_stats using the filename \"_list_<name>.txt\". Example: list_add name=\"tasks\" creates file \"_list_tasks.txt\".",
			Parameters: &ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"name": {
						Type:        "string",
						Description: "List name. Case-insensitive, spaces become underscores. The actual file is stored as \"_list_<name>.txt\". Invalid filename characters cause an error.",
					},
					"items": {
						Type:        "array",
						Description: "One or more lines to append. Pass an empty array to create an empty list.",
						Items:       itemsSchema,
					},
				},
				Required: []string{"name", "items"},
			},
			Execute: func(params string) string {
				return listAddExecute(tempDir, params)
			},
		},
	}
}

type listAddParams struct {
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

func listAddExecute(tempDir string, params string) string {
	var p listAddParams
	if err := json.Unmarshal([]byte(params), &p); err != nil {
		return fmt.Sprintf("Error: invalid JSON parameters: %v", err)
	}

	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return "Error: list name cannot be empty."
	}

	safeName, err := sanitizeListName(p.Name)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	path := filepath.Join(tempDir, listFilePrefix+safeName+".txt")

	// Ensure tempDir exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Sprintf("Error: cannot create temp directory: %v", err)
	}

	// Process items: replace \n and \r with space
	cleaned := make([]string, len(p.Items))
	for i, item := range p.Items {
		item = strings.ReplaceAll(item, "\r\n", " ")
		item = strings.ReplaceAll(item, "\n", " ")
		item = strings.ReplaceAll(item, "\r", " ")
		cleaned[i] = item
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Sprintf("Error: cannot open list file: %v", err)
	}
	defer f.Close()

	for _, line := range cleaned {
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Sprintf("Error: cannot write to list file: %v", err)
		}
	}

	plural := ""
	if len(cleaned) != 1 {
		plural = "s"
	}
	return fmt.Sprintf("Added %d line%s to list %q.", len(cleaned), plural, p.Name)
}

func sanitizeListName(name string) (string, error) {
	lower := strings.ToLower(name)

	if strings.Contains(lower, "../") {
		return "", fmt.Errorf("list name %q must not contain \"../\"", name)
	}

	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, ch := range invalid {
		if strings.Contains(lower, ch) {
			return "", fmt.Errorf("list name %q must not contain character %q", name, ch)
		}
	}

	for _, r := range lower {
		if r <= 0x1F {
			return "", fmt.Errorf("list name %q must not contain control characters", name)
		}
	}

	replacer := strings.NewReplacer(" ", "_")
	safe := replacer.Replace(lower)

	return safe, nil
}
