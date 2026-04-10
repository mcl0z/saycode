package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"saycoding/internal/collab"
)

func (r *Runner) runLeadCoordinator(ctx context.Context, cfg struct {
	team *collab.Runtime
	cwd  string
	mode string
}) {
	storeSess := r.store.New(cfg.cwd)
	lead := New(r.Config(), r.store, storeSess)
	lead.SetMode(cfg.mode)
	lead.SetTeam(cfg.team, "lead")
	lead.SetHooks(Hooks{
		OnAssistantStart: func() {},
		OnAssistantChunk: func(string) {},
		OnAssistantDone:  func() {},
		OnToolBatch:      func([]string) {},
		OnToolEvent:      func(string, string) {},
	})
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msgs := cfg.team.ReadInbox("lead")
			if len(msgs) == 0 {
				continue
			}
			prompt := buildCoordinatorPrompt(cfg.team, msgs)
			_ = lead.RunPrompt(ctx, prompt)
		}
	}
}

func buildCoordinatorPrompt(team *collab.Runtime, msgs []collab.TeamMessage) string {
	lines := []string{
		"Coordinate active child agents.",
		"Read the pending inbox messages below and respond briefly.",
		"If a child agent requests file-edit permission or shell/test permission and it is reasonable, use grant_permissions.",
		"If a child agent asks a question, answer with send_message.",
		"Do not restate the whole task. Keep replies short and operational.",
		"Pending inbox:",
	}
	for _, msg := range msgs {
		lines = append(lines, fmt.Sprintf("- from %s: %s", msg.From, msg.Content))
	}
	agents := team.Snapshot()
	if len(agents) > 0 {
		lines = append(lines, "Current agents:")
		for _, agent := range agents {
			lines = append(lines, fmt.Sprintf("- %s status=%s write=%t shell=%t last=%s", agent.Name, agent.Status, agent.CanWrite, agent.CanShell, clipEvent(agent.LastEvent)))
		}
	}
	return strings.Join(lines, "\n")
}
