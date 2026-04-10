package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"saycoding/internal/model"
	"saycoding/internal/safety"
)

type shellArgs struct {
	Command    string `json:"command"`
	Cwd        string `json:"cwd"`
	TimeoutSec int    `json:"timeout_sec"`
}

type shellTool struct {
	workspace  *safety.Workspace
	timeoutSec int
	canShell   func() bool
}

func newShellTool(w *safety.Workspace, timeoutSec int, canShell func() bool) Tool {
	return &shellTool{workspace: w, timeoutSec: timeoutSec, canShell: canShell}
}

func (t *shellTool) Name() string {
	return "run_shell"
}

func (t *shellTool) Schema() model.ToolSchema {
	return model.ToolSchema{
		Type: "function",
		Function: model.ToolDefinition{
			Name:        "run_shell",
			Description: "Run a shell command inside the workspace and return stdout/stderr.",
			Parameters:  objectSchema("command", prop("command", "string"), prop("cwd", "string"), prop("timeout_sec", "integer")),
		},
	}
}

func (t *shellTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	if t.canShell != nil && !t.canShell() {
		return "", fmt.Errorf("shell permission denied; ask lead via send_message for shell/test approval")
	}
	var args shellArgs
	if err := json.Unmarshal(input, &args); err != nil {
		return "", err
	}
	cwd := t.workspace.Root()
	if args.Cwd != "" {
		resolved, err := t.workspace.Resolve(args.Cwd)
		if err != nil {
			return "", err
		}
		cwd = resolved
	}
	timeout := t.timeoutSec
	if args.TimeoutSec > 0 {
		timeout = args.TimeoutSec
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "sh", "-lc", args.Command)
	cmd.Dir = cwd
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())
	combined := "stdout:\n" + truncate(output) + "\n\nstderr:\n" + truncate(errOut)
	if runCtx.Err() == context.DeadlineExceeded {
		return combined, runCtx.Err()
	}
	return combined, err
}

func truncate(text string) string {
	if len(text) <= 4000 {
		return text
	}
	return text[:4000] + "\n...[truncated]"
}
