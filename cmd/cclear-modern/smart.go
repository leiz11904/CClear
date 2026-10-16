//go:build (production || dev) && windows

package main

import (
	"context"
	"path/filepath"
	"time"

	"cclear/internal/desk"
	"cclear/internal/space"
	"cclear/internal/syswin"
)

type Snapshot struct {
	Task   TaskState       `json:"task"`
	Clean  CleanScanResult `json:"clean"`
	Dupes  DuplicateResult `json:"dupes"`
	Apps   AppList         `json:"apps"`
	System SystemResult    `json:"system"`
	Smart  SmartSummary    `json:"smart"`
	Status desk.Status     `json:"status"`
}

func (a *App) Snapshot() Snapshot {
	a.tools.mu.Lock()
	clean := a.storeCleanLocked()
	dupes := duplicateResult(a.tools.dupes)
	list := appList(a.tools.apps)
	system := a.tools.system
	smart := a.tools.smart
	a.tools.mu.Unlock()
	if system.Items == nil {
		system.Items = []SystemItem{}
	}
	status := desk.Status{}
	if a.svc != nil {
		status = a.svc.Status()
	}
	return Snapshot{Task: a.Task(), Clean: clean, Dupes: dupes, Apps: list, System: system, Smart: smart, Status: status}
}

func (a *App) storeCleanLocked() CleanScanResult {
	result := CleanScanResult{Items: cleanItems(a.tools.clean), Cancelled: a.tools.cleanCancelled}
	if result.Cancelled {
		result.Note = "扫描已取消，以下为已完成的部分结果。"
	}
	if result.Items == nil {
		result.Items = []CleanItem{}
	}
	return result
}

func (a *App) StartSmart() error {
	ctx, err := a.acquire("smart", "智能扫描")
	if err != nil {
		return err
	}
	go func() {
		defer a.release()
		summary := a.runSmart(ctx)
		a.tools.mu.Lock()
		a.tools.smart = summary
		a.tools.mu.Unlock()
		a.emit("task:done", summary)
	}()
	return nil
}

func (a *App) runSmart(ctx context.Context) SmartSummary {
	summary := SmartSummary{When: time.Now().Format("2006-01-02 15:04"), Completed: []string{}}
	if ctx.Err() != nil {
		summary.Cancelled = true
		summary.Note = "智能扫描已取消，已完成的阶段结果已保留。"
		return summary
	}

	a.setPhase("垃圾候选项", "正在统计可清理候选项")
	clean := a.scanClean(ctx)
	summary.CleanCount = len(clean.Items)
	for _, item := range clean.Items {
		if item.Bytes > 0 {
			summary.CleanBytes += item.Bytes
		}
	}
	summary.Completed = append(summary.Completed, "垃圾候选项")
	if clean.Cancelled || ctx.Err() != nil {
		return cancelledSmart(summary)
	}

	a.setPhase("磁盘占用", "正在读取目录占用")
	drive := syswin.SystemDrive()
	rep := space.Scan(ctx, drive+`\`, func(files int, where string) {
		detail := "正在读取目录占用"
		if where != "" {
			detail = filepath.Base(where)
		}
		a.setPhase("磁盘占用", detail)
		a.emit("analyze:progress", map[string]any{"files": files, "where": where})
	})
	if a.svc != nil {
		status := a.svc.Store(rep)
		summary.DiskAllocated = status.Allocated
		summary.DiskFiles = status.Files
		a.emit("analyze:done", status)
		for _, file := range a.svc.Large() {
			summary.LargeBytes += file.Allocated
			summary.LargeCount++
		}
	}
	summary.Completed = append(summary.Completed, "磁盘占用")
	if rep.Cancelled || ctx.Err() != nil {
		return cancelledSmart(summary)
	}

	a.setPhase("重复文件", "枚举")
	dupes := a.findDupes(ctx, DuplicateQuery{Roots: []string{drive + `\`}})
	summary.DupeBytes = dupes.Reclaimable
	summary.DupeGroups = len(dupes.Groups)
	summary.Completed = append(summary.Completed, "重复文件")
	if dupes.Cancelled || ctx.Err() != nil {
		return cancelledSmart(summary)
	}

	a.setPhase("软件信息", "正在读取卸载登记")
	list := a.loadApps(ctx)
	summary.AppCount = len(list.Apps)
	summary.Completed = append(summary.Completed, "软件信息")
	if list.Cancelled || ctx.Err() != nil {
		return cancelledSmart(summary)
	}

	a.setPhase("系统与开发环境", "正在读取占用")
	system := a.readSystem(ctx)
	summary.SystemCount = len(system.Items)
	summary.Completed = append(summary.Completed, "系统与开发环境")
	if ctx.Err() != nil {
		return cancelledSmart(summary)
	}
	summary.Note = "扫描完成。选择一个模块继续处理，不会自动清理。"
	return summary
}

func cancelledSmart(summary SmartSummary) SmartSummary {
	summary.Cancelled = true
	summary.Note = "智能扫描已取消，已完成的阶段结果已保留。"
	return summary
}
