package payload

import (
	"encoding/binary"
	"errors"
)

const magic = "CCLEARSETUP1"

// Append stores the application at the end of the installer.
func Append(stub, app []byte) []byte {
	out := make([]byte, 0, len(stub)+len(app)+len(magic)+8)
	out = append(out, stub...)
	out = append(out, app...)
	out = append(out, magic...)
	var size [8]byte
	binary.LittleEndian.PutUint64(size[:], uint64(len(app)))
	out = append(out, size[:]...)
	return out
}

// Extract returns the application embedded by Append.
func Extract(blob []byte) ([]byte, error) {
	footer := len(magic) + 8
	if len(blob) < footer {
		return nil, errors.New("安装包不完整")
	}
	tail := blob[len(blob)-footer:]
	if string(tail[:len(magic)]) != magic {
		return nil, errors.New("安装包不完整")
	}
	size := binary.LittleEndian.Uint64(tail[len(magic):])
	if size == 0 || size > uint64(len(blob)-footer) {
		return nil, errors.New("安装包已损坏")
	}
	start := len(blob) - footer - int(size)
	app := blob[start : start+int(size)]
	if len(app) < 2 || app[0] != 'M' || app[1] != 'Z' {
		return nil, errors.New("安装包里的程序无效")
	}
	return app, nil
}
