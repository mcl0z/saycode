package ui

import (
	"fmt"
	"strings"

	"saycoding/internal/collab"
	"saycoding/internal/types"
)

type ScreenState struct {
	Title      string
	Status     string
	Session    *types.Session
	Config     types.Config
	Events     []string
	Draft      string
	Running    bool
	Usage      *types.Usage
	LastErr    string
	UserTurns  int
	TokPerSec  float64
	Mode       string
	AlwaysStart bool
	Agents     []collab.AgentInfo
	ShowAgents bool
	Width      int
	Input      string
	Scroll     int
	BodyHeight int
	HelpLines  []string
	Palette    *PaletteState
	Sessions   *SessionListState
}

type PaletteState struct {
	Query    string
	Items    []string
	Selected int
	MaxItems int
}

type SessionListState struct {
	Items    []string
	Selected int
	MaxItems int
}

func RenderScreen(state ScreenState) {
	fmt.Print(RenderScreenString(state))
}

func ShowBanner(msg string) {
	clearScreen()
	renderMuted("SayCoding")
	fmt.Println(msg)
	fmt.Println()
}

func renderHeader(state ScreenState) {
	left := fmt.Sprintf("%s  %s", state.Title, safeSessionID(state.Session))
	right := fmt.Sprintf("%s  %s  %s", state.Config.Model, state.Mode, clipRight(state.Session.CWD, 28))
	if state.AlwaysStart {
		right = fmt.Sprintf("%s  %s  alwaysstart  %s", state.Config.Model, state.Mode, clipRight(state.Session.CWD, 28))
	}
	renderAccent(left + "  " + faintDot() + "  " + right)
	base := "base " + clipRight(state.Config.BaseURL, 72)
	ctxSummary := formatContextUsage(state)
	if ctxSummary != "" {
		base += "  " + faintDot() + "  " + ctxSummary
	}
	if state.Status != "" {
		base += "  " + faintDot() + "  " + clipBody(state.Status, 40)
	}
	renderMuted(base)
	if len(state.Session.Messages) == 0 && strings.TrimSpace(state.Draft) == "" {
		renderMuted("Type a request. /help shows commands. Esc stops current response.")
	}
}

func renderMessages(messages []types.Message, draft string, running bool) {
	start := 0
	if len(messages) > 10 {
		start = len(messages) - 10
	}
	if len(messages) == 0 {
		if !running && strings.TrimSpace(draft) == "" {
			return
		}
	} else {
		for _, msg := range messages[start:] {
			if msg.Role == "tool" {
				continue
			}
			body := msg.Content
			if len(msg.ToolCalls) > 0 {
				continue
			}
			renderMessageRow(msg.Role, formatMessageBody(msg.Role, body))
		}
	}
	if running || strings.TrimSpace(draft) != "" {
		renderMessageRow("assistant*", formatMessageBody("assistant*", draft))
	}
}

func renderEvents(events []string) {
	if len(events) == 0 {
		return
	}
	if len(events) > 2 {
		events = events[len(events)-2:]
	}
	renderMuted("recent activity")
	for _, item := range events {
		renderMuted("  " + faintArrow() + " " + clipBody(item, 120))
	}
}

func joinToolNames(calls []types.ToolCall) string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		names = append(names, call.Name)
	}
	return strings.Join(names, ", ")
}

func clipBody(text string, max int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func formatMessageBody(role, text string) string {
	if role == "assistant" || role == "assistant*" {
		return renderMarkdown(text)
	}
	if role == "user" {
		return compactPlain(text)
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return ""
	}
	limit := len(lines)
	if role == "tool" {
		limit = 6
	}
	if len(lines) < limit {
		limit = len(lines)
	}
	out := make([]string, 0, limit+1)
	for i := 0; i < limit; i++ {
		line := lines[i]
		if role != "tool" {
			line = clipBody(line, 120)
		} else {
			line = clipBody(line, 140)
		}
		out = append(out, line)
	}
	if role == "tool" && len(lines) > limit {
		out = append(out, fmt.Sprintf("... (%d more lines)", len(lines)-limit))
	}
	return strings.Join(out, "\n    ")
}

func compactPlain(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return ""
	}
	for i := range lines {
		lines[i] = clipBody(lines[i], 120)
	}
	return strings.Join(lines, "\n    ")
}

func clipRight(text string, max int) string {
	if len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

func safeSessionID(sess *types.Session) string {
	if sess == nil {
		return "-"
	}
	return sess.ID
}

func clearScreen() {
	fmt.Print("\033[H\033[J")
}

func renderMessageRow(role, body string) {
	label := formatRole(role)
	fmt.Printf("%s %s\n", label, body)
}

func formatRole(role string) string {
	switch role {
	case "user":
		return accent("you ")
	case "assistant":
		return accent("ai  ")
	case "assistant*":
		return accent("ai* ")
	case "tool":
		return muted("tool")
	default:
		return muted(clipRight(role, 4))
	}
}

func accent(text string) string {
	return "\033[1;36m" + text + "\033[0m"
}

func bold(text string) string {
	return "\033[1m" + text + "\033[0m"
}

func inlineCode(text string) string {
	return "\033[48;5;236m\033[38;5;252m " + text + " \033[0m"
}

func muted(text string) string {
	return "\033[2m" + text + "\033[0m"
}

func faint(text string) string {
	return "\033[3;37m" + text + "\033[0m"
}

func codeLine(text string) string {
	return "\033[38;5;151m" + text + "\033[0m"
}

func calloutStyle(label string) string {
	kind := strings.ToLower(strings.TrimSpace(label))
	switch kind {
	case "info", "note", "tip", "hint":
		return "\033[1;30;46m " + label + " \033[0m"
	case "warning", "warn", "caution":
		return "\033[1;30;43m " + label + " \033[0m"
	case "error", "danger", "fatal":
		return "\033[1;37;41m " + label + " \033[0m"
	case "success", "done", "ok":
		return "\033[1;30;42m " + label + " \033[0m"
	default:
		return accent(label)
	}
}

func renderAccent(text string) {
	fmt.Println(accent(text))
}

func renderMuted(text string) {
	fmt.Println(muted(text))
}

func faintDot() string {
	return "·"
}

func faintArrow() string {
	return "›"
}

func renderFooter(state ScreenState) {
	statusLamp := footerLamp(state)
	tokenLine := "tok " + footerTokenUsage(state)
	userLine := fmt.Sprintf("usr %d", state.UserTurns)
	speedLine := footerSpeed(state.TokPerSec)
	fmt.Println(muted(strings.Repeat("─", 72)))
	fmt.Println(statusLamp + "  " + muted(tokenLine+"  "+faintDot()+"  "+userLine+"  "+faintDot()+"  "+speedLine))
}

func footerLamp(state ScreenState) string {
	switch {
	case state.LastErr != "":
		return "\033[31m●\033[0m"
	case state.Running:
		return "\033[33m●\033[0m"
	default:
		return "\033[32m●\033[0m"
	}
}

func footerTokenUsage(state ScreenState) string {
	if state.Usage != nil && state.Usage.TotalTokens > 0 {
		return fmt.Sprintf("%s in %s out %s total", shortCount(state.Usage.InputTokens), shortCount(state.Usage.OutputTokens), shortCount(state.Usage.TotalTokens))
	}
	used := estimateContextTokens(state.Session, state.Draft)
	return fmt.Sprintf("%s est", shortCount(used))
}

func footerSpeed(v float64) string {
	if v <= 0 {
		return "speed --"
	}
	return fmt.Sprintf("speed %.1f tok/s", v)
}

func formatContextUsage(state ScreenState) string {
	limit := contextWindowForConfig(state.Config)
	if limit <= 0 {
		return ""
	}
	used := estimateContextTokens(state.Session, state.Draft)
	if state.Usage != nil && state.Usage.TotalTokens > 0 {
		used = state.Usage.TotalTokens
	}
	percent := used * 100 / limit
	return fmt.Sprintf("ctx %s/%s %d%%", shortCount(used), shortCount(limit), percent)
}

func contextWindowForConfig(cfg types.Config) int {
	if cfg.ContextWindow > 0 {
		return cfg.ContextWindow
	}
	return contextWindowForModel(cfg.Model)
}

func estimateContextTokens(sess *types.Session, draft string) int {
	if sess == nil {
		return estimateTokens(draft)
	}
	total := 0
	for _, msg := range sess.Messages {
		total += estimateTokens(msg.Content)
		total += estimateTokens(msg.Name)
		for _, call := range msg.ToolCalls {
			total += estimateTokens(call.Name)
			total += estimateTokens(call.Arguments)
		}
	}
	total += estimateTokens(draft)
	return total
}

func estimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runes := len([]rune(text))
	return max(1, runes/4)
}

func contextWindowForModel(model string) int {
	name := strings.ToLower(model)
	switch {
	case strings.Contains(name, "gpt-4.1"):
		return 128000
	case strings.Contains(name, "gpt-4o"):
		return 128000
	case strings.Contains(name, "gpt-5"):
		return 128000
	case strings.Contains(name, "o1"):
		return 200000
	case strings.Contains(name, "o3"):
		return 200000
	default:
		return 128000
	}
}

func shortCount(v int) string {
	switch {
	case v >= 1000000:
		return fmt.Sprintf("%.1fm", float64(v)/1000000)
	case v >= 1000:
		return fmt.Sprintf("%.1fk", float64(v)/1000)
	default:
		return fmt.Sprintf("%d", v)
	}
}
