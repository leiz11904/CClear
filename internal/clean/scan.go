package clean

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cclear/internal/winpath"
)

// IsReparse reports a junction or symlink. Windows replaces this at startup.
var IsReparse = func(string) bool { return false }

// Checker is the path allowlist used while scanning and deleting.
type Checker interface {
	ValidateRoot(root string) error
	ValidateDelete(path, root string, isDir bool) error
}

// WinChecker allows only the built-in Windows cleanup locations.
type WinChecker struct {
	Drive string
}

func (w WinChecker) ValidateRoot(root string) error {
	return winpath.ValidateRoot(root, w.Drive)
}

func (w WinChecker) ValidateDelete(path, root string, isDir bool) error {
	return winpath.ValidateDelete(path, root, w.Drive, isDir)
}

// SkipError means a file could not be removed because it is in use or protected.
type SkipError struct{ Err error }

func (e SkipError) Error() string {
	if e.Err == nil {
		return "已跳过"
	}
	return e.Err.Error()
}

func (e SkipError) Unwrap() error { return e.Err }

// FileHit is one file selected for deletion.
type FileHit struct {
	Path      string
	Size      int64
	Allocated int64
	ModUnix   int64
	Links     uint32
	Root      string
	ID        Identity
}

// DirHit is a directory that may be removed after its files are gone.
type DirHit struct {
	Path string
	Root string
	ID   Identity
}

// RootResult is the scan of a single directory.
type RootResult struct {
	Files           []FileHit
	Dirs            []DirHit
	Count           int
	Logical         int64
	Reclaimable     int64
	SkippedRecent   int
	SkippedIdentity int
	DirsDropped     bool
	Errors          []string
}

// ScanOptions controls one directory scan.
type ScanOptions struct {
	Now           time.Time
	MinAge        time.Duration
	RemoveRoot    bool
	ProgressEvery int
	MaxFiles      int
	MaxDirs       int
	SkipDirs      bool
	Skip          map[string]bool
	OnProgress    func(files int)
}

// ScanRoot lists files under root that the checker allows.
// It does not follow symlinks or junctions.
func ScanRoot(ctx context.Context, root string, check Checker, opt ScanOptions) (RootResult, error) {
	var result RootResult
	if check == nil {
		return result, errors.New("缺少路径检查")
	}
	if err := check.ValidateRoot(root); err != nil {
		return result, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return result, err
	}
	if info.Mode()&os.ModeSymlink != 0 || IsReparse(root) {
		return result, errors.New("已跳过联接点或快捷方式")
	}
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	if opt.ProgressEvery <= 0 {
		opt.ProgressEvery = 200
	}
	if !info.IsDir() {
		if opt.MinAge > 0 && opt.Now.Sub(info.ModTime()) < opt.MinAge {
			result.SkippedRecent++
			return result, nil
		}
		if err := check.ValidateDelete(root, root, false); err != nil {
			if errors.Is(err, winpath.ErrFileKind) {
				return result, nil
			}
			return result, err
		}
		snap, err := ReadSnapshot(root, false)
		if err != nil || !snap.ID.Valid {
			result.SkippedIdentity++
			return result, nil
		}
		if opt.MinAge > 0 && opt.Now.Sub(time.Unix(snap.ModUnix, 0)) < opt.MinAge {
			result.SkippedRecent++
			return result, nil
		}
		result.remember(FileHit{
			Path: root, Size: snap.Size, Allocated: snap.Allocated, ModUnix: snap.ModUnix,
			Links: snap.Links, Root: root, ID: snap.ID,
		}, opt.MaxFiles)
		return result, nil
	}

	seen := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			result.Errors = append(result.Errors, walkErr.Error())
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || IsReparse(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		same := strings.EqualFold(path, root)
		if d.IsDir() {
			if err := check.ValidateDelete(path, root, true); err != nil {
				if !same && !errors.Is(err, winpath.ErrFileKind) && !errors.Is(err, winpath.ErrRoot) {
					result.Errors = append(result.Errors, err.Error())
				}
				if same {
					return nil
				}
				return filepath.SkipDir
			}
			snap, err := ReadSnapshot(path, true)
			if err != nil || !snap.ID.Valid {
				result.SkippedIdentity++
				if same {
					return errors.New("无法确认目录身份")
				}
				return filepath.SkipDir
			}
			if !opt.SkipDirs && (!same || opt.RemoveRoot) {
				result.rememberDir(DirHit{Path: path, Root: root, ID: snap.ID}, opt.MaxDirs)
			}
			return nil
		}
		if opt.Skip[path] {
			return nil
		}
		if err := check.ValidateDelete(path, root, false); err != nil {
			if errors.Is(err, winpath.ErrFileKind) {
				return nil
			}
			result.Errors = append(result.Errors, err.Error())
			return nil
		}
		snap, err := ReadSnapshot(path, false)
		if err != nil || !snap.ID.Valid {
			result.SkippedIdentity++
			return nil
		}
		if opt.MinAge > 0 && opt.Now.Sub(time.Unix(snap.ModUnix, 0)) < opt.MinAge {
			result.SkippedRecent++
			return nil
		}
		hit := FileHit{
			Path: path, Size: snap.Size, Allocated: snap.Allocated, ModUnix: snap.ModUnix,
			Links: snap.Links, Root: root, ID: snap.ID,
		}
		result.remember(hit, opt.MaxFiles)
		seen++
		if opt.OnProgress != nil && seen%opt.ProgressEvery == 0 {
			opt.OnProgress(seen)
		}
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		return result, err
	}
	sort.Slice(result.Dirs, func(i, j int) bool {
		return len(result.Dirs[i].Path) > len(result.Dirs[j].Path)
	})
	return result, err
}

func (result *RootResult) remember(hit FileHit, maxFiles int) {
	result.Count++
	result.Logical += hit.Size
	result.Reclaimable += Reclaimable(hit.Size, hit.Allocated, hit.Links)
	if maxFiles > 0 && len(result.Files) >= maxFiles {
		return
	}
	result.Files = append(result.Files, hit)
}

func (result *RootResult) rememberDir(hit DirHit, maxDirs int) {
	if maxDirs > 0 && len(result.Dirs) >= maxDirs {
		result.DirsDropped = true
		shortest := 0
		for i := 1; i < len(result.Dirs); i++ {
			if len(result.Dirs[i].Path) < len(result.Dirs[shortest].Path) {
				shortest = i
			}
		}
		if len(hit.Path) > len(result.Dirs[shortest].Path) {
			result.Dirs[shortest] = hit
		}
		return
	}
	result.Dirs = append(result.Dirs, hit)
}

func (result RootResult) truncated() bool {
	return result.DirsDropped || result.Count > len(result.Files)
}

// Deleter removes one file or an empty directory.
// id and size are the snapshot from scan. The implementation must open the
// object once, compare that snapshot, and delete that same handle.
type Deleter func(path, root string, isDir bool, id Identity, size int64) error

// DeleteStats counts a cleanup pass.
type DeleteStats struct {
	Freed     int64
	Estimated int64
	Deleted   int
	Skipped   int
	Failed    int
	Errors    []string
}

// Delete removes files and then empty directories. Directory paths must be
// ordered deepest first.
func Delete(ctx context.Context, files []FileHit, dirs []DirHit, delete Deleter, progress func(done, total int)) DeleteStats {
	var stats DeleteStats
	total := len(files) + len(dirs)
	done := 0
	report := func() {
		if progress != nil {
			progress(done, total)
		}
	}
	for _, file := range files {
		if ctx.Err() != nil {
			stats.Errors = append(stats.Errors, "已取消")
			return stats
		}
		if !file.ID.Valid || file.Links != 1 {
			stats.Skipped++
			done++
			continue
		}
		if err := delete(file.Path, file.Root, false, file.ID, file.Size); err != nil {
			noteDeleteError(&stats, err, false)
		} else {
			stats.Deleted++
			stats.Freed += file.Size
			stats.Estimated += Reclaimable(file.Size, file.Allocated, file.Links)
		}
		done++
		if done%50 == 0 {
			report()
		}
	}
	for _, dir := range dirs {
		if ctx.Err() != nil {
			stats.Errors = append(stats.Errors, "已取消")
			return stats
		}
		if !dir.ID.Valid {
			stats.Skipped++
			done++
			continue
		}
		if err := delete(dir.Path, dir.Root, true, dir.ID, 0); err != nil {
			noteDeleteError(&stats, err, true)
		} else {
			stats.Deleted++
		}
		done++
	}
	report()
	return stats
}

func noteDeleteError(stats *DeleteStats, err error, isDir bool) {
	if errors.Is(err, winpath.ErrOutside) || errors.Is(err, winpath.ErrForbidden) || errors.Is(err, winpath.ErrDrive) || errors.Is(err, winpath.ErrEscape) {
		stats.Failed++
		if len(stats.Errors) < 20 {
			stats.Errors = append(stats.Errors, err.Error())
		}
		return
	}
	var skip SkipError
	if isDir || errors.As(err, &skip) || errors.Is(err, os.ErrNotExist) {
		stats.Skipped++
		return
	}
	stats.Failed++
	if len(stats.Errors) < 20 {
		stats.Errors = append(stats.Errors, err.Error())
	}
}

// OSDeleter removes a path with os.Remove after the checker accepts it.
// Tests use it. Windows cleanup uses a handle-checked deleter instead.
func OSDeleter(check Checker) Deleter {
	return func(path, root string, isDir bool, id Identity, size int64) error {
		if !id.Valid {
			return SkipError{Err: errNoIdentity}
		}
		if err := check.ValidateDelete(path, root, isDir); err != nil {
			return err
		}
		snap, err := ReadSnapshot(path, isDir)
		if err != nil || !id.Same(snap.ID) {
			return SkipError{Err: errors.New("文件已变化或无法确认身份")}
		}
		if !isDir && (snap.Links != 1 || snap.Size != size) {
			return SkipError{Err: errors.New("文件已变化或还有其他硬链接")}
		}
		if IsReparse(path) {
			return SkipError{Err: errors.New("跳过联接点或快捷方式")}
		}
		return os.Remove(path)
	}
}
