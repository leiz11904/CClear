package devspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanFindsCacheAndDoesNotDelete(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, "AppData", "Local", "npm-cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "x"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(home, "AppData", "Local", "Packages", "CanonicalGroupLimited.Ubuntu_demo", "LocalState")
	if err := os.MkdirAll(disk, 0o755); err != nil {
		t.Fatal(err)
	}
	vhdx := filepath.Join(disk, "ext4.vhdx")
	if err := os.WriteFile(vhdx, make([]byte, 32), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := Scan(context.Background(), Env{
		UserProfile:  home,
		LocalAppData: filepath.Join(home, "AppData", "Local"),
		UserHome:     home,
	})
	if !strings.Contains(rep.Note, "不会删除") {
		t.Fatal(rep.Note)
	}
	var foundCache, foundDisk bool
	for _, item := range rep.Items {
		if item.ID == "npm" && item.Kind == "缓存" && item.Bytes >= 5 {
			foundCache = true
		}
		if strings.HasPrefix(item.ID, "wsl-") && item.Kind == "虚拟磁盘" && strings.Contains(item.Note, "禁止直接删除") {
			foundDisk = true
		}
	}
	if !foundCache || !foundDisk {
		t.Fatalf("%+v", rep.Items)
	}
	if _, err := os.Stat(vhdx); err != nil {
		t.Fatal("vhdx was removed")
	}
}
