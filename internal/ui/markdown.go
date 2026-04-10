package ui

import (
	"regexp"
	"slices"
	"strings"
)

var (
	multiBlankRE = regexp.MustCompile(`\n{3,}`)
	boldRE       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRE     = regexp.MustCompile(`\*([^*\n]+)\*`)
	codeRE       = regexp.MustCompile("`([^`]+)`")
	linkRE       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	labelRE      = regexp.MustCompile(`^([A-Z][A-Za-z0-9 /_-]{1,30}:)\s+`)
)

func renderMarkdown(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	inCode := false
	codeLang := ""
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			if inCode {
				codeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				out = append(out, renderCodeFenceHeader(codeLang))
			} else {
				out = append(out, muted("└"))
				codeLang = ""
			}
			continue
		}
		if inCode {
			out = append(out, renderCodeFenceLine(line))
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "# "):
			out = append(out, bold(accent(strings.TrimPrefix(trimmed, "# "))))
		case strings.HasPrefix(trimmed, "## "):
			out = append(out, bold(accent(strings.TrimPrefix(trimmed, "## "))))
		case strings.HasPrefix(trimmed, "### "):
			out = append(out, bold(accent(strings.TrimPrefix(trimmed, "### "))))
		case isDivider(trimmed):
			out = append(out, muted(strings.Repeat("─", 28)))
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
			out = append(out, "  • "+applyInlineStyles(strings.TrimSpace(trimmed[2:])))
		case isOrderedList(trimmed):
			out = append(out, "  "+applyInlineStyles(trimmed))
		case isMarkdownTable(trimmed):
			out = append(out, formatTableRow(trimmed))
		case strings.HasPrefix(trimmed, "> "):
			out = append(out, quoteLine(strings.TrimSpace(trimmed[2:])))
		case isCallout(trimmed):
			out = append(out, renderCallout(trimmed))
		default:
			out = append(out, applyInlineStyles(line))
		}
	}
	return compactMarkdown(strings.Join(out, "\n"))
}

func compactMarkdown(text string) string {
	text = strings.TrimSpace(text)
	text = multiBlankRE.ReplaceAllString(text, "\n\n")
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}

func isOrderedList(text string) bool {
	if len(text) < 3 {
		return false
	}
	dot := strings.Index(text, ". ")
	if dot <= 0 {
		return false
	}
	for _, ch := range text[:dot] {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func isMarkdownTable(text string) bool {
	return strings.Count(text, "|") >= 2
}

func isDivider(text string) bool {
	if len(text) < 3 {
		return false
	}
	return strings.Trim(text, "-*_ ") == ""
}

func isCallout(text string) bool {
	return labelRE.MatchString(text)
}

func formatTableRow(text string) string {
	parts := strings.Split(text, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cell := strings.TrimSpace(part)
		if cell == "" {
			continue
		}
		if isTableDivider(cell) {
			return ""
		}
		cells = append(cells, cell)
	}
	if len(cells) == 0 {
		return ""
	}
	for i := range cells {
		cells[i] = applyInlineStyles(cells[i])
	}
	return "  " + strings.Join(cells, " | ")
}

func isTableDivider(text string) bool {
	if text == "" {
		return false
	}
	return slices.ContainsFunc([]rune(text), func(r rune) bool {
		return r != '-' && r != ':' && r != ' '
	}) == false
}

func applyInlineStyles(text string) string {
	text = linkRE.ReplaceAllStringFunc(text, func(m string) string {
		parts := linkRE.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		return accent(parts[1]) + muted(" <" + parts[2] + ">")
	})
	text = codeRE.ReplaceAllStringFunc(text, func(m string) string {
		parts := codeRE.FindStringSubmatch(m)
		if len(parts) < 2 {
			return m
		}
		return inlineCode(parts[1])
	})
	text = boldRE.ReplaceAllStringFunc(text, func(m string) string {
		parts := boldRE.FindStringSubmatch(m)
		if len(parts) < 2 {
			return m
		}
		return bold(parts[1])
	})
	text = italicRE.ReplaceAllStringFunc(text, func(m string) string {
		parts := italicRE.FindStringSubmatch(m)
		if len(parts) < 2 {
			return m
		}
		return faint(parts[1])
	})
	if loc := labelRE.FindStringIndex(text); loc != nil && loc[0] == 0 {
		label := text[:loc[1]]
		return accent(label) + text[loc[1]:]
	}
	return text
}

func renderCodeFenceHeader(lang string) string {
	if lang == "" {
		lang = "text"
	}
	return muted("┌ code") + " " + accent(lang)
}

func renderCodeFenceLine(line string) string {
	if strings.TrimSpace(line) == "" {
		return muted("│")
	}
	return muted("│ ") + codeLine(line)
}

func quoteLine(text string) string {
	return muted("│ ") + applyInlineStyles(text)
}

func renderCallout(text string) string {
	loc := labelRE.FindStringIndex(text)
	if loc == nil || loc[0] != 0 {
		return applyInlineStyles(text)
	}
	label := strings.TrimSuffix(text[:loc[1]], ":")
	body := strings.TrimSpace(text[loc[1]:])
	return calloutStyle(label) + " " + applyInlineStyles(body)
}
