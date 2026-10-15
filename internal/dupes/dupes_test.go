package dupes

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFindGroupsByContentNotName(t *testing.T) {
	root := t.TempDir()
	payload := make([]byte, 80<<10)
	for i := range payload {
		payload[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(root, "one.dat"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "two.dat"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "other.dat"), append([]byte("x"), payload[1:]...), 0o644); err != nil {
		t.Fatal(err)
	}
	sameName := filepath.Join(root, "sub")
	if err := os.Mkdir(sameName, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sameName, "one.dat"), []byte("not-dup-content-and-still-long-enough-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := Find(context.Background(), root, nil)
	if len(rep.Groups) != 1 || len(rep.Groups[0].Files) != 2 {
		t.Fatalf("%+v", rep.Groups)
	}
	if rep.Note == "" {
		t.Fatal("missing safety note")
	}
}

func TestHardLinkIsNotTwoCopies(t *testing.T) {
	root := t.TempDir()
	payload := make([]byte, 80<<10)
	a := filepath.Join(root, "a.dat")
	if err := os.WriteFile(a, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(a, filepath.Join(root, "b.dat")); err != nil {
		t.Fatal(err)
	}
	rep := Find(context.Background(), root, nil)
	if len(rep.Groups) != 0 {
		t.Fatalf("hard links reported as duplicates: %+v", rep.Groups)
	}
}
