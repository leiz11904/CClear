package space

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanHardLinkCountedOnce(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.bin")
	b := filepath.Join(root, "b.bin")
	if err := os.WriteFile(a, []byte("12345678"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(a, b); err != nil {
		t.Fatal(err)
	}
	rep := Scan(context.Background(), root, nil)
	if rep.HardLinks < 1 {
		t.Fatalf("hard links = %d", rep.HardLinks)
	}
	if rep.Allocated >= 16 && rep.Logical >= 16 {
		t.Fatalf("hard link space was doubled: logical %d allocated %d", rep.Logical, rep.Allocated)
	}
	if HighRiskDelete(`/Windows/System32/drivers/etc/hosts`) == false {
		t.Fatal("system path was allowed")
	}
	if HighRiskDelete(filepath.Join(root, "a.bin")) {
		t.Fatal("isolated test file treated as system")
	}
	if !HighRiskDelete(filepath.Join(root, "disk.vhdx")) || !HighRiskDelete(filepath.Join(root, "db.mdf")) {
		t.Fatal("virtual disk or database was allowed")
	}
}

func TestScanSkipsSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.bin")
	if err := os.WriteFile(secret, make([]byte, 200), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip("NOT RUN: symlink creation failed: " + err.Error())
	}
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := Scan(context.Background(), root, nil)
	if strings.Contains(rep.Note, "垃圾删除") == false {
		t.Fatal(rep.Note)
	}
	for _, file := range append(rep.TopFiles, rep.Large...) {
		if strings.Contains(file.Path, "secret") {
			t.Fatal("followed symlink")
		}
	}
}

func TestFilterAndTreemap(t *testing.T) {
	files := []File{{Allocated: 200 << 20, Logical: 200 << 20}, {Allocated: 50 << 20}}
	got := FilterLarge(files, 100<<20)
	if len(got) != 1 {
		t.Fatalf("%d", len(got))
	}
	SortFiles(got, "size")
	nodes := []*Node{{Name: "a", Path: "a", Allocated: 75}, {Name: "b", Path: "b", Allocated: 25}}
	cells := LayoutTreemap(nodes, 0, 0, 100, 50)
	if len(cells) != 2 {
		t.Fatalf("%d", len(cells))
	}
	area := 0.0
	for _, c := range cells {
		area += c.W * c.H
	}
	if area < 100*50-1 || area > 100*50+1 {
		t.Fatalf("area %f", area)
	}
	if HitCell(cells, cells[0].X+0.1, cells[0].Y+0.1) == nil {
		t.Fatal("hit missed")
	}
}

func TestScanCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rep := Scan(ctx, t.TempDir(), nil)
	if !rep.Cancelled && ctx.Err() == nil {
		t.Fatal("expected cancel or empty")
	}
}

func TestMillionFilesScanNotRunByDefault(t *testing.T) {
	if os.Getenv("CCLEAR_STRESS") != "1" {
		t.Skip("百万文件压力测试未执行。设置 CCLEAR_STRESS=1 才会生成并扫描。")
	}
	root := t.TempDir()
	const n = 1_000_000
	for i := 0; i < n; i++ {
		if i%10000 == 0 {
			if err := os.MkdirAll(filepath.Join(root, "d"+itoa(int64(i/10000))), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		name := filepath.Join(root, "d"+itoa(int64(i/10000)), "f"+itoa(int64(i))+".tmp")
		if err := os.WriteFile(name, []byte("1234"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	rep := Scan(context.Background(), root, nil)
	if rep.Files < n {
		t.Fatalf("files %d", rep.Files)
	}
	t.Logf("scanned %d files in %s", rep.Files, time.Since(start))
}

func TestManyFilesScan(t *testing.T) {
	root := t.TempDir()
	const n = 4000
	for i := 0; i < n; i++ {
		name := filepath.Join(root, "f"+itoa(int64(i))+".tmp")
		if err := os.WriteFile(name, []byte("1234"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	rep := Scan(context.Background(), root, nil)
	elapsed := time.Since(start)
	if rep.Files < n {
		t.Fatalf("files %d", rep.Files)
	}
	if elapsed > 30*time.Second {
		t.Fatalf("scan too slow: %s", elapsed)
	}
	t.Logf("scanned %d files in %s", rep.Files, elapsed)
}
