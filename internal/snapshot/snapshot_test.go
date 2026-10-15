package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cclear/internal/space"
)

func TestSaveCompareAndPrune(t *testing.T) {
	base := t.TempDir()
	older := Record{When: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Free: 10, Folders: []space.Dir{{Name: "Downloads", Allocated: 1 << 30}, {Name: "AppData", Allocated: 2 << 30}}}
	newer := Record{When: time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), Free: 8, Folders: []space.Dir{{Name: "Downloads", Allocated: 9 << 30}, {Name: "AppData", Allocated: 5 << 30}}}
	if err := Save(base, older); err != nil {
		t.Fatal(err)
	}
	if err := Save(base, newer); err != nil {
		t.Fatal(err)
	}
	list, err := Load(base)
	if err != nil || len(list) != 2 {
		t.Fatalf("%v %d", err, len(list))
	}
	diff := Compare(older, newer)
	if len(diff) != 2 || diff[0].Name != "Downloads" || diff[0].Delta != 8<<30 {
		t.Fatalf("%+v", diff)
	}
}

func TestRedactsPaths(t *testing.T) {
	base := t.TempDir()
	rec := Record{When: time.Now(), Folders: []space.Dir{{Name: "Temp", Path: `C:\Users\ada\AppData\Local\Temp`, Allocated: 10}}}
	if err := Save(base, rec); err != nil {
		t.Fatal(err)
	}
	list, err := Load(base)
	if err != nil || len(list) != 1 {
		t.Fatal(err)
	}
	if list[0].Folders[0].Path != "" {
		t.Fatalf("path stored: %s", list[0].Folders[0].Path)
	}
	if strings.Contains(list[0].Folders[0].Name, "ada") {
		t.Fatalf("username stored: %s", list[0].Folders[0].Name)
	}
}

func TestSanitizeDirRewritesOldSnapshots(t *testing.T) {
	base := t.TempDir()
	dir := Dir(base)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"when":"2026-10-01T00:00:00Z","folders":[{"Name":"C:\\Users\\ada\\AppData\\Local\\models","Path":"C:\\Users\\ada\\AppData\\Local\\models","Allocated":10}]}`)
	if err := os.WriteFile(filepath.Join(dir, "old.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SanitizeDir(base); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "old.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "ada") || strings.Contains(string(got), `C:\\Users`) {
		t.Fatalf("old snapshot still has identity: %s", got)
	}
}
