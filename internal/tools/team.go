package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"saycoding/internal/collab"
	"saycoding/internal/model"
)

type sendMessageArgs struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

type grantPermissionArgs struct {
	To       string `json:"to"`
	CanWrite bool   `json:"can_write"`
	CanShell bool   `json:"can_shell"`
}

type resetAgentArgs struct {
	To string `json:"to"`
}

type teamTool struct {
	name        string
	description string
	params      map[string]any
	run         func(json.RawMessage) (string, error)
}

func (t teamTool) Name() string { return t.name }
func (t teamTool) Schema() model.ToolSchema {
	return model.ToolSchema{
		Type: "function",
		Function: model.ToolDefinition{
			Name:        t.name,
			Description: t.description,
			Parameters:  t.params,
		},
	}
}
func (t teamTool) Run(_ context.Context, input json.RawMessage) (string, error) { return t.run(input) }

func newSendMessageTool(team *collab.Runtime, self string) Tool {
	return teamTool{
		name:        "send_message",
		description: "Send a short message to another agent in the team.",
		params:      objectSchema("to", prop("to", "string"), prop("message", "string")),
		run: func(input json.RawMessage) (string, error) {
			var args sendMessageArgs
			if err := json.Unmarshal(input, &args); err != nil {
				return "", err
			}
			if err := team.Send(self, args.To, args.Message); err != nil {
				return "", err
			}
			return "sent to " + args.To, nil
		},
	}
}

func newReadInboxTool(team *collab.Runtime, self string) Tool {
	return teamTool{
		name:        "read_inbox",
		description: "Read pending messages sent by other agents.",
		params:      objectSchema("", prop("limit", "integer")),
		run: func(_ json.RawMessage) (string, error) {
			msgs := team.ReadInbox(self)
			if len(msgs) == 0 {
				return "no messages", nil
			}
			out := make([]string, 0, len(msgs))
			for _, msg := range msgs {
				out = append(out, fmt.Sprintf("%s: %s", msg.From, msg.Content))
			}
			return strings.Join(out, "\n"), nil
		},
	}
}

func newListAgentsTool(team *collab.Runtime, self string) Tool {
	return teamTool{
		name:        "list_agents",
		description: "List current agents and their statuses.",
		params:      objectSchema(""),
		run: func(_ json.RawMessage) (string, error) {
			agents := team.Snapshot()
			out := make([]string, 0, len(agents))
			for _, agent := range agents {
				marker := ""
				if agent.Name == self {
					marker = " (you)"
				}
				out = append(out, fmt.Sprintf("%s%s [%s] %s", agent.Name, marker, agent.Status, agent.Task))
			}
			return strings.Join(out, "\n"), nil
		},
	}
}

func newGrantPermissionsTool(team *collab.Runtime, self string) Tool {
	return teamTool{
		name:        "grant_permissions",
		description: "Lead-only tool to grant a child agent write and shell/test permissions.",
		params:      objectSchema("to", prop("to", "string"), prop("can_write", "boolean"), prop("can_shell", "boolean")),
		run: func(input json.RawMessage) (string, error) {
			if self != "lead" {
				return "", fmt.Errorf("only lead can grant permissions")
			}
			var args grantPermissionArgs
			if err := json.Unmarshal(input, &args); err != nil {
				return "", err
			}
			if err := team.Grant(args.To, args.CanWrite, args.CanShell); err != nil {
				return "", err
			}
			return fmt.Sprintf("granted %s write=%t shell=%t", args.To, args.CanWrite, args.CanShell), nil
		},
	}
}

func newResetAgentTool(team *collab.Runtime, self string) Tool {
	return teamTool{
		name:        "reset_agent",
		description: "Lead-only tool to reset a specific agent's state and pending inbox.",
		params:      objectSchema("to", prop("to", "string")),
		run: func(input json.RawMessage) (string, error) {
			if self != "lead" {
				return "", fmt.Errorf("only lead can reset agents")
			}
			var args resetAgentArgs
			if err := json.Unmarshal(input, &args); err != nil {
				return "", err
			}
			if err := team.Reset(args.To); err != nil {
				return "", err
			}
			return "reset " + args.To, nil
		},
	}
}
