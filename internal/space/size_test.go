package space

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseSizeMB(t *testing.T) {
	got, err := ParseSizeMB("250")
	if err != nil || got != 250<<20 {
		t.Fatalf("%d %v", got, err)
	}
	if _, err := ParseSizeMB("0"); err == nil {
		t.Fatal("zero accepted")
	}
	if _, err := ParseSizeMB("abc"); err == nil {
		t.Fatal("text accepted")
	}
}

func TestScanKeepsBoundedFileList(t *testing.T) {
	root := t.TempDir()
	const n = 1200
	for i := 0; i < n; i++ {
		name := filepath.Join(root, "f"+itoa(int64(i))+".bin")
		if err := os.WriteFile(name, []byte("1234"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rep := Scan(context.Background(), root, nil)
	if rep.Files < n {
		t.Fatalf("files %d", rep.Files)
	}
	if len(rep.Large) > maxLargeKeep {
		t.Fatalf("kept %d files", len(rep.Large))
	}
	if len(rep.TopFiles) > maxTopFiles {
		t.Fatalf("top %d", len(rep.TopFiles))
	}
}

func TestLogicalIsNotAllocated(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.bin")
	if err := os.WriteFile(path, make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := Scan(context.Background(), root, nil)
	if rep.Logical <= 0 {
		t.Fatal("missing logical size")
	}
	if rep.Allocated <= 0 {
		t.Fatal("missing allocated size")
	}
}

func TestRenderUsesScanData(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "big"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "big", "one.bin"), make([]byte, 32<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "two.bin"), make([]byte, 8<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := Scan(context.Background(), root, nil)
	img := RenderUIPreview(rep, 100<<20, 40<<20)
	if img.Bounds().Dx() < 1000 {
		t.Fatal("image too small")
	}
	out := filepath.Join(t.TempDir(), "preview.png")
	if err := WritePNG(out, img); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(out)
	if err != nil || info.Size() < 100 {
		t.Fatal("png missing")
	}
}

func TestHeapDoesNotGrowUnbounded(t *testing.T) {
	h := fileHeap{}
	for i := 0; i < 5000; i++ {
		keepLargest(&h, File{Allocated: int64(i + 1), Name: itoa(int64(i))}, 400)
	}
	if h.Len() != 400 {
		t.Fatalf("len %d", h.Len())
	}
	out := dumpLargest(h)
	if out[0].Allocated < out[len(out)-1].Allocated {
		t.Fatal("not sorted desc")
	}
}

func TestScanDoesNotDelete(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = Scan(context.Background(), root, nil)
	if _, err := os.Stat(path); err != nil {
		t.Fatal("scan deleted a file")
	}
}

func TestHighRiskCoversVirtualDisks(t *testing.T) {
	if !HighRiskDelete(`C:\Users\a\AppData\Local\Docker\wsl\disk.vhdx`) {
		t.Fatal("docker vhdx allowed")
	}
	if !HighRiskDelete(`C:\Users\a\ext4.vhdx`) {
		t.Fatal("wsl vhdx allowed")
	}
}

func TestMemBoundedScan(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	root := t.TempDir()
	const n = 8000
	for i := 0; i < n; i++ {
		dir := filepath.Join(root, "d"+itoa(int64(i/500)))
		_ = os.MkdirAll(dir, 0o755)
		if err := os.WriteFile(filepath.Join(dir, "f"+itoa(int64(i))), []byte("abcd"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	rep := Scan(context.Background(), root, nil)
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if rep.Files < n {
		t.Fatalf("files %d", rep.Files)
	}
	if len(rep.Large) > maxLargeKeep {
		t.Fatalf("kept %d", len(rep.Large))
	}
	used := after.HeapAlloc
	t.Logf("scanned %d files; heap alloc %d", rep.Files, used)
	if used > 256<<20 {
		t.Fatalf("scan heap grew without bound: %d", used)
	}
}
