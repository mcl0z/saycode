package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"saycoding/internal/collab"
	"saycoding/internal/types"
)

type AgentResult struct {
	Task      string
	SessionID string
	Output    string
	Err       string
	Duration  time.Duration
}

func (r *Runner) RunParallel(ctx context.Context, tasks []string) ([]AgentResult, error) {
	clean := make([]string, 0, len(tasks))
	for _, task := range tasks {
		task = strings.TrimSpace(task)
		if task != "" {
			clean = append(clean, task)
		}
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("no agent tasks")
	}
	cfg := r.Config()
	cfg.MaxSteps = max(cfg.MaxSteps, 48)
	mode := r.Mode()
	base := r.Session()
	r.mu.Lock()
	team := r.team
	parentName := r.agentName
	r.mu.Unlock()
	if team == nil {
		team = collab.NewRuntime()
		r.SetTeam(team, "lead")
		parentName = "lead"
	}
	results := make([]AgentResult, len(clean))
	var wg sync.WaitGroup
	parentDepth := agentDepth(parentName)
	team.Register(parentName, "", parentDepth, "coordinate parallel work", base.ID)
	coordCtx, cancelCoord := context.WithCancel(ctx)
	defer cancelCoord()
	if parentName == "lead" {
		go r.runLeadCoordinator(coordCtx, struct {
			team *collab.Runtime
			cwd  string
			mode string
		}{
			team: team,
			cwd:  base.CWD,
			mode: mode,
		})
	}
	for i, task := range clean {
		wg.Add(1)
		go func(index int, prompt string) {
			defer wg.Done()
			start := time.Now()
			child := r.store.New(base.CWD)
			worker := New(cfg, r.store, child)
			worker.SetHooks(Hooks{
				OnAssistantStart: func() {},
				OnAssistantChunk: func(string) {},
				OnAssistantDone:  func() {},
				OnToolBatch:      func([]string) {},
				OnToolEvent:      func(string, string) {},
			})
			worker.SetMode(mode)
			agentName := childAgentName(parentName, index+1)
			worker.SetTeam(team, agentName)
			team.Register(agentName, parentName, parentDepth+1, prompt, child.ID)
			team.Update(agentName, "running", "starting")
			err := worker.RunPrompt(ctx, prompt)
			result := AgentResult{Task: prompt, SessionID: child.ID, Duration: time.Since(start)}
			if err != nil {
				result.Err = err.Error()
				team.Update(agentName, "error", err.Error())
			} else {
				result.Output = assistantText(worker.Session())
				team.Update(agentName, "done", clipEvent(result.Output))
			}
			results[index] = result
		}(i, task)
	}
	wg.Wait()
	cancelCoord()
	return results, nil
}

func childAgentName(parentName string, index int) string {
	if parentName == "" || parentName == "lead" {
		return fmt.Sprintf("agent-%d", index)
	}
	return fmt.Sprintf("%s.%d", parentName, index)
}

func agentDepth(name string) int {
	if name == "" || name == "lead" {
		return 0
	}
	return strings.Count(name, ".") + 1
}

func assistantText(sess *types.Session) string {
	if sess == nil {
		return ""
	}
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		msg := sess.Messages[i]
		if msg.Role == "assistant" && len(msg.ToolCalls) == 0 {
			return msg.Content
		}
	}
	return ""
}

func (r *Runner) spawnAgents(ctx context.Context, tasks []string) (string, error) {
	results, err := r.RunParallel(ctx, tasks)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(results)*2)
	for i, item := range results {
		if item.Err != "" {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, item.Task))
			lines = append(lines, "error: "+item.Err)
			continue
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, item.Task))
		lines = append(lines, "result: "+clipEvent(item.Output))
	}
	return strings.Join(lines, "\n"), nil
}
