//go:build windows

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"cclear/internal/appinfo"
	itxn "cclear/internal/install"
	"cclear/internal/payload"
	"cclear/internal/winpath"
	"cclear/internal/winptr"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func isUninstall(args []string) bool {
	if strings.EqualFold(filepath.Base(args[0]), "Uninstall.exe") {
		return true
	}
	for _, arg := range args[1:] {
		if strings.EqualFold(arg, "/uninstall") {
			return true
		}
	}
	return false
}

func isSilent(args []string) bool {
	for _, arg := range args[1:] {
		if strings.EqualFold(arg, "/S") {
			return true
		}
	}
	return false
}

func defaultDir() string {
	root := os.Getenv("ProgramFiles")
	if root == "" {
		root = `C:\Program Files`
	}
	return filepath.Join(root, "CClear")
}

func validInstallDir(path string) (string, error) {
	cleaned, err := winpath.Clean(path)
	if err != nil {
		return "", errors.New("请选择一个有效的文件夹")
	}
	if len(cleaned) <= 3 {
		return "", errors.New("不能安装到盘符根目录")
	}
	for _, ch := range cleaned {
		switch ch {
		case '"', '&', '%', '!', '^', '|', '<', '>', '\n', '\r':
			return "", errors.New("安装路径包含不能使用的字符")
		}
	}
	lower := strings.ToLower(cleaned)
	drive := winpath.Drive(cleaned)
	blocked := []string{
		drive + `\windows`,
		drive + `\windows\system32`,
		drive + `\windows\syswow64`,
		drive + `\windows\winsxs`,
	}
	for _, item := range blocked {
		if lower == strings.ToLower(item) || strings.HasPrefix(lower, strings.ToLower(item)+`\`) {
			return "", errors.New("不能安装到 Windows 系统目录")
		}
	}
	return cleaned, nil
}

func installFolder(path string) string {
	if strings.EqualFold(filepath.Base(path), "CClear") {
		return path
	}
	return filepath.Join(path, "CClear")
}

func embeddedApp() ([]byte, error) {
	path, err := os.Executable()
	if err != nil {
		return nil, err
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return payload.Extract(blob)
}

func install(dir string) error {
	cleaned, err := validInstallDir(dir)
	if err != nil {
		return err
	}
	if err := ensureRealDir(cleaned); err != nil {
		return err
	}
	if err := closeRunningApp(); err != nil {
		return err
	}
	app, err := embeddedApp()
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	selfBytes, err := os.ReadFile(self)
	if err != nil {
		return err
	}
	tx, err := itxn.Begin(cleaned)
	if err != nil {
		return err
	}
	if err := tx.Stage("cclear.exe", app); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Stage("Uninstall.exe", selfBytes); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	dir = tx.Dir()
	target := filepath.Join(dir, "cclear.exe")
	uninst := filepath.Join(dir, "Uninstall.exe")
	programsRoot := knownOr(commonProgramsID, filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`))
	desktopRoot := knownOr(publicDesktopID, filepath.Join(os.Getenv("PUBLIC"), "Desktop"))
	shortcutSnap, err := SnapshotManagedShortcuts(programsRoot, desktopRoot)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	regSnap, err := snapshotUninstallRegistry()
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	createdProgramsDir, err := createShortcuts(shortcutSnap, dir, target, uninst)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := writeUninstallKey(dir, target, uninst); err != nil {
		_ = regSnap.restoreUninstallRegistry()
		_ = shortcutSnap.Restore(createdProgramsDir)
		_ = tx.Rollback()
		return err
	}
	tx.Cleanup()
	return nil
}

func uninstall() error {
	dir := installedDir()
	if dir == "" {
		dir = filepath.Dir(currentExe())
	}
	if err := closeRunningApp(); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(dir, "cclear.exe"))
	removeShortcuts()
	_ = registry.DeleteKey(registry.LOCAL_MACHINE, uninstallKey)
	self := currentExe()
	if err := scheduleDelete(dir, self); err != nil {
		return err
	}
	return nil
}

func installedDir() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, uninstallKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	dir, _, err := k.GetStringValue("InstallLocation")
	if err != nil {
		return ""
	}
	return dir
}

func writeUninstallKey(dir, target, uninst string) error {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, uninstallKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("无法登记卸载信息：%w", err)
	}
	defer k.Close()
	values := map[string]string{
		"DisplayName":          "CClear C盘安全清理",
		"DisplayVersion":       appinfo.Version,
		"Publisher":            "CClear",
		"InstallLocation":      dir,
		"DisplayIcon":          target,
		"UninstallString":      `"` + uninst + `"`,
		"QuietUninstallString": `"` + uninst + `" /S`,
	}
	for name, value := range values {
		if err := k.SetStringValue(name, value); err != nil {
			return err
		}
	}
	size := 0
	if info, err := os.Stat(target); err == nil {
		size += int(info.Size() / 1024)
	}
	if info, err := os.Stat(uninst); err == nil {
		size += int(info.Size() / 1024)
	}
	_ = k.SetStringValue("InstallDate", time.Now().Format("20060102"))
	_ = k.SetDWordValue("EstimatedSize", uint32(size))
	_ = k.SetDWordValue("NoModify", 1)
	_ = k.SetDWordValue("NoRepair", 1)
	return nil
}

func createShortcuts(snap ManagedShortcutSnapshot, dir, target, uninst string) (createdProgramsDir bool, err error) {
	return CreateManagedShortcuts(snap, dir, target, uninst, shortcut)
}

func removeShortcuts() {
	programsRoot := knownOr(commonProgramsID, filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`))
	desktopRoot := knownOr(publicDesktopID, filepath.Join(os.Getenv("PUBLIC"), "Desktop"))
	programsDir, appLink, uninstallLink, desktopLink := ManagedShortcutPaths(programsRoot, desktopRoot)
	_ = os.Remove(appLink)
	_ = os.Remove(uninstallLink)
	_ = os.Remove(desktopLink)
	if empty, err := dirEmpty(programsDir); err == nil && empty {
		_ = os.Remove(programsDir)
	}
}

func shortcut(link, target, workdir string) error {
	script := filepath.Join(os.TempDir(), "cclear-shortcut.ps1")
	body := "$shell = New-Object -ComObject WScript.Shell\r\n" +
		"$lnk = $shell.CreateShortcut($env:CCLEAR_LNK)\r\n" +
		"$lnk.TargetPath = $env:CCLEAR_TARGET\r\n" +
		"$lnk.WorkingDirectory = $env:CCLEAR_WORKDIR\r\n" +
		"$lnk.Description = 'CClear'\r\n" +
		"$lnk.Save()\r\n"
	if err := os.WriteFile(script, []byte(body), 0o600); err != nil {
		return err
	}
	defer os.Remove(script)
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	cmd := exec.Command(filepath.Join(root, `System32\WindowsPowerShell\v1.0\powershell.exe`),
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script)
	cmd.Env = append(os.Environ(),
		"CCLEAR_LNK="+link,
		"CCLEAR_TARGET="+target,
		"CCLEAR_WORKDIR="+workdir,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func closeRunningApp() error {
	name, err := windows.UTF16PtrFromString("CClearMainWindow")
	if err != nil {
		return err
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	find := user32.NewProc("FindWindowW")
	hwnd, _, _ := find.Call(uintptr(unsafe.Pointer(name)), 0)
	if hwnd == 0 {
		return nil
	}
	var result uintptr
	ret, _, _ := user32.NewProc("SendMessageTimeoutW").Call(
		hwnd,
		uintptr(win.WM_CLOSE),
		0,
		0,
		0x0002, // SMTO_ABORTIFHUNG
		uintptr(itxn.CloseTimeoutMs),
		uintptr(unsafe.Pointer(&result)),
	)
	if ret == 0 {
		return itxn.AppStillRunningError()
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hwnd, _, _ = find.Call(uintptr(unsafe.Pointer(name)), 0)
		if hwnd == 0 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return itxn.AppStillRunningError()
}

func scheduleDelete(dir, self string) error {
	cleaned, err := validInstallDir(dir)
	if err != nil {
		return err
	}
	dir = cleaned
	bat := filepath.Join(os.TempDir(), "cclear-uninstall.cmd")
	body := "@echo off\r\n" +
		"set /a n=0\r\n" +
		":wait\r\n" +
		"set /a n+=1\r\n" +
		"if %n% gtr 30 goto cleanup\r\n" +
		"timeout /t 1 /nobreak >nul\r\n" +
		"del /f /q " + quoteCmd(self) + "\r\n" +
		"if exist " + quoteCmd(self) + " goto wait\r\n" +
		":cleanup\r\n" +
		"rmdir " + quoteCmd(dir) + "\r\n" +
		"del /f /q \"%~f0\"\r\n"
	if err := os.WriteFile(bat, []byte(body), 0o600); err != nil {
		return err
	}
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	cmd := exec.Command(filepath.Join(root, `System32\cmd.exe`), "/C", bat)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd.Start()
}

func quoteCmd(path string) string {
	return `"` + path + `"`
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func currentExe() string {
	path, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	return path
}

var (
	commonProgramsID = windows.GUID{Data1: 0x0139D44E, Data2: 0x6AFE, Data3: 0x49F2, Data4: [8]byte{0x86, 0x90, 0x3D, 0xAF, 0xCA, 0xE6, 0xFF, 0xB8}}
	publicDesktopID  = windows.GUID{Data1: 0xC4AA340D, Data2: 0xF20F, Data3: 0x4863, Data4: [8]byte{0xAF, 0xEF, 0xF8, 0x7E, 0xF2, 0xE6, 0xBA, 0x25}}
)

func ensureRealDir(path string) error {
	current := path
	for {
		ptr, err := windows.UTF16PtrFromString(`\\?\` + current)
		if err != nil {
			return errors.New("请选择一个有效的文件夹")
		}
		attrs, err := windows.GetFileAttributes(ptr)
		if err == nil {
			if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
				return errors.New("不能安装到快捷方式或目录联接点")
			}
		} else if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) && !errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return errors.New("无法检查安装文件夹")
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}

func knownOr(id windows.GUID, fallback string) string {
	path, err := knownFolder(id)
	if err != nil || path == "" {
		return fallback
	}
	return path
}

func knownFolder(id windows.GUID) (string, error) {
	var out uintptr
	r, _, err := windows.NewLazySystemDLL("shell32.dll").NewProc("SHGetKnownFolderPath").Call(
		uintptr(unsafe.Pointer(&id)),
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r != 0 {
		if err != nil {
			return "", err
		}
		return "", errors.New("无法定位系统文件夹")
	}
	defer windows.CoTaskMemFree(winptr.Ptr(out))
	return windows.UTF16PtrToString((*uint16)(winptr.Ptr(out))), nil
}
