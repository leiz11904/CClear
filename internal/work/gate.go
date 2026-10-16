package work

import (
	"context"
	"fmt"
	"sync"
)

// State is the single heavy task the product is allowed to run.
type State struct {
	Running bool
	Kind    string
	Label   string
	Phase   string
	Detail  string
}

// Gate rejects a second heavy task instead of cancelling the one already running.
type Gate struct {
	mu     sync.Mutex
	kind   string
	label  string
	phase  string
	detail string
	cancel context.CancelFunc
}

func (g *Gate) Begin(parent context.Context, kind, label string) (context.Context, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancel != nil {
		return nil, fmt.Errorf("正在执行%s。请等待完成，或先取消当前任务", g.label)
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	g.kind = kind
	g.label = label
	g.phase = "准备"
	g.detail = ""
	g.cancel = cancel
	return ctx, nil
}

// Cancel asks the running task to stop. The task keeps the gate until Release,
// so its partial result can be stored before another task starts.
func (g *Gate) Cancel() {
	g.mu.Lock()
	cancel := g.cancel
	g.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (g *Gate) Release() {
	g.release("")
}

func (g *Gate) ReleaseIf(kind string) {
	g.release(kind)
}

func (g *Gate) release(kind string) {
	g.mu.Lock()
	if kind != "" && g.kind != kind {
		g.mu.Unlock()
		return
	}
	cancel := g.cancel
	g.cancel = nil
	g.kind = ""
	g.label = ""
	g.phase = ""
	g.detail = ""
	g.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (g *Gate) Set(phase, detail string) {
	g.mu.Lock()
	g.phase = phase
	g.detail = detail
	g.mu.Unlock()
}

func (g *Gate) State() State {
	g.mu.Lock()
	defer g.mu.Unlock()
	return State{
		Running: g.cancel != nil,
		Kind:    g.kind,
		Label:   g.label,
		Phase:   g.phase,
		Detail:  g.detail,
	}
}
