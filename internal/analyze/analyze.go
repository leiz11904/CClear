// Package analyze reports how the system drive is used. It never deletes files
// and never treats user documents as junk for cleanup.
package analyze

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	minLargeBytes = 50 << 20
	maxLargeFiles = 30
	maxFolders    = 16
)

// Env is the machine layout. Tests supply isolated directories.
type Env struct {
	Drive           string
	UserProfile     string
	Public          string
	LocalAppData    string
	RoamingAppData  string
	Windows         string
	ProgramFiles    string
	ProgramFilesX86 string
	ProgramData     string
}

// Bucket is a read-only usage group.
type Bucket struct {
	ID    string
	Name  string
	Kind  string
	Note  string
	Bytes int64
	Files int
}

// LargeFile is a file large enough to mention. It is not a cleanup candidate.
type LargeFile struct {
	Path  string
	Name  string
	Bytes int64
}

// Folder is a top-level directory on the system drive.
type Folder struct {
	Name  string
	Path  string
	Bytes int64
}

// Report is the result of one read-only pass.
type Report struct {
	Drive     string
	Started   time.Time
	Finished  time.Time
	Buckets   []Bucket
	Folders   []Folder
	Large     []LargeFile
	Files     int
	Errors    []string
	Cancelled bool
	Note      string
}

type largeHit struct {
	path  string
	bytes int64
}

// Scan walks selected locations. It skips reparse points and does not follow links.
func Scan(ctx context.Context, env Env, onProgress func(files int, where string)) Report {
	rep := Report{
		Drive:   env.Drive,
		Started: time.Now(),
		Note:    "这些数字只说明空间占用。CClear 不会把用户文档、桌面、下载或照片当作垃圾删除。",
		Buckets: []Bucket{
			{ID: "userdata", Name: "文档和媒体", Kind: "user", Note: "用户文件，不是垃圾"},
			{ID: "appdata", Name: "应用数据", Kind: "app", Note: "程序配置和本地数据，不是自动清理目标"},
			{ID: "apps", Name: "已安装程序", Kind: "app", Note: "Program Files，不会卸载软件"},
			{ID: "windows", Name: "Windows 系统", Kind: "system", Note: "系统目录只统计占用，不会作为垃圾清理"},
			{ID: "other", Name: "其他文件", Kind: "other", Note: "未能归入以上类别的内容"},
		},
	}
	index := map[string]int{}
	for i, b := range rep.Buckets {
		index[b.ID] = i
	}
	add := func(id string, size int64) {
		i := index[id]
		rep.Buckets[i].Bytes += size
		rep.Buckets[i].Files++
		rep.Files++
	}

	var large []largeHit
	seen := map[string]bool{}
	walk := func(root, bucket string, keepLarge bool) {
		if root == "" {
			return
		}
		root = filepath.Clean(root)
		key := strings.ToLower(root)
		if seen[key] {
			return
		}
		seen[key] = true
		info, err := os.Lstat(root)
		if err != nil {
			if !os.IsNotExist(err) {
				rep.Errors = append(rep.Errors, err.Error())
			}
			return
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				if len(rep.Errors) < 8 {
					rep.Errors = append(rep.Errors, err.Error())
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
			name := d.Name()
			if d.IsDir() && skipDir(name) {
				return filepath.SkipDir
			}
			if d.IsDir() {
				if onProgress != nil {
					onProgress(rep.Files, path)
				}
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			size := info.Size()
			add(bucket, size)
			if keepLarge && size >= minLargeBytes {
				large = rememberLarge(large, path, size)
			}
			return nil
		})
	}

	for _, folder := range userFolders(env) {
		walk(folder, "userdata", true)
	}
	walk(env.LocalAppData, "appdata", false)
	walk(env.RoamingAppData, "appdata", false)
	walk(env.ProgramFiles, "apps", false)
	walk(env.ProgramFilesX86, "apps", false)
	walk(env.Windows, "windows", false)
	walk(env.ProgramData, "other", false)

	rep.Folders = driveFolders(ctx, env.Drive, onProgress)
	rep.Large = formatLarge(large)
	rep.Finished = time.Now()
	if ctx.Err() != nil {
		rep.Cancelled = true
		rep.Note = "分析已取消。已统计的部分仍只用于展示占用，不会删除。"
	}
	return rep
}

func skipDir(name string) bool {
	switch strings.ToLower(name) {
	case "$recycle.bin", "system volume information", "winsxs", "installer", "csc":
		return true
	default:
		return false
	}
}

func userFolders(env Env) []string {
	names := []string{"Documents", "Desktop", "Downloads", "Pictures", "Videos", "Music", "文档", "桌面", "下载", "图片", "视频", "音乐"}
	var out []string
	for _, base := range []string{env.UserProfile, env.Public} {
		if base == "" {
			continue
		}
		for _, name := range names {
			out = append(out, filepath.Join(base, name))
		}
	}
	return out
}

func rememberLarge(list []largeHit, path string, size int64) []largeHit {
	list = append(list, largeHit{path: path, bytes: size})
	sort.Slice(list, func(i, j int) bool { return list[i].bytes > list[j].bytes })
	if len(list) > maxLargeFiles {
		list = list[:maxLargeFiles]
	}
	return list
}

func formatLarge(list []largeHit) []LargeFile {
	out := make([]LargeFile, 0, len(list))
	for _, item := range list {
		out = append(out, LargeFile{Path: item.path, Name: filepath.Base(item.path), Bytes: item.bytes})
	}
	return out
}

func driveFolders(ctx context.Context, drive string, onProgress func(int, string)) []Folder {
	root := drive
	if root == "" {
		return nil
	}
	if !strings.HasSuffix(root, string(os.PathSeparator)) && !strings.HasSuffix(root, `\`) && !strings.HasSuffix(root, `/`) {
		root += string(os.PathSeparator)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var folders []Folder
	for _, entry := range entries {
		if ctx.Err() != nil {
			break
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if skipDir(entry.Name()) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		size := folderSize(ctx, path, onProgress)
		folders = append(folders, Folder{Name: entry.Name(), Path: path, Bytes: size})
	}
	sort.Slice(folders, func(i, j int) bool { return folders[i].Bytes > folders[j].Bytes })
	if len(folders) > maxFolders {
		folders = folders[:maxFolders]
	}
	return folders
}

func folderSize(ctx context.Context, root string, onProgress func(int, string)) int64 {
	var total int64
	var files int
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
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
		if d.IsDir() && skipDir(d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		files++
		if onProgress != nil && files%400 == 0 {
			onProgress(files, path)
		}
		return nil
	})
	return total
}
