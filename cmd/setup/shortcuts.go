package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ManagedShortcutNames are the only start-menu / desktop links the installer owns.
var ManagedShortcutNames = struct {
	App       string
	Uninstall string
	Desktop   string
}{
	App:       "CClear.lnk",
	Uninstall: "卸载 CClear.lnk",
	Desktop:   "CClear.lnk",
}

// ShortcutCreator writes one .lnk file. Production uses PowerShell; tests inject failures.
type ShortcutCreator func(link, target, workdir string) error

type fileSnapshot struct {
	path    string
	existed bool
	data    []byte
}

// ManagedShortcutSnapshot captures the three managed .lnk files and the start-menu
// folder before any install-time shortcut changes.
type ManagedShortcutSnapshot struct {
	ProgramsDir        string
	ProgramsDirExisted bool
	App                fileSnapshot
	Uninstall          fileSnapshot
	Desktop            fileSnapshot
}

// ManagedShortcutPaths resolves the three managed link paths under test or production roots.
func ManagedShortcutPaths(programsRoot, desktopRoot string) (programsDir, appLink, uninstallLink, desktopLink string) {
	programsDir = filepath.Join(programsRoot, "CClear")
	appLink = filepath.Join(programsDir, ManagedShortcutNames.App)
	uninstallLink = filepath.Join(programsDir, ManagedShortcutNames.Uninstall)
	desktopLink = filepath.Join(desktopRoot, ManagedShortcutNames.Desktop)
	return programsDir, appLink, uninstallLink, desktopLink
}

func snapshotFile(path string) (fileSnapshot, error) {
	snap := fileSnapshot{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return snap, nil
		}
		return snap, err
	}
	snap.existed = true
	snap.data = append([]byte(nil), data...)
	return snap, nil
}

// SnapshotManagedShortcuts records existence and raw bytes for each managed .lnk and
// whether the CClear start-menu folder already existed.
func SnapshotManagedShortcuts(programsRoot, desktopRoot string) (ManagedShortcutSnapshot, error) {
	programsDir, appLink, uninstallLink, desktopLink := ManagedShortcutPaths(programsRoot, desktopRoot)
	out := ManagedShortcutSnapshot{ProgramsDir: programsDir}
	if _, err := os.Stat(programsDir); err != nil {
		if !os.IsNotExist(err) {
			return out, err
		}
	} else {
		out.ProgramsDirExisted = true
	}
	var err error
	if out.App, err = snapshotFile(appLink); err != nil {
		return out, err
	}
	if out.Uninstall, err = snapshotFile(uninstallLink); err != nil {
		return out, err
	}
	if out.Desktop, err = snapshotFile(desktopLink); err != nil {
		return out, err
	}
	return out, nil
}

// CreateManagedShortcuts creates the three managed links. createdProgramsDir is true when
// this call created the CClear start-menu folder (it did not exist before).
func CreateManagedShortcuts(
	snap ManagedShortcutSnapshot,
	installDir, target, uninst string,
	create ShortcutCreator,
) (createdProgramsDir bool, err error) {
	if create == nil {
		return false, errors.New("shortcut creator is nil")
	}
	createdProgramsDir = !snap.ProgramsDirExisted
	if createdProgramsDir {
		if err := os.MkdirAll(snap.ProgramsDir, 0o755); err != nil {
			return false, err
		}
	}
	links := []struct {
		path   string
		target string
		label  string
	}{
		{snap.App.path, target, "开始菜单快捷方式"},
		{snap.Uninstall.path, uninst, "卸载快捷方式"},
		{snap.Desktop.path, target, "桌面快捷方式"},
	}
	for _, item := range links {
		if err := create(item.path, item.target, installDir); err != nil {
			_ = snap.Restore(createdProgramsDir)
			return createdProgramsDir, fmt.Errorf("无法创建%s：%w", item.label, err)
		}
	}
	return createdProgramsDir, nil
}

// Restore puts managed .lnk files back to their pre-install bytes. Missing files are
// removed when they did not exist before. The start-menu folder is removed only when
// createdProgramsDir is true and the folder is empty after restore.
func (snap ManagedShortcutSnapshot) Restore(createdProgramsDir bool) error {
	for _, item := range []fileSnapshot{snap.App, snap.Uninstall, snap.Desktop} {
		if err := restoreFile(item); err != nil {
			return err
		}
	}
	if createdProgramsDir && snap.ProgramsDir != "" {
		if empty, err := dirEmpty(snap.ProgramsDir); err != nil {
			return err
		} else if empty {
			if err := os.Remove(snap.ProgramsDir); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func restoreFile(s fileSnapshot) error {
	if s.existed {
		if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(s.path, s.data, 0o644)
	}
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func dirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}
