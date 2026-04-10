package diff

import (
	"fmt"
	"strings"
)

const (
	searchMarker  = "<<<<<<< SEARCH"
	replaceMarker = "======="
	endMarker     = ">>>>>>> REPLACE"
)

type block struct {
	search  string
	replace string
}

func Apply(original, patch string) (string, error) {
	if strings.TrimSpace(patch) == "" {
		return "", fmt.Errorf("patch is empty")
	}
	blocks, err := parseBlocks(patch)
	if err != nil {
		return "", err
	}
	out := original
	for i, block := range blocks {
		next, err := applyBlock(out, block)
		if err != nil {
			return "", fmt.Errorf("block %d: %w", i+1, err)
		}
		out = next
	}
	return out, nil
}

func applyBlock(original string, block block) (string, error) {
	if block.search == "" {
		if strings.HasSuffix(original, "\n") || original == "" {
			return original + block.replace, nil
		}
		return original + "\n" + block.replace, nil
	}
	positions := allIndexes(original, block.search)
	switch len(positions) {
	case 0:
		return "", fmt.Errorf("search block not found; make SEARCH smaller or include exact current text")
	case 1:
		pos := positions[0]
		return original[:pos] + block.replace + original[pos+len(block.search):], nil
	default:
		return "", fmt.Errorf("search block matched %d locations; add more unique context", len(positions))
	}
}

func allIndexes(text, needle string) []int {
	if needle == "" {
		return nil
	}
	var out []int
	for start := 0; start <= len(text)-len(needle); {
		idx := strings.Index(text[start:], needle)
		if idx < 0 {
			break
		}
		absolute := start + idx
		out = append(out, absolute)
		start = absolute + len(needle)
	}
	return out
}

func Preview(path, before, after string) string {
	if before == after {
		return "no changes"
	}
	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")
	start := 0
	for start < len(beforeLines) && start < len(afterLines) && beforeLines[start] == afterLines[start] {
		start++
	}
	endBefore := len(beforeLines) - 1
	endAfter := len(afterLines) - 1
	for endBefore >= start && endAfter >= start && beforeLines[endBefore] == afterLines[endAfter] {
		endBefore--
		endAfter--
	}
	var out []string
	out = append(out, fmt.Sprintf("--- %s", path), fmt.Sprintf("+++ %s", path))
	if start > 0 {
		out = append(out, fmt.Sprintf(" %s", contextLine(start, beforeLines[start-1])))
	}
	for i := start; i <= endBefore && i < len(beforeLines); i++ {
		out = append(out, "-"+contextLine(i+1, beforeLines[i]))
	}
	for i := start; i <= endAfter && i < len(afterLines); i++ {
		out = append(out, "+"+contextLine(i+1, afterLines[i]))
	}
	if endBefore+1 < len(beforeLines) && endAfter+1 < len(afterLines) {
		out = append(out, fmt.Sprintf(" %s", contextLine(endAfter+2, afterLines[endAfter+1])))
	}
	return strings.Join(limitLines(out, 40), "\n")
}

func contextLine(line int, text string) string {
	return fmt.Sprintf("%4d | %s", line, text)
}

func limitLines(lines []string, max int) []string {
	if len(lines) <= max {
		return lines
	}
	head := lines[:max]
	head = append(head, fmt.Sprintf("... (%d more lines)", len(lines)-max))
	return head
}

func parseBlocks(patch string) ([]block, error) {
	parts := strings.Split(patch, searchMarker)
	if len(parts) <= 1 {
		return nil, fmt.Errorf("patch must contain %s", searchMarker)
	}
	if strings.TrimSpace(parts[0]) != "" {
		return nil, fmt.Errorf("patch must start with %s", searchMarker)
	}
	blocks := make([]block, 0, len(parts)-1)
	for _, part := range parts[1:] {
		block, err := parseBlock(part)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func parseBlock(part string) (block, error) {
	trimmed := strings.TrimPrefix(part, "\n")
	mid := strings.Index(trimmed, "\n"+replaceMarker)
	if mid < 0 && strings.HasPrefix(trimmed, replaceMarker) {
		mid = 0
	}
	end := strings.Index(trimmed, "\n"+endMarker)
	if end < 0 && strings.HasPrefix(trimmed, endMarker) {
		end = 0
	}
	if mid < 0 || end < 0 || mid > end {
		return block{}, fmt.Errorf("invalid patch block; expected SEARCH / ======= / REPLACE markers")
	}
	search := trimmed[:mid]
	replaceStart := mid + len(replaceMarker)
	if strings.HasPrefix(trimmed[mid:], "\n"+replaceMarker) {
		replaceStart++
	}
	replace := trimmed[replaceStart:end]
	search = strings.TrimPrefix(search, "\n")
	search = strings.TrimSuffix(search, "\n")
	replace = strings.TrimPrefix(replace, "\n")
	replace = strings.TrimSuffix(replace, "\n")
	return block{
		search:  normalizeBlock(search),
		replace: normalizeBlock(replace),
	}, nil
}

func normalizeBlock(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return text
}
