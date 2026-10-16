//go:build (production || dev) && windows

package main

import (
	"context"

	"cclear/internal/desk"
	"cclear/internal/work"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails binding for the space manager. Disk scans stay read-only.
// Cleanup, recycle, and uninstall stay behind their confirmation strings.
type App struct {
	ctx   context.Context
	svc   *desk.Service
	tools featureState
	gate  work.Gate
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.svc = desk.New(func(files int, where string) {
		runtime.EventsEmit(ctx, "analyze:progress", map[string]any{
			"files": files,
			"where": where,
		})
		if a.currentKind() == "analyze" {
			detail := "正在读取目录"
			if where != "" {
				detail = where
			}
			a.setPhase("磁盘占用", detail)
		}
	}, func(st desk.Status) {
		runtime.EventsEmit(ctx, "analyze:done", st)
		a.gate.ReleaseIf("analyze")
	})
	a.tools.init(ctx)
}

func (a *App) Volume() (desk.Volume, error) {
	return a.svc.Volume()
}

func (a *App) Start() error {
	ctx, err := a.acquire("analyze", "磁盘分析")
	if err != nil {
		return err
	}
	a.setPhase("磁盘占用", "正在读取目录")
	if err := a.svc.StartContext(ctx); err != nil {
		a.release()
		return err
	}
	return nil
}

func (a *App) Cancel() {
	a.gate.Cancel()
}

func (a *App) Status() desk.Status {
	return a.svc.Status()
}

func (a *App) Level(path string) desk.Level {
	return a.svc.Level(path)
}

func (a *App) Types() []desk.TypeRow {
	return a.svc.Types()
}

func (a *App) Large() []desk.FileRow {
	return a.svc.Large()
}

func (a *App) Reveal(path string) error {
	return a.svc.Reveal(path)
}
