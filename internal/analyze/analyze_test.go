package analyze

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanDoesNotTreatUserFilesAsJunk(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "Documents")
	if err := os.Mkdir(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	photo := filepath.Join(docs, "family.jpg")
	payload := make([]byte, 60<<20)
	if err := os.WriteFile(photo, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(root, "AppData", "Local", "Temp")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "tiny.tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := Scan(context.Background(), Env{
		Drive:        root,
		UserProfile:  root,
		LocalAppData: filepath.Join(root, "AppData", "Local"),
	}, nil)
	if rep.Buckets[0].Bytes < 60<<20 {
		t.Fatalf("user data bytes = %d", rep.Buckets[0].Bytes)
	}
	if !strings.Contains(rep.Buckets[0].Note, "不是垃圾") {
		t.Fatal(rep.Buckets[0].Note)
	}
	if len(rep.Large) != 1 || !strings.Contains(rep.Large[0].Path, "family.jpg") {
		t.Fatalf("large = %+v", rep.Large)
	}
	if strings.Contains(strings.ToLower(rep.Note), "删除") && !strings.Contains(rep.Note, "不会") {
		t.Fatal(rep.Note)
	}
}

func TestScanSkipsSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.bin")
	if err := os.WriteFile(secret, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	docs := filepath.Join(root, "Documents")
	if err := os.Mkdir(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(docs, "link")); err != nil {
		t.Skip("NOT RUN: symlink creation failed: " + err.Error())
	}
	rep := Scan(context.Background(), Env{UserProfile: root}, nil)
	for _, file := range rep.Large {
		if strings.Contains(file.Path, "secret") {
			t.Fatal("followed symlink")
		}
	}
}

func TestScanCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rep := Scan(ctx, Env{UserProfile: t.TempDir()}, nil)
	if !rep.Cancelled {
		t.Fatal("cancel was ignored")
	}
}
