package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"saycoding/internal/types"
)

func cloneUsage(usage *types.Usage) *types.Usage {
	if usage == nil {
		return nil
	}
	copy := *usage
	return &copy
}

func countUserTurns(messages []types.Message) int {
	count := 0
	for _, msg := range messages {
		if msg.Role == "user" {
			count++
		}
	}
	return count
}

func tokensPerSecond(metrics metricsState) float64 {
	if metrics.lastDuration <= 0 || metrics.lastOutputTokens <= 0 {
		return 0
	}
	return float64(metrics.lastOutputTokens) / metrics.lastDuration.Seconds()
}

func toolNames(calls []types.ToolCall) []string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		names = append(names, call.Name)
	}
	return names
}

func clipEvent(text string) string {
	if len(text) <= 80 {
		return text
	}
	return text[:80] + "..."
}

func summarizeResult(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return ""
	}
	summary := strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		summary += fmt.Sprintf(" (+%d lines)", len(lines)-1)
	}
	return clipEvent(summary)
}

func describeToolCalls(calls []types.ToolCall) []string {
	out := make([]string, 0, len(calls))
	for _, call := range calls {
		out = append(out, describeToolCall(call))
	}
	return out
}

func describeToolCall(call types.ToolCall) string {
	args := parseToolArgs(call.Arguments)
	switch call.Name {
	case "read_file":
		path := stringArg(args, "path")
		start := intArg(args, "start_line")
		end := intArg(args, "end_line")
		switch {
		case start > 0 && end > 0:
			return fmt.Sprintf("read %s lines %d-%d", path, start, end)
		case start > 0:
			return fmt.Sprintf("read %s from line %d", path, start)
		default:
			return fmt.Sprintf("read %s", path)
		}
	case "list_dir":
		path := stringArg(args, "path")
		if path == "" {
			path = "."
		}
		return "list " + path
	case "search_files":
		query := stringArg(args, "query")
		glob := stringArg(args, "glob")
		if glob != "" {
			return fmt.Sprintf("search %q in %s", query, glob)
		}
		return fmt.Sprintf("search %q", query)
	case "grep_files":
		query := stringArg(args, "query")
		glob := stringArg(args, "glob")
		if glob != "" {
			return fmt.Sprintf("grep %q in %s", query, glob)
		}
		return fmt.Sprintf("grep %q", query)
	case "write_file":
		return "write " + stringArg(args, "path")
	case "edit_file":
		path := stringArg(args, "path")
		if boolArg(args, "replace_all") {
			return "edit-all " + path
		}
		return "edit " + path
	case "apply_patch":
		return "patch " + stringArg(args, "path")
	case "show_diff":
		return "diff " + stringArg(args, "path")
	case "run_shell":
		cmd := stringArg(args, "command")
		return "shell " + clipEvent(cmd)
	default:
		return call.Name
	}
}

func parseToolArgs(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, _ := args[key].(string)
	return value
}

func intArg(args map[string]any, key string) int {
	if args == nil {
		return 0
	}
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func boolArg(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	value, _ := args[key].(bool)
	return value
}
