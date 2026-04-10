package types

import "time"

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Timestamp  time.Time  `json:"timestamp"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Session struct {
	ID        string    `json:"id"`
	CWD       string    `json:"cwd"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
}

type SessionIndexEntry struct {
	ID        string    `json:"id"`
	CWD       string    `json:"cwd"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Config struct {
	BaseURL              string `json:"base_url"`
	APIKey               string `json:"api_key,omitempty"`
	APIKeyEnv            string `json:"api_key_env"`
	Model                string `json:"model"`
	ContextWindow        int    `json:"context_window"`
	MaxSteps             int    `json:"max_steps"`
	ShellTimeoutSec      int    `json:"shell_timeout_sec"`
	AutoApproveWorkspace bool   `json:"auto_approve_workspace"`
	Theme                string `json:"theme"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
