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

func LogicalDrives() ([]string, error) {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	get := kernel32.NewProc("GetLogicalDriveStringsW")
	n, _, _ := get.Call(0, 0)
	if n == 0 {
		return nil, errors.New("无法列出磁盘")
	}
	buf := make([]uint16, n+1)
	r, _, err := get.Call(uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if r == 0 {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("无法列出磁盘")
	}
	var drives []string
	start := 0
	for i, c := range buf {
		if c != 0 {
			continue
		}
		if i > start {
			drives = append(drives, strings.TrimRight(windows.UTF16ToString(buf[start:i]), `\`))
		}
		start = i + 1
		if i+1 < len(buf) && buf[i+1] == 0 {
			break
		}
	}
	return drives, nil
}

func KnownFolder(id string) (string, error) {
	guid, ok := knownFolders[id]
	if !ok {
		return "", errors.New("未知的常用位置")
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	var out *uint16
	r, _, _ := shell32.NewProc("SHGetKnownFolderPath").Call(uintptr(unsafe.Pointer(&guid)), 0, 0, uintptr(unsafe.Pointer(&out)))
	if r != 0 || out == nil {
		return fallbackFolder(id)
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(out))
	path := windows.UTF16PtrToString(out)
	if path == "" {
		return fallbackFolder(id)
	}
	return path, nil
}

var knownFolders = map[string]windows.GUID{
	"downloads": {Data1: 0x374DE290, Data2: 0x123F, Data3: 0x4565, Data4: [8]byte{0x91, 0x64, 0x39, 0xC4, 0x92, 0x5E, 0x46, 0x7B}},
	"pictures":  {Data1: 0x33E28130, Data2: 0x4E1E, Data3: 0x4676, Data4: [8]byte{0x83, 0x5A, 0x98, 0x39, 0x5C, 0x3B, 0xC3, 0xBB}},
	"videos":    {Data1: 0x18989B1D, Data2: 0x99B5, Data3: 0x455B, Data4: [8]byte{0x84, 0x1C, 0xAB, 0x7C, 0x74, 0xE4, 0xDD, 0xFC}},
	"documents": {Data1: 0xFDD39AD0, Data2: 0x238F, Data3: 0x46AF, Data4: [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7}},
}

func fallbackFolder(id string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errors.New("无法定位常用文件夹")
	}
	name := map[string]string{"downloads": "Downloads", "pictures": "Pictures", "videos": "Videos", "documents": "Documents"}[id]
	if name == "" {
		return "", errors.New("未知的常用位置")
	}
	return filepath.Join(home, name), nil
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
