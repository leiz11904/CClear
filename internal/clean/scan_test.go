package clean

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cclear/internal/winpath"
)

type osChecker struct{}

func (osChecker) ValidateRoot(root string) error {
	if !filepath.IsAbs(root) {
		return winpath.ErrNotAbs
	}
	return nil
}

func (osChecker) ValidateDelete(path, root string, isDir bool) error {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return winpath.ErrOutside
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return winpath.ErrOutside
	}
	if rel == "." && isDir {
		return winpath.ErrRoot
	}
	return nil
}

func TestScanSkipsSymlinksAndRecentFiles(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("keep-me"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(root, "old.tmp")
	if err := os.WriteFile(old, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	recent := filepath.Join(root, "new.tmp")
	if err := os.WriteFile(recent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "sub")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	nestedOld := filepath.Join(nested, "nested.tmp")
	if err := os.WriteFile(nestedOld, []byte("abcd"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "link.tmp")); err != nil {
		t.Skip("NOT RUN: symlink creation failed: " + err.Error())
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked-dir")); err != nil {
		t.Skip("NOT RUN: symlink creation failed: " + err.Error())
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	for _, path := range []string{old, nestedOld} {
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ScanRoot(context.Background(), root, osChecker{}, ScanOptions{
		Now:    time.Now(),
		MinAge: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 2 {
		t.Fatalf("files = %+v, want 2", got.Files)
	}
	if got.SkippedRecent != 1 {
		t.Fatalf("skipped recent = %d", got.SkippedRecent)
	}
	for _, file := range got.Files {
		if strings.Contains(file.Path, "secret") || strings.Contains(file.Path, outside) {
			t.Fatalf("followed symlink to %s", file.Path)
		}
	}

	stats := Delete(context.Background(), got.Files, got.Dirs, OSDeleter(osChecker{}), nil)
	if stats.Failed != 0 {
		t.Fatal(stats.Errors)
	}
	if stats.Freed != 8 {
		t.Fatalf("freed %d", stats.Freed)
	}
	if _, err := os.Stat(secret); err != nil {
		t.Fatal("symlink target was deleted")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatal("recent file was deleted")
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old file still exists")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatal("scan root was removed")
	}
}

func TestDeleteSkipsChangedFileAndHardLink(t *testing.T) {
	root := t.TempDir()
	changed := filepath.Join(root, "changed.tmp")
	if err := os.WriteFile(changed, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(root, "linked.tmp")
	if err := os.WriteFile(linked, []byte("abcd"), 0o644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias.tmp")
	if err := os.Link(linked, alias); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	for _, path := range []string{changed, linked, alias} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ScanRoot(context.Background(), root, osChecker{}, ScanOptions{Now: time.Now(), MinAge: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 3 || got.Logical != 12 {
		t.Fatalf("count %d logical %d", got.Count, got.Logical)
	}
	var hard int
	for _, file := range got.Files {
		if file.Links > 1 {
			hard++
			if Reclaimable(file.Size, file.Allocated, file.Links) != 0 {
				t.Fatal("hard link counted as free space")
			}
		}
	}
	if hard != 2 {
		t.Fatalf("hard links = %d", hard)
	}
	if err := os.WriteFile(changed, []byte("changed-after-scan"), 0o644); err != nil {
		t.Fatal(err)
	}
	stats := Delete(context.Background(), got.Files, nil, OSDeleter(osChecker{}), nil)
	if _, err := os.Stat(changed); err != nil {
		t.Fatal("changed file was deleted")
	}
	if _, err := os.Stat(linked); err != nil {
		t.Fatal("hard link was deleted")
	}
	if stats.Deleted != 0 {
		t.Fatalf("deleted %d", stats.Deleted)
	}
}

func TestScanCanLimitStoredFiles(t *testing.T) {
	root := t.TempDir()
	old := time.Now().Add(-48 * time.Hour)
	for _, name := range []string{"a.tmp", "b.tmp"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("1234"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ScanRoot(context.Background(), root, osChecker{}, ScanOptions{Now: time.Now(), MaxFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 1 || got.Count != 2 || got.Logical != 8 {
		t.Fatalf("stored %d count %d logical %d", len(got.Files), got.Count, got.Logical)
	}
}

func TestRunningBrowserSkipsCache(t *testing.T) {
	previous := ProcessRunning
	ProcessRunning = func(name string) bool { return name == "chrome.exe" }
	t.Cleanup(func() { ProcessRunning = previous })
	root := t.TempDir()
	report := ScanCategory(context.Background(), Category{
		ID:            "browser-chrome",
		SkipProcesses: []string{"chrome.exe"},
		Roots:         []string{root},
	}, "C:", osChecker{}, nil)
	if report.Count != 0 || report.Note == "" {
		t.Fatalf("running browser was scanned: %+v", report)
	}
}

func TestScanRefusesSymlinkedRoot(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	link := filepath.Join(parent, "root-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("NOT RUN: symlink creation failed: " + err.Error())
	}
	_, err := ScanRoot(context.Background(), link, osChecker{}, ScanOptions{})
	if err == nil {
		t.Fatal("symlinked root was scanned")
	}
	if _, err := os.Stat(secret); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyIdentityIsNotAMatch(t *testing.T) {
	var a, b Identity
	if a.Same(b) {
		t.Fatal("an unconfirmed identity matched another")
	}
	root := t.TempDir()
	path := filepath.Join(root, "a.tmp")
	if err := os.WriteFile(path, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-96 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	got, err := ScanRoot(context.Background(), root, osChecker{}, ScanOptions{Now: time.Now(), MinAge: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 1 || !got.Files[0].ID.Valid {
		t.Fatalf("snapshot = %+v", got.Files)
	}
	got.Files[0].ID = Identity{}
	stats := Delete(context.Background(), got.Files, nil, OSDeleter(osChecker{}), nil)
	if stats.Deleted != 0 {
		t.Fatalf("deleted without identity: %+v", stats)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("file was deleted without an identity")
	}
}

func TestCleanCancelKeepsFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.tmp")
	if err := os.WriteFile(path, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-96 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	got, err := ScanRoot(context.Background(), root, osChecker{}, ScanOptions{Now: time.Now(), MinAge: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stats := Delete(ctx, got.Files, got.Dirs, OSDeleter(osChecker{}), nil)
	if stats.Deleted != 0 {
		t.Fatalf("cancel deleted %+v", stats)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestPreviewCapRewalksAndSkipsReplacement(t *testing.T) {
	root := t.TempDir()
	old := time.Now().Add(-96 * time.Hour)
	for i := 0; i < PreviewLimit+5; i++ {
		path := filepath.Join(root, "f"+string(rune('a'+(i%26)))+string(rune('a'+((i/26)%26)))+string(rune('0'+(i%10))))
		if err := os.WriteFile(path, []byte("1234"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	report := ScanCategory(context.Background(), Category{ID: "batch", Roots: []string{root}, MinAge: time.Hour}, "C:", osChecker{}, nil)
	if !report.Truncated || len(report.Files) != PreviewLimit || report.Count != PreviewLimit+5 {
		t.Fatalf("stored %d count %d truncated %v", len(report.Files), report.Count, report.Truncated)
	}
	changed := report.Files[0].Path
	if err := os.WriteFile(changed, []byte("replaced-contents"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(changed, old, old); err != nil {
		t.Fatal(err)
	}
	prevCheck := PathChecker
	prevDelete := SecureDelete
	PathChecker = func(string) Checker { return osChecker{} }
	SecureDelete = func(path, root, drive string, isDir bool, id Identity, size int64) error {
		return OSDeleter(osChecker{})(path, root, isDir, id, size)
	}
	t.Cleanup(func() {
		PathChecker = prevCheck
		SecureDelete = prevDelete
	})
	stats := CleanCategory(context.Background(), report, "C:", nil, nil)
	if _, err := os.Stat(changed); err != nil {
		t.Fatal("replaced file was deleted")
	}
	if stats.Deleted != PreviewLimit+4 || stats.Failed != 0 {
		t.Fatalf("deleted %d failed %d errors %v", stats.Deleted, stats.Failed, stats.Errors)
	}
}

func TestUpdateCleanupDoesNotStopServices(t *testing.T) {
	prevPause := PauseWindowsUpdate
	prevBusy := WindowsUpdateBusy
	prevDelete := SecureDelete
	t.Cleanup(func() {
		PauseWindowsUpdate = prevPause
		WindowsUpdateBusy = prevBusy
		SecureDelete = prevDelete
	})
	called := false
	PauseWindowsUpdate = func() (func(), error) {
		called = true
		return func() {}, nil
	}
	WindowsUpdateBusy = func() bool { return false }
	SecureDelete = func(path, root, drive string, isDir bool, id Identity, size int64) error {
		return nil
	}
	report := Report{Category: Category{Special: "winupdate"}, Files: []FileHit{{
		Path: "unused", Size: 1, Links: 1, ID: Identity{Valid: true, Volume: 1, File: [16]byte{1}},
	}}}
	stats := CleanCategory(context.Background(), report, "C:", nil, nil)
	if called {
		t.Fatal("cleanup stopped Windows Update")
	}
	if stats.Deleted != 1 || stats.Failed != 0 {
		t.Fatalf("%+v", stats)
	}
	WindowsUpdateBusy = func() bool { return true }
	busy := CleanCategory(context.Background(), Report{Category: Category{Special: "dism"}}, "C:", nil, nil)
	if busy.Deleted != 0 || busy.Skipped == 0 {
		t.Fatalf("busy update was cleaned: %+v", busy)
	}
}
