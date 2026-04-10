package repl

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"saycoding/internal/app"
	"saycoding/internal/provider"
	"saycoding/internal/ui"
)

func Run(ctx context.Context, runner *app.Runner) error {
	status := "ready"
	var help []string
	for {
		snap := runner.Snapshot()
		ui.RenderScreen(ui.ScreenState{
			Title:      "SayCoding",
			Status:     status,
			Session:    &snap.Session,
			Config:     snap.Config,
			Events:     snap.Events,
			Draft:      snap.Draft,
			Running:    snap.Running,
			Usage:      snap.Usage,
			LastErr:    snap.LastErr,
			UserTurns:  snap.UserTurns,
			TokPerSec:  snap.TokPerSec,
			Mode:       snap.Mode,
			AlwaysStart: snap.AlwaysStart,
			Agents:     snap.Agents,
			HelpLines:  help,
			BodyHeight: 18,
		})

		line, err := ui.ReadLine()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			done, nextStatus, nextHelp, err := handleSlash(ctx, runner, line)
			help = nextHelp
			status = nextStatus
			if done || err != nil {
				return err
			}
			continue
		}

		help = nil
		status = "thinking"
		promptCtx, cancel := context.WithCancel(ctx)
		stopWatch := ui.WatchEscape(promptCtx, cancel)
		err = runner.RunPrompt(promptCtx, line)
		cancel()
		stopWatch()
		if err != nil {
			if promptCtx.Err() == context.Canceled {
				status = "stopped by Esc"
				continue
			}
			return err
		}
		status = "ready"
	}
}

func handleSlash(ctx context.Context, runner *app.Runner, line string) (bool, string, []string, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false, "ready", nil, nil
	}
	switch fields[0] {
	case "/exit", "/quit":
		return true, "bye", nil, nil
	case "/help":
		return false, "help", helpLines(), nil
	case "/plan":
		runner.SetMode("plan")
		return false, "mode: plan", nil, nil
	case "/chat":
		runner.SetMode("chat")
		return false, "mode: chat", nil, nil
	case "/alwaysstart":
		if runner.ToggleAlwaysStart() {
			return false, "alwaysstart on", nil, nil
		}
		return false, "alwaysstart off", nil, nil
	case "/clear":
		if err := runner.ClearSession(); err != nil {
			return false, "", nil, err
		}
		return false, "conversation cleared", nil, nil
	case "/status":
		snap := runner.Snapshot()
		status := fmt.Sprintf("model=%s user_turns=%d", snap.Config.Model, snap.UserTurns)
		if snap.Usage != nil && snap.Usage.TotalTokens > 0 {
			status += fmt.Sprintf(" total_tokens=%d", snap.Usage.TotalTokens)
		}
		return false, status, nil, nil
	case "/session":
		sess := runner.Session()
		return false, fmt.Sprintf("session=%s updated=%s", sess.ID, sess.UpdatedAt.Format(time.RFC3339)), nil, nil
	case "/sessions":
		idx, err := runner.ListSessions()
		if err != nil {
			return false, "", nil, err
		}
		if len(idx) == 0 {
			return false, "no saved sessions", nil, nil
		}
		lines := make([]string, 0, min(8, len(idx)))
		for i, item := range idx {
			if i >= 8 {
				break
			}
			lines = append(lines, fmt.Sprintf("%s  %s", item.ID, item.CWD))
		}
		return false, strings.Join(lines, " | "), nil, nil
	case "/resume":
		if len(fields) < 2 {
			return false, "usage: /resume <session-id|latest|recent>", nil, nil
		}
		target := fields[1]
		if target == "latest" || target == "recent" {
			latest, err := runner.LoadLatestSession()
			if err != nil {
				return false, "", nil, err
			}
			if latest == nil {
				return false, "no saved sessions", nil, nil
			}
			return false, "resumed "+latest.ID, nil, nil
		}
		if err := runner.LoadSession(target); err != nil {
			return false, "", nil, err
		}
		return false, "resumed "+target, nil, nil
	case "/provider":
		cfg, err := provider.RunSetup(ctx, runner.Config())
		if err != nil {
			return false, "", nil, err
		}
		runner.UpdateConfig(cfg)
		return false, "provider updated: " + cfg.Model, nil, nil
	case "/tools":
		return false, "tools: " + strings.Join(runner.ToolNames(), ", "), nil, nil
	case "/agents":
		raw := strings.TrimSpace(strings.TrimPrefix(line, "/agents"))
		parts := strings.Split(raw, "||")
		tasks := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				tasks = append(tasks, part)
			}
		}
		if len(tasks) < 2 {
			return false, "usage: /agents task a || task b || task c", nil, nil
		}
		if _, err := runner.RunParallel(ctx, tasks); err != nil {
			return false, "", nil, err
		}
		return false, fmt.Sprintf("agents completed: %d", len(tasks)), nil, nil
	default:
		return false, "unknown command", nil, nil
	}
}

func helpLines() []string {
	return []string{
		"/help      show help",
		"/status    show current model and usage summary",
		"/session   show current session metadata",
		"/sessions  show recent sessions",
		"/resume    resume a saved session: /resume recent or /resume <id>",
		"/alwaysstart toggle auto-continue after each completed reply",
		"/provider  configure base_url, key, model",
		"/agents    run parallel subtasks: /agents a || b || c",
		"/plan      switch to planning mode",
		"/chat      switch back to chat mode",
		"/clear     clear current conversation",
		"/exit      quit",
		"Esc        stop current output",
	}
}
