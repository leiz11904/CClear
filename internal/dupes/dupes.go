package dupes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"cclear/internal/space"
)

const (
	minSize      = 64 << 10
	headBytes    = 4096
	maxPerSize   = 256
	maxHashFiles = 4000
)

// Item is one file in a duplicate group.
type Item struct {
	Path      string
	Logical   int64
	Allocated int64
	Volume    uint64
	FileID    uint64
	HasID     bool
}

// Group is a set of files with the same content.
type Group struct {
	Size  int64
	Hash  string
	Files []Item
}

// Report is a read-only duplicate analysis. Nothing is selected for deletion.
type Report struct {
	Groups    []Group
	Files     int
	Bytes     int64
	Errors    []string
	Cancelled bool
	Note      string
	Truncated bool
}

type candidate struct {
	item Item
}

func skipDirName(name string) bool {
	switch name {
	case "$Recycle.Bin", "System Volume Information", "WinSxS":
		return true
	default:
		return false
	}
}

func walkFiles(ctx context.Context, root string, errors *[]string, fn func(path string, d fs.DirEntry) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if len(*errors) < 8 {
				*errors = append(*errors, err.Error())
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if skipDirName(d.Name()) && !equalFold(path, root) {
				return filepath.SkipDir
			}
			return nil
		}
		return fn(path, d)
	})
}

// Find walks root and groups files by size, then by content hash.
// Hard links with the same identity count as one file.
func Find(ctx context.Context, root string, onProgress func(files int, where string)) Report {
	rep := Report{Note: "重复文件按内容哈希确认。不会自动删除，也不会当作垃圾清理项。"}
	counts := map[int64]int{}
	seen := map[[2]uint64]bool{}
	files := 0
	err := walkFiles(ctx, root, &rep.Errors, func(path string, d fs.DirEntry) error {
		info, err := d.Info()
		if err != nil || info.Size() < minSize {
			return nil
		}
		meta := space.ReadMeta(path, info)
		if meta.Reparse {
			return nil
		}
		if meta.HasID && meta.Links > 1 {
			id := [2]uint64{meta.Volume, meta.FileID}
			if seen[id] {
				return nil
			}
			seen[id] = true
		}
		counts[info.Size()]++
		files++
		if onProgress != nil && files%400 == 0 {
			onProgress(files, path)
		}
		return nil
	})
	if ctx.Err() != nil {
		rep.Cancelled = true
		rep.Note = "查找已取消。"
		return rep
	}
	if err != nil && ctx.Err() == nil {
		rep.Errors = append(rep.Errors, err.Error())
	}

	want := map[int64]bool{}
	for size, n := range counts {
		if n >= 2 {
			want[size] = true
		}
	}
	bySize := map[int64][]Item{}
	hashed := 0
	seen = map[[2]uint64]bool{}
	_ = walkFiles(ctx, root, &rep.Errors, func(path string, d fs.DirEntry) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		info, err := d.Info()
		if err != nil || !want[info.Size()] {
			return nil
		}
		meta := space.ReadMeta(path, info)
		if meta.Reparse || meta.Logical < minSize {
			return nil
		}
		item := Item{Path: path, Logical: meta.Logical, Allocated: meta.Allocated}
		if item.Allocated <= 0 {
			item.Allocated = meta.Logical
		}
		if meta.HasID {
			id := [2]uint64{meta.Volume, meta.FileID}
			if meta.Links > 1 {
				if seen[id] {
					return nil
				}
				seen[id] = true
			}
			item.Volume, item.FileID, item.HasID = meta.Volume, meta.FileID, true
		}
		if len(bySize[info.Size()]) >= maxPerSize {
			rep.Truncated = true
			return nil
		}
		bySize[info.Size()] = append(bySize[info.Size()], item)
		hashed++
		if hashed >= maxHashFiles {
			rep.Truncated = true
			return errCap
		}
		return nil
	})
	if ctx.Err() != nil {
		rep.Cancelled = true
		rep.Note = "查找已取消。"
		return rep
	}

	for size, list := range bySize {
		if len(list) < 2 {
			continue
		}
		heads := map[string][]candidate{}
		for _, item := range list {
			if ctx.Err() != nil {
				rep.Cancelled = true
				return rep
			}
			sum, err := hashPrefix(item.Path, headBytes)
			if err != nil {
				continue
			}
			heads[sum] = append(heads[sum], candidate{item: item})
		}
		for _, group := range heads {
			if len(group) < 2 {
				continue
			}
			full := map[string][]Item{}
			for _, cand := range group {
				sum, err := hashFile(cand.item.Path)
				if err != nil {
					continue
				}
				full[sum] = append(full[sum], cand.item)
			}
			for hash, files := range full {
				if len(files) < 2 {
					continue
				}
				rep.Groups = append(rep.Groups, Group{Size: size, Hash: hash, Files: files})
				rep.Files += len(files)
				rep.Bytes += size * int64(len(files)-1)
			}
		}
	}
	if rep.Truncated {
		rep.Note += " 候选数量已封顶，避免为百万文件无限制占用内存。"
	}
	return rep
}

var errCap = errLimited{}

type errLimited struct{}

func (errLimited) Error() string { return "candidate cap" }

func hashPrefix(path string, n int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(file, n)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func equalFold(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
