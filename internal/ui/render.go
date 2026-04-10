package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"saycoding/internal/collab"
	"saycoding/internal/types"
)

func RenderScreenString(state ScreenState) string {
	var b strings.Builder
	renderHeaderTo(&b, state)
	b.WriteString("\n")
	if state.Sessions != nil {
		renderSessionsOverlayTo(&b, state.Sessions)
		b.WriteString("\n")
		renderFooterTo(&b, state)
		b.WriteString("> ")
		b.WriteString(state.Input)
		return wrapScreen(b.String(), state.Width)
	}
	if state.Palette != nil {
		renderPaletteOverlayTo(&b, state.Palette)
		b.WriteString("\n")
		renderFooterTo(&b, state)
		b.WriteString("> ")
		b.WriteString(state.Input)
		return wrapScreen(b.String(), state.Width)
	}
	renderPaletteTo(&b, state.Palette)
	renderHelpTo(&b, state.HelpLines)
	renderMessagesTo(&b, state.Session.Messages, state.Draft, state.Running, state.Scroll, state.BodyHeight)
	b.WriteString("\n")
	if state.ShowAgents {
		renderAgentsTo(&b, state.Agents)
	}
	renderEventsTo(&b, state.Events)
	b.WriteString("\n")
	renderFooterTo(&b, state)
	b.WriteString("> ")
	b.WriteString(state.Input)
	return wrapScreen(b.String(), state.Width)
}

func wrapScreen(text string, width int) string {
	if width <= 1 {
		return text
	}
	maxWidth := width - 1
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, wrapVisibleLine(line, maxWidth)...)
	}
	return strings.Join(out, "\n")
}

func wrapVisibleLine(line string, width int) []string {
	if width <= 0 || visibleWidth(line) <= width {
		return []string{line}
	}
	var out []string
	var current strings.Builder
	visible := 0
	inEscape := false
	for _, r := range line {
		current.WriteRune(r)
		switch {
		case r == '\033':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			visible++
		}
		if visible >= width {
			out = append(out, strings.TrimRight(current.String(), " "))
			current.Reset()
			visible = 0
		}
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func renderHeaderTo(b *strings.Builder, state ScreenState) {
	left := fmt.Sprintf("%s  %s", state.Title, safeSessionID(state.Session))
	right := fmt.Sprintf("%s  %s", state.Config.Model, clipRight(state.Session.CWD, 36))
	writeLine(b, accent(left+"  "+faintDot()+"  "+right))
	base := "base " + clipRight(state.Config.BaseURL, 72)
	ctxSummary := formatContextUsage(state)
	if ctxSummary != "" {
		base += "  " + faintDot() + "  " + ctxSummary
	}
	if state.Status != "" {
		base += "  " + faintDot() + "  " + clipBody(state.Status, 40)
	}
	writeLine(b, muted(base))
	if len(state.Session.Messages) == 0 && strings.TrimSpace(state.Draft) == "" {
		writeLine(b, muted("Type a request. /help shows commands. Esc stops current response."))
	}
}

func renderHelpTo(b *strings.Builder, lines []string) {
	if len(lines) == 0 {
		return
	}
	writeLine(b, muted("help"))
	for _, line := range lines {
		writeLine(b, muted("  "+line))
	}
	writeLine(b, "")
}

func renderPaletteTo(b *strings.Builder, palette *PaletteState) {
	if palette == nil {
		return
	}
	writeLine(b, muted("command palette"))
	writeLine(b, accent("> ")+palette.Query)
	limit := len(palette.Items)
	if limit > 8 {
		limit = 8
	}
	for i := 0; i < limit; i++ {
		item := palette.Items[i]
		prefix := "  "
		if i == palette.Selected {
			prefix = accent("› ")
		}
		writeLine(b, prefix+item)
	}
	writeLine(b, "")
}

func renderPaletteOverlayTo(b *strings.Builder, palette *PaletteState) {
	writeLine(b, bold(accent("Command Palette")))
	query := strings.TrimSpace(palette.Query)
	if query == "" {
		query = "type to filter commands"
		writeLine(b, muted("  > "+query))
	} else {
		writeLine(b, accent("  > ")+query)
	}
	items := paletteWindow(palette)
	if len(items) == 0 {
		writeLine(b, muted("  no matching commands"))
		return
	}
	for _, line := range items {
		writeLine(b, line)
	}
	writeLine(b, "")
	writeLine(b, muted("Enter run  Esc close  Up/Down move"))
}

func renderSessionsOverlayTo(b *strings.Builder, sessions *SessionListState) {
	writeLine(b, bold(accent("Sessions")))
	items := sessionsWindow(sessions)
	if len(items) == 0 {
		writeLine(b, muted("  no saved sessions"))
		return
	}
	for _, line := range items {
		writeLine(b, line)
	}
	writeLine(b, "")
	writeLine(b, muted("Enter resume  Esc close  Up/Down move"))
}

func paletteWindow(palette *PaletteState) []string {
	limit := palette.MaxItems
	if limit <= 0 {
		limit = 8
	}
	if len(palette.Items) <= limit {
		return formatPaletteItems(palette.Items, palette.Selected, 0)
	}
	start := palette.Selected - limit/2
	if start < 0 {
		start = 0
	}
	if start > len(palette.Items)-limit {
		start = len(palette.Items) - limit
	}
	window := palette.Items[start : start+limit]
	return formatPaletteItems(window, palette.Selected-start, start)
}

func sessionsWindow(sessions *SessionListState) []string {
	limit := sessions.MaxItems
	if limit <= 0 {
		limit = 8
	}
	if len(sessions.Items) <= limit {
		return formatSelectableItems(sessions.Items, sessions.Selected)
	}
	start := sessions.Selected - limit/2
	if start < 0 {
		start = 0
	}
	if start > len(sessions.Items)-limit {
		start = len(sessions.Items) - limit
	}
	window := sessions.Items[start : start+limit]
	return formatSelectableItems(window, sessions.Selected-start)
}

func formatPaletteItems(items []string, selected int, _ int) []string {
	return formatSelectableItems(items, selected)
}

func formatSelectableItems(items []string, selected int) []string {
	out := make([]string, 0, len(items))
	for i, item := range items {
		prefix := "    "
		line := item
		if i == selected {
			prefix = "  " + accent("› ")
			line = bold(item)
		}
		out = append(out, prefix+line)
	}
	return out
}

func renderMessagesTo(b *strings.Builder, messages []types.Message, draft string, running bool, scroll int, bodyHeight int) {
	lines := conversationLines(trimConversationMessages(messages, bodyHeight, scroll), draft, running)
	if len(lines) == 0 {
		return
	}
	if bodyHeight <= 0 {
		bodyHeight = 18
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > len(lines)-1 {
		scroll = max(0, len(lines)-1)
	}
	end := len(lines) - scroll
	if end < 0 {
		end = 0
	}
	start := end - bodyHeight
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:end] {
		writeLine(b, line)
	}
}

func trimConversationMessages(messages []types.Message, bodyHeight, scroll int) []types.Message {
	if len(messages) <= 80 {
		return messages
	}
	window := bodyHeight*4 + scroll*2
	if window < 80 {
		window = 80
	}
	if window > len(messages) {
		window = len(messages)
	}
	return messages[len(messages)-window:]
}

func conversationLines(messages []types.Message, draft string, running bool) []string {
	out := make([]string, 0, len(messages)*3)
	for _, msg := range messages {
		if len(msg.ToolCalls) > 0 {
			out = append(out, prefixedLines(formatRole("tool"), formatToolCalls(msg.ToolCalls))...)
			continue
		}
		if msg.Role == "tool" {
			out = append(out, prefixedLines(formatRole("tool"), formatToolResult(msg))...)
			continue
		}
		out = append(out, prefixedLines(formatRole(msg.Role), formatMessageBody(msg.Role, msg.Content))...)
	}
	if running || strings.TrimSpace(draft) != "" {
		out = append(out, prefixedLines(formatRole("assistant*"), formatMessageBody("assistant*", draft))...)
	}
	return out
}

func formatToolCalls(calls []types.ToolCall) string {
	lines := make([]string, 0, len(calls))
	for _, call := range calls {
		line := toolCallTitle(call)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func formatToolResult(msg types.Message) string {
	var payload struct {
		OK     bool   `json:"ok"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(msg.Content), &payload); err != nil {
		if msg.Name == "read_file" {
			return toolResultTitle(msg.Name, true)
		}
		return formatMessageBody("tool", msg.Content)
	}
	title := toolResultTitle(msg.Name, payload.OK)
	body := toolResultBody(msg.Name, payload.Result)
	if body == "" {
		return title
	}
	return title + "\n" + body
}

func toolCallTitle(call types.ToolCall) string {
	path := toolPathFromArgs(call.Arguments)
	switch call.Name {
	case "read_file":
		return "read " + fallbackToolTarget(path, call.Arguments)
	case "write_file":
		return "write " + fallbackToolTarget(path, call.Arguments)
	case "apply_patch":
		return "patch " + fallbackToolTarget(path, call.Arguments)
	case "show_diff":
		return "diff " + fallbackToolTarget(path, call.Arguments)
	case "list_dir":
		return "list " + fallbackToolTarget(path, call.Arguments)
	case "search_files":
		return "search " + fallbackToolTarget(toolQueryFromArgs(call.Arguments), call.Arguments)
	default:
		if path != "" {
			return call.Name + " " + path
		}
		return call.Name
	}
}

func toolResultTitle(name string, ok bool) string {
	status := toolStatusBadge(ok)
	switch name {
	case "read_file":
		if ok {
			return status + " ok"
		}
		return status + " read failed"
	case "write_file":
		return status + " " + toolOutcomeLabel("write", ok)
	case "apply_patch":
		return status + " " + toolOutcomeLabel("patch", ok)
	case "show_diff":
		return status + " " + toolOutcomeLabel("diff", ok)
	default:
		return status + " " + toolOutcomeLabel(name, ok)
	}
}

func toolResultBody(name, result string) string {
	result = strings.TrimSpace(result)
	if result == "" {
		return ""
	}
	switch name {
	case "write_file", "apply_patch", "show_diff":
		if strings.HasPrefix(result, "tool error:") {
			return indentToolBody(clipToolBody(strings.TrimSpace(strings.TrimPrefix(result, "tool error:")), 8))
		}
		return indentToolBody(formatDiffPreview(result))
	case "read_file":
		return ""
	default:
		return indentToolBody(clipToolBody(result, 8))
	}
}

func toolStatusBadge(ok bool) string {
	if ok {
		return "\033[1;30;42m success \033[0m"
	}
	return "\033[1;37;41m failed \033[0m"
}

func toolOutcomeLabel(name string, ok bool) string {
	if ok {
		switch name {
		case "diff":
			return "diff ready"
		default:
			return name + " ok"
		}
	}
	return name + " failed"
}

func toolPathFromArgs(raw string) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return ""
	}
	path, _ := args["path"].(string)
	return strings.TrimSpace(path)
}

func toolQueryFromArgs(raw string) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return ""
	}
	query, _ := args["query"].(string)
	return strings.TrimSpace(query)
}

func fallbackToolTarget(primary, raw string) string {
	if primary != "" {
		return primary
	}
	return clipBody(strings.TrimSpace(raw), 80)
}

func formatDiffPreview(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return ""
	}
	limit := len(lines)
	if limit > 14 {
		limit = 14
	}
	out := make([]string, 0, limit+1)
	for i := 0; i < limit; i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "+"):
			out = append(out, "\033[32m"+clipBody(line, 120)+"\033[0m")
		case strings.HasPrefix(line, "-"):
			out = append(out, "\033[31m"+clipBody(line, 120)+"\033[0m")
		default:
			out = append(out, clipBody(line, 120))
		}
	}
	if len(lines) > limit {
		out = append(out, muted(fmt.Sprintf("... (%d more lines)", len(lines)-limit)))
	}
	return strings.Join(out, "\n")
}

func clipToolBody(text string, maxLines int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return ""
	}
	limit := len(lines)
	if limit > maxLines {
		limit = maxLines
	}
	out := make([]string, 0, limit+1)
	for i := 0; i < limit; i++ {
		out = append(out, clipBody(lines[i], 120))
	}
	if len(lines) > limit {
		out = append(out, fmt.Sprintf("... (%d more lines)", len(lines)-limit))
	}
	return strings.Join(out, "\n")
}

func indentToolBody(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return strings.Join(lines, "\n")
}

func renderEventsTo(b *strings.Builder, events []string) {
	if len(events) == 0 {
		return
	}
	if len(events) > 2 {
		events = events[len(events)-2:]
	}
	writeLine(b, muted("recent activity"))
	for _, item := range events {
		writeLine(b, muted("  "+faintArrow()+" "+clipBody(item, 120)))
	}
}

func renderAgentsTo(b *strings.Builder, agents []collab.AgentInfo) {
	if len(agents) == 0 {
		return
	}
	writeLine(b, muted("agents"))
	for _, agent := range agents {
		writeLine(b, formatAgentHeadline(agent))
		if strings.TrimSpace(agent.LastEvent) != "" {
			writeLine(b, muted(agentIndent(agent.Depth+1)+clipBody(agent.LastEvent, 72)))
		} else if strings.TrimSpace(agent.Task) != "" {
			writeLine(b, muted(agentIndent(agent.Depth+1)+clipBody(agent.Task, 72)))
		}
	}
	writeLine(b, "")
}

func renderFooterTo(b *strings.Builder, state ScreenState) {
	writeLine(b, muted(strings.Repeat("─", 72)))
	statusLamp := footerLamp(state)
	tokenLine := "tok " + footerTokenUsage(state)
	userLine := fmt.Sprintf("usr %d", state.UserTurns)
	speedLine := footerSpeed(state.TokPerSec)
	writeLine(b, statusLamp+"  "+muted(tokenLine+"  "+faintDot()+"  "+userLine+"  "+faintDot()+"  "+speedLine))
}

func writeLine(b *strings.Builder, line string) {
	b.WriteString(line)
	b.WriteByte('\n')
}

func prefixedLines(prefix, body string) []string {
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return []string{prefix}
	}
	parts := strings.Split(body, "\n")
	out := make([]string, 0, len(parts))
	indent := strings.Repeat(" ", visibleWidth(prefix)+1)
	for i, part := range parts {
		if i == 0 {
			out = append(out, prefix+" "+part)
		} else {
			out = append(out, indent+part)
		}
	}
	return out
}

func visibleWidth(text string) int {
	width := 0
	inEscape := false
	for _, r := range text {
		switch {
		case r == '\033':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			width++
		}
	}
	return width
}

func agentStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "thinking":
		return "\033[1;30;46m 规划中 \033[0m"
	case "output":
		return "\033[1;30;47m 输出中 \033[0m"
	case "tool":
		return "\033[1;30;43m 调工具 \033[0m"
	case "running":
		return "\033[1;30;44m 运行中 \033[0m"
	case "done":
		return "\033[1;30;42m 已完成 \033[0m"
	case "error":
		return "\033[1;37;41m 出错 \033[0m"
	case "queued":
		return "\033[1;30;100m 排队中 \033[0m"
	default:
		return muted("[" + status + "]")
	}
}

func formatAgentHeadline(agent collab.AgentInfo) string {
	name := agentTreeLabel(agent)
	tools := muted(fmt.Sprintf("工具 %d", agent.ToolCalls))
	tokens := muted(fmt.Sprintf("tok %s", formatAgentCount(agent.TotalTokens)))
	return fmt.Sprintf("%s%s  %s  %s  %s", agentIndent(agent.Depth), bold(name), tools, tokens, agentStatusLabel(agent.Status))
}

func agentTreeLabel(agent collab.AgentInfo) string {
	if agent.Depth <= 1 {
		return agent.Name
	}
	if idx := strings.LastIndex(agent.Name, "."); idx >= 0 && idx+1 < len(agent.Name) {
		return "└─ agent-" + agent.Name[idx+1:]
	}
	return "└─ " + agent.Name
}

func agentIndent(depth int) string {
	if depth <= 1 {
		return "  "
	}
	return "  " + strings.Repeat("  ", depth-1)
}

func formatAgentCount(v int) string {
	switch {
	case v >= 1000000:
		return fmt.Sprintf("%.1fm", float64(v)/1000000)
	case v >= 1000:
		return fmt.Sprintf("%.1fk", float64(v)/1000)
	default:
		return fmt.Sprintf("%d", v)
	}
}
