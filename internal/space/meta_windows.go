//go:build windows

package space

import (
	"io/fs"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileStandardInfo struct {
	AllocationSize int64
	EndOfFile      int64
	NumberOfLinks  uint32
	DeletePending  uint8
	Directory      uint8
	_              uint16
}

func readMeta(path string, info fs.FileInfo) Meta {
	meta := fallbackMeta(info)
	ptr, err := windows.UTF16PtrFromString(`\\?\` + path)
	if err != nil {
		return meta
	}
	handle, err := windows.CreateFile(ptr, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return meta
	}
	defer windows.CloseHandle(handle)
	var fi windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &fi); err != nil {
		return meta
	}
	if fi.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		meta.Reparse = true
		return meta
	}
	meta.Logical = int64(fi.FileSizeHigh)<<32 | int64(fi.FileSizeLow)
	meta.Allocated = meta.Logical
	meta.Links = fi.NumberOfLinks
	var std fileStandardInfo
	if windows.GetFileInformationByHandleEx(handle, windows.FileStandardInfo, (*byte)(unsafe.Pointer(&std)), uint32(unsafe.Sizeof(std))) == nil && std.AllocationSize > 0 {
		meta.Allocated = std.AllocationSize
	}
	meta.Volume = uint64(fi.VolumeSerialNumber)
	meta.FileID = uint64(fi.FileIndexHigh)<<32 | uint64(fi.FileIndexLow)
	meta.HasID = meta.FileID != 0
	return meta
}
