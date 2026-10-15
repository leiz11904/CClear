//go:build !unix && !windows

package space

import "io/fs"

func readMeta(_ string, info fs.FileInfo) Meta {
	return fallbackMeta(info)
}
