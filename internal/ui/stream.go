package ui

import (
	"fmt"
	"strings"
)

func StreamStart() {
	fmt.Print("\n" + accent("ai  ") + " ")
}

func AssistantChunk(text string) {
	fmt.Print(text)
}

func AssistantDone() {
	fmt.Print("\n")
}

func ToolBatch(names []string) {
	if len(names) == 0 {
		return
	}
	fmt.Println(muted("tools: " + strings.Join(names, " -> ")))
}

func ToolEvent(label, detail string) {
	line := label
	if strings.TrimSpace(detail) != "" {
		line += " " + detail
	}
	fmt.Println(muted(line))
}
