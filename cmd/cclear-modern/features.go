//go:build (production || dev) && windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cclear/internal/appinfo"
	"cclear/internal/apps"
	"cclear/internal/clean"
	"cclear/internal/devspace"
	"cclear/internal/dupes"
	"cclear/internal/history"
	"cclear/internal/sysspace"
	"cclear/internal/syswin"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type CleanItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Group       string   `json:"group"`
	Warning     string   `json:"warning"`
	Note        string   `json:"note"`
	Risk        int      `json:"risk"`
	Count       int      `json:"count"`
	Bytes       int64    `json:"bytes"`
	Unknown     bool     `json:"unknown"`
	DefaultOn   bool     `json:"defaultOn"`
	NeedsAdmin  bool     `json:"needsAdmin"`
	Truncated   bool     `json:"truncated"`
	Errors      []string `json:"errors"`
	Preview     []string `json:"preview"`
}

type CleanRunResult struct {
	Freed     int64    `json:"freed"`
	Estimated int64    `json:"estimated"`
	Deleted   int      `json:"deleted"`
	Skipped   int      `json:"skipped"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors"`
	LogPath   string   `json:"logPath"`
	Cancelled bool     `json:"cancelled"`
	Partial   bool     `json:"partial"`
}

type CleanScanResult struct {
	Items     []CleanItem `json:"items"`
	Cancelled bool        `json:"cancelled"`
	Note      string      `json:"note"`
}

type DuplicateFile struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}
type DuplicateGroup struct {
	Hash  string          `json:"hash"`
	Bytes int64           `json:"bytes"`
	Files []DuplicateFile `json:"files"`
}
type DuplicateResult struct {
	Groups      []DuplicateGroup `json:"groups"`
	Reclaimable int64            `json:"reclaimable"`
	Note        string           `json:"note"`
	Errors      []string         `json:"errors"`
	Truncated   bool             `json:"truncated"`
	Cancelled   bool             `json:"cancelled"`
}

type DuplicateQuery struct {
	Roots   []string `json:"roots"`
	Exclude []string `json:"exclude"`
}

type AppItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Publisher    string `json:"publisher"`
	Location     string `json:"location"`
	Note         string `json:"note"`
	Bytes        int64  `json:"bytes"`
	SizeKnown    bool   `json:"sizeKnown"`
	Kind         string `json:"kind"`
	CanUninstall bool   `json:"canUninstall"`
}

type AppList struct {
	Apps      []AppItem `json:"apps"`
	Note      string    `json:"note"`
	Errors    []string  `json:"errors"`
	Cancelled bool      `json:"cancelled"`
}
type SystemItem struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Status  string `json:"status"`
	Note    string `json:"note"`
	Bytes   int64  `json:"bytes"`
	Known   bool   `json:"known"`
	Group   string `json:"group"`
	Setting string `json:"setting"`
}
type SystemResult struct {
	Items []SystemItem `json:"items"`
	Note  string       `json:"note"`
}
type HistoryItem struct {
	When      string   `json:"when"`
	Version   string   `json:"version"`
	Logical   string   `json:"logical"`
	Estimated string   `json:"estimated"`
	Deleted   int      `json:"deleted"`
	Skipped   int      `json:"skipped"`
	Failed    int      `json:"failed"`
	Items     []string `json:"items"`
}
type SecurityStatus struct {
	Elevated bool `json:"elevated"`
}

type SmartSummary struct {
	When          string   `json:"when"`
	Cancelled     bool     `json:"cancelled"`
	Completed     []string `json:"completed"`
	CleanBytes    int64    `json:"cleanBytes"`
	CleanCount    int      `json:"cleanCount"`
	DiskAllocated int64    `json:"diskAllocated"`
	DiskFiles     int      `json:"diskFiles"`
	DupeBytes     int64    `json:"dupeBytes"`
	DupeGroups    int      `json:"dupeGroups"`
	AppCount      int      `json:"appCount"`
	LargeBytes    int64    `json:"largeBytes"`
	LargeCount    int      `json:"largeCount"`
	SystemCount   int      `json:"systemCount"`
	Note          string   `json:"note"`
}

type TaskState struct {
	Running bool   `json:"running"`
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Phase   string `json:"phase"`
	Detail  string `json:"detail"`
}

type featureState struct {
	mu             sync.Mutex
	clean          []clean.Report
	cleanCancelled bool
	dupes          dupes.Report
	apps           apps.Report
	system         SystemResult
	smart          SmartSummary
}

func (s *featureState) init(ctx context.Context) { _ = ctx }

func (a *App) acquire(kind, label string) (context.Context, error) {
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	return a.gate.Begin(parent, kind, label)
}

func (a *App) release() { a.gate.Release() }

func (a *App) setPhase(phase, detail string) {
	a.gate.Set(phase, detail)
	state := a.gate.State()
	a.emit("task:progress", TaskState{Running: state.Running, Kind: state.Kind, Label: state.Label, Phase: phase, Detail: detail})
}

func (a *App) emit(name string, payload any) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, payload)
}

func (a *App) Task() TaskState {
	state := a.gate.State()
	return TaskState{Running: state.Running, Kind: state.Kind, Label: state.Label, Phase: state.Phase, Detail: state.Detail}
}

func (a *App) CancelToolWork() { a.gate.Cancel() }

func (a *App) currentKind() string { return a.gate.State().Kind }

func (a *App) SecurityStatus() SecurityStatus { return SecurityStatus{Elevated: syswin.IsElevated()} }

func (a *App) RelaunchElevated(confirmation string) error {
	if confirmation != "管理员重启" {
		return errors.New("需要明确确认管理员重启")
	}
	if syswin.IsElevated() {
		return nil
	}
	if err := syswin.RelaunchElevated(); err != nil {
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}

func machineCategories() []clean.Category {
	drive := syswin.SystemDrive()
	env := clean.Env{SystemDrive: drive, SystemRoot: os.Getenv("SystemRoot"), LocalAppData: os.Getenv("LOCALAPPDATA"), ProgramData: os.Getenv("ProgramData"), Temp: os.TempDir(), UserProfile: os.Getenv("USERPROFILE")}
	all := clean.Categories(env)
	out := make([]clean.Category, 0, len(all))
	for _, cat := range all {
		if cat.Visible() {
			out = append(out, cat)
		}
	}
	return out
}

func (a *App) ScanCleanup() (CleanScanResult, error) {
	ctx, err := a.acquire("clean", "垃圾扫描")
	if err != nil {
		return CleanScanResult{}, err
	}
	defer a.release()
	a.setPhase("垃圾候选项", "正在统计可清理候选项")
	return a.scanClean(ctx), nil
}

func (a *App) scanClean(ctx context.Context) CleanScanResult {
	drive := syswin.SystemDrive()
	cats := machineCategories()
	reports := make([]clean.Report, 0, len(cats))
	for index, cat := range cats {
		if ctx.Err() != nil {
			return a.storeClean(reports, true)
		}
		a.setPhase("垃圾候选项", cat.Name)
		a.emit("tool:progress", map[string]any{"kind": "clean", "done": index, "total": len(cats), "label": cat.Name})
		reports = append(reports, clean.ScanCategory(ctx, cat, drive, clean.WinChecker{Drive: drive}, nil))
	}
	a.emit("tool:progress", map[string]any{"kind": "clean", "done": len(cats), "total": len(cats), "label": "扫描完成"})
	return a.storeClean(reports, ctx.Err() != nil)
}

func (a *App) storeClean(reports []clean.Report, cancelled bool) CleanScanResult {
	a.tools.mu.Lock()
	a.tools.clean = append([]clean.Report(nil), reports...)
	a.tools.cleanCancelled = cancelled
	a.tools.mu.Unlock()
	result := CleanScanResult{Items: cleanItems(reports), Cancelled: cancelled}
	if cancelled {
		result.Note = "扫描已取消，以下为已完成的部分结果。"
	}
	if result.Items == nil {
		result.Items = []CleanItem{}
	}
	return result
}

func cleanItems(reports []clean.Report) []CleanItem {
	out := make([]CleanItem, 0, len(reports))
	for _, rep := range reports {
		preview := make([]string, 0, len(rep.Files)+len(rep.Dirs))
		for _, hit := range rep.Files {
			if len(preview) < 40 {
				preview = append(preview, hit.Path)
			}
		}
		for _, hit := range rep.Dirs {
			if len(preview) < 40 {
				preview = append(preview, hit.Path)
			}
		}
		out = append(out, CleanItem{ID: rep.Category.ID, Name: rep.Category.Name, Description: rep.Category.Description, Group: rep.Category.Group, Warning: rep.Category.Warning, Note: rep.Note, Risk: rep.Category.Risk, Count: rep.Count, Bytes: rep.Bytes, Unknown: rep.Unknown, DefaultOn: rep.Category.DefaultOn, NeedsAdmin: rep.Category.NeedsAdmin, Truncated: rep.Truncated, Errors: append([]string(nil), rep.Errors...), Preview: preview})
	}
	return out
}

func (a *App) RunCleanup(ids []string, confirmation string) (CleanRunResult, error) {
	if confirmation != "确认清理" {
		return CleanRunResult{}, errors.New("需要明确确认清理")
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	a.tools.mu.Lock()
	stored := append([]clean.Report(nil), a.tools.clean...)
	a.tools.mu.Unlock()
	var selected []clean.Report
	for _, rep := range stored {
		if wanted[rep.Category.ID] {
			if rep.Category.NeedsAdmin && !syswin.IsElevated() {
				return CleanRunResult{}, fmt.Errorf("%s 需要管理员权限", rep.Category.Name)
			}
			selected = append(selected, rep)
		}
	}
	if len(selected) == 0 {
		return CleanRunResult{}, errors.New("没有已扫描且已选择的清理项")
	}
	ctx, err := a.acquire("clean-run", "垃圾清理")
	if err != nil {
		return CleanRunResult{}, err
	}
	defer a.release()
	log, path, err := createModernLog()
	if err != nil {
		return CleanRunResult{}, err
	}
	defer log.Close()
	fmt.Fprintf(log, "CClear %s\r\nFormat 2\r\n任务 %s\r\n\r\n", appinfo.Version, time.Now().Format(time.RFC3339))
	var result CleanRunResult
	result.LogPath = path
	for index, rep := range selected {
		if ctx.Err() != nil {
			result.Cancelled = true
			result.Partial = true
			return result, nil
		}
		a.setPhase("垃圾清理", rep.Category.Name)
		a.emit("tool:progress", map[string]any{"kind": "clean-run", "done": index, "total": len(selected), "label": rep.Category.Name})
		fmt.Fprintf(log, "---- %s (%s) ----\r\n", rep.Category.Name, rep.Category.ID)
		stats := clean.CleanCategory(ctx, rep, syswin.SystemDrive(), log, nil)
		result.Freed += stats.Freed
		result.Estimated += stats.Estimated
		result.Deleted += stats.Deleted
		result.Skipped += stats.Skipped
		result.Failed += stats.Failed
		result.Errors = append(result.Errors, stats.Errors...)
		fmt.Fprintf(log, "逻辑大小 %s，预计占用 %s，删除 %d，跳过 %d，失败 %d\r\n\r\n", clean.FormatBytes(stats.Freed), clean.FormatBytes(stats.Estimated), stats.Deleted, stats.Skipped, stats.Failed)
		if ctx.Err() != nil {
			result.Cancelled = true
			result.Partial = true
			return result, nil
		}
	}
	a.tools.mu.Lock()
	a.tools.clean = nil
	a.tools.mu.Unlock()
	return result, nil
}

func (a *App) FindDuplicates(query DuplicateQuery) (DuplicateResult, error) {
	ctx, err := a.acquire("dupes", "重复文件")
	if err != nil {
		return DuplicateResult{}, err
	}
	defer a.release()
	a.setPhase("重复文件", "准备扫描范围")
	return a.findDupes(ctx, query), nil
}

func (a *App) findDupes(ctx context.Context, query DuplicateQuery) DuplicateResult {
	roots := make([]string, 0, len(query.Roots))
	for _, root := range query.Roots {
		root = strings.TrimSpace(root)
		if root != "" {
			roots = append(roots, root)
		}
	}
	if len(roots) == 0 {
		roots = []string{syswin.SystemDrive() + `\`}
	}
	rep := dupes.FindScoped(ctx, roots, query.Exclude, func(phase string, files int, where string) {
		label := map[string]string{"enumerate": "枚举", "filter": "候选筛选", "hash": "内容哈希"}[phase]
		if label == "" {
			label = "扫描"
		}
		detail := label
		if where != "" {
			detail += " · " + filepath.Base(where)
		}
		a.setPhase("重复文件", detail)
		a.emit("tool:progress", map[string]any{"kind": "dupes", "phase": phase, "done": files, "total": 0, "label": detail})
	})
	a.tools.mu.Lock()
	a.tools.dupes = rep
	a.tools.mu.Unlock()
	return duplicateResult(rep)
}

func duplicateResult(rep dupes.Report) DuplicateResult {
	out := DuplicateResult{Groups: []DuplicateGroup{}, Reclaimable: rep.Bytes, Note: rep.Note, Errors: append([]string(nil), rep.Errors...), Truncated: rep.Truncated, Cancelled: rep.Cancelled}
	if out.Errors == nil {
		out.Errors = []string{}
	}
	if rep.Cancelled {
		out.Note = "查找已取消，以下为已完成的部分结果。"
		if rep.Note != "" && !strings.Contains(rep.Note, "部分结果") {
			out.Note = strings.TrimSpace(rep.Note + " 以下为已完成的部分结果。")
		}
	}
	for _, group := range rep.Groups {
		row := DuplicateGroup{Hash: group.Hash, Bytes: group.Size}
		for _, file := range group.Files {
			row.Files = append(row.Files, DuplicateFile{Path: file.Path, Bytes: file.Allocated})
		}
		out.Groups = append(out.Groups, row)
	}
	sort.Slice(out.Groups, func(i, j int) bool { return out.Groups[i].Bytes > out.Groups[j].Bytes })
	return out
}

func (a *App) RecycleDuplicate(path, confirmation string) error {
	if confirmation != "移到回收站" {
		return errors.New("需要明确确认移到回收站")
	}
	a.tools.mu.Lock()
	rep := a.tools.dupes
	a.tools.mu.Unlock()
	if err := dupes.CanRemove(rep.Groups, path); err != nil {
		return err
	}
	if err := syswin.RecycleFile(path); err != nil {
		return err
	}
	a.tools.mu.Lock()
	for gi := range a.tools.dupes.Groups {
		files := a.tools.dupes.Groups[gi].Files[:0]
		for _, f := range a.tools.dupes.Groups[gi].Files {
			if !strings.EqualFold(f.Path, path) {
				files = append(files, f)
			}
		}
		a.tools.dupes.Groups[gi].Files = files
	}
	a.tools.mu.Unlock()
	return nil
}

func (a *App) InstalledApps() (AppList, error) {
	ctx, err := a.acquire("apps", "软件信息")
	if err != nil {
		return AppList{}, err
	}
	defer a.release()
	a.setPhase("软件信息", "正在读取卸载登记")
	return a.loadApps(ctx), nil
}

func (a *App) loadApps(ctx context.Context) AppList {
	rep := apps.ListInstalled(ctx)
	a.tools.mu.Lock()
	a.tools.apps = rep
	a.tools.mu.Unlock()
	return appList(rep)
}

func appList(rep apps.Report) AppList {
	out := AppList{Note: rep.Note, Errors: append([]string(nil), rep.Errors...), Cancelled: rep.Cancelled, Apps: make([]AppItem, 0, len(rep.Apps))}
	if out.Errors == nil {
		out.Errors = []string{}
	}
	for index, item := range rep.Apps {
		kind := item.Kind
		if kind == "" {
			kind = "win32"
		}
		out.Apps = append(out.Apps, AppItem{
			ID: strconv.Itoa(index), Name: item.Name, Version: item.Version, Publisher: item.Publisher,
			Location: item.Location, Note: item.Note, Bytes: item.Estimated, SizeKnown: item.EstimatedOK,
			Kind: kind, CanUninstall: !item.System && kind != "system" && item.Uninstall != "",
		})
	}
	return out
}

func (a *App) LaunchUninstall(id, confirmation string) error {
	if confirmation != "启动卸载" {
		return errors.New("需要明确确认启动卸载")
	}
	a.tools.mu.Lock()
	list := append([]apps.App(nil), a.tools.apps.Apps...)
	a.tools.mu.Unlock()
	index, err := strconv.Atoi(id)
	if err != nil || index < 0 || index >= len(list) {
		return errors.New("软件不在刚读取的登记列表中")
	}
	if list[index].System || list[index].Kind == "system" {
		return errors.New("系统组件不提供卸载")
	}
	exe, args, err := apps.UninstallCommand(list[index])
	if err != nil {
		return err
	}
	return syswin.RunCommand(exe, args)
}

func (a *App) ReadSystemTools() (SystemResult, error) {
	ctx, err := a.acquire("system", "系统占用")
	if err != nil {
		return SystemResult{}, err
	}
	defer a.release()
	a.setPhase("系统与开发环境", "正在读取占用")
	return a.readSystem(ctx), nil
}

func (a *App) readSystem(ctx context.Context) SystemResult {
	drive := syswin.SystemDrive()
	sys := sysspace.Read(drive)
	dev := devspace.Scan(ctx, devspace.Env{UserProfile: os.Getenv("USERPROFILE"), LocalAppData: os.Getenv("LOCALAPPDATA"), UserHome: os.Getenv("USERPROFILE")})
	out := SystemResult{Note: strings.TrimSpace(sys.Note + " " + dev.Note), Items: []SystemItem{}}
	if ctx.Err() != nil {
		out.Note = strings.TrimSpace("读取已取消，以下为已完成的部分结果。" + out.Note)
	}
	for _, item := range sys.Items {
		row := SystemItem{Name: item.Name, Kind: "系统", Path: item.Path, Status: item.Status, Note: item.Note, Bytes: item.Bytes, Known: item.Known}
		row.Group, row.Setting = systemGroup(row)
		out.Items = append(out.Items, row)
	}
	for _, item := range dev.Items {
		row := SystemItem{Name: item.Name, Kind: item.Kind, Path: item.Path, Status: "只读", Note: item.Note, Bytes: item.Bytes, Known: true}
		row.Group, row.Setting = systemGroup(row)
		out.Items = append(out.Items, row)
	}
	a.tools.mu.Lock()
	a.tools.system = out
	a.tools.mu.Unlock()
	return out
}

func systemGroup(item SystemItem) (string, string) {
	name := strings.ToLower(item.Name + " " + item.Path + " " + item.Kind)
	switch {
	case strings.Contains(name, "pagefile") || strings.Contains(name, "hiberfil") || strings.Contains(name, "swapfile"):
		return "虚拟内存和休眠文件", "pagefile"
	case strings.Contains(name, "docker"):
		return "Docker", ""
	case strings.Contains(name, "wsl") || strings.Contains(name, "ext4.vhdx"):
		return "WSL", ""
	case strings.Contains(item.Kind, "缓存"):
		return "开发工具缓存", ""
	case item.Kind == "系统":
		return "Windows 系统文件", "storage"
	default:
		return "其他开发环境", ""
	}
}

func (a *App) CleanupHistory() ([]HistoryItem, error) {
	records, err := history.Load(modernLogDir(), 20)
	if err != nil {
		return nil, err
	}
	out := make([]HistoryItem, 0, len(records))
	for _, rec := range records {
		out = append(out, HistoryItem{When: history.FormatWhen(rec.When), Version: rec.Version, Logical: rec.Logical, Estimated: rec.Estimated, Deleted: rec.Deleted, Skipped: rec.Skipped, Failed: rec.Failed, Items: append([]string(nil), rec.Items...)})
	}
	return out, nil
}

func modernLogDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "CClear", "logs")
}
func createModernLog() (*os.File, string, error) {
	dir := modernLogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, "clean-"+time.Now().Format("20060102-150405")+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	return file, path, err
}

var _ io.Writer = (*os.File)(nil)
