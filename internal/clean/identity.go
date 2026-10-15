package clean

import (
	"encoding/binary"
	"errors"
)

var errNoIdentity = errors.New("无法确认文件身份")

// Identity is a volume id plus file id captured from an open file.
// A zero Identity is never a match. Callers must refuse the delete.
type Identity struct {
	Volume uint64
	File   [16]byte
	Valid  bool
}

// Same reports whether both identities were captured and refer to one file.
func (a Identity) Same(b Identity) bool {
	return a.Valid && b.Valid && a.Volume == b.Volume && a.File == b.File
}

// Snapshot is the size and identity read from one open of a file.
type Snapshot struct {
	ID        Identity
	Size      int64
	Allocated int64
	ModUnix   int64
	Links     uint32
}

// ReadSnapshot reads identity without deleting.
// Windows replaces it with a handle walk that does not follow reparse points.
var ReadSnapshot = func(path string, dir bool) (Snapshot, error) {
	return Snapshot{}, errNoIdentity
}

func putID(id *Identity, volume, file uint64) {
	id.Volume = volume
	id.Valid = file != 0
	binary.LittleEndian.PutUint64(id.File[:8], file)
}
