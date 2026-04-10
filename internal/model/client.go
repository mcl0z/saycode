package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"saycoding/internal/types"
)

type ToolSchema struct {
	Type     string         `json:"type"`
	Function ToolDefinition `json:"function"`
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ChatRequest struct {
	Model         string            `json:"model"`
	Messages      []apiMessage      `json:"messages"`
	Tools         []ToolSchema      `json:"tools,omitempty"`
	Stream        bool              `json:"stream"`
	StreamOptions *streamOptionsReq `json:"stream_options,omitempty"`
}

type Event struct {
	Content   string
	ToolCalls []types.ToolCall
	Usage     *types.Usage
	Done      bool
}

type streamOptionsReq struct {
	IncludeUsage bool `json:"include_usage"`
}

type ModelInfo struct {
	ID               string `json:"id"`
	ContextWindow    int    `json:"context_window,omitempty"`
	MaxContextLength int    `json:"max_context_length,omitempty"`
	InputTokenLimit  int    `json:"input_token_limit,omitempty"`
	TokenLimit       int    `json:"token_limit,omitempty"`
}

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	onRetry func(attempt, maxAttempts int, err error)
}

func NewClient(baseURL, apiKey string, timeoutSec int) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{},
	}
}

func (c *Client) SetRetryHook(fn func(attempt, maxAttempts int, err error)) {
	c.onRetry = fn
}

func (c *Client) Stream(ctx context.Context, cfg types.Config, messages []types.Message, tools []ToolSchema) (<-chan Event, <-chan error) {
	events := make(chan Event, 16)
	errs := make(chan error, 1)
	go func() {
		defer close(events)
		defer close(errs)
		if err := c.streamWithRetry(ctx, cfg, messages, tools, events); err != nil {
			errs <- err
		}
	}()
	return events, errs
}

func (c *Client) streamWithRetry(ctx context.Context, cfg types.Config, messages []types.Message, tools []ToolSchema, events chan<- Event) error {
	const maxAttempts = 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		emitted, err := c.stream(ctx, cfg, messages, tools, events)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if emitted || !shouldRetryStreamError(err) || attempt == maxAttempts {
			return err
		}
		if c.onRetry != nil {
			c.onRetry(attempt+1, maxAttempts, err)
		}
		timer := time.NewTimer(time.Duration(attempt) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func (c *Client) stream(ctx context.Context, cfg types.Config, messages []types.Message, tools []ToolSchema, events chan<- Event) (bool, error) {
	reqBody := ChatRequest{
		Model:    cfg.Model,
		Messages: toAPIMessages(messages),
		Tools:    tools,
		Stream:   true,
		StreamOptions: &streamOptionsReq{
			IncludeUsage: true,
		},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, markRetryable(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := ioReadAll(resp)
		err := fmt.Errorf("model API %s: %s", resp.Status, strings.TrimSpace(string(body)))
		if isRetryableStatus(resp.StatusCode) {
			return false, markRetryable(err)
		}
		return false, err
	}
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	acc := map[int]*types.ToolCall{}
	emitted := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			events <- Event{Done: true}
			return true, nil
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			emitted = true
			events <- Event{Usage: &types.Usage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
				TotalTokens:  chunk.Usage.TotalTokens,
			}}
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				emitted = true
				events <- Event{Content: choice.Delta.Content}
			}
			for _, tc := range choice.Delta.ToolCalls {
				emitted = true
				item := acc[tc.Index]
				if item == nil {
					item = &types.ToolCall{ID: tc.ID, Name: tc.Function.Name}
					acc[tc.Index] = item
				}
				if tc.Function.Name != "" {
					item.Name = tc.Function.Name
				}
				item.Arguments += tc.Function.Arguments
			}
			if choice.FinishReason == "tool_calls" && len(acc) > 0 {
				out := make([]types.ToolCall, 0, len(acc))
				for i := 0; i < len(acc); i++ {
					if acc[i] != nil {
						out = append(out, *acc[i])
					}
				}
				emitted = true
				events <- Event{ToolCalls: out}
				acc = map[int]*types.ToolCall{}
			}
		}
	}
	err = scanner.Err()
	if err == nil {
		return emitted, nil
	}
	if emitted {
		return true, err
	}
	return false, markRetryable(err)
}

type retryableStreamError struct {
	err error
}

func (e retryableStreamError) Error() string {
	return e.err.Error()
}

func (e retryableStreamError) Unwrap() error {
	return e.err
}

func markRetryable(err error) error {
	if err == nil {
		return nil
	}
	return retryableStreamError{err: err}
}

func shouldRetryStreamError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var retryable retryableStreamError
	if errors.As(err, &retryable) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

type apiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	Name       string        `json:"name,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	ToolCalls  []apiToolCall `json:"tool_calls,omitempty"`
}

type apiToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function apiFunction `json:"function"`
}

type apiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type streamChunk struct {
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func toAPIMessages(messages []types.Message) []apiMessage {
	out := make([]apiMessage, 0, len(messages))
	for _, msg := range messages {
		item := apiMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		}
		for _, tc := range msg.ToolCalls {
			item.ToolCalls = append(item.ToolCalls, apiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: apiFunction{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		out = append(out, item)
	}
	return out
}

func ioReadAll(resp *http.Response) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(resp.Body)
	return buf.Bytes(), err
}

func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	modelsCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(modelsCtx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := ioReadAll(resp)
		return nil, fmt.Errorf("list models %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func (m ModelInfo) ContextLimit() int {
	switch {
	case m.ContextWindow > 0:
		return m.ContextWindow
	case m.MaxContextLength > 0:
		return m.MaxContextLength
	case m.InputTokenLimit > 0:
		return m.InputTokenLimit
	case m.TokenLimit > 0:
		return m.TokenLimit
	default:
		return 0
	}
}
