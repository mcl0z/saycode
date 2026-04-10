package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"saycoding/internal/model"
	"saycoding/internal/todo"
)

type todoArgs struct {
	Action string `json:"action"`
	ID     int    `json:"id"`
	Text   string `json:"text"`
}

type todoTool struct{}

func newTodoTool() Tool { return todoTool{} }

func (t todoTool) Name() string { return "todo_list" }

func (t todoTool) Schema() model.ToolSchema {
	return model.ToolSchema{
		Type: "function",
		Function: model.ToolDefinition{
			Name:        "todo_list",
			Description: "Manage the shared todo list. Use action=list|add|done|undo|remove|clear.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{"type": "string"},
					"id":     map[string]any{"type": "integer"},
					"text":   map[string]any{"type": "string"},
				},
				"required": []string{"action"},
			},
		},
	}
}

func (t todoTool) Run(_ context.Context, input json.RawMessage) (string, error) {
	var args todoArgs
	if err := json.Unmarshal(input, &args); err != nil {
		return "", err
	}
	store, err := todo.NewStore()
	if err != nil {
		return "", err
	}
	switch args.Action {
	case "list":
		items, err := store.List()
		if err != nil {
			return "", err
		}
		return formatTodos(items), nil
	case "add":
		if strings.TrimSpace(args.Text) == "" {
			return "", fmt.Errorf("todo add requires text")
		}
		items, err := store.Add(strings.TrimSpace(args.Text))
		if err != nil {
			return "", err
		}
		return formatTodos(items), nil
	case "done":
		items, err := store.SetDone(args.ID, true)
		if err != nil {
			return "", err
		}
		return formatTodos(items), nil
	case "undo":
		items, err := store.SetDone(args.ID, false)
		if err != nil {
			return "", err
		}
		return formatTodos(items), nil
	case "remove":
		items, err := store.Remove(args.ID)
		if err != nil {
			return "", err
		}
		return formatTodos(items), nil
	case "clear":
		if err := store.Clear(); err != nil {
			return "", err
		}
		return "todo list cleared", nil
	default:
		return "", fmt.Errorf("unknown todo action: %s", args.Action)
	}
}

func formatTodos(items []todo.Item) string {
	if len(items) == 0 {
		return "todo list is empty"
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		state := "[ ]"
		if item.Done {
			state = "[x]"
		}
		lines = append(lines, fmt.Sprintf("%d. %s %s", item.ID, state, item.Text))
	}
	return strings.Join(lines, "\n")
}
