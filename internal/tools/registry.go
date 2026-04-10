package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"saycoding/internal/collab"
	"saycoding/internal/model"
	"saycoding/internal/safety"
	"saycoding/internal/types"
)

type Tool interface {
	Name() string
	Schema() model.ToolSchema
	Run(context.Context, json.RawMessage) (string, error)
}

type Registry struct {
	workspace *safety.Workspace
	tools     map[string]Tool
}

func NewRegistry(root string, cfg types.Config, team *collab.Runtime, agentName string, spawn func(context.Context, []string) (string, error)) *Registry {
	workspace, _ := safety.NewWorkspace(root)
	canWrite := func() bool { return team == nil || agentName == "" || allowWrite(team, agentName) }
	canShell := func() bool { return team == nil || agentName == "" || allowShell(team, agentName) }
	reg := &Registry{workspace: workspace, tools: map[string]Tool{}}
	reg.add(newReadFileTool(workspace))
	reg.add(newListDirTool(workspace))
	reg.add(newSearchFilesTool(workspace))
	reg.add(newGrepFilesTool(workspace))
	reg.add(newEditFileTool(workspace, canWrite))
	reg.add(newWriteFileTool(workspace, canWrite))
	reg.add(newApplyPatchTool(workspace, canWrite))
	reg.add(newShowDiffTool(workspace))
	reg.add(newShellTool(workspace, cfg.ShellTimeoutSec, canShell))
	if team != nil && agentName != "" {
		reg.add(newSendMessageTool(team, agentName))
		reg.add(newReadInboxTool(team, agentName))
		reg.add(newListAgentsTool(team, agentName))
		if agentName == "lead" {
			reg.add(newGrantPermissionsTool(team, agentName))
			reg.add(newResetAgentTool(team, agentName))
		}
	}
	if spawn != nil && canSpawnNestedAgents(agentName) {
		reg.add(newSpawnAgentsTool(spawn))
	}
	return reg
}

func allowWrite(team *collab.Runtime, agentName string) bool {
	canWrite, _ := team.Permissions(agentName)
	return canWrite
}

func allowShell(team *collab.Runtime, agentName string) bool {
	_, canShell := team.Permissions(agentName)
	return canShell
}

func canSpawnNestedAgents(agentName string) bool {
	if agentName == "" || agentName == "lead" {
		return true
	}
	return strings.Count(agentName, ".") == 0
}

func (r *Registry) add(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) Schemas() []model.ToolSchema {
	out := make([]model.ToolSchema, 0, len(r.tools))
	for _, name := range r.Names() {
		out = append(out, r.tools[name].Schema())
	}
	return out
}

func (r *Registry) Execute(ctx context.Context, call types.ToolCall) (string, error) {
	tool, ok := r.tools[call.Name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", call.Name)
	}
	return tool.Run(ctx, json.RawMessage(call.Arguments))
}
