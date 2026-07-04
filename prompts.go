package creative

import (
	"embed"
	"fmt"
)

//go:embed prompts
var promptsFS embed.FS

func GetPrompt(name string) (string, error) {
	data, err := promptsFS.ReadFile("prompts/" + name + ".promt")
	if err != nil {
		return "", fmt.Errorf("prompt %q: %w", name, err)
	}
	return string(data), nil
}
