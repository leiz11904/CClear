//go:build windows

package clean

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestNativeJunctionIsNotFollowed(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "junction")
	if err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).Run(); err != nil {
		t.Skip("NOT RUN: mklink /J failed: " + err.Error())
	}
	if _, err := ReadSnapshot(filepath.Join(link, "secret.txt"), false); err == nil {
		t.Fatal("snapshot followed a junction")
	}
	if _, err := os.Stat(secret); err != nil {
		t.Fatal(err)
	}
}

func TestNativeSymlinkIsNotFollowed(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Skip("NOT RUN: symlink creation failed: " + err.Error())
	}
	if _, err := ReadSnapshot(link, false); err == nil {
		t.Fatal("snapshot followed a symbolic link")
	}
}

func TestNativeMountPoint(t *testing.T) {
	t.Skip("NOT RUN: creating a volume mount point needs a spare volume and is not attempted")
}

func TestNativeLockedFileIsNotOpenedExclusively(t *testing.T) {
	path := filepath.Join(t.TempDir(), "busy.tmp")
	if err := os.WriteFile(path, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	held, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	handle, err := openVerified(path, false, true)
	if err == nil {
		windows.CloseHandle(handle)
		t.Fatal("exclusive open succeeded while the file was held")
	}
}

func TestNativeReplacementChangesFileID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.tmp")
	if err := os.WriteFile(path, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := ReadSnapshot(path, false)
	if err != nil || !before.ID.Valid {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := ReadSnapshot(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if before.ID.Same(after.ID) {
		t.Fatal("replaced file kept the same identity")
	}
}

func TestNativeDeleteUsesTheVerifiedHandle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone.tmp")
	if err := os.WriteFile(path, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := ReadSnapshot(path, false)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := openVerified(path, false, true)
	if err != nil {
		t.Fatal(err)
	}
	live, err := snapshotHandle(handle)
	if err != nil || !snap.ID.Same(live.ID) {
		windows.CloseHandle(handle)
		t.Fatal("handle identity did not match the snapshot")
	}
	if err := dispose(handle); err != nil {
		windows.CloseHandle(handle)
		t.Fatal(err)
	}
	// FileDispositionInfo removes the file when the last handle closes.
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("isolated test file was not deleted through its handle")
	}
}

func TestNativeDeletePermission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked.tmp")
	if err := os.WriteFile(path, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	user := os.Getenv("USERNAME")
	if user == "" {
		t.Skip("NOT RUN: USERNAME is empty")
	}
	if err := exec.Command("icacls", path, "/deny", user+":(D)").Run(); err != nil {
		t.Skip("NOT RUN: icacls failed: " + err.Error())
	}
	t.Cleanup(func() {
		_ = exec.Command("icacls", path, "/remove:d", user).Run()
	})
	if _, err := openVerified(path, false, true); err == nil {
		t.Fatal("delete access was granted after it was denied")
	}
}

func TestNativeUACPrompt(t *testing.T) {
	t.Skip("NOT RUN: this test must not launch a UAC prompt or change consent policy")
}

func TestNativeWindowsUpdateInstall(t *testing.T) {
	t.Skip("NOT RUN: refusing to simulate or interrupt a Windows Update install")
}

func TestNativeCleanupToolsAreNotExecuted(t *testing.T) {
	root := os.Getenv("SystemRoot")
	if root == "" {
		t.Fatal("SystemRoot is empty")
	}
	for _, name := range []string{"dism.exe", "cleanmgr.exe"} {
		if _, err := os.Stat(filepath.Join(root, "System32", name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, phase := range []string{"analyze", "cleanup"} {
		if strings.Contains(strings.ToLower(strings.Join(DISMArgs(phase), " ")), "resetbase") {
			t.Fatalf("DISM %s includes ResetBase", phase)
		}
	}
}
