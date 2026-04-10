package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"saycoding/internal/model"
)

type spawnAgentsArgs struct {
	Tasks []string `json:"tasks"`
}

type spawnAgentsTool struct {
	spawn func(context.Context, []string) (string, error)
}

func newSpawnAgentsTool(spawn func(context.Context, []string) (string, error)) Tool {
	return &spawnAgentsTool{spawn: spawn}
}

func (t *spawnAgentsTool) Name() string { return "spawn_agents" }

func (t *spawnAgentsTool) Schema() model.ToolSchema {
	return model.ToolSchema{
		Type: "function",
		Function: model.ToolDefinition{
			Name:        "spawn_agents",
			Description: "Spawn multiple parallel agents to work on separate subtasks and return a merged summary.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tasks": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
				"required": []string{"tasks"},
			},
		},
	}
}

func (t *spawnAgentsTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args spawnAgentsArgs
	if err := json.Unmarshal(input, &args); err != nil {
		return "", err
	}
	if len(args.Tasks) < 2 {
		return "", fmt.Errorf("spawn_agents requires at least 2 tasks")
	}
	return t.spawn(ctx, args.Tasks)
}
