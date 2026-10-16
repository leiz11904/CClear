package work

import (
	"context"
	"strings"
	"testing"
)

func TestBeginDoesNotCancelTheRunningTask(t *testing.T) {
	var gate Gate
	ctx, err := gate.Begin(context.Background(), "clean", "垃圾扫描")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Begin(context.Background(), "analyze", "磁盘分析"); err == nil || !strings.Contains(err.Error(), "垃圾扫描") {
		t.Fatalf("conflict error = %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("starting another task cancelled the running one")
	}
	gate.Cancel()
	if ctx.Err() == nil {
		t.Fatal("cancel did not stop the running task")
	}
	if _, err := gate.Begin(context.Background(), "dupes", "重复文件"); err == nil {
		t.Fatal("cancel released the gate before the task stored its partial result")
	}
	gate.Release()
	next, err := gate.Begin(context.Background(), "dupes", "重复文件")
	if err != nil {
		t.Fatal(err)
	}
	if next.Err() != nil {
		t.Fatal(next.Err())
	}
}

func TestReleaseIfLeavesAnotherKindRunning(t *testing.T) {
	var gate Gate
	if _, err := gate.Begin(context.Background(), "smart", "智能扫描"); err != nil {
		t.Fatal(err)
	}
	gate.ReleaseIf("analyze")
	state := gate.State()
	if !state.Running || state.Kind != "smart" {
		t.Fatalf("smart task was cleared: %+v", state)
	}
	gate.ReleaseIf("smart")
	if gate.State().Running {
		t.Fatal("smart task still running")
	}
}
