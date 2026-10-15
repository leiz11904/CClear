//go:build unix

package space

import (
	"io/fs"
	"syscall"
)

func readMeta(_ string, info fs.FileInfo) Meta {
	meta := fallbackMeta(info)
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return meta
	}
	meta.Links = uint32(st.Nlink)
	if st.Blocks > 0 {
		meta.Allocated = int64(st.Blocks) * 512
	}
	meta.Volume = uint64(st.Dev)
	meta.FileID = uint64(st.Ino)
	meta.HasID = st.Ino != 0
	return meta
}
