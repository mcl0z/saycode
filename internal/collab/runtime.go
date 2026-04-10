package collab

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type AgentInfo struct {
	Name        string    `json:"name"`
	ParentName  string    `json:"parent_name"`
	Depth       int       `json:"depth"`
	Task        string    `json:"task"`
	Status      string    `json:"status"`
	SessionID   string    `json:"session_id"`
	LastEvent   string    `json:"last_event"`
	ToolCalls   int       `json:"tool_calls"`
	TotalTokens int       `json:"total_tokens"`
	CanWrite    bool      `json:"can_write"`
	CanShell    bool      `json:"can_shell"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TeamMessage struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type Runtime struct {
	mu      sync.Mutex
	agents  map[string]*AgentInfo
	inboxes map[string][]TeamMessage
	events  []string
}

func NewRuntime() *Runtime {
	return &Runtime{
		agents:  map[string]*AgentInfo{},
		inboxes: map[string][]TeamMessage{},
		events:  nil,
	}
}

func (r *Runtime) Register(name, parentName string, depth int, task, sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.agents[name]; exists {
		return
	}
	r.agents[name] = &AgentInfo{
		Name:       name,
		ParentName: parentName,
		Depth:      depth,
		Task:       task,
		Status:     "queued",
		SessionID:  sessionID,
		CanWrite:   depth == 0,
		CanShell:   depth == 0,
		UpdatedAt:  time.Now().UTC(),
	}
	r.pushEventLocked(fmt.Sprintf("%s joined team", name))
}

func (r *Runtime) Update(name, status, event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[name]
	if !ok {
		return
	}
	agent.Status = status
	if event != "" {
		agent.LastEvent = event
	}
	agent.UpdatedAt = time.Now().UTC()
	if event != "" {
		r.pushEventLocked(fmt.Sprintf("%s [%s] %s", name, status, event))
	}
}

func (r *Runtime) IncrementToolCalls(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[name]
	if !ok {
		return
	}
	agent.ToolCalls++
	agent.UpdatedAt = time.Now().UTC()
}

func (r *Runtime) UpdateUsage(name string, totalTokens int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[name]
	if !ok {
		return
	}
	if totalTokens > 0 {
		agent.TotalTokens = totalTokens
	}
	agent.UpdatedAt = time.Now().UTC()
}

func (r *Runtime) Snapshot() []AgentInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]AgentInfo, 0, len(r.agents))
	for _, agent := range r.agents {
		out = append(out, *agent)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *Runtime) AgentNames(except string) []string {
	all := r.Snapshot()
	out := make([]string, 0, len(all))
	for _, item := range all {
		if item.Name != except {
			out = append(out, item.Name)
		}
	}
	return out
}

func (r *Runtime) Send(from, to, content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.agents[to]; !ok {
		return fmt.Errorf("unknown agent: %s", to)
	}
	r.inboxes[to] = append(r.inboxes[to], TeamMessage{
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now().UTC(),
	})
	r.pushEventLocked(fmt.Sprintf("%s -> %s: %s", from, to, content))
	if agent, ok := r.agents[from]; ok {
		agent.LastEvent = "sent to " + to + ": " + content
		agent.UpdatedAt = time.Now().UTC()
	}
	if agent, ok := r.agents[to]; ok {
		agent.LastEvent = "from " + from + ": " + content
		agent.UpdatedAt = time.Now().UTC()
	}
	return nil
}

func (r *Runtime) ReadInbox(name string) []TeamMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	msgs := append([]TeamMessage(nil), r.inboxes[name]...)
	delete(r.inboxes, name)
	if len(msgs) > 0 {
		r.pushEventLocked(fmt.Sprintf("%s read %d message(s)", name, len(msgs)))
	}
	return msgs
}

func (r *Runtime) Grant(name string, canWrite, canShell bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[name]
	if !ok {
		return fmt.Errorf("unknown agent: %s", name)
	}
	agent.CanWrite = canWrite
	agent.CanShell = canShell
	agent.UpdatedAt = time.Now().UTC()
	r.pushEventLocked(fmt.Sprintf("lead granted %s permissions: write=%t shell=%t", name, canWrite, canShell))
	return nil
}

func (r *Runtime) Reset(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[name]
	if !ok {
		return fmt.Errorf("unknown agent: %s", name)
	}
	agent.Status = "queued"
	agent.LastEvent = "reset by lead"
	agent.ToolCalls = 0
	agent.TotalTokens = 0
	agent.UpdatedAt = time.Now().UTC()
	delete(r.inboxes, name)
	r.pushEventLocked(fmt.Sprintf("lead reset %s", name))
	return nil
}

func (r *Runtime) Permissions(name string) (canWrite, canShell bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[name]
	if !ok {
		return false, false
	}
	return agent.CanWrite, agent.CanShell
}

func (r *Runtime) Events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

func (r *Runtime) pushEventLocked(text string) {
	r.events = append(r.events, text)
	if len(r.events) > 24 {
		r.events = r.events[len(r.events)-24:]
	}
}
