package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSummarizesRedactedLog(t *testing.T) {
	dir := t.TempDir()
	body := "CClear 1.1.0\r\n任务 2026-10-14T08:00:00Z\r\n\r\n---- 用户临时文件 (user-temp) ----\r\nC:\\Users\\~\\AppData\\Local\\Temp\\a.tmp\r\n逻辑大小 12 KB，预计占用 16 KB，删除 3，跳过 1，失败 0\r\n"
	if err := os.WriteFile(filepath.Join(dir, "20261014-080000.log"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := Load(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("%d", len(list))
	}
	rec := list[0]
	if rec.Deleted != 3 || rec.Skipped != 1 || rec.Failed != 0 {
		t.Fatalf("%+v", rec)
	}
	if rec.Logical != "12 KB" || rec.Estimated != "16 KB" {
		t.Fatalf("%q %q", rec.Logical, rec.Estimated)
	}
	if len(rec.Items) != 1 || rec.Items[0] != "用户临时文件" {
		t.Fatalf("%v", rec.Items)
	}
	if rec.Version != "1.1.0" {
		t.Fatalf("version %s", rec.Version)
	}
}

func TestLoadFormat2Log(t *testing.T) {
	dir := t.TempDir()
	body := "CClear 2.4.0\r\nFormat 2\r\n任务 2026-10-14T09:00:00Z\r\n\r\n---- 用户临时文件 (user-temp) ----\r\nC:\\Users\\~\\AppData\\Local\\Temp\\a.tmp\r\n逻辑大小 12 KB，预计占用 16 KB，删除 2，跳过 0，失败 0\r\n"
	if err := os.WriteFile(filepath.Join(dir, "20261014-090000.log"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := Load(dir, 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %d", err, len(list))
	}
	if list[0].Version != "2.4.0" || list[0].Deleted != 2 {
		t.Fatalf("%+v", list[0])
	}
}

func TestLoadMissingDir(t *testing.T) {
	list, err := Load(filepath.Join(t.TempDir(), "nope"), 5)
	if err != nil || list != nil {
		t.Fatalf("%v %v", list, err)
	}
}
