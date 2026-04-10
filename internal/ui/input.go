package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
)

func ReadLineWithPrompt(prompt string) (string, error) {
	return readLine(prompt)
}

func Choose(title string, options []string) (int, error) {
	fmt.Println(title)
	for i, item := range options {
		fmt.Printf("  %d. %s\n", i+1, item)
	}
	for {
		text, err := ReadLineWithPrompt("select> ")
		if err != nil {
			return 0, err
		}
		idx, err := strconv.Atoi(strings.TrimSpace(text))
		if err == nil && idx >= 1 && idx <= len(options) {
			return idx - 1, nil
		}
		fmt.Println("invalid selection")
	}
}

func ReadLine() (string, error) {
	return readLine("")
}

func readLine(prompt string) (string, error) {
	cfg := &readline.Config{
		Prompt:                 prompt,
		DisableAutoSaveHistory: true,
		HistoryLimit:           -1,
		InterruptPrompt:        "^C",
		EOFPrompt:              "exit",
	}
	rl, err := readline.NewEx(cfg)
	if err != nil {
		return "", err
	}
	defer rl.Close()
	line, err := rl.Readline()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
