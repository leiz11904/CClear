package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRollbackRestoresAndKeepsUnknownFiles(t *testing.T) {
	dir := t.TempDir()
	original := []byte("old-cclear")
	if err := os.WriteFile(filepath.Join(dir, "cclear.exe"), original, 0o644); err != nil {
		t.Fatal(err)
	}
	unknown := []byte("keep-me")
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), unknown, 0o644); err != nil {
		t.Fatal(err)
	}
	tx, err := Begin(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Stage("cclear.exe", []byte("new-cclear")); err != nil {
		t.Fatal(err)
	}
	if err := tx.Stage("Uninstall.exe", []byte("uninstaller")); err != nil {
		t.Fatal(err)
	}
	if err := tx.Stage("notes.txt", []byte("nope")); err == nil {
		t.Fatal("staging must reject unknown files")
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "cclear.exe"))
	if err != nil || string(got) != "new-cclear" {
		t.Fatalf("commit %q %v", got, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(filepath.Join(dir, "cclear.exe"))
	if err != nil || string(got) != "old-cclear" {
		t.Fatalf("rollback %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Uninstall.exe")); !os.IsNotExist(err) {
		t.Fatal("created uninstaller should be removed")
	}
	kept, err := os.ReadFile(filepath.Join(dir, "notes.txt"))
	if err != nil || string(kept) != "keep-me" {
		t.Fatal("unknown file was touched")
	}
	if _, err := os.Stat(filepath.Join(dir, ".cclear-new")); !os.IsNotExist(err) {
		t.Fatal("staging directory left behind")
	}
	if _, err := os.Stat(filepath.Join(dir, ".cclear-bak")); !os.IsNotExist(err) {
		t.Fatal("backup directory left behind")
	}
}

func TestFailedCommitLeavesPreviousInstall(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cclear.exe"), []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	tx, err := Begin(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Stage("cclear.exe", []byte("partial")); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err == nil {
		t.Fatal("closed transaction must not commit")
	}
	got, err := os.ReadFile(filepath.Join(dir, "cclear.exe"))
	if err != nil || string(got) != "stable" {
		t.Fatalf("file changed %q %v", got, err)
	}
	kept, err := os.ReadFile(filepath.Join(dir, "notes.txt"))
	if err != nil || string(kept) != "keep" {
		t.Fatal("unknown file was touched")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "cclear.exe" && entry.Name() != "notes.txt" {
			t.Fatalf("leftover %s", entry.Name())
		}
	}
}

func TestRollbackBeforeCommitRemovesNewDirectory(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "CClear")
	tx, err := Begin(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Stage("cclear.exe", []byte("fresh")); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("incomplete new directory should be removed")
	}
}

func TestBeginRecoversInterruptedReplacement(t *testing.T) {
	tests := []struct {
		name  string
		crash func(t *testing.T, dir, staging, backup string)
	}{
		{
			name: "after target moved to backup",
			crash: func(t *testing.T, dir, staging, backup string) {
				t.Helper()
				mustRename(t, filepath.Join(dir, "cclear.exe"), filepath.Join(backup, "cclear.exe"))
			},
		},
		{
			name: "after first replacement",
			crash: func(t *testing.T, dir, staging, backup string) {
				t.Helper()
				mustRename(t, filepath.Join(dir, "cclear.exe"), filepath.Join(backup, "cclear.exe"))
				mustRename(t, filepath.Join(staging, "cclear.exe"), filepath.Join(dir, "cclear.exe"))
			},
		},
		{
			name: "after both replacements before cleanup",
			crash: func(t *testing.T, dir, staging, backup string) {
				t.Helper()
				for _, name := range AllowedNames() {
					mustRename(t, filepath.Join(dir, name), filepath.Join(backup, name))
					mustRename(t, filepath.Join(staging, name), filepath.Join(dir, name))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			staging := filepath.Join(dir, ".cclear-new")
			backup := filepath.Join(dir, ".cclear-bak")
			if err := os.MkdirAll(staging, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(backup, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range AllowedNames() {
				if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(staging, name), []byte("new"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("root-unknown"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(staging, "keep.new"), []byte("staging-unknown"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(backup, "keep.bak"), []byte("backup-unknown"), 0o644); err != nil {
				t.Fatal(err)
			}

			tt.crash(t, dir, staging, backup)
			if _, err := Begin(dir); err != nil {
				t.Fatal(err)
			}

			cclear := mustRead(t, filepath.Join(dir, "cclear.exe"))
			uninstall := mustRead(t, filepath.Join(dir, "Uninstall.exe"))
			if cclear != uninstall {
				t.Fatalf("recovery mixed versions: cclear.exe=%q Uninstall.exe=%q", cclear, uninstall)
			}
			if cclear != "old" && cclear != "new" {
				t.Fatalf("recovery produced unknown version %q", cclear)
			}
			if got := mustRead(t, filepath.Join(dir, "notes.txt")); got != "root-unknown" {
				t.Fatalf("unknown root file changed: %q", got)
			}
			if got := mustRead(t, filepath.Join(staging, "keep.new")); got != "staging-unknown" {
				t.Fatalf("unknown staging file changed: %q", got)
			}
			if got := mustRead(t, filepath.Join(backup, "keep.bak")); got != "backup-unknown" {
				t.Fatalf("unknown backup file changed: %q", got)
			}
		})
	}
}

func mustRename(t *testing.T, oldPath, newPath string) {
	t.Helper()
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCloseErrorAsksForRetry(t *testing.T) {
	err := AppStillRunningError()
	if !strings.Contains(err.Error(), "请先关闭 CClear") || !strings.Contains(err.Error(), "重试") {
		t.Fatal(err)
	}
	if !strings.Contains(err.Error(), "没有改写") {
		t.Fatal(err)
	}
	if CloseTimeoutMs < 1000 || CloseTimeoutMs > 10000 {
		t.Fatalf("timeout %d", CloseTimeoutMs)
	}
	busy := ReplaceBusyError("cclear.exe")
	if !strings.Contains(busy.Error(), "重试") || !strings.Contains(busy.Error(), "撤销") {
		t.Fatal(busy)
	}
}
