package space

import "io/fs"

// Meta is size and identity from one filesystem object.
type Meta struct {
	Logical   int64
	Allocated int64
	Links     uint32
	Volume    uint64
	FileID    uint64
	HasID     bool
	Reparse   bool
}

func fallbackMeta(info fs.FileInfo) Meta {
	size := info.Size()
	return Meta{Logical: size, Allocated: size, Links: 1}
}

// ReadMeta returns size and identity for one filesystem object.
func ReadMeta(path string, info fs.FileInfo) Meta {
	return readMeta(path, info)
}
