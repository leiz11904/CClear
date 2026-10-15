//go:build windows

package clean

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"cclear/internal/winpath"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func init() {
	IsReparse = fileIsReparse
	SecureDelete = secureDelete
	QueryRecycle = queryRecycle
	EmptyRecycle = emptyRecycle
	RunComponentCleanup = runDISM
	PauseWindowsUpdate = pauseUpdate
	RemoveOldWindows = removeOldWindows
	ProcessRunning = processRunning
	WindowsUpdateBusy = updateBusy
	ReadSnapshot = readSnapshotWindows
	windowsMajor = func() int {
		info := windows.RtlGetVersion()
		return int(info.MajorVersion)
	}
}

type fileDispositionInfo struct {
	DeleteFile byte
}

type fileAttributeTagInfo struct {
	FileAttributes uint32
	ReparseTag     uint32
}

func longPath(path string) string {
	cleaned, err := winpath.Clean(path)
	if err != nil {
		return path
	}
	return `\\?\` + cleaned
}

func fileIsReparse(path string) bool {
	ptr, err := windows.UTF16PtrFromString(longPath(path))
	if err != nil {
		return true
	}
	attrs, err := windows.GetFileAttributes(ptr)
	if err != nil {
		return true
	}
	return attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

var errReparse = errors.New("路径中有联接点、符号链接或挂载点，已跳过")

type fileIDInfo struct {
	VolumeSerialNumber uint64
	FileID             [16]byte
}

type fileStandardInfo struct {
	AllocationSize int64
	EndOfFile      int64
	NumberOfLinks  uint32
	DeletePending  uint8
	Directory      uint8
	_              uint16
}

type fileBasicInfo struct {
	CreationTime   int64
	LastAccessTime int64
	LastWriteTime  int64
	ChangeTime     int64
	FileAttributes uint32
	_              uint32
}

func secureDelete(path, root, drive string, isDir bool, expect Identity, expectSize int64) error {
	if !expect.Valid {
		return SkipError{Err: errNoIdentity}
	}
	if err := winpath.ValidateDelete(path, root, drive, isDir); err != nil {
		return err
	}
	handle, err := openVerified(path, isDir, true)
	if err != nil {
		return skipUse(err)
	}
	defer windows.CloseHandle(handle)
	snap, err := snapshotHandle(handle)
	if err != nil || !expect.Same(snap.ID) {
		return SkipError{Err: errors.New("文件已变化或无法确认身份，已跳过")}
	}
	if !isDir && (snap.Links != 1 || snap.Size != expectSize) {
		return SkipError{Err: errors.New("文件已变化或还有其他硬链接，已跳过")}
	}
	final, err := finalPath(handle)
	if err != nil {
		return SkipError{Err: errors.New("无法确认文件的真实路径")}
	}
	if !sameCleanPath(path, final) {
		return errors.New("打开的文件和扫描路径不一致")
	}
	if err := winpath.Authorize(path, final, root, drive, isDir, false); err != nil {
		return err
	}
	return dispose(handle)
}

func readSnapshotWindows(path string, dir bool) (Snapshot, error) {
	handle, err := openVerified(path, dir, false)
	if err != nil {
		return Snapshot{}, err
	}
	defer windows.CloseHandle(handle)
	return snapshotHandle(handle)
}

// openVerified walks each path component from the volume root with
// FILE_OPEN_REPARSE_POINT. A reparse point anywhere on the path is rejected.
// The returned handle is the object itself. exclusive is only applied to the
// last component so a locked file fails the delete open instead of being removed.
func openVerified(path string, directory, exclusive bool) (windows.Handle, error) {
	cleaned, err := winpath.Clean(path)
	if err != nil {
		return 0, err
	}
	if len(cleaned) < 2 || cleaned[1] != ':' {
		return 0, errors.New("需要本机绝对路径")
	}
	handle, err := ntOpen(0, `\??\`+cleaned[:2]+`\`, true, false)
	if err != nil {
		return 0, err
	}
	if reject, why := rejectReparse(handle); reject {
		windows.CloseHandle(handle)
		return 0, why
	}
	rest := ""
	if len(cleaned) > 3 {
		rest = cleaned[3:]
	}
	if rest == "" {
		if directory {
			return handle, nil
		}
		windows.CloseHandle(handle)
		return 0, windows.ERROR_FILE_NOT_FOUND
	}
	parts := strings.Split(rest, `\`)
	for i, part := range parts {
		last := i == len(parts)-1
		child, err := ntOpen(handle, part, !last || directory, last && exclusive)
		windows.CloseHandle(handle)
		if err != nil {
			return 0, err
		}
		if reject, why := rejectReparse(child); reject {
			windows.CloseHandle(child)
			return 0, why
		}
		handle = child
	}
	return handle, nil
}

func rejectReparse(handle windows.Handle) (bool, error) {
	reparse, err := handleIsReparse(handle)
	if err != nil {
		return true, err
	}
	if reparse {
		return true, errReparse
	}
	return false, nil
}

func ntOpen(parent windows.Handle, name string, directory, exclusive bool) (windows.Handle, error) {
	us, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, err
	}
	oa := &windows.OBJECT_ATTRIBUTES{
		RootDirectory: parent,
		ObjectName:    us,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	oa.Length = uint32(unsafe.Sizeof(*oa))
	var iosb windows.IO_STATUS_BLOCK
	var handle windows.Handle
	access := uint32(windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE)
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	options := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_OPEN_FOR_BACKUP_INTENT | windows.FILE_SYNCHRONOUS_IO_NONALERT)
	if directory {
		access |= windows.FILE_LIST_DIRECTORY
		options |= windows.FILE_DIRECTORY_FILE
	} else {
		options |= windows.FILE_NON_DIRECTORY_FILE
	}
	if exclusive {
		access |= uint32(windows.DELETE | windows.FILE_WRITE_ATTRIBUTES)
		share = 0
	}
	err = windows.NtCreateFile(&handle, access, oa, &iosb, nil, 0, share, windows.FILE_OPEN, options, 0, 0)
	runtime.KeepAlive(us)
	return handle, err
}

func snapshotHandle(handle windows.Handle) (Snapshot, error) {
	var idInfo fileIDInfo
	err := windows.GetFileInformationByHandleEx(handle, windows.FileIdInfo, (*byte)(unsafe.Pointer(&idInfo)), uint32(unsafe.Sizeof(idInfo)))
	if err != nil {
		return Snapshot{}, err
	}
	var id Identity
	id.Volume = idInfo.VolumeSerialNumber
	id.File = idInfo.FileID
	id.Valid = id.Volume != 0 || id.File != [16]byte{}
	if !id.Valid {
		return Snapshot{}, errNoIdentity
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return Snapshot{}, err
	}
	if info.NumberOfLinks == 0 {
		return Snapshot{}, errNoIdentity
	}
	size := int64(info.FileSizeHigh)<<32 | int64(info.FileSizeLow)
	allocated := size
	var std fileStandardInfo
	if windows.GetFileInformationByHandleEx(handle, windows.FileStandardInfo, (*byte)(unsafe.Pointer(&std)), uint32(unsafe.Sizeof(std))) == nil && std.AllocationSize > 0 {
		allocated = std.AllocationSize
	}
	return Snapshot{
		ID:        id,
		Size:      size,
		Allocated: allocated,
		ModUnix:   filetimeUnix(info.LastWriteTime),
		Links:     info.NumberOfLinks,
	}, nil
}

func filetimeUnix(ft windows.Filetime) int64 {
	ticks := int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)
	const unixEpoch = 116444736000000000
	if ticks < unixEpoch {
		return 0
	}
	return (ticks - unixEpoch) / 10000000
}

func sameCleanPath(a, b string) bool {
	ca, ea := winpath.Clean(a)
	cb, eb := winpath.Clean(b)
	if ea != nil || eb != nil {
		return false
	}
	return strings.EqualFold(ca, cb)
}

func dispose(handle windows.Handle) error {
	info := fileDispositionInfo{DeleteFile: 1}
	err := windows.SetFileInformationByHandle(handle, windows.FileDispositionInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err == nil {
		return nil
	}
	if !accessDenied(err) || clearReadOnlyHandle(handle) != nil {
		return skipUse(err)
	}
	err = windows.SetFileInformationByHandle(handle, windows.FileDispositionInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		return skipUse(err)
	}
	return nil
}

func clearReadOnlyHandle(handle windows.Handle) error {
	var tag fileAttributeTagInfo
	err := windows.GetFileInformationByHandleEx(handle, windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&tag)), uint32(unsafe.Sizeof(tag)))
	if err != nil {
		return err
	}
	if tag.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errReparse
	}
	if tag.FileAttributes&windows.FILE_ATTRIBUTE_READONLY == 0 {
		return errors.New("文件不是只读")
	}
	basic := fileBasicInfo{FileAttributes: tag.FileAttributes &^ windows.FILE_ATTRIBUTE_READONLY}
	return windows.SetFileInformationByHandle(handle, windows.FileBasicInfo, (*byte)(unsafe.Pointer(&basic)), uint32(unsafe.Sizeof(basic)))
}

func handleIsReparse(handle windows.Handle) (bool, error) {
	var info fileAttributeTagInfo
	err := windows.GetFileInformationByHandleEx(handle, windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		return true, err
	}
	return info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}

func finalPath(handle windows.Handle) (string, error) {
	buf := make([]uint16, 32768)
	// FILE_NAME_OPENED: the path used to open the handle, without following a reparse point.
	const fileNameOpened = 0x8
	n, err := windows.GetFinalPathNameByHandle(handle, &buf[0], uint32(len(buf)), fileNameOpened)
	if err != nil {
		return "", err
	}
	if n == 0 || int(n) > len(buf) {
		return "", errors.New("无法确认文件的真实路径")
	}
	return windows.UTF16ToString(buf[:n]), nil
}

func skipUse(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_DIR_NOT_EMPTY) || errors.Is(err, errReparse) || errors.Is(err, errNoIdentity) {
		return SkipError{Err: err}
	}
	var status windows.NTStatus
	if errors.As(err, &status) {
		switch status {
		case windows.STATUS_SHARING_VIOLATION, windows.STATUS_ACCESS_DENIED, windows.STATUS_DELETE_PENDING, windows.STATUS_REPARSE_POINT_ENCOUNTERED, windows.STATUS_FILE_IS_A_DIRECTORY, windows.STATUS_NOT_A_DIRECTORY:
			return SkipError{Err: err}
		}
	}
	return err
}

func accessDenied(err error) bool {
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return true
	}
	var status windows.NTStatus
	return errors.As(err, &status) && status == windows.STATUS_ACCESS_DENIED
}

type shQueryRBInfo struct {
	CbSize      uint32
	_           uint32
	I64Size     int64
	I64NumItems int64
}

var (
	shell32          = windows.NewLazySystemDLL("shell32.dll")
	procQueryRecycle = shell32.NewProc("SHQueryRecycleBinW")
	procEmptyRecycle = shell32.NewProc("SHEmptyRecycleBinW")
)

func queryRecycle(drive string) (int64, int64, error) {
	var info shQueryRBInfo
	info.CbSize = uint32(unsafe.Sizeof(info))
	path, err := windows.UTF16PtrFromString(drive + `\`)
	if err != nil {
		return 0, 0, err
	}
	hr, _, callErr := procQueryRecycle.Call(uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(&info)))
	if hr != 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return 0, 0, callErr
		}
		return 0, 0, errors.New("无法查询回收站")
	}
	return info.I64Size, info.I64NumItems, nil
}

func emptyRecycle(drive string) error {
	path, err := windows.UTF16PtrFromString(drive + `\`)
	if err != nil {
		return err
	}
	const flags = 0x1 | 0x2 | 0x4 // no confirmation, no progress UI, no sound
	hr, _, callErr := procEmptyRecycle.Call(0, uintptr(unsafe.Pointer(path)), uintptr(flags))
	if hr != 0 && hr != 1 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return callErr
		}
		return errors.New("无法清空回收站")
	}
	return nil
}

func runDISM(ctx context.Context, log io.Writer) error {
	if err := runDISMPhase(ctx, log, "analyze"); err != nil {
		return err
	}
	return runDISMPhase(ctx, log, "cleanup")
}

func runDISMPhase(ctx context.Context, log io.Writer, phase string) error {
	args := DISMArgs(phase)
	if len(args) == 0 {
		return errors.New("未知的组件清理步骤")
	}
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	dism := filepath.Join(root, "System32", "dism.exe")
	cmd := exec.CommandContext(ctx, dism, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if log != nil {
		_, _ = log.Write(out)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func pauseUpdate() (func(), error) {
	var resumeNames []string
	var firstErr error
	for _, name := range []string{"wuauserv", "bits"} {
		wasRunning, err := stopService(name)
		if wasRunning {
			resumeNames = append(resumeNames, name)
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	resume := func() {
		for i := len(resumeNames) - 1; i >= 0; i-- {
			_ = startService(resumeNames[i])
		}
	}
	return resume, firstErr
}

func stopService(name string) (bool, error) {
	m, err := mgr.Connect()
	if err != nil {
		return false, err
	}
	defer m.Disconnect()
	service, err := m.OpenService(name)
	if err != nil {
		return false, err
	}
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return false, err
	}
	if status.State == svc.Stopped {
		return false, nil
	}
	_, err = service.Control(svc.Stop)
	if err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return true, err
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		status, err = service.Query()
		if err == nil && status.State == svc.Stopped {
			return true, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return true, errors.New(name + " 没有在限定时间内停止")
}

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	service, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer service.Close()
	return service.Start()
}

func processRunning(name string) bool {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snap)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snap, &entry); err != nil {
		return false
	}
	for {
		exe := windows.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(exe, name) {
			return true
		}
		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return false
}

func updateBusy() bool {
	if registryKeyExists(`SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`) {
		return true
	}
	if registryKeyExists(`SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending`) {
		return true
	}
	if cbsSessionsPending() {
		return true
	}
	for _, name := range []string{"wuauserv", "TrustedInstaller"} {
		state, err := serviceState(name)
		if UpdateServiceBusy(err != nil, err == nil && state == svc.Stopped) {
			return true
		}
	}
	return false
}

func cbsSessionsPending() bool {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\SessionsPending`, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return false
	}
	defer key.Close()
	names, err := key.ReadSubKeyNames(1)
	return err == nil && len(names) > 0
}

func registryKeyExists(path string) bool {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	key.Close()
	return true
}

func serviceState(name string) (svc.State, error) {
	m, err := mgr.Connect()
	if err != nil {
		return 0, err
	}
	defer m.Disconnect()
	service, err := m.OpenService(name)
	if err != nil {
		return 0, err
	}
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return 0, err
	}
	return status.State, nil
}

func removeOldWindows(ctx context.Context, log io.Writer) error {
	handler, value, args := OldWindowsPlan()
	if err := armCleanmgr(handler, value); err != nil {
		return err
	}
	defer disarmCleanmgr(value)
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	cmd := exec.CommandContext(ctx, filepath.Join(root, "System32", "cleanmgr.exe"), args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if log != nil {
		_, _ = log.Write(out)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	return nil
}

func armCleanmgr(handler, value string) error {
	return editCleanmgr(handler, value, true)
}

func disarmCleanmgr(value string) {
	_ = editCleanmgr("", value, false)
}

func editCleanmgr(handler, value string, arm bool) error {
	const keyPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\VolumeCaches`
	parent, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.READ)
	if err != nil {
		return errors.New("系统没有提供磁盘清理")
	}
	defer parent.Close()
	names, err := parent.ReadSubKeyNames(-1)
	if err != nil {
		return err
	}
	found := !arm
	for _, name := range names {
		sub, err := registry.OpenKey(parent, name, registry.SET_VALUE)
		if err != nil {
			continue
		}
		_ = sub.DeleteValue(value)
		if arm && name == handler {
			err = sub.SetDWordValue(value, 2)
			found = err == nil
		}
		sub.Close()
		if arm && name == handler && !found {
			return errors.New("无法启用旧版 Windows 的官方清理项")
		}
	}
	if !found {
		return errors.New("系统没有提供旧版 Windows 的官方清理项")
	}
	return nil
}
