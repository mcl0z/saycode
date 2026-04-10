package prompt

import (
	"os"
	"path/filepath"
	"strings"
)

func LoadProjectInstructions(cwd string) string {
	candidates := []string{
		filepath.Join(cwd, "AGENTS.md"),
		filepath.Join(cwd, "Agents.md"),
		filepath.Join(cwd, "agents.md"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}
