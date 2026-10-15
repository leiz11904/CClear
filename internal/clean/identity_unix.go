//go:build unix

package clean

import (
	"errors"
	"os"
	"syscall"
)

func init() {
	ReadSnapshot = readSnapshotUnix
}

func readSnapshotUnix(path string, dir bool) (Snapshot, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Snapshot{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Snapshot{}, errors.New("跳过符号链接")
	}
	if info.IsDir() != dir {
		return Snapshot{}, errors.New("文件类型已变化")
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return Snapshot{}, errNoIdentity
	}
	var id Identity
	putID(&id, uint64(st.Dev), uint64(st.Ino))
	if !id.Valid {
		return Snapshot{}, errNoIdentity
	}
	links := uint32(1)
	if st.Nlink > 1 {
		links = uint32(st.Nlink)
	}
	allocated := info.Size()
	if st.Blocks > 0 {
		allocated = int64(st.Blocks) * 512
	}
	return Snapshot{
		ID:        id,
		Size:      info.Size(),
		Allocated: allocated,
		ModUnix:   info.ModTime().Unix(),
		Links:     links,
	}, nil
}
