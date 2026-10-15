package dupes

import (
	"path/filepath"
	"testing"
)

func TestCanRemoveKeepsOneCopy(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.bin")
	b := filepath.Join(root, "b.bin")
	g := []Group{{Size: 10, Files: []Item{{Path: a}, {Path: b}}}}
	if err := CanRemove(g, a); err != nil {
		t.Fatal(err)
	}
	if err := CanRemove([]Group{{Size: 10, Files: []Item{{Path: a}}}}, a); err == nil {
		t.Fatal("last copy was allowed")
	}
	if err := CanRemove(g, `C:\Windows\System32\x.dll`); err == nil {
		t.Fatal("system file allowed")
	}
	if err := CanRemove(g, filepath.Join(root, "missing.bin")); err == nil {
		t.Fatal("unknown path allowed")
	}
}

func TestCanRemoveBlocksVHDX(t *testing.T) {
	g := []Group{{Files: []Item{{Path: `C:\vm\a.vhdx`}, {Path: `C:\vm\b.vhdx`}}}}
	if err := CanRemove(g, `C:\vm\a.vhdx`); err == nil {
		t.Fatal("vhdx allowed")
	}
}
