package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"saycoding/internal/app"
	"saycoding/internal/collab"
	"saycoding/internal/config"
	"saycoding/internal/provider"
	"saycoding/internal/ui"
)

type assistantStartMsg struct{}
type assistantChunkMsg string
type assistantDoneMsg struct{}
type toolBatchMsg []string
type toolEventMsg struct {
	label  string
	detail string
}
type refreshTickMsg struct{}
type runDoneMsg struct{ err error }
type agentsDoneMsg struct{ err error }

type model struct {
	runner     *app.Runner
	status     string
	input      []rune
	stream     string
	running    bool
	cancel     context.CancelFunc
	scroll     int
	width      int
	height     int
	help       []string
	palette    bool
	pquery     []rune
	psel       int
	sessions   bool
	sitems     []string
	ssel       int
	showAgents bool
}

func Run(ctx context.Context, runner *app.Runner) error {
	m := &model{runner: runner, status: "ready"}
	p := tea.NewProgram(m, tea.WithMouseCellMotion())
	runner.SetHooks(app.Hooks{
		OnAssistantStart: func() { p.Send(assistantStartMsg{}) },
		OnAssistantChunk: func(s string) { p.Send(assistantChunkMsg(s)) },
		OnAssistantDone:  func() { p.Send(assistantDoneMsg{}) },
		OnToolBatch:      func(names []string) { p.Send(toolBatchMsg(names)) },
		OnToolEvent: func(label, detail string) {
			p.Send(toolEventMsg{label: label, detail: detail})
		},
	})
	defer runner.SetHooks(app.Hooks{})
	_, err := p.Run()
	return err
}

func (m *model) Init() tea.Cmd { return refreshTickCmd() }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.onKey(msg)
	case tea.MouseMsg:
		return m.onMouse(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.normalizePalette()
		m.normalizeSessions()
	case refreshTickMsg:
		if m.running || m.showAgents {
			return m, refreshTickCmd()
		}
		return m, refreshTickCmd()
	case assistantStartMsg:
		m.running = true
		m.status = "thinking"
		m.stream = ""
	case assistantChunkMsg:
		m.stream += string(msg)
	case assistantDoneMsg:
	case toolBatchMsg:
		m.status = "tools: " + strings.Join([]string(msg), " -> ")
	case toolEventMsg:
		m.status = strings.TrimSpace(msg.label + " " + msg.detail)
	case runDoneMsg:
		m.running = false
		m.cancel = nil
		m.stream = ""
		m.showAgents = false
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = "ready"
		}
	case agentsDoneMsg:
		m.running = false
		m.cancel = nil
		m.stream = ""
		m.showAgents = false
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = "agents completed"
		}
	}
	return m, nil
}

func refreshTickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m *model) onMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.scroll++
	case tea.MouseButtonWheelDown:
		if m.scroll > 0 {
			m.scroll--
		}
	}
	return m, nil
}

func (m *model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		_ = m.runner.SaveSession()
		return m, tea.Quit
	case tea.KeyCtrlP:
		m.sessions = false
		m.help = nil
		m.palette = true
		m.pquery = nil
		m.psel = 0
		m.normalizePalette()
		return m, nil
	case tea.KeyEsc:
		if m.cancel != nil {
			m.cancel()
			m.status = "stopped by Esc"
		} else if m.sessions {
			m.sessions = false
			m.sitems = nil
			m.ssel = 0
		} else if len(m.help) > 0 {
			m.help = nil
		} else if m.palette {
			m.palette = false
			m.pquery = nil
			m.psel = 0
		}
	case tea.KeyUp:
		if m.sessions {
			if m.ssel > 0 {
				m.ssel--
			}
			m.normalizeSessions()
			return m, nil
		}
		if m.palette {
			if m.psel > 0 {
				m.psel--
			}
			m.normalizePalette()
			return m, nil
		}
		m.scroll++
	case tea.KeyDown:
		if m.sessions {
			if m.ssel < len(m.sitems)-1 {
				m.ssel++
			}
			m.normalizeSessions()
			return m, nil
		}
		if m.palette {
			items := m.paletteItems()
			if m.psel < len(items)-1 {
				m.psel++
			}
			m.normalizePalette()
			return m, nil
		}
		if m.scroll > 0 {
			m.scroll--
		}
	case tea.KeyPgUp:
		if m.palette {
			return m, nil
		}
		m.scroll += 5
	case tea.KeyPgDown:
		if m.palette {
			return m, nil
		}
		m.scroll -= 5
		if m.scroll < 0 {
			m.scroll = 0
		}
	case tea.KeyEnter:
		if m.sessions {
			if len(m.sitems) == 0 {
				m.sessions = false
				return m, nil
			}
			return m.resumeSelectedSession()
		}
		if m.palette {
			items := m.paletteItems()
			if len(items) == 0 {
				m.palette = false
				return m, nil
			}
			cmd := items[m.psel]
			m.palette = false
			m.pquery = nil
			m.psel = 0
			if strings.HasPrefix(cmd, "session:") || strings.HasPrefix(cmd, "context:") {
				return m.handlePaletteAction(cmd)
			}
			return m.handleSlash(cmd)
		}
		if m.running {
			return m, nil
		}
		text := strings.TrimSpace(string(m.input))
		m.input = nil
		if text == "" {
			return m, nil
		}
		if strings.HasPrefix(text, "/") {
			return m.handleSlash(text)
		}
		runCtx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		m.running = true
		return m, func() tea.Msg { return runDoneMsg{err: m.runner.RunPrompt(runCtx, text)} }
	case tea.KeyBackspace, tea.KeyCtrlH:
		if m.palette {
			if len(m.pquery) > 0 {
				m.pquery = m.pquery[:len(m.pquery)-1]
				m.psel = 0
			}
			m.normalizePalette()
			return m, nil
		}
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	default:
		if msg.Type == tea.KeyRunes {
			if m.palette {
				m.pquery = append(m.pquery, msg.Runes...)
				m.psel = 0
				m.normalizePalette()
				return m, nil
			}
			m.input = append(m.input, msg.Runes...)
		}
	}
	return m, nil
}

func (m *model) handleSlash(text string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return m, nil
	}
	switch fields[0] {
	case "/exit", "/quit":
		_ = m.runner.SaveSession()
		return m, tea.Quit
	case "/help":
		m.help = helpLines()
		m.status = "help"
	case "/plan":
		m.help = nil
		m.runner.SetMode("plan")
		m.status = "mode: plan"
	case "/chat":
		m.help = nil
		m.runner.SetMode("chat")
		m.status = "mode: chat"
	case "/alwaysstart":
		m.help = nil
		if m.runner.ToggleAlwaysStart() {
			m.status = "alwaysstart on"
		} else {
			m.status = "alwaysstart off"
		}
	case "/clear":
		m.help = nil
		if err := m.runner.ClearSession(); err != nil {
			m.status = err.Error()
		} else {
			m.status = "conversation cleared"
		}
	case "/status":
		m.help = nil
		snap := m.runner.Snapshot()
		m.status = fmt.Sprintf("model=%s user_turns=%d", snap.Config.Model, snap.UserTurns)
	case "/sessions":
		m.help = nil
		if err := m.openSessions(); err != nil {
			m.status = err.Error()
		} else if len(m.sitems) == 0 {
			m.status = "no saved sessions"
		} else {
			m.status = fmt.Sprintf("recent sessions: %d", len(m.sitems))
		}
	case "/session", "/season":
		m.help = nil
		sess := m.runner.Session()
		m.status = fmt.Sprintf("session=%s updated=%s", sess.ID, sess.UpdatedAt.Format("2006-01-02T15:04:05Z"))
	case "/resume", "/reseason":
		m.help = nil
		if len(fields) < 2 {
			m.status = "usage: /resume <session-id|latest|recent>"
			return m, nil
		}
		if fields[1] == "latest" || fields[1] == "recent" {
			latest, err := m.runner.LoadLatestSession()
			if err != nil {
				m.status = err.Error()
			} else if latest == nil {
				m.status = "no saved sessions"
			} else {
				m.status = "resumed " + latest.ID
				m.scroll = 0
			}
			return m, nil
		}
		if err := m.runner.LoadSession(fields[1]); err != nil {
			m.status = err.Error()
		} else {
			m.status = "resumed " + fields[1]
			m.scroll = 0
		}
	case "/context":
		m.help = nil
		if len(fields) < 2 {
			m.status = "usage: /context <128k|1m|number>"
			return m, nil
		}
		value, err := parseCount(fields[1])
		if err != nil {
			m.status = "invalid context value"
			return m, nil
		}
		cfg := m.runner.Config()
		cfg.ContextWindow = value
		if err := config.Save(cfg); err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.runner.UpdateConfig(cfg)
		m.status = "context window set to " + formatCount(value)
	case "/provider":
		m.help = nil
		cfg, err := provider.RunSetup(context.Background(), m.runner.Config())
		if err != nil {
			m.status = err.Error()
		} else {
			m.runner.UpdateConfig(cfg)
			m.status = "provider updated: " + cfg.Model
		}
	case "/agents":
		m.help = nil
		parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(text, "/agents")), "||")
		tasks := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				tasks = append(tasks, part)
			}
		}
		if len(tasks) < 2 {
			m.status = "usage: /agents task a || task b || task c"
			return m, nil
		}
		runCtx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		m.running = true
		m.status = fmt.Sprintf("running %d agents", len(tasks))
		return m, func() tea.Msg {
			_, err := m.runner.RunParallel(runCtx, tasks)
			return agentsDoneMsg{err: err}
		}
	default:
		m.help = nil
		m.status = "unknown command"
	}
	return m, nil
}

func (m *model) View() string {
	snap := m.runner.Snapshot()
	if m.running && hasActiveAgents(snap.Agents) {
		m.showAgents = true
	}
	bodyHeight := m.height - 8
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	var palette *ui.PaletteState
	var sessions *ui.SessionListState
	if m.sessions {
		m.normalizeSessions()
		sessions = &ui.SessionListState{
			Items:    append([]string(nil), m.sitems...),
			Selected: m.ssel,
			MaxItems: m.paletteVisibleCount(),
		}
	}
	if m.palette {
		m.normalizePalette()
		palette = &ui.PaletteState{
			Query:    string(m.pquery),
			Items:    m.paletteItems(),
			Selected: m.psel,
			MaxItems: m.paletteVisibleCount(),
		}
	}
	return ui.RenderScreenString(ui.ScreenState{
		Title:      "SayCoding",
		Status:     m.status,
		Session:    &snap.Session,
		Config:     snap.Config,
		Events:     snap.Events,
		Draft:      m.stream,
		Running:    m.running,
		Usage:      snap.Usage,
		LastErr:    snap.LastErr,
		UserTurns:  snap.UserTurns,
		TokPerSec:  snap.TokPerSec,
		Mode:       snap.Mode,
		AlwaysStart: snap.AlwaysStart,
		Agents:     snap.Agents,
		ShowAgents: m.showAgents,
		Width:      m.width,
		Input:      string(m.input),
		Scroll:     m.scroll,
		BodyHeight: bodyHeight,
		HelpLines:  m.help,
		Palette:    palette,
		Sessions:   sessions,
	})
}

func (m *model) paletteItems() []string {
	if !m.palette {
		return nil
	}
	all := []string{
		"/help",
		"/session",
		"/status",
		"/sessions",
		"/resume recent",
		"/alwaysstart",
		"context: 128k",
		"context: 256k",
		"context: 1m",
		"/provider",
		"/agents",
		"/plan",
		"/chat",
		"/clear",
		"/exit",
	}
	if idx, err := m.runner.ListSessions(); err == nil {
		for i, item := range idx {
			if i >= 6 {
				break
			}
			all = append(all, fmt.Sprintf("session: %s  %s", item.ID, item.CWD))
		}
	}
	query := strings.ToLower(strings.TrimSpace(string(m.pquery)))
	if query == "" {
		return all
	}
	out := make([]string, 0, len(all))
	for _, item := range all {
		if strings.Contains(strings.ToLower(item), query) {
			out = append(out, item)
		}
	}
	return out
}

func (m *model) handlePaletteAction(text string) (tea.Model, tea.Cmd) {
	switch {
	case strings.HasPrefix(text, "session: "):
		body := strings.TrimSpace(strings.TrimPrefix(text, "session: "))
		parts := strings.Fields(body)
		if len(parts) == 0 {
			m.status = "invalid session entry"
			return m, nil
		}
		if err := m.runner.LoadSession(parts[0]); err != nil {
			m.status = err.Error()
		} else {
			m.status = "resumed " + parts[0]
			m.scroll = 0
		}
	case strings.HasPrefix(text, "context: "):
		valueText := strings.TrimSpace(strings.TrimPrefix(text, "context: "))
		value, err := parseCount(valueText)
		if err != nil {
			m.status = "invalid context value"
			return m, nil
		}
		cfg := m.runner.Config()
		cfg.ContextWindow = value
		if err := config.Save(cfg); err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.runner.UpdateConfig(cfg)
		m.status = "context window set to " + formatCount(value)
	}
	return m, nil
}

func (m *model) normalizePalette() {
	if !m.palette {
		return
	}
	items := m.paletteItems()
	if len(items) == 0 {
		m.psel = 0
		return
	}
	if m.psel < 0 {
		m.psel = 0
	}
	if m.psel >= len(items) {
		m.psel = len(items) - 1
	}
}

func (m *model) normalizeSessions() {
	if !m.sessions {
		return
	}
	if len(m.sitems) == 0 {
		m.ssel = 0
		return
	}
	if m.ssel < 0 {
		m.ssel = 0
	}
	if m.ssel >= len(m.sitems) {
		m.ssel = len(m.sitems) - 1
	}
}

func (m *model) paletteVisibleCount() int {
	if m.height <= 0 {
		return 8
	}
	limit := m.height - 10
	if limit < 4 {
		limit = 4
	}
	if limit > 10 {
		limit = 10
	}
	return limit
}

func hasActiveAgents(agents []collab.AgentInfo) bool {
	for _, agent := range agents {
		switch agent.Status {
		case "queued", "running", "thinking", "output", "tool":
			return true
		}
	}
	return false
}

func (m *model) openSessions() error {
	idx, err := m.runner.ListSessions()
	if err != nil {
		return err
	}
	m.sessions = true
	m.palette = false
	m.help = nil
	m.sitems = m.sitems[:0]
	for _, item := range idx {
		m.sitems = append(m.sitems, fmt.Sprintf("%s  %s", item.ID, item.CWD))
	}
	m.ssel = 0
	m.normalizeSessions()
	return nil
}

func (m *model) resumeSelectedSession() (tea.Model, tea.Cmd) {
	if len(m.sitems) == 0 {
		m.sessions = false
		return m, nil
	}
	fields := strings.Fields(m.sitems[m.ssel])
	if len(fields) == 0 {
		m.status = "invalid session entry"
		return m, nil
	}
	if err := m.runner.LoadSession(fields[0]); err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.sessions = false
	m.sitems = nil
	m.ssel = 0
	m.scroll = 0
	m.status = "resumed " + fields[0]
	return m, nil
}

func parseCount(text string) (int, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	multiplier := 1
	switch {
	case strings.HasSuffix(text, "k"):
		multiplier = 1000
		text = strings.TrimSuffix(text, "k")
	case strings.HasSuffix(text, "m"):
		multiplier = 1000000
		text = strings.TrimSuffix(text, "m")
	}
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("must be non-negative")
	}
	return value * multiplier, nil
}

func formatCount(v int) string {
	switch {
	case v >= 1000000:
		return fmt.Sprintf("%.1fm", float64(v)/1000000)
	case v >= 1000:
		return fmt.Sprintf("%.1fk", float64(v)/1000)
	default:
		return strconv.Itoa(v)
	}
}

func helpLines() []string {
	return []string{
		"/help      show this help",
		"/session   show current session metadata",
		"/status    show current model and usage summary",
		"/sessions  show recent sessions",
		"/resume    resume a saved session: /resume recent or /resume <id>",
		"/alwaysstart toggle auto-continue after each completed reply",
		"/context   set context window: /context 128k or /context 1m",
		"/provider  configure base_url, key, model",
		"/agents    run parallel subtasks: /agents a || b || c",
		"/plan      switch to planning mode",
		"/chat      switch back to normal chat mode",
		"/clear     clear current conversation",
		"/exit      quit",
		"Esc        stop current output or close help",
		"Wheel/Up/Down/PgUp/PgDn scroll conversation",
	}
}
