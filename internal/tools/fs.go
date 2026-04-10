package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"saycoding/internal/diff"
	"saycoding/internal/model"
	"saycoding/internal/safety"
)

type readFileArgs struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type listDirArgs struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

type searchArgs struct {
	Query string `json:"query"`
	Glob  string `json:"glob"`
}

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type editFileArgs struct {
	Path       string `json:"path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

type applyPatchArgs struct {
	Path  string `json:"path"`
	Patch string `json:"patch"`
}

type showDiffArgs struct {
	Path string `json:"path"`
}

type fsTool struct {
	name        string
	description string
	params      map[string]any
	run         func(json.RawMessage) (string, error)
}

func requireWrite(check func() bool) error {
	if check == nil || check() {
		return nil
	}
	return fmt.Errorf("write permission denied; ask lead via send_message for write approval")
}

func (t fsTool) Name() string { return t.name }
func (t fsTool) Schema() model.ToolSchema {
	return model.ToolSchema{
		Type: "function",
		Function: model.ToolDefinition{
			Name:        t.name,
			Description: t.description,
			Parameters:  t.params,
		},
	}
}
func (t fsTool) Run(_ context.Context, input json.RawMessage) (string, error) { return t.run(input) }

func newReadFileTool(w *safety.Workspace) Tool {
	return fsTool{
		name:        "read_file",
		description: "Read a file from the workspace. Optionally return a line range.",
		params:      objectSchema("path", prop("path", "string"), prop("start_line", "integer"), prop("end_line", "integer")),
		run: func(input json.RawMessage) (string, error) {
			var args readFileArgs
			if err := json.Unmarshal(input, &args); err != nil {
				return "", err
			}
			path, err := w.Resolve(args.Path)
			if err != nil {
				return "", err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			return sliceLines(string(data), args.StartLine, args.EndLine), nil
		},
	}
}

func newListDirTool(w *safety.Workspace) Tool {
	return fsTool{
		name:        "list_dir",
		description: "List files under a workspace directory.",
		params:      objectSchema("", prop("path", "string"), prop("recursive", "boolean")),
		run: func(input json.RawMessage) (string, error) {
			var args listDirArgs
			if err := json.Unmarshal(input, &args); err != nil {
				return "", err
			}
			root := "."
			if args.Path != "" {
				root = args.Path
			}
			path, err := w.Resolve(root)
			if err != nil {
				return "", err
			}
			var out []string
			walk := func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if p == path {
					return nil
				}
				rel, _ := filepath.Rel(w.Root(), p)
				out = append(out, rel)
				if !args.Recursive && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if err := filepath.WalkDir(path, walk); err != nil {
				return "", err
			}
			return strings.Join(out, "\n"), nil
		},
	}
}

func newSearchFilesTool(w *safety.Workspace) Tool {
	return newSearchTool(
		w,
		"search_files",
		"Search text across workspace files.",
	)
}

func newGrepFilesTool(w *safety.Workspace) Tool {
	return newSearchTool(
		w,
		"grep_files",
		"Grep text across workspace files with optional glob filtering.",
	)
}

func newSearchTool(w *safety.Workspace, name, description string) Tool {
	return fsTool{
		name:        name,
		description: description,
		params:      objectSchema("query", prop("query", "string"), prop("glob", "string")),
		run: func(input json.RawMessage) (string, error) {
			var args searchArgs
			if err := json.Unmarshal(input, &args); err != nil {
				return "", err
			}
			var out []string
			err := filepath.WalkDir(w.Root(), func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				if args.Glob != "" {
					matched, _ := filepath.Match(args.Glob, filepath.Base(path))
					if !matched {
						return nil
					}
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				lines := strings.Split(string(data), "\n")
				for idx, line := range lines {
					if strings.Contains(line, args.Query) {
						rel, _ := filepath.Rel(w.Root(), path)
						out = append(out, rel+":"+itoa(idx+1)+": "+strings.TrimSpace(line))
					}
				}
				return nil
			})
			if err != nil {
				return "", err
			}
			return strings.Join(out, "\n"), nil
		},
	}
}

func newEditFileTool(w *safety.Workspace, canWrite func() bool) Tool {
	return fsTool{
		name:        "edit_file",
		description: "Edit a file by exact old_string/new_string replacement. Fails if old_string is missing or ambiguous unless replace_all is true.",
		params: objectSchema(
			"path",
			prop("path", "string"),
			prop("old_string", "string"),
			prop("new_string", "string"),
			prop("replace_all", "boolean"),
		),
		run: func(input json.RawMessage) (string, error) {
			if err := requireWrite(canWrite); err != nil {
				return "", err
			}
			var args editFileArgs
			if err := json.Unmarshal(input, &args); err != nil {
				return "", err
			}
			if args.OldString == args.NewString {
				return "", fmt.Errorf("old_string and new_string are identical")
			}
			path, err := w.Resolve(args.Path)
			if err != nil {
				return "", err
			}
			beforeBytes, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			before := strings.ReplaceAll(string(beforeBytes), "\r\n", "\n")
			oldString := strings.ReplaceAll(args.OldString, "\r\n", "\n")
			newString := strings.ReplaceAll(args.NewString, "\r\n", "\n")
			if oldString == "" {
				if strings.TrimSpace(before) != "" {
					return "", fmt.Errorf("old_string is empty but file already has content")
				}
				if err := os.WriteFile(path, []byte(newString), 0o644); err != nil {
					return "", err
				}
				return diff.Preview(args.Path, before, newString), nil
			}
			matches := strings.Count(before, oldString)
			switch {
			case matches == 0:
				return "", fmt.Errorf("old_string not found; read the file again and use exact text")
			case matches > 1 && !args.ReplaceAll:
				return "", fmt.Errorf("old_string matched %d locations; add unique context or set replace_all=true", matches)
			}
			after := before
			if args.ReplaceAll {
				after = strings.ReplaceAll(before, oldString, newString)
			} else {
				after = strings.Replace(before, oldString, newString, 1)
			}
			if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
				return "", err
			}
			return diff.Preview(args.Path, before, after), nil
		},
	}
}

func newWriteFileTool(w *safety.Workspace, canWrite func() bool) Tool {
	return fsTool{
		name:        "write_file",
		description: "Write a whole file inside the workspace.",
		params:      objectSchema("path", prop("path", "string"), prop("content", "string")),
		run: func(input json.RawMessage) (string, error) {
			if err := requireWrite(canWrite); err != nil {
				return "", err
			}
			var args writeFileArgs
			if err := json.Unmarshal(input, &args); err != nil {
				return "", err
			}
			path, err := w.Resolve(args.Path)
			if err != nil {
				return "", err
			}
			before := ""
			if data, err := os.ReadFile(path); err == nil {
				before = string(data)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(path, []byte(args.Content), 0o644); err != nil {
				return "", err
			}
			return diff.Preview(args.Path, before, args.Content), nil
		},
	}
}

func newApplyPatchTool(w *safety.Workspace, canWrite func() bool) Tool {
	return fsTool{
		name:        "apply_patch",
		description: "Apply SEARCH/REPLACE patch blocks to an existing file.",
		params:      objectSchema("path", prop("path", "string"), prop("patch", "string")),
		run: func(input json.RawMessage) (string, error) {
			if err := requireWrite(canWrite); err != nil {
				return "", err
			}
			var args applyPatchArgs
			if err := json.Unmarshal(input, &args); err != nil {
				return "", err
			}
			path, err := w.Resolve(args.Path)
			if err != nil {
				return "", err
			}
			before, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			after, err := diff.Apply(string(before), args.Patch)
			if err != nil {
				return "", err
			}
			if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
				return "", err
			}
			return diff.Preview(args.Path, string(before), after), nil
		},
	}
}

func newShowDiffTool(w *safety.Workspace) Tool {
	return fsTool{
		name:        "show_diff",
		description: "Show a simple diff summary for a file.",
		params:      objectSchema("path", prop("path", "string")),
		run: func(input json.RawMessage) (string, error) {
			var args showDiffArgs
			if err := json.Unmarshal(input, &args); err != nil {
				return "", err
			}
			path, err := w.Resolve(args.Path)
			if err != nil {
				return "", err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			return diff.Preview(args.Path, "", string(data)), nil
		},
	}
}

func objectSchema(required string, props ...map[string]any) map[string]any {
	properties := map[string]any{}
	for _, item := range props {
		for key, value := range item {
			properties[key] = value
		}
	}
	schema := map[string]any{"type": "object", "properties": properties}
	if required != "" {
		schema["required"] = []string{required}
	}
	return schema
}

func prop(name, typ string) map[string]any {
	return map[string]any{name: map[string]any{"type": typ}}
}

func sliceLines(text string, start, end int) string {
	if start <= 0 && end <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start > end || start > len(lines) {
		return ""
	}
	return strings.Join(lines[start-1:end], "\n")
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
