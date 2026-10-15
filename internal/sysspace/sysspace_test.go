package sysspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDoesNotDelete(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "pagefile.sys")
	if err := os.WriteFile(page, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := Read(root)
	if !strings.Contains(rep.Note, "不会删除") {
		t.Fatal(rep.Note)
	}
	if _, err := os.Stat(page); err != nil {
		t.Fatal("pagefile was removed")
	}
	found := false
	for _, item := range rep.Items {
		if strings.Contains(item.Path, "pagefile.sys") && item.Bytes == 4 {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", rep.Items)
	}
	reserved := false
	for _, item := range rep.Items {
		if item.Name == "系统保留存储" {
			reserved = true
			if item.Known || item.Bytes != 0 {
				t.Fatal("reserved storage was invented")
			}
		}
	}
	if !reserved {
		t.Fatal("reserved storage row missing")
	}
}
