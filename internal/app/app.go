package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"saycoding/internal/collab"
	"saycoding/internal/model"
	"saycoding/internal/prompt"
	"saycoding/internal/session"
	"saycoding/internal/tools"
	"saycoding/internal/types"
	"saycoding/internal/ui"
)

type Runner struct {
	mu                  sync.Mutex
	cfg                 types.Config
	store               *session.Store
	client              *model.Client
	tools               *tools.Registry
	session             *types.Session
	events              []string
	draft               string
	running             bool
	usage               *types.Usage
	lastErr             string
	metrics             metricsState
	mode                string
	alwaysStart         bool
	projectInstructions string
	team                *collab.Runtime
	agentName           string
	hooks               Hooks
	revision            uint64
}

type Hooks struct {
	OnAssistantStart func()
	OnAssistantChunk func(string)
	OnAssistantDone  func()
	OnToolBatch      func([]string)
	OnToolEvent      func(string, string)
	OnRetry          func(int, int, string)
}

type metricsState struct {
	lastStart        time.Time
	lastDuration     time.Duration
	lastOutputTokens int
}

type Snapshot struct {
	Config    types.Config
	Session   types.Session
	Events    []string
	Draft     string
	Running   bool
	Usage     *types.Usage
	LastErr   string
	UserTurns int
	TokPerSec float64
	Mode      string
	AlwaysStart bool
	Agents    []collab.AgentInfo
	Revision  uint64
}

const maxAgentToolCalls = 512

func New(cfg types.Config, store *session.Store, sess *types.Session) *Runner {
	client := model.NewClient(cfg.BaseURL, cfg.APIKey, cfg.ShellTimeoutSec)
	runner := &Runner{
		cfg: cfg, store: store, client: client, session: sess,
		mode: "chat", projectInstructions: prompt.LoadProjectInstructions(sess.CWD), agentName: "lead",
	}
	client.SetRetryHook(runner.handleRetry)
	runner.tools = tools.NewRegistry(sess.CWD, cfg, nil, "", runner.spawnAgents)
	return runner
}

func (r *Runner) Config() types.Config {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cfg
}

func (r *Runner) Session() *types.Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *r.session
	copy.Messages = append([]types.Message(nil), r.session.Messages...)
	return &copy
}

func (r *Runner) ToolNames() []string { return r.tools.Names() }

func (r *Runner) SaveSession() error {
	r.mu.Lock()
	sess := *r.session
	sess.Messages = append([]types.Message(nil), r.session.Messages...)
	r.mu.Unlock()
	return r.store.Save(&sess)
}

func (r *Runner) ListSessions() ([]types.SessionIndexEntry, error) {
	return r.store.List()
}

func (r *Runner) LoadSession(id string) error {
	if err := r.SaveSession(); err != nil {
		return err
	}
	sess, err := r.store.Load(id)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.session = sess
	r.events = nil
	r.draft = ""
	r.running = false
	r.usage = nil
	r.lastErr = ""
	r.metrics = metricsState{}
	team := r.team
	agentName := r.agentName
	cfg := r.cfg
	r.projectInstructions = prompt.LoadProjectInstructions(sess.CWD)
	r.mu.Unlock()
	r.tools = tools.NewRegistry(sess.CWD, cfg, team, agentName, r.spawnAgents)
	return nil
}

func (r *Runner) LoadLatestSession() (*types.SessionIndexEntry, error) {
	latest, err := r.store.Latest()
	if err != nil || latest == nil {
		return latest, err
	}
	return latest, r.LoadSession(latest.ID)
}

func (r *Runner) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	sess := *r.session
	sess.Messages = append([]types.Message(nil), r.session.Messages...)
	events := append([]string(nil), r.events...)
	if r.team != nil {
		for _, item := range r.team.Events() {
			events = append(events, "team: "+item)
		}
		if len(events) > 16 {
			events = events[len(events)-16:]
		}
	}
	return Snapshot{
		Config:    r.cfg,
		Session:   sess,
		Events:    events,
		Draft:     r.draft,
		Running:   r.running,
		Usage:     cloneUsage(r.usage),
		LastErr:   r.lastErr,
		UserTurns: countUserTurns(sess.Messages),
		TokPerSec: tokensPerSecond(r.metrics),
		Mode:      r.mode,
		AlwaysStart: r.alwaysStart,
		Agents:    r.teamSnapshotLocked(),
		Revision:  r.revision,
	}
}

func (r *Runner) UpdateConfig(cfg types.Config) {
	r.mu.Lock()
	r.cfg = cfg
	cwd := r.session.CWD
	team := r.team
	agentName := r.agentName
	r.mu.Unlock()
	r.client = model.NewClient(cfg.BaseURL, cfg.APIKey, cfg.ShellTimeoutSec)
	r.client.SetRetryHook(r.handleRetry)
	r.tools = tools.NewRegistry(cwd, cfg, team, agentName, r.spawnAgents)
}

func (r *Runner) SetHooks(h Hooks) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = h
}

func (r *Runner) Mode() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mode
}

func (r *Runner) SetMode(mode string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if mode != "plan" {
		mode = "chat"
	}
	r.mode = mode
	r.revision++
}

func (r *Runner) AlwaysStart() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.alwaysStart
}

func (r *Runner) SetAlwaysStart(value bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.alwaysStart = value
	r.revision++
}

func (r *Runner) ToggleAlwaysStart() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.alwaysStart = !r.alwaysStart
	r.revision++
	return r.alwaysStart
}

func (r *Runner) SetTeam(team *collab.Runtime, agentName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.team = team
	r.agentName = agentName
	r.tools = tools.NewRegistry(r.session.CWD, r.cfg, team, agentName, r.spawnAgents)
	r.revision++
}

func (r *Runner) ClearSession() error {
	r.mu.Lock()
	r.session.Messages = nil
	r.session.UpdatedAt = time.Now().UTC()
	r.events = nil
	r.draft = ""
	r.usage = nil
	r.lastErr = ""
	sess := *r.session
	r.mu.Unlock()
	return r.store.Save(&sess)
}

func (r *Runner) RunPrompt(ctx context.Context, prompt string) error {
	return r.runPrompt(ctx, prompt, true)
}

func (r *Runner) runPrompt(ctx context.Context, prompt string, allowAlwaysStart bool) error {
	r.updateTeamStatus("thinking", "starting")
	r.setRunning(true)
	defer r.setRunning(false)
	r.setDraft("")
	r.setLastErr("")
	r.startMetrics()
	r.addEvent("user: " + clipEvent(prompt))
	r.append(types.Message{Role: "user", Content: prompt, Timestamp: time.Now().UTC()})
	_ = r.SaveSession()
	for step := 0; step < r.cfg.MaxSteps; step++ {
		if err := r.step(ctx); err != nil {
			r.updateTeamStatus("error", err.Error())
			return err
		}
		if r.hasFinalAssistantReply() {
			last := r.session.Messages[len(r.session.Messages)-1]
			r.updateTeamStatus("done", clipEvent(last.Content))
			if err := r.store.Save(r.session); err != nil {
				return err
			}
			if allowAlwaysStart && r.AlwaysStart() {
				r.addEvent("alwaysstart: continue")
				return r.runPrompt(ctx, "继续写或者继续优化", false)
			}
			return nil
		}
		r.addEvent("assistant produced no final reply, continuing")
	}
	r.updateTeamStatus("error", fmt.Sprintf("stopped after %d steps", r.cfg.MaxSteps))
	return fmt.Errorf("stopped after %d steps", r.cfg.MaxSteps)
}

func (r *Runner) step(ctx context.Context) error {
	events, errs := r.client.Stream(ctx, r.cfg, r.messagesWithSystem(), r.tools.Schemas())
	assistant := types.Message{Role: "assistant", Timestamp: time.Now().UTC()}
	r.addEvent("assistant: streaming")
	r.emitAssistantStart()
	for ev := range events {
		if ev.Content != "" {
			assistant.Content += ev.Content
			r.appendDraft(ev.Content)
			r.emitAssistantChunk(ev.Content)
		}
		if ev.Usage != nil {
			r.setUsage(ev.Usage)
		}
		if len(ev.ToolCalls) > 0 {
			assistant.ToolCalls = ev.ToolCalls
			names := toolNames(ev.ToolCalls)
			r.addEvent("tool batch: " + strings.Join(names, ", "))
			r.emitAssistantDone()
			r.emitToolBatch(describeToolCalls(ev.ToolCalls))
			for _, call := range ev.ToolCalls {
				r.addEvent("tool requested: " + describeToolCall(call))
			}
		}
	}
	if err := <-errs; err != nil {
		r.emitAssistantDone()
		r.addEvent("stream error: " + err.Error())
		r.setLastErr(err.Error())
		return err
	}
	r.emitAssistantDone()
	r.setDraft("")
	r.append(assistant)
	_ = r.SaveSession()
	if len(assistant.ToolCalls) == 0 {
		r.addEvent("assistant: completed")
		r.finishMetrics()
		return nil
	}
	totalCalls := len(assistant.ToolCalls)
	for idx, call := range assistant.ToolCalls {
		if r.currentTeamToolCalls() >= maxAgentToolCalls {
			return fmt.Errorf("stopped after %d tool calls", maxAgentToolCalls)
		}
		desc := describeToolCall(call)
		r.bumpTeamToolCalls()
		r.addEvent(fmt.Sprintf("tool running %d/%d: %s", idx+1, totalCalls, desc))
		result, err := r.tools.Execute(ctx, call)
		if err != nil {
			result = "tool error: " + err.Error()
		}
		if err != nil {
			r.addEvent("tool error: " + desc + " " + clipEvent(err.Error()))
			r.emitToolEvent("tool error:", desc)
		} else {
			r.addEvent("tool done: " + desc + " " + summarizeResult(result))
			r.emitToolEvent("tool done:", desc)
		}
		out, _ := json.Marshal(map[string]any{"ok": err == nil, "result": result})
		r.append(types.Message{
			Role:       "tool",
			Name:       call.Name,
			ToolCallID: call.ID,
			Content:    string(out),
			Timestamp:  time.Now().UTC(),
		})
		_ = r.SaveSession()
	}
	r.finishMetrics()
	return r.store.Save(r.session)
}

func (r *Runner) currentTeamToolCalls() int {
	r.mu.Lock()
	team := r.team
	agentName := r.agentName
	r.mu.Unlock()
	if team == nil || agentName == "" {
		return 0
	}
	for _, agent := range team.Snapshot() {
		if agent.Name == agentName {
			return agent.ToolCalls
		}
	}
	return 0
}

func (r *Runner) messagesWithSystem() []types.Message {
	r.mu.Lock()
	cfg := r.cfg
	sess := *r.session
	sess.Messages = append([]types.Message(nil), r.session.Messages...)
	r.mu.Unlock()
	system := types.Message{
		Role:      "system",
		Content:   r.systemPrompt(sess.CWD, cfg.AutoApproveWorkspace, r.Mode(), r.projectInstructions),
		Timestamp: time.Now().UTC(),
	}
	out := []types.Message{system}
	out = append(out, sess.Messages...)
	return out
}

func (r *Runner) hasFinalAssistantReply() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.session.Messages) == 0 {
		return false
	}
	last := r.session.Messages[len(r.session.Messages)-1]
	if last.Role != "assistant" || len(last.ToolCalls) > 0 {
		return false
	}
	return strings.TrimSpace(last.Content) != ""
}

func (r *Runner) systemPrompt(cwd string, autoApprove bool, mode, projectInstructions string) string {
	lines := []string{
		"You are SayCoding, a terminal coding assistant.",
		"Prefer reading files before editing them.",
		"Use apply_patch for code edits when changing existing files.",
		"Keep changes small and explain results briefly.",
		"When a task splits cleanly into 2 or more independent subtasks, prefer spawn_agents to run them in parallel and then continue with the merged result.",
		"Agent nesting is limited: the lead agent may spawn child agents, and child agents may spawn one more level, but the deepest agents cannot spawn more agents.",
		"Do not use spawn_agents for tiny tasks or tightly coupled work that needs immediate sequential context.",
		"Workspace root: " + cwd,
	}
	if mode == "plan" {
		lines = append(lines,
			"Current mode: plan.",
			"In plan mode, prioritize analysis, implementation steps, risks, and acceptance criteria.",
			"Do not make code changes unless the user explicitly asks to switch back to chat mode or to implement now.",
		)
	} else {
		lines = append(lines, "Current mode: chat.")
	}
	team := r.team
	agentName := r.agentName
	if team != nil && agentName != "" {
		teammates := team.AgentNames(agentName)
		canWrite, canShell := team.Permissions(agentName)
		lines = append(lines,
			"You are part of a parallel agent team.",
			"Your agent name is: "+agentName,
			"Use list_agents to inspect teammates, read_inbox to check messages, and send_message to coordinate.",
			"Send brief coordination messages when work is split across agents or when you need to hand off findings.",
		)
		if agentName == "lead" {
			lines = append(lines,
				"As lead, stay responsive to child-agent inbox requests while they run.",
				"Use grant_permissions to allow a child agent to edit files or run shell/tests when needed.",
				"Use reset_agent if a child agent is stuck, confused, or needs its local state reset.",
			)
		} else {
			lines = append(lines,
				fmt.Sprintf("Current permissions: write=%t shell=%t.", canWrite, canShell),
				"You start read-only unless lead grants more access.",
				"If you need to edit files or run tests, ask lead with send_message and explain why.",
			)
		}
		if len(teammates) > 0 {
			lines = append(lines, "Available teammates: "+strings.Join(teammates, ", "))
		}
	}
	if autoApprove {
		lines = append(lines, "Workspace tools execute automatically inside the workspace root.")
	}
	if strings.TrimSpace(projectInstructions) != "" {
		lines = append(lines, "Project instructions:\n"+projectInstructions)
	}
	return strings.Join(lines, "\n")
}

func (r *Runner) teamSnapshot() []collab.AgentInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.teamSnapshotLocked()
}

func (r *Runner) teamSnapshotLocked() []collab.AgentInfo {
	team := r.team
	if team == nil {
		return nil
	}
	return team.Snapshot()
}

func (r *Runner) append(msg types.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.session.Messages = append(r.session.Messages, msg)
	r.session.UpdatedAt = time.Now().UTC()
	r.revision++
}

func (r *Runner) addEvent(msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, msg)
	if len(r.events) > 12 {
		r.events = r.events[len(r.events)-12:]
	}
	r.revision++
}

func (r *Runner) setDraft(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.draft = text
	r.revision++
}

func (r *Runner) appendDraft(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.draft += text
	r.revision++
}

func (r *Runner) setRunning(value bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = value
	r.revision++
}

func (r *Runner) setLastErr(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastErr = text
	r.revision++
}

func (r *Runner) setUsage(usage *types.Usage) {
	r.mu.Lock()
	r.usage = cloneUsage(usage)
	r.revision++
	r.mu.Unlock()
	r.publishTeamUsage(cloneUsage(usage))
}

func (r *Runner) startMetrics() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics.lastStart = time.Now()
	r.metrics.lastDuration = 0
	r.metrics.lastOutputTokens = 0
	r.revision++
}

func (r *Runner) finishMetrics() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.metrics.lastStart.IsZero() {
		r.metrics.lastDuration = time.Since(r.metrics.lastStart)
	}
	if r.usage != nil {
		r.metrics.lastOutputTokens = r.usage.OutputTokens
	}
	r.revision++
}

func NewSession(store *session.Store) (*types.Session, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return store.New(cwd), nil
}

func (r *Runner) emitAssistantStart() {
	r.updateTeamStatus("thinking", "正在规划回复")
	r.mu.Lock()
	h := r.hooks.OnAssistantStart
	r.mu.Unlock()
	if h != nil {
		h()
		return
	}
	ui.StreamStart()
}

func (r *Runner) emitAssistantChunk(text string) {
	r.updateTeamStatus("output", "正在输出："+clipEvent(text))
	r.mu.Lock()
	h := r.hooks.OnAssistantChunk
	r.mu.Unlock()
	if h != nil {
		h(text)
		return
	}
	ui.AssistantChunk(text)
}

func (r *Runner) emitAssistantDone() {
	r.mu.Lock()
	h := r.hooks.OnAssistantDone
	r.mu.Unlock()
	if h != nil {
		h()
		return
	}
	ui.AssistantDone()
}

func (r *Runner) emitToolBatch(names []string) {
	r.updateTeamStatus("tool", "准备调用工具："+strings.Join(names, ", "))
	r.mu.Lock()
	h := r.hooks.OnToolBatch
	r.mu.Unlock()
	if h != nil {
		h(names)
		return
	}
	ui.ToolBatch(names)
}

func (r *Runner) emitToolEvent(label, detail string) {
	r.updateTeamStatus("tool", toolEventSummary(label, detail))
	r.mu.Lock()
	h := r.hooks.OnToolEvent
	r.mu.Unlock()
	if h != nil {
		h(label, detail)
		return
	}
	ui.ToolEvent(label, detail)
}

func (r *Runner) handleRetry(attempt, maxAttempts int, err error) {
	detail := fmt.Sprintf("retry %d/%d: %s", attempt, maxAttempts, clipEvent(err.Error()))
	r.addEvent(detail)
	r.mu.Lock()
	h := r.hooks.OnRetry
	r.mu.Unlock()
	if h != nil {
		h(attempt, maxAttempts, detail)
		return
	}
	ui.ToolEvent("retry", detail)
}

func (r *Runner) updateTeamStatus(status, event string) {
	r.mu.Lock()
	team := r.team
	agentName := r.agentName
	r.mu.Unlock()
	if team == nil || agentName == "" {
		return
	}
	team.Update(agentName, status, clipEvent(event))
}

func (r *Runner) bumpTeamToolCalls() {
	r.mu.Lock()
	team := r.team
	agentName := r.agentName
	r.mu.Unlock()
	if team == nil || agentName == "" {
		return
	}
	team.IncrementToolCalls(agentName)
}

func (r *Runner) publishTeamUsage(usage *types.Usage) {
	if usage == nil {
		return
	}
	r.mu.Lock()
	team := r.team
	agentName := r.agentName
	r.mu.Unlock()
	if team == nil || agentName == "" {
		return
	}
	team.UpdateUsage(agentName, usage.TotalTokens)
}

func toolEventSummary(label, detail string) string {
	label = strings.TrimSpace(strings.TrimSuffix(label, ":"))
	detail = strings.TrimSpace(detail)
	switch label {
	case "tool error":
		return "工具出错：" + detail
	case "tool done":
		return "工具完成：" + detail
	default:
		return clipEvent(strings.TrimSpace(label + " " + detail))
	}
}
