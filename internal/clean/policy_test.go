package clean

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchIdentityAndHardLinks(t *testing.T) {
	if !MatchIdentity(10, 100, 10, 101) {
		t.Fatal("a one-second timestamp difference should be tolerated")
	}
	if MatchIdentity(10, 100, 11, 100) {
		t.Fatal("a file that changed size after the scan was accepted")
	}
	if Reclaimable(100, 80, 2) != 0 {
		t.Fatal("a hard link was counted as free space")
	}
	if Reclaimable(100, 80, 1) != 80 {
		t.Fatal("allocated size was ignored")
	}
}

func TestUpdateAndOfficialCommands(t *testing.T) {
	if !UpdateShouldSkip(true, false) || !UpdateShouldSkip(false, true) {
		t.Fatal("busy Windows Update should be skipped")
	}
	if UpdateShouldSkip(false, false) {
		t.Fatal("an idle update service should not block cache cleanup")
	}
	if !UpdateServiceBusy(true, true) || !UpdateServiceBusy(false, false) {
		t.Fatal("a failed service query or a running service must block cleanup")
	}
	if UpdateServiceBusy(false, true) {
		t.Fatal("a stopped update service should not block by itself")
	}
	files := make([]FileHit, DetailPageSize+3)
	view, pages, page := DetailPage(files, 1)
	if pages != 2 || page != 1 || len(view) != 3 {
		t.Fatalf("pages %d page %d len %d", pages, page, len(view))
	}
	for _, phase := range []string{"analyze", "cleanup"} {
		args := strings.Join(DISMArgs(phase), " ")
		if strings.Contains(strings.ToLower(args), "resetbase") {
			t.Fatalf("DISM %s includes ResetBase: %s", phase, args)
		}
	}
	if len(DISMArgs("analyze")) == 0 || len(DISMArgs("cleanup")) == 0 {
		t.Fatal("missing DISM command")
	}
	handler, value, args := OldWindowsPlan()
	if handler != "Previous Installations" || value != "StateFlags64721" || strings.Join(args, " ") != "/sagerun:64721" {
		t.Fatalf("old windows plan = %s %s %v", handler, value, args)
	}
	if strings.Contains(strings.ToLower(strings.Join(args, " ")), "rmdir") {
		t.Fatal("Windows.old plan deletes recursively")
	}
}

func TestRedactAndLogPrune(t *testing.T) {
	got := RedactPath(`C:\Users\ada\AppData\Local\Temp\cache.tmp`)
	if strings.Contains(got, "ada") || !strings.Contains(got, `Users\~\`) {
		t.Fatalf("redact = %s", got)
	}
	line := RedactText(`deleted C:\Users\ada\AppData\Local\pip\log.txt and C:/Users/ada/go/mod`)
	if strings.Contains(line, "ada") {
		t.Fatalf("line still has name: %s", line)
	}
	dir := t.TempDir()
	for _, name := range []string{"20200101.log", "20200102.log", "20200103.log"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("1234567890"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := PruneLogs(dir, 15); err != nil {
		t.Fatal(err)
	}
	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0].Name() != "20200103.log" {
		t.Fatalf("kept %#v", left)
	}
	raw := `CClear 1.0.0\r\nC:\Users\ada\AppData\Local\Temp\a.log\r\n`
	if err := os.WriteFile(filepath.Join(dir, "old.log"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SanitizeLogDir(dir); err != nil {
		t.Fatal(err)
	}
	rewritten, err := os.ReadFile(filepath.Join(dir, "old.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rewritten), "ada") {
		t.Fatalf("%s", rewritten)
	}
}

func TestSpaceNote(t *testing.T) {
	if !strings.Contains(SpaceNote(), "可能") {
		t.Fatal(SpaceNote())
	}
}
