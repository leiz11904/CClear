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

func TestFindScopedSkipsExcludedDirectoryAndReportsPhases(t *testing.T) {
	root := t.TempDir()
	payload := make([]byte, 80<<10)
	for i := range payload {
		payload[i] = byte(i)
	}
	for _, name := range []string{"one.dat", "two.dat", filepath.Join("skip", "three.dat")} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	phases := map[string]bool{}
	rep := FindScoped(context.Background(), []string{root}, []string{filepath.Join(root, "skip")}, func(phase string, _ int, _ string) {
		phases[phase] = true
	})
	if len(rep.Groups) != 1 || len(rep.Groups[0].Files) != 2 {
		t.Fatalf("exclude failed: %+v", rep.Groups)
	}
	for _, phase := range []string{"enumerate", "filter", "hash"} {
		if !phases[phase] {
			t.Fatalf("missing phase %s in %v", phase, phases)
		}
	}
}

func TestFindScopedCancelKeepsPartialReport(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rep := FindScoped(ctx, []string{root}, nil, nil)
	if !rep.Cancelled || rep.Note == "" {
		t.Fatalf("cancelled scan dropped its partial report: %+v", rep)
	}
}
