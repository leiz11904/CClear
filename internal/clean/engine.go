package clean

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"cclear/internal/winpath"
)

// ProcessRunning reports whether a process image is running. Windows replaces it.
var ProcessRunning = func(string) bool { return false }

// WindowsUpdateBusy reports an install or a pending reboot. Windows replaces it.
var WindowsUpdateBusy = func() bool { return false }

// RemoveOldWindows deletes a previous installation through Windows Disk Cleanup.
var RemoveOldWindows func(ctx context.Context, log io.Writer) error

// Hooks are filled in on Windows. They stay nil in tests.
var (
	QueryRecycle        func(drive string) (size int64, items int64, err error)
	EmptyRecycle        func(drive string) error
	RunComponentCleanup func(ctx context.Context, log io.Writer) error
	PauseWindowsUpdate  func() (resume func(), err error)
	SecureDelete        func(path, root, drive string, isDir bool, id Identity, size int64) error
)

// PathChecker is the allowlist used when a cleanup pass scans again.
// Tests replace it. Production uses the Windows allowlist.
var PathChecker = func(drive string) Checker { return WinChecker{Drive: drive} }

const (
	// PreviewLimit is the number of candidate paths kept for the detail view.
	PreviewLimit = 200
	// MaxStoredDirs bounds directory paths kept beside that preview.
	MaxStoredDirs = 2000
	// CleanBatch is how many files one cleanup pass holds before deleting them.
	CleanBatch = 400
	// DetailPageSize is how many preview rows one detail page renders.
	DetailPageSize = 50
)

// Report is the scan result shown for one category.
type Report struct {
	Category        Category
	Files           []FileHit
	Dirs            []DirHit
	Bytes           int64
	Logical         int64
	Reclaimable     int64
	Count           int
	SkippedRecent   int
	Unknown         bool
	Truncated       bool
	Note            string
	Errors          []string
	RecycleItems    int64
	SkippedIdentity int
}

// ScanCategory measures one category. It does not delete anything.
func ScanCategory(ctx context.Context, cat Category, drive string, check Checker, onFiles func(int)) Report {
	report := Report{Category: cat}
	if blocked := runningSkip(cat); blocked != "" {
		report.Note = blocked
		return report
	}
	if cat.Special == "dism" || cat.Special == "windowsold" {
		report.Unknown = true
		report.Bytes = -1
		if cat.Special == "windowsold" {
			report.Note = "将调用 Windows 磁盘清理删除旧版系统，本程序不会逐个删除其中的文件。"
		} else {
			report.Note = "先分析组件存储，再由 Windows 清理已被取代的组件。不会删除 WinSxS，也不会执行 ResetBase。"
		}
		return report
	}
	if cat.Special == "recycle" {
		if QueryRecycle == nil {
			report.Note = "无法查询回收站"
			return report
		}
		size, items, err := QueryRecycle(drive)
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
			return report
		}
		report.Bytes = size
		report.Count = int(items)
		report.RecycleItems = items
		return report
	}

	roots := append([]string{}, cat.Roots...)
	if cat.Discover != nil {
		roots = append(roots, cat.Discover()...)
	}
	seen := map[string]bool{}
	opt := ScanOptions{
		Now:        time.Now(),
		MinAge:     cat.MinAge,
		RemoveRoot: cat.RemoveRoot,
		MaxFiles:   PreviewLimit,
		MaxDirs:    MaxStoredDirs,
		OnProgress: onFiles,
	}
	for _, root := range roots {
		if ctx.Err() != nil {
			report.Errors = append(report.Errors, "已取消")
			break
		}
		key := root
		if seen[key] {
			continue
		}
		seen[key] = true
		got, err := ScanRoot(ctx, root, check, opt)
		if err != nil {
			if isMissing(err) {
				continue
			}
			report.Errors = append(report.Errors, err.Error())
			continue
		}
		report.Files = append(report.Files, got.Files...)
		report.Dirs = append(report.Dirs, got.Dirs...)
		report.SkippedRecent += got.SkippedRecent
		report.SkippedIdentity += got.SkippedIdentity
		report.Count += got.Count
		report.Logical += got.Logical
		report.Reclaimable += got.Reclaimable
		if got.truncated() {
			report.Truncated = true
		}
		for _, item := range got.Errors {
			if len(report.Errors) < 20 {
				report.Errors = append(report.Errors, item)
			}
		}
	}
	if report.Count == 0 {
		report.Count = len(report.Files)
	}
	report.Bytes = report.Reclaimable
	return report
}

func runningSkip(cat Category) string {
	for _, name := range cat.SkipProcesses {
		if ProcessRunning(name) {
			return name + " 正在运行，已跳过相关缓存，不会结束该程序。"
		}
	}
	return ""
}

// CleanCategory deletes what ScanCategory selected.
func CleanCategory(ctx context.Context, report Report, drive string, log io.Writer, progress func(done, total int)) DeleteStats {
	if log == nil {
		log = io.Discard
	}
	switch report.Category.Special {
	case "dism":
		return cleanDISM(ctx, log)
	case "recycle":
		return cleanRecycle(drive, report, log)
	case "winupdate":
		return cleanUpdateCache(ctx, report, drive, log, progress)
	case "windowsold":
		return cleanOldWindows(ctx, log)
	default:
		if runningSkip(report.Category) != "" {
			return DeleteStats{Skipped: 1, Errors: []string{runningSkip(report.Category)}}
		}
		return deleteReport(ctx, report, drive, log, progress)
	}
}

func cleanDISM(ctx context.Context, log io.Writer) DeleteStats {
	var stats DeleteStats
	if WindowsUpdateBusy() {
		stats.Skipped++
		stats.Errors = append(stats.Errors, "Windows 更新正在进行或等待重启，已跳过组件清理。")
		return stats
	}
	if RunComponentCleanup == nil {
		stats.Failed++
		stats.Errors = append(stats.Errors, "当前系统不能执行组件清理")
		return stats
	}
	if err := RunComponentCleanup(ctx, log); err != nil {
		stats.Failed++
		stats.Errors = append(stats.Errors, err.Error())
		return stats
	}
	stats.Deleted++
	return stats
}

func cleanRecycle(drive string, report Report, log io.Writer) DeleteStats {
	var stats DeleteStats
	if EmptyRecycle == nil {
		stats.Failed++
		stats.Errors = append(stats.Errors, "当前系统不能清空回收站")
		return stats
	}
	if err := EmptyRecycle(drive); err != nil {
		stats.Failed++
		stats.Errors = append(stats.Errors, err.Error())
		return stats
	}
	stats.Freed = report.Bytes
	stats.Deleted = report.Count
	_, _ = io.WriteString(log, "已清空回收站\r\n")
	return stats
}

func cleanUpdateCache(ctx context.Context, report Report, drive string, log io.Writer, progress func(done, total int)) DeleteStats {
	if WindowsUpdateBusy() {
		return DeleteStats{Skipped: 1, Errors: []string{"Windows 更新正在进行或等待重启，已跳过下载缓存。"}}
	}
	_, _ = io.WriteString(log, "不停止 Windows Update 服务。服务正在运行时这一项会直接跳过。\r\n")
	return deleteReport(ctx, report, drive, log, progress)
}

func cleanOldWindows(ctx context.Context, log io.Writer) DeleteStats {
	var stats DeleteStats
	if WindowsUpdateBusy() {
		stats.Skipped++
		stats.Errors = append(stats.Errors, "Windows 更新正在进行或等待重启，已跳过旧版 Windows。")
		return stats
	}
	if RemoveOldWindows == nil {
		stats.Failed++
		stats.Errors = append(stats.Errors, "当前系统不能通过官方接口删除旧版 Windows")
		return stats
	}
	if err := RemoveOldWindows(ctx, log); err != nil {
		stats.Failed++
		stats.Errors = append(stats.Errors, err.Error())
		return stats
	}
	stats.Deleted++
	return stats
}

func deleteReport(ctx context.Context, report Report, drive string, log io.Writer, progress func(done, total int)) DeleteStats {
	if SecureDelete == nil {
		return DeleteStats{Failed: 1, Errors: []string{"缺少安全删除，已取消这次清理"}}
	}
	if report.Truncated {
		return streamClean(ctx, report, drive, log, progress)
	}
	return Delete(ctx, report.Files, report.Dirs, logDelete(drive, log, nil), progress)
}

func logDelete(drive string, log io.Writer, known map[string]FileHit) Deleter {
	return func(path, root string, isDir bool, id Identity, size int64) error {
		if prev, ok := known[path]; ok && !isDir {
			if size != prev.Size {
				return SkipError{Err: errors.New("文件已变化，已跳过")}
			}
			id = prev.ID
			size = prev.Size
		}
		err := SecureDelete(path, root, drive, isDir, id, size)
		if err == nil && !isDir {
			_, _ = io.WriteString(log, RedactPath(path)+"\r\n")
		}
		return err
	}
}

func streamClean(ctx context.Context, report Report, drive string, log io.Writer, progress func(done, total int)) DeleteStats {
	known := make(map[string]FileHit, len(report.Files))
	for _, file := range report.Files {
		known[file.Path] = file
	}
	report.Files = nil
	report.Dirs = nil
	check := PathChecker(drive)
	skip := map[string]bool{}
	var stats DeleteStats
	roots := append([]string{}, report.Category.Roots...)
	if report.Category.Discover != nil {
		roots = append(roots, report.Category.Discover()...)
	}
	seen := map[string]bool{}
	done := 0
	for _, root := range roots {
		if seen[root] {
			continue
		}
		seen[root] = true
		for round := 0; round < 10000; round++ {
			if ctx.Err() != nil {
				stats.Errors = append(stats.Errors, "已取消")
				return stats
			}
			got, err := ScanRoot(ctx, root, check, ScanOptions{
				Now:      time.Now(),
				MinAge:   report.Category.MinAge,
				MaxFiles: CleanBatch,
				SkipDirs: true,
				Skip:     skip,
			})
			if err != nil && !errors.Is(err, context.Canceled) {
				if len(stats.Errors) < 20 {
					stats.Errors = append(stats.Errors, err.Error())
				}
				break
			}
			if len(got.Files) == 0 {
				break
			}
			before := stats.Deleted
			batch := Delete(ctx, got.Files, nil, logDelete(drive, log, known), nil)
			mergeStats(&stats, batch)
			done += len(got.Files)
			if progress != nil {
				progress(done, report.Count)
			}
			if stats.Deleted == before {
				if len(skip)+len(got.Files) > 10000 {
					stats.Errors = append(stats.Errors, "未处理的文件过多，已停止这一项")
					return stats
				}
				for _, file := range got.Files {
					skip[file.Path] = true
				}
			}
			got.Files = nil
		}
		for round := 0; round < 10000; round++ {
			if ctx.Err() != nil {
				stats.Errors = append(stats.Errors, "已取消")
				return stats
			}
			got, err := ScanRoot(ctx, root, check, ScanOptions{
				Now:      time.Now(),
				MaxFiles: 1,
				MaxDirs:  CleanBatch,
			})
			if err != nil && !errors.Is(err, context.Canceled) {
				break
			}
			if len(got.Dirs) == 0 {
				got.Files = nil
				break
			}
			before := stats.Deleted
			batch := Delete(ctx, nil, got.Dirs, logDelete(drive, log, nil), nil)
			mergeStats(&stats, batch)
			got.Files = nil
			got.Dirs = nil
			if stats.Deleted == before {
				break
			}
		}
	}
	if progress != nil {
		total := report.Count
		if done > total {
			total = done
		}
		progress(done, total)
	}
	return stats
}

func mergeStats(dst *DeleteStats, src DeleteStats) {
	dst.Freed += src.Freed
	dst.Estimated += src.Estimated
	dst.Deleted += src.Deleted
	dst.Skipped += src.Skipped
	dst.Failed += src.Failed
	for _, err := range src.Errors {
		if len(dst.Errors) < 20 {
			dst.Errors = append(dst.Errors, err)
		}
	}
}

func isMissing(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

// Guarded reports whether a delete would be rejected before any system call.
func Guarded(path, root, drive string, isDir bool) error {
	return winpath.ValidateDelete(path, root, drive, isDir)
}
