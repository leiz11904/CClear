package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func oldInstallLinkBytes(oldExe string) []byte {
	return []byte("OLD-LNK:" + oldExe)
}

func newInstallLinkBytes(newExe string) []byte {
	return []byte("NEW-LNK:" + newExe)
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testRoots(t *testing.T) (programsRoot, desktopRoot string) {
	t.Helper()
	root := t.TempDir()
	return filepath.Join(root, "Programs"), filepath.Join(root, "Desktop")
}

func recordingCreator(t *testing.T, failOn map[string]error) ShortcutCreator {
	t.Helper()
	return func(link, target, workdir string) error {
		if err, ok := failOn[link]; ok {
			return err
		}
		body := newInstallLinkBytes(target)
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			return err
		}
		return os.WriteFile(link, body, 0o644)
	}
}

func TestSnapshotRestoreOldInstallDifferentDirectory(t *testing.T) {
	programsRoot, desktopRoot := testRoots(t)
	programsDir, appLink, uninstallLink, desktopLink := ManagedShortcutPaths(programsRoot, desktopRoot)
	oldDir := filepath.Join(t.TempDir(), "OldCClear")
	oldTarget := filepath.Join(oldDir, "cclear.exe")
	oldUninstPath := filepath.Join(oldDir, "Uninstall.exe")
	oldApp := oldInstallLinkBytes(oldTarget)
	oldUninstBytes := oldInstallLinkBytes(oldUninstPath)
	oldDesktop := oldInstallLinkBytes(oldTarget)

	writeFile(t, appLink, oldApp)
	writeFile(t, uninstallLink, oldUninstBytes)
	writeFile(t, desktopLink, oldDesktop)

	snap, err := SnapshotManagedShortcuts(programsRoot, desktopRoot)
	if err != nil {
		t.Fatal(err)
	}
	newDir := filepath.Join(t.TempDir(), "NewCClear")
	newTarget := filepath.Join(newDir, "cclear.exe")
	newUninst := filepath.Join(newDir, "Uninstall.exe")

	_, err = CreateManagedShortcuts(snap, newDir, newTarget, newUninst, recordingCreator(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFile(t, appLink), newInstallLinkBytes(newTarget)) {
		t.Fatal("app link not updated during install attempt")
	}

	if err := snap.Restore(false); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFile(t, appLink), oldApp) {
		t.Fatal("app link bytes not restored to old install path")
	}
	if !bytes.Equal(readFile(t, uninstallLink), oldUninstBytes) {
		t.Fatal("uninstall link bytes not restored")
	}
	if !bytes.Equal(readFile(t, desktopLink), oldDesktop) {
		t.Fatal("desktop link bytes not restored")
	}
	if _, err := os.Stat(programsDir); err != nil {
		t.Fatal("existing start-menu folder should remain")
	}
}

func TestPartialShortcutsMissingThenRestore(t *testing.T) {
	programsRoot, desktopRoot := testRoots(t)
	_, appLink, _, desktopLink := ManagedShortcutPaths(programsRoot, desktopRoot)
	oldDesktop := oldInstallLinkBytes(`D:\legacy\cclear.exe`)
	writeFile(t, desktopLink, oldDesktop)

	snap, err := SnapshotManagedShortcuts(programsRoot, desktopRoot)
	if err != nil {
		t.Fatal(err)
	}
	if snap.App.existed || snap.Uninstall.existed || !snap.Desktop.existed {
		t.Fatalf("unexpected snapshot: app=%v uninstall=%v desktop=%v", snap.App.existed, snap.Uninstall.existed, snap.Desktop.existed)
	}

	newDir := filepath.Join(t.TempDir(), "CClear")
	createdDir, err := CreateManagedShortcuts(snap, newDir, filepath.Join(newDir, "cclear.exe"), filepath.Join(newDir, "Uninstall.exe"), recordingCreator(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if !createdDir {
		t.Fatal("expected created start-menu folder")
	}
	if _, err := os.Stat(appLink); err != nil {
		t.Fatal("app link should exist after create")
	}

	if err := snap.Restore(createdDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(appLink); !os.IsNotExist(err) {
		t.Fatal("app link should be removed after restore")
	}
	if _, err := os.Stat(filepath.Join(programsRoot, "CClear")); !os.IsNotExist(err) {
		t.Fatal("created empty start-menu folder should be removed")
	}
	if !bytes.Equal(readFile(t, desktopLink), oldDesktop) {
		t.Fatal("desktop link should be unchanged")
	}
}

func TestCreateFailsOnSecondAndThirdLink(t *testing.T) {
	oldApp := oldInstallLinkBytes(`C:\Old\cclear.exe`)

	for name, failIndex := range map[string]int{
		"second": 1,
		"third":  2,
	} {
		t.Run(name, func(t *testing.T) {
			programsRoot, desktopRoot := testRoots(t)
			_, appLink, uninstallLink, desktopLink := ManagedShortcutPaths(programsRoot, desktopRoot)
			writeFile(t, appLink, oldApp)
			snap, err := SnapshotManagedShortcuts(programsRoot, desktopRoot)
			if err != nil {
				t.Fatal(err)
			}
			newDir := filepath.Join(t.TempDir(), "CClear")
			target := filepath.Join(newDir, "cclear.exe")
			uninst := filepath.Join(newDir, "Uninstall.exe")
			failLinks := []string{appLink, uninstallLink, desktopLink}
			failLink := failLinks[failIndex]
			fail := map[string]error{failLink: errors.New("injected failure")}
			_, err = CreateManagedShortcuts(snap, newDir, target, uninst, recordingCreator(t, fail))
			if err == nil {
				t.Fatal("expected create failure")
			}
			if !bytes.Equal(readFile(t, appLink), oldApp) {
				t.Fatal("app link should be rolled back to original bytes")
			}
			if _, err := os.Stat(uninstallLink); !os.IsNotExist(err) {
				t.Fatal("uninstall link should not remain after rollback")
			}
			if _, err := os.Stat(desktopLink); !os.IsNotExist(err) {
				t.Fatal("desktop link should not remain after rollback")
			}
		})
	}
}

func TestRegistryFailureStyleRestore(t *testing.T) {
	programsRoot, desktopRoot := testRoots(t)
	_, appLink, uninstallLink, desktopLink := ManagedShortcutPaths(programsRoot, desktopRoot)
	oldApp := oldInstallLinkBytes(`C:\v1\cclear.exe`)
	oldUninst := oldInstallLinkBytes(`C:\v1\Uninstall.exe`)
	writeFile(t, appLink, oldApp)
	writeFile(t, uninstallLink, oldUninst)

	snap, err := SnapshotManagedShortcuts(programsRoot, desktopRoot)
	if err != nil {
		t.Fatal(err)
	}
	newDir := filepath.Join(t.TempDir(), "CClear")
	createdDir, err := CreateManagedShortcuts(snap, newDir, filepath.Join(newDir, "cclear.exe"), filepath.Join(newDir, "Uninstall.exe"), recordingCreator(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate successful shortcuts then registry write failure.
	if err := snap.Restore(createdDir); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFile(t, appLink), oldApp) {
		t.Fatal("app link restore after registry failure")
	}
	if !bytes.Equal(readFile(t, uninstallLink), oldUninst) {
		t.Fatal("uninstall link restore after registry failure")
	}
	if _, err := os.Stat(desktopLink); !os.IsNotExist(err) {
		t.Fatal("desktop link should be absent after restore")
	}
}

func TestUnknownFilePreservedInStartMenuFolder(t *testing.T) {
	programsRoot, desktopRoot := testRoots(t)
	programsDir, appLink, _, _ := ManagedShortcutPaths(programsRoot, desktopRoot)
	unknown := filepath.Join(programsDir, "README.txt")
	writeFile(t, unknown, []byte("keep me"))
	oldApp := oldInstallLinkBytes(`C:\Old\cclear.exe`)
	writeFile(t, appLink, oldApp)

	snap, err := SnapshotManagedShortcuts(programsRoot, desktopRoot)
	if err != nil {
		t.Fatal(err)
	}
	newDir := filepath.Join(t.TempDir(), "CClear")
	createdDir, err := CreateManagedShortcuts(snap, newDir, filepath.Join(newDir, "cclear.exe"), filepath.Join(newDir, "Uninstall.exe"), recordingCreator(t, map[string]error{
		appLink: errors.New("fail after touching folder"),
	}))
	if err == nil {
		t.Fatal("expected failure")
	}
	if createdDir {
		t.Fatal("start-menu folder already existed")
	}
	if !bytes.Equal(readFile(t, unknown), []byte("keep me")) {
		t.Fatal("unknown file must remain")
	}
	if !bytes.Equal(readFile(t, appLink), oldApp) {
		t.Fatal("managed link restored")
	}
	if _, err := os.Stat(programsDir); err != nil {
		t.Fatal("start-menu folder with unknown file must remain")
	}
}

func TestRestoreDoesNotRecreateLinksAtNewPaths(t *testing.T) {
	programsRoot, desktopRoot := testRoots(t)
	snap, err := SnapshotManagedShortcuts(programsRoot, desktopRoot)
	if err != nil {
		t.Fatal(err)
	}
	newDir := filepath.Join(t.TempDir(), "BrandNew")
	createdDir, err := CreateManagedShortcuts(snap, newDir, filepath.Join(newDir, "cclear.exe"), filepath.Join(newDir, "Uninstall.exe"), recordingCreator(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := snap.Restore(createdDir); err != nil {
		t.Fatal(err)
	}
	_, appLink, uninstallLink, desktopLink := ManagedShortcutPaths(programsRoot, desktopRoot)
	for _, path := range []string{appLink, uninstallLink, desktopLink} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("link should not exist after restore: %s", path)
		}
	}
	data, err := os.ReadFile(appLink)
	if err == nil && strings.Contains(string(data), "BrandNew") {
		t.Fatal("must not leave new-path shortcut content")
	}
}
