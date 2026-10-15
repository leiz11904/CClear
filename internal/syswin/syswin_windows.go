//go:build windows

package syswin

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func IsElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

var instanceHandle windows.Handle

func ReleaseSingleInstance() {
	if instanceHandle == 0 {
		return
	}
	windows.ReleaseMutex(instanceHandle)
	windows.CloseHandle(instanceHandle)
	instanceHandle = 0
}

func RelaunchElevated() error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	path, err := os.Executable()
	if err != nil {
		path = os.Args[0]
	}
	exe, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	r, _, _ := shell32.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(exe)), 0, 0, 1)
	runtime.KeepAlive(verb)
	runtime.KeepAlive(exe)
	if r <= 32 {
		return errors.New("没有获得管理员权限")
	}
	return nil
}

func Disk(drive string) (total, free uint64, err error) {
	path, err := windows.UTF16PtrFromString(drive + `\`)
	if err != nil {
		return 0, 0, err
	}
	var avail, tot, totFree uint64
	if err := windows.GetDiskFreeSpaceEx(path, &avail, &tot, &totFree); err != nil {
		return 0, 0, err
	}
	return tot, avail, nil
}

func Windows10OrNewer() bool {
	info := windows.RtlGetVersion()
	return info.MajorVersion >= 10
}

func AcquireSingleInstance() (bool, error) {
	name, err := windows.UTF16PtrFromString(`Local\CClear.SingleInstance.v1`)
	if err != nil {
		return false, err
	}
	handle, err := windows.CreateMutex(nil, true, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return false, err
	}
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if handle != 0 {
			windows.CloseHandle(handle)
		}
		return false, nil
	}
	instanceHandle = handle
	return true, nil
}

func SystemDrive() string {
	drive := os.Getenv("SystemDrive")
	if len(drive) >= 2 && drive[1] == ':' {
		return string([]byte{drive[0] & 0xDF, ':'})
	}
	return "C:"
}

func OpenPath(path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	r, _, _ := shell32.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
	if r <= 32 {
		return errors.New("无法打开 " + path)
	}
	return nil
}

func Reveal(path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	exe, err := windows.UTF16PtrFromString("explorer.exe")
	if err != nil {
		return err
	}
	args, err := windows.UTF16PtrFromString(`/select,` + path)
	if err != nil {
		return err
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	r, _, _ := shell32.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(exe)), uintptr(unsafe.Pointer(args)), 0, 1)
	runtime.KeepAlive(verb)
	runtime.KeepAlive(exe)
	runtime.KeepAlive(args)
	if r <= 32 {
		return OpenPath(filepath.Dir(path))
	}
	return nil
}

func CopyText(text string) error {
	utf16, err := windows.UTF16FromString(text)
	if err != nil {
		return err
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	if r, _, _ := user32.NewProc("OpenClipboard").Call(0); r == 0 {
		return errors.New("无法打开剪贴板")
	}
	defer user32.NewProc("CloseClipboard").Call()
	user32.NewProc("EmptyClipboard").Call()
	mem, _, _ := kernel32.NewProc("GlobalAlloc").Call(0x0002, uintptr(len(utf16)*2))
	if mem == 0 {
		return errors.New("无法复制")
	}
	locked, _, _ := kernel32.NewProc("GlobalLock").Call(mem)
	if locked == 0 {
		return errors.New("无法复制")
	}
	dst := *(*unsafe.Pointer)(unsafe.Pointer(&locked))
	copy(unsafe.Slice((*uint16)(dst), len(utf16)), utf16)
	kernel32.NewProc("GlobalUnlock").Call(mem)
	if r, _, _ := user32.NewProc("SetClipboardData").Call(13, mem); r == 0 {
		return errors.New("无法写入剪贴板")
	}
	return nil
}

func Properties(path string) error {
	verb, err := windows.UTF16PtrFromString("properties")
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	r, _, _ := shell32.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
	runtime.KeepAlive(verb)
	runtime.KeepAlive(target)
	if r <= 32 {
		return Reveal(path)
	}
	return nil
}

func SafeOpen(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".exe", ".bat", ".cmd", ".ps1", ".msi", ".scr", ".com", ".js", ".vbs", ".lnk", ".pif", ".cpl", ".msc":
		return errors.New("不能直接打开可执行或脚本文件")
	}
	return OpenPath(path)
}

func RecycleFile(path string) error {
	from, err := windows.UTF16FromString(path)
	if err != nil {
		return err
	}
	from = append(from, 0)
	op := struct {
		hwnd                  uintptr
		wFunc                 uint32
		pFrom                 *uint16
		pTo                   *uint16
		fFlags                uint16
		fAnyOperationsAborted int32
		hNameMappings         uintptr
		lpszProgressTitle     *uint16
	}{
		wFunc:  3, // FO_DELETE
		pFrom:  &from[0],
		fFlags: 0x0040 | 0x0010 | 0x0004, // FOF_ALLOWUNDO | FOF_NOCONFIRMATION | FOF_SILENT
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	r, _, callErr := shell32.NewProc("SHFileOperationW").Call(uintptr(unsafe.Pointer(&op)))
	runtime.KeepAlive(from)
	runtime.KeepAlive(op)
	if r != 0 {
		if callErr != nil {
			return callErr
		}
		return errors.New("无法移到回收站")
	}
	if op.fAnyOperationsAborted != 0 {
		return errors.New("已取消")
	}
	return nil
}

func OpenSettings(uri string) error {
	return OpenPath(uri)
}

func RunCommand(exe, args string) error {
	lower := strings.ToLower(exe)
	if strings.Contains(lower, "cmd.exe") || strings.Contains(lower, "powershell") || strings.Contains(lower, "wscript") {
		return errors.New("拒绝执行脚本命令")
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	param, err := windows.UTF16PtrFromString(args)
	if err != nil {
		return err
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	r, _, _ := shell32.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), uintptr(unsafe.Pointer(param)), 0, 1)
	runtime.KeepAlive(verb)
	runtime.KeepAlive(file)
	runtime.KeepAlive(param)
	if r <= 32 {
		return errors.New("无法启动程序")
	}
	return nil
}
