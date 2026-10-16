//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"cclear/internal/appinfo"
	"cclear/internal/apps"
	"cclear/internal/clean"
	"cclear/internal/devspace"
	"cclear/internal/dupes"
	"cclear/internal/history"
	"cclear/internal/prefs"
	"cclear/internal/snapshot"
	"cclear/internal/space"
	"cclear/internal/sysspace"
	"cclear/internal/syswin"
	"cclear/internal/ui"
	"cclear/internal/winptr"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

const (
	idPagePrev = 1101
	idPageNext = 1102

	wmScanDone    = win.WM_APP + 1
	wmCleanDone   = win.WM_APP + 2
	wmAnalyzeDone = win.WM_APP + 3
	wmDupesDone   = win.WM_APP + 4
	wmAppsDone    = win.WM_APP + 5
	wmSysDone     = win.WM_APP + 6

	phaseIdle = iota
	phaseScan
	phaseReady
	phaseClean
	phaseAnalyze
	phaseDupes
	phaseApps
	phaseSys
)

type app struct {
	win          win.HWND
	hwnd         atomic.Uintptr
	detail       win.HWND
	detailPage   int
	detailIndex  int
	font         win.HFONT
	titleFont    win.HFONT
	subtitleFont win.HFONT
	smallFont    win.HFONT
	types        typekit
	pageBrush    win.HBRUSH
	cardBrush    win.HBRUSH
	dpi          int

	drive      string
	elevated   bool
	total      uint64
	free       uint64
	freeBefore uint64
	page       int
	scroll     int
	contentH   int
	scanned    bool
	expanded   map[string]bool

	mu            sync.Mutex
	cancel        context.CancelFunc
	generation    int
	phase         int
	statusText    string
	progDone      int
	progTotal     int
	scanName      string
	cleanName     string
	reports       []clean.Report
	scanning      []clean.Report
	pending       []clean.Report
	pendingNote   string
	pendingGen    int
	pendingCancel bool
	choices       map[string]bool
	logPath       string
	result        string
	cleanStarted  time.Time
	space         space.Report
	dupes         dupes.Report
	apps          apps.Report
	sys           sysspace.Report
	dev           devspace.Report
	snaps         []snapshot.Record
	treeOpen      map[string]bool
	selected      string
	analyzeTab    int
	treeScroll    int
	treeCount     int
	treeShown     int
	hotBtn        win.HWND
	leaveTracked  win.HWND
	tip           win.HWND
	tipHot        win.HWND
	tipLines      []string
	largeMin      int64
	largeSort     string
	spaceOK       bool
	dupesOK       bool
	appsOK        bool
	sysOK         bool
	history       []history.Record
	settingCats   []clean.Category
	host          win.HWND
	scrollBar     win.HWND
	viewH         int
	painting      bool
	navBtns       []win.HWND
	checkBtns     []win.HWND
	checkLaid     []placedCheck
	detailBtns    []win.HWND
	detailLaid    []placedBtn
	actBtns       []win.HWND
	actLaid       []actionPlace
	btnScan       win.HWND
	btnClean      win.HWND
	btnSelect     win.HWND
	btnWork       win.HWND
	btnAdmin      win.HWND
	selRC         win.RECT
	selShow       bool
	dark          bool
	logKeepMB     int
	scanAt        time.Time
	scanFailed    bool
	advancedOpen  bool
	cleanOut      cleanOutcome
}

type cleanOutcome struct {
	show, detail, restart    bool
	logical, estimated       int64
	diskDelta                int64
	deleted, skipped, failed int
	notes                    []string
	logPath                  string
}

func main() {
	runtime.LockOSThread()
	setDPIAware()
	if !syswin.Windows10OrNewer() {
		win.MessageBox(0, u16("CClear 需要 Windows 10 或更高版本。"), u16(appinfo.Name), win.MB_ICONWARNING)
		return
	}
	ok, err := syswin.AcquireSingleInstance()
	if err != nil {
		win.MessageBox(0, u16("无法启动："+err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
		return
	}
	if !ok {
		activateExisting()
		return
	}
	stored := prefs.Load(prefBase())
	a := &app{
		drive:       syswin.SystemDrive(),
		elevated:    syswin.IsElevated(),
		dpi:         96,
		phase:       phaseIdle,
		choices:     stored.Choices,
		expanded:    map[string]bool{},
		treeOpen:    map[string]bool{},
		largeMin:    space.DefaultLargeMin,
		largeSort:   "size",
		settingCats: visibleCategories(syswin.SystemDrive()),
		dark:        stored.Theme == "dark",
		logKeepMB:   prefs.ClampLogKeepMB(stored.LogKeepMB),
	}
	applyPalette(ui.Named(stored.Theme))
	if a.choices == nil {
		a.choices = map[string]bool{}
	}
	riskOf := map[string]int{}
	for _, cat := range a.settingCats {
		riskOf[cat.ID] = cat.Risk
	}
	clean.ClampChoices(a.choices, riskOf)
	a.persistChoices()
	_ = clean.SanitizeLogDir(logDir())
	_ = snapshot.SanitizeDir(prefBase())
	a.refreshDisk()
	if err := a.run(); err != nil {
		win.MessageBox(0, u16(err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
	}
}

func activateExisting() {
	hwnd := win.FindWindow(u16(classMain), nil)
	if hwnd == 0 {
		return
	}
	win.ShowWindow(hwnd, win.SW_RESTORE)
	win.SetForegroundWindow(hwnd)
}

func (a *app) run() error {
	uiPtr = a
	var icc win.INITCOMMONCONTROLSEX
	icc.DwSize = uint32(unsafe.Sizeof(icc))
	icc.DwICC = win.ICC_STANDARD_CLASSES
	win.InitCommonControlsEx(&icc)

	inst := win.GetModuleHandle(nil)
	cursor := win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW))
	a.pageBrush = solidBrush(colorBg)
	a.cardBrush = solidBrush(colorCard)
	icon := loadAppIcon(inst)

	cls := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		Style:         win.CS_HREDRAW | win.CS_VREDRAW | win.CS_DBLCLKS,
		LpfnWndProc:   windows.NewCallback(mainWndProc),
		HInstance:     inst,
		HCursor:       cursor,
		HbrBackground: a.pageBrush,
		LpszClassName: u16(classMain),
		HIcon:         icon,
		HIconSm:       icon,
	}
	if win.RegisterClassEx(&cls) == 0 {
		return fmt.Errorf("无法注册窗口")
	}
	detailClass := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   detailProcPtr,
		HInstance:     inst,
		HCursor:       cursor,
		HbrBackground: a.pageBrush,
		LpszClassName: u16(classDetail),
		HIcon:         icon,
		HIconSm:       icon,
	}
	win.RegisterClassEx(&detailClass)
	contentClass := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   windows.NewCallback(contentProc),
		HInstance:     inst,
		HCursor:       cursor,
		HbrBackground: a.pageBrush,
		LpszClassName: u16(classContent),
		HIcon:         icon,
		HIconSm:       icon,
	}
	win.RegisterClassEx(&contentClass)
	tipClass := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		Style:         0x00020000,
		LpfnWndProc:   windows.NewCallback(tipProc),
		HInstance:     inst,
		HCursor:       cursor,
		HbrBackground: a.cardBrush,
		LpszClassName: u16(classTip),
	}
	win.RegisterClassEx(&tipClass)

	a.win = win.CreateWindowEx(
		0x00010000, // WS_EX_CONTROLPARENT
		u16(classMain),
		u16(appinfo.Title),
		win.WS_OVERLAPPEDWINDOW|win.WS_CLIPCHILDREN,
		win.CW_USEDEFAULT, win.CW_USEDEFAULT, 1100, 720,
		0, 0, inst, nil,
	)
	if a.win == 0 {
		return fmt.Errorf("无法创建窗口")
	}
	a.tip = win.CreateWindowEx(
		0x08000088, // WS_EX_NOACTIVATE | WS_EX_TOOLWINDOW | WS_EX_TOPMOST
		u16(classTip),
		u16(""),
		0x80000000, // WS_POPUP
		0, 0, 0, 0,
		a.win, 0, inst, nil,
	)
	a.hwnd.Store(uintptr(a.win))
	a.host = win.CreateWindowEx(
		0x00010000, // WS_EX_CONTROLPARENT
		u16(classContent),
		u16(""),
		win.WS_CHILD|win.WS_VISIBLE|win.WS_CLIPCHILDREN|win.WS_CLIPSIBLINGS,
		0, 0, 10, 10,
		a.win, 0, inst, nil,
	)
	a.scrollBar = win.CreateWindowEx(
		0,
		u16("SCROLLBAR"),
		u16("页面滚动"),
		win.WS_CHILD|win.WS_TABSTOP|sbsVert,
		0, 0, 10, 10,
		a.win, win.HMENU(idScroll), inst, nil,
	)
	setImmersiveDark(a.win, a.dark)
	if dpi := win.GetDpiForWindow(a.win); dpi != 0 {
		a.dpi = int(dpi)
	}
	win.SetWindowPos(a.win, 0, 0, 0, int32(a.scale(1100)), int32(a.scale(720)), win.SWP_NOMOVE|win.SWP_NOZORDER)
	roundWindow(a.win)
	a.createFonts()
	win.ShowWindow(a.win, win.SW_SHOW)
	win.UpdateWindow(a.win)
	win.SetTimer(a.win, 1, 100, 0)
	a.setStatus("选择“开始扫描”后，会显示真实的可释放空间。")

	var msg win.MSG
	for {
		ret := win.GetMessage(&msg, 0, 0, 0)
		if ret == 0 || ret == -1 {
			break
		}
		handled, _, _ := user32.NewProc("IsDialogMessageW").Call(uintptr(a.win), uintptr(unsafe.Pointer(&msg)))
		if handled != 0 {
			continue
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
	return nil
}

func (a *app) wndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_SIZE:
		a.redraw()
		return 0
	case win.WM_DPICHANGED:
		a.dpi = int(uint32(wParam) & 0xFFFF)
		if a.dpi == 0 {
			a.dpi = 96
		}
		a.createFonts()
		if lParam != 0 {
			rc := (*win.RECT)(winptr.Ptr(lParam))
			win.SetWindowPos(hwnd, 0, rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, win.SWP_NOZORDER|win.SWP_NOACTIVATE)
		}
		a.redraw()
		return 0
	case win.WM_GETMINMAXINFO:
		info := (*win.MINMAXINFO)(winptr.Ptr(lParam))
		info.PtMinTrackSize.X = int32(a.scale(960))
		info.PtMinTrackSize.Y = int32(a.scale(640))
		return 0
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_PAINT:
		a.paint()
		return 0
	case win.WM_COMMAND:
		a.onCommand(wParam, lParam)
		return 0
	case win.WM_VSCROLL:
		a.onVScroll(wParam)
		return 0
	case win.WM_SETTINGCHANGE, win.WM_SYSCOLORCHANGE, 0x031A: // WM_THEMECHANGED
		a.redraw()
		return 0
	case win.WM_CTLCOLORBTN, win.WM_CTLCOLORSTATIC:
		if a.isCheck(win.HWND(lParam)) {
			hdc := win.HDC(wParam)
			win.SetTextColor(hdc, colorText)
			win.SetBkColor(hdc, colorCard)
			win.SetBkMode(hdc, win.TRANSPARENT)
			if a.cardBrush != 0 {
				return uintptr(a.cardBrush)
			}
		}
	case win.WM_MOUSEWHEEL:
		a.onWheel(int16(wParam >> 16))
		return 0
	case win.WM_TIMER:
		if a.busy() {
			a.redraw()
		}
		return 0
	case wmScanDone:
		a.applyScan()
		return 0
	case wmCleanDone:
		a.applyClean()
		return 0
	case wmAnalyzeDone:
		a.applyAnalyze()
		return 0
	case wmDupesDone:
		a.applyDupes()
		return 0
	case wmAppsDone:
		a.applyApps()
		return 0
	case wmSysDone:
		a.applySys()
		return 0
	case win.WM_CLOSE:
		if a.busy() {
			if win.MessageBox(hwnd, u16("任务还在进行。确定要退出吗？"), u16(appinfo.Name), win.MB_YESNO|win.MB_ICONWARNING|win.MB_DEFBUTTON2) != win.IDYES {
				return 0
			}
			a.stopWork()
		}
		win.DestroyWindow(hwnd)
		return 0
	case win.WM_DESTROY:
		a.hwnd.Store(0)
		a.stopWork()
		win.KillTimer(hwnd, 1)
		win.PostQuitMessage(0)
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func (a *app) paint() {
	a.syncDPI()
	var client win.RECT
	win.GetClientRect(a.win, &client)
	a.layoutNative(client)
	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(a.win, &ps)
	mem := win.CreateCompatibleDC(hdc)
	bmp := win.CreateCompatibleBitmap(hdc, client.Right, client.Bottom)
	old := win.SelectObject(mem, win.HGDIOBJ(bmp))
	a.paintShell(mem, client)
	a.excludeShell(hdc)
	win.BitBlt(hdc, 0, 0, client.Right, client.Bottom, mem, 0, 0, win.SRCCOPY)
	win.SelectObject(mem, old)
	win.DeleteObject(win.HGDIOBJ(bmp))
	win.DeleteDC(mem)
	win.EndPaint(a.win, &ps)
	a.refreshShell()
	if a.host != 0 {
		win.InvalidateRect(a.host, nil, false)
		win.UpdateWindow(a.host)
	}
}

func (a *app) redraw() {
	if a.win != 0 {
		win.InvalidateRect(a.win, nil, false)
	}
}

func (a *app) onClickHit(h hit) {
	switch h.kind {
	case hitNav:
		a.setPage(h.index)
	case hitScan:
		if a.busy() {
			a.stopWork()
			a.setStatus("正在取消…")
			a.redraw()
			return
		}
		a.startScan()
	case hitClean:
		a.confirmClean()
	case hitSelect:
		a.toggleSelectAll()
	case hitElevate:
		if !a.elevated {
			a.elevate()
		}
	case hitCheck, hitChoice:
		a.toggleChoice(h.id)
	case hitExpand:
		if h.id == "clean-result" {
			a.cleanOut.detail = !a.cleanOut.detail
			a.redraw()
			return
		}
		if a.expanded == nil {
			a.expanded = map[string]bool{}
		}
		a.expanded[h.id] = !a.expanded[h.id]
		a.redraw()
	case hitDetail:
		a.showDetailID(h.id)
	case hitAnalyze:
		if a.phase == phaseAnalyze {
			a.stopWork()
			a.setStatus("正在取消分析…")
			a.redraw()
			return
		}
		a.startAnalyze()
	case hitDupes:
		if a.phase == phaseDupes {
			a.stopWork()
			a.setStatus("正在取消查找…")
			a.redraw()
			return
		}
		a.startDupes()
	case hitApps:
		if !a.busy() {
			a.startApps()
		}
	case hitSys:
		if !a.busy() {
			a.startSys()
		}
	case hitFilter:
		a.applyFilter(&h)
	case hitTree:
		a.toggleTree(h.id)
	case hitMap:
		a.selectTree(h.id)
	case hitReveal:
		if err := syswin.Reveal(h.id); err != nil {
			win.MessageBox(a.win, u16(err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
		}
	case hitCopy:
		if err := syswin.CopyText(h.id); err != nil {
			win.MessageBox(a.win, u16(err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
		} else {
			a.setStatus("已复制路径。")
			a.redraw()
		}
	case hitOpen:
		if err := syswin.SafeOpen(h.id); err != nil {
			_ = syswin.Reveal(h.id)
			win.MessageBox(a.win, u16(err.Error()+"\n已改为打开所在位置。"), u16(appinfo.Name), win.MB_OK)
		}
	case hitProps:
		if err := syswin.Properties(h.id); err != nil {
			win.MessageBox(a.win, u16(err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
		}
	case hitRecycle:
		a.recyclePath(h.id)
	case hitUninstall:
		a.uninstallApp(h.index)
	case hitSnapPrune:
		a.pruneSnapshots()
	case hitWinStorage:
		if err := syswin.OpenSettings(h.id); err != nil {
			win.MessageBox(a.win, u16(err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
		}
	case hitOpenLog:
		dir := logDir()
		_ = os.MkdirAll(dir, 0o700)
		_ = syswin.OpenPath(dir)
	case hitPrune:
		limit := int64(prefs.ClampLogKeepMB(a.logKeepMB)) << 20
		_ = clean.PruneLogs(logDir(), limit)
		a.alert("日志清理", fmt.Sprintf("已按 %d MB 上限清理过期日志。", prefs.ClampLogKeepMB(a.logKeepMB)))
		a.reloadHistory()
		a.redraw()
	case hitAdvanced:
		a.advancedOpen = !a.advancedOpen
		a.redraw()
	case hitTheme:
		a.setTheme(h.id)
	case hitGo:
		a.setPage(h.index)
	case hitAnalyzeTab:
		if h.index >= 0 && h.index < 3 {
			a.analyzeTab = h.index
			a.redraw()
		}
	case hitFileMenu:
		a.showFileMenu(h.id)
	case hitLogKeep:
		a.logKeepMB = prefs.ClampLogKeepMB(h.index)
		a.persistChoices()
		a.redraw()
	case hitHistory:
		_ = syswin.OpenPath(filepath.Join(logDir(), h.id))
	case hitScroll:
		a.scrollBy(a.scale(80))
	}
}

var (
	procCreatePopup = windows.NewLazySystemDLL("user32.dll").NewProc("CreatePopupMenu")
	procAppendMenuW = windows.NewLazySystemDLL("user32.dll").NewProc("AppendMenuW")
	procTrackPopup  = windows.NewLazySystemDLL("user32.dll").NewProc("TrackPopupMenu")
	procDestroyMenu = windows.NewLazySystemDLL("user32.dll").NewProc("DestroyMenu")
	procGetCursor   = windows.NewLazySystemDLL("user32.dll").NewProc("GetCursorPos")
)

func (a *app) showFileMenu(path string) {
	if path == "" || a.win == 0 {
		return
	}
	menu, _, _ := procCreatePopup.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	add := func(id uintptr, label string) {
		text, err := windows.UTF16PtrFromString(label)
		if err != nil {
			return
		}
		procAppendMenuW.Call(menu, 0, id, uintptr(unsafe.Pointer(text)))
		runtime.KeepAlive(text)
	}
	add(1, "打开")
	add(2, "打开位置")
	add(3, "复制路径")
	add(4, "属性")
	add(5, "移到回收站")
	var pt win.POINT
	procGetCursor.Call(uintptr(unsafe.Pointer(&pt)))
	cmd, _, _ := procTrackPopup.Call(menu, 0x0100|0x0080, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(a.win), 0)
	switch cmd {
	case 1:
		a.onClickHit(hit{kind: hitOpen, id: path})
	case 2:
		a.onClickHit(hit{kind: hitReveal, id: path})
	case 3:
		a.onClickHit(hit{kind: hitCopy, id: path})
	case 4:
		a.onClickHit(hit{kind: hitProps, id: path})
	case 5:
		a.onClickHit(hit{kind: hitRecycle, id: path})
	}
}

func (a *app) setPage(page int) {
	a.page = page
	a.scroll = 0
	a.checkLaid = a.checkLaid[:0]
	if page == ui.PageHistory {
		a.reloadHistory()
	}
	a.redraw()
}

func (a *app) revealTreeRow(path string) {
	rows := flattenTree(a.space.Tree, 0, a.treeOpen, nil)
	idx := -1
	for i, row := range rows {
		if row.node != nil && strings.EqualFold(row.node.Path, path) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	if a.treeShown > 0 && idx >= a.treeScroll && idx < a.treeScroll+a.treeShown {
		return
	}
	a.treeScroll = idx
}

func (a *app) onWheel(delta int16) {
	a.hideTip()
	if a.page == ui.PageAnalyze && a.contentH <= a.viewH+4 && a.treeCount > a.treeShown {
		step := 3
		if delta > 0 {
			a.scrollTree(-step)
		} else if delta < 0 {
			a.scrollTree(step)
		}
		return
	}
	a.scrollBy(-int(delta) / 2)
}

func (a *app) scrollTree(rows int) {
	a.treeScroll += rows
	if a.treeScroll < 0 {
		a.treeScroll = 0
	}
	max := a.treeCount - a.treeShown
	if max < 0 {
		max = 0
	}
	if a.treeScroll > max {
		a.treeScroll = max
	}
	a.redraw()
}

func (a *app) scrollBy(delta int) {
	a.scrollTo(a.scroll + delta)
}

func (a *app) scrollTo(pos int) {
	a.scroll = pos
	a.clampScroll()
	a.redraw()
}

func (a *app) clampScroll() {
	view := a.viewH
	max := a.contentH - view
	if max < 0 {
		max = 0
	}
	if a.scroll > max {
		a.scroll = max
	}
	if a.scroll < 0 {
		a.scroll = 0
	}
}

func (a *app) toggleChoice(id string) {
	if a.choices == nil {
		a.choices = map[string]bool{}
	}
	a.choices[id] = !a.choices[id]
	a.persistChoices()
	a.redraw()
}

func (a *app) pageHasCards() bool {
	return a.page == ui.PageClean
}

func (a *app) persistChoices() {
	riskOf := map[string]int{}
	for _, cat := range a.settingCats {
		riskOf[cat.ID] = cat.Risk
	}
	stored := map[string]bool{}
	for id, on := range a.choices {
		stored[id] = on
	}
	clean.ClampChoices(stored, riskOf)
	theme := "light"
	if a.dark {
		theme = "dark"
	}
	_ = prefs.Save(prefBase(), prefs.File{Choices: stored, Theme: theme, LogKeepMB: a.logKeepMB})
}

func (a *app) selectLabel() string {
	list := ui.ReportsInSection(a.reports, ui.SectionRecommend)
	flags := make([]bool, len(list))
	for i, rep := range list {
		flags[i] = a.choices[rep.Category.ID]
	}
	return clean.SelectAllLabel(clean.AllSelected(flags))
}

func (a *app) toggleSelectAll() {
	if a.busy() || !a.scanned || !a.pageHasCards() {
		return
	}
	list := ui.ReportsInSection(a.reports, ui.SectionRecommend)
	current := make([]bool, len(list))
	for i, rep := range list {
		current[i] = a.choices[rep.Category.ID]
	}
	next := clean.ToggleChecks(current)
	if a.choices == nil {
		a.choices = map[string]bool{}
	}
	for i, on := range next {
		a.choices[list[i].Category.ID] = on
	}
	a.persistChoices()
	a.redraw()
}

func (a *app) startScan() {
	ctx, gen := a.begin(phaseScan)
	a.setStatus("正在扫描 " + a.drive[:1] + " 盘…")
	cats := visibleCategories(a.drive)
	a.mu.Lock()
	a.progDone, a.progTotal = 0, len(cats)
	a.mu.Unlock()
	a.redraw()
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				a.finish(gen, wmScanDone, nil, fmt.Sprint(rec), true)
			}
		}()
		check := clean.WinChecker{Drive: a.drive}
		var reports []clean.Report
		for i, cat := range cats {
			if ctx.Err() != nil {
				break
			}
			a.mu.Lock()
			a.scanName = cat.Name
			a.statusText = "正在扫描：" + cat.Name
			a.mu.Unlock()
			reports = append(reports, clean.ScanCategory(ctx, cat, a.drive, check, func(int) {}))
			a.mu.Lock()
			a.scanning = append([]clean.Report(nil), reports...)
			a.progDone = i + 1
			a.progTotal = len(cats)
			a.mu.Unlock()
		}
		note := ""
		cancelled := false
		if ctx.Err() != nil {
			note = "已取消"
			cancelled = true
		}
		a.finish(gen, wmScanDone, reports, note, cancelled)
	}()
}

func (a *app) confirmClean() {
	if a.busy() || !a.scanned {
		return
	}
	selected := a.checkedReports()
	if len(selected) == 0 {
		a.alert("请先选择清理项", "勾选要清理的项目后，主按钮会显示预计释放的空间。")
		return
	}
	var allowed, blocked []clean.Report
	for _, rep := range selected {
		if rep.Category.NeedsAdmin && !a.elevated {
			blocked = append(blocked, rep)
			continue
		}
		allowed = append(allowed, rep)
	}
	if len(blocked) > 0 {
		names := make([]string, 0, len(blocked))
		for _, rep := range blocked {
			names = append(names, rep.Category.Name)
		}
		answer := a.askYesNoCancel("需要管理员权限", "以下项目需要管理员权限：\n\n"+strings.Join(names, "、")+"\n\n确定：以管理员身份重新启动。\n跳过：只清理当前能清理的项目。\n取消：不开始清理。")
		if answer == win.IDYES {
			a.elevate()
			return
		}
		if answer != win.IDNO {
			return
		}
	}
	if len(allowed) == 0 {
		return
	}
	if !a.askYesNo("确认清理", confirmText(allowed)) {
		return
	}
	on := map[string]bool{}
	for _, rep := range allowed {
		on[rep.Category.ID] = true
	}
	extra := ui.ConfirmExtra(allowed, on)
	for _, msg := range extra {
		if !a.askYesNo("高风险确认", msg+"\n\n仍然继续？") {
			return
		}
	}
	a.startClean(allowed)
}

func (a *app) begin(phase int) (context.Context, int) {
	a.stopWork()
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.generation++
	gen := a.generation
	a.cancel = cancel
	a.phase = phase
	a.progDone, a.progTotal = 0, 0
	a.mu.Unlock()
	return ctx, gen
}

func (a *app) startClean(selected []clean.Report) {
	logFile, logPath, err := newLog()
	if err != nil {
		win.MessageBox(a.win, u16("无法写日志："+err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
		return
	}
	ctx, gen := a.begin(phaseClean)
	a.refreshDisk()
	a.freeBefore = a.free
	a.cleanStarted = time.Now()
	a.mu.Lock()
	a.logPath = logPath
	a.progTotal = 1
	a.statusText = "正在清理…"
	a.mu.Unlock()
	a.redraw()
	go func() {
		defer logFile.Close()
		defer func() {
			if rec := recover(); rec != nil {
				a.finish(gen, wmCleanDone, nil, fmt.Sprint(rec), false)
			}
		}()
		fmt.Fprintf(logFile, "CClear %s\r\nFormat 2\r\n任务 %s\r\n\r\n", appinfo.Version, time.Now().Format(time.RFC3339))
		var freed, estimated int64
		var deleted, skipped, failed int
		var notes []string
		for _, rep := range selected {
			if ctx.Err() != nil {
				notes = append(notes, "已取消")
				break
			}
			a.mu.Lock()
			a.cleanName = rep.Category.Name
			a.statusText = "正在清理：" + rep.Category.Name
			a.mu.Unlock()
			fmt.Fprintf(logFile, "---- %s (%s) ----\r\n", rep.Category.Name, rep.Category.ID)
			stats := clean.CleanCategory(ctx, rep, a.drive, logFile, func(done, total int) {
				a.mu.Lock()
				a.progDone = done
				a.progTotal = total
				a.mu.Unlock()
			})
			freed += stats.Freed
			estimated += stats.Estimated
			deleted += stats.Deleted
			skipped += stats.Skipped
			failed += stats.Failed
			notes = append(notes, stats.Errors...)
			fmt.Fprintf(logFile, "逻辑大小 %s，预计占用 %s，删除 %d，跳过 %d，失败 %d\r\n\r\n", clean.FormatBytes(stats.Freed), clean.FormatBytes(stats.Estimated), stats.Deleted, stats.Skipped, stats.Failed)
		}
		for i := range selected {
			selected[i].Files = nil
			selected[i].Dirs = nil
		}
		elapsed := time.Since(a.cleanStarted).Round(time.Second)
		restart := false
		for _, rep := range selected {
			if rep.Category.ID == "dism" || rep.Category.ID == "winupdate" {
				restart = true
			}
		}
		a.mu.Lock()
		a.result = resultText(freed, estimated, deleted, skipped, failed, notes, logPath) + "\r\n清理耗时：" + elapsed.String()
		a.cleanOut = cleanOutcome{
			logical: freed, estimated: estimated, deleted: deleted, skipped: skipped, failed: failed,
			notes: notes, logPath: logPath, restart: restart,
		}
		a.mu.Unlock()
		a.finish(gen, wmCleanDone, nil, "", false)
	}()
}

func (a *app) applyScan() {
	a.mu.Lock()
	if a.pendingGen != a.generation {
		a.mu.Unlock()
		return
	}
	next := a.pending
	note := a.pendingNote
	cancelled := a.pendingCancel
	a.phase = phaseReady
	a.mu.Unlock()
	if clean.KeepPreviousScan(cancelled, len(a.reports)) {
		a.setStatus("已取消，仍显示上一次的扫描结果。")
		if note != "" && note != "已取消" {
			a.setStatus(note)
		}
		a.redraw()
		return
	}
	a.reports = next
	if a.choices == nil {
		a.choices = map[string]bool{}
	}
	for _, rep := range a.reports {
		a.choices[rep.Category.ID] = clean.RememberedCheck(rep.Category.ID, rep.Category.DefaultOn, rep.Bytes, rep.Unknown, a.choices)
	}
	a.persistChoices()
	a.scanned = len(a.reports) > 0
	a.scanAt = time.Now()
	a.scanFailed = note != "" && !cancelled && !a.scanned
	a.cleanOut.show = false
	head := ui.Headline(a.reports, a.scanned)
	if note != "" {
		a.setStatus(note)
	} else if a.scanned {
		a.setStatus(fmt.Sprintf("扫描完成。发现 %d 项可清理内容，预计可释放 %s。", head.Items, clean.FormatBytes(head.Reclaim)))
	} else {
		a.setStatus("扫描完成，没有发现可清理内容。")
	}
	a.redraw()
}

func (a *app) applyClean() {
	if a.hwnd.Load() == 0 {
		return
	}
	a.mu.Lock()
	if a.pendingGen != a.generation {
		a.mu.Unlock()
		return
	}
	out := a.cleanOut
	a.phase = phaseReady
	a.mu.Unlock()
	before := a.freeBefore
	a.releaseReports()
	a.scanned = false
	a.refreshDisk()
	if before > 0 {
		out.diskDelta = int64(a.free) - int64(before)
	}
	out.show = true
	a.cleanOut = out
	a.reloadHistory()
	a.setStatus("清理完成。可在上方查看删除、跳过和失败数量。")
	a.redraw()
}

func (a *app) startAnalyze() {
	ctx, gen := a.begin(phaseAnalyze)
	a.setStatus("正在分析磁盘占用，不会删除任何文件…")
	a.redraw()
	root := a.drive + `\`
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				a.finish(gen, wmAnalyzeDone, nil, fmt.Sprint(rec), true)
			}
		}()
		rep := space.Scan(ctx, root, func(files int, where string) {
			a.mu.Lock()
			a.progDone = files
			a.statusText = fmt.Sprintf("正在分析：%s（%d）", filepath.Base(where), files)
			a.mu.Unlock()
		})
		a.mu.Lock()
		a.space = rep
		a.mu.Unlock()
		a.finish(gen, wmAnalyzeDone, nil, "", rep.Cancelled)
	}()
}

func (a *app) applyAnalyze() {
	a.mu.Lock()
	if a.pendingGen != a.generation {
		a.mu.Unlock()
		return
	}
	a.phase = phaseReady
	cancelled := a.pendingCancel
	rep := a.space
	a.mu.Unlock()
	a.spaceOK = true
	a.treeOpen = map[string]bool{}
	if rep.Tree != nil {
		a.treeOpen[rep.Tree.Path] = true
		a.selected = rep.Tree.Path
	}
	if rep.Files > 0 {
		_ = snapshot.Save(prefBase(), snapshot.Record{
			When:      time.Now(),
			Drive:     a.drive,
			Total:     a.total,
			Free:      a.free,
			Allocated: rep.Allocated,
			Files:     rep.Files,
			Folders:   rep.TopDirs,
			Types:     rep.Types,
		})
		a.reloadSnaps()
	}
	if cancelled {
		a.setStatus("分析已取消。已统计的部分只用于展示占用。")
	} else {
		a.setStatus(fmt.Sprintf("分析完成。文件 %d，实际占用 %s。不会用于自动删除。", rep.Files, clean.FormatBytes(rep.Allocated)))
	}
	a.page = ui.PageAnalyze
	a.redraw()
}

func (a *app) startDupes() {
	ctx, gen := a.begin(phaseDupes)
	a.setStatus("正在按内容和大小查找重复文件…")
	a.redraw()
	root := a.drive + `\`
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				a.finish(gen, wmDupesDone, nil, fmt.Sprint(rec), true)
			}
		}()
		rep := dupes.Find(ctx, root, func(files int, where string) {
			a.mu.Lock()
			a.progDone = files
			a.statusText = fmt.Sprintf("正在哈希：%s（%d）", filepath.Base(where), files)
			a.mu.Unlock()
		})
		a.mu.Lock()
		a.dupes = rep
		a.mu.Unlock()
		a.finish(gen, wmDupesDone, nil, "", rep.Cancelled)
	}()
}

func (a *app) applyDupes() {
	a.mu.Lock()
	if a.pendingGen != a.generation {
		a.mu.Unlock()
		return
	}
	a.phase = phaseReady
	cancelled := a.pendingCancel
	rep := a.dupes
	a.mu.Unlock()
	a.dupesOK = true
	if cancelled {
		a.setStatus("查找已取消。")
	} else {
		a.setStatus(fmt.Sprintf("找到 %d 组重复文件。不会自动删除。", len(rep.Groups)))
	}
	a.page = ui.PageDupes
	a.redraw()
}

func (a *app) startApps() {
	ctx, gen := a.begin(phaseApps)
	a.setStatus("正在读取已安装软件登记…")
	a.redraw()
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				a.finish(gen, wmAppsDone, nil, fmt.Sprint(rec), true)
			}
		}()
		rep := apps.ListInstalled(ctx)
		a.mu.Lock()
		a.apps = rep
		a.mu.Unlock()
		a.finish(gen, wmAppsDone, nil, "", rep.Cancelled)
	}()
}

func (a *app) applyApps() {
	a.mu.Lock()
	if a.pendingGen != a.generation {
		a.mu.Unlock()
		return
	}
	a.phase = phaseReady
	rep := a.apps
	a.mu.Unlock()
	a.appsOK = true
	a.setStatus(fmt.Sprintf("读到 %d 个带卸载入口的软件。", len(rep.Apps)))
	a.page = ui.PageApps
	a.redraw()
}

func (a *app) startSys() {
	ctx, gen := a.begin(phaseSys)
	a.setStatus("正在读取系统占用和开发环境…")
	a.redraw()
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				a.finish(gen, wmSysDone, nil, fmt.Sprint(rec), true)
			}
		}()
		sys := sysspace.Read(a.drive)
		dev := devspace.Scan(ctx, devspace.Env{
			UserProfile:  os.Getenv("USERPROFILE"),
			LocalAppData: os.Getenv("LOCALAPPDATA"),
			UserHome:     os.Getenv("USERPROFILE"),
		})
		a.mu.Lock()
		a.sys = sys
		a.dev = dev
		a.mu.Unlock()
		a.finish(gen, wmSysDone, nil, "", dev.Cancelled)
	}()
}

func (a *app) applySys() {
	a.mu.Lock()
	if a.pendingGen != a.generation {
		a.mu.Unlock()
		return
	}
	a.phase = phaseReady
	a.mu.Unlock()
	a.sysOK = true
	a.reloadSnaps()
	a.setStatus("系统占用已刷新。这些项目不会被删除。")
	a.page = ui.PageSystem
	a.redraw()
}

func (a *app) applyFilter(h *hit) {
	mins := []int64{100 << 20, 500 << 20, 1 << 30, 5 << 30}
	if h.id == "custom" {
		n, ok := promptSizeMB(a.win, a.dpi)
		if ok {
			a.largeMin = n
		}
	} else if h.id == "min" && h.index >= 0 && h.index < len(mins) {
		a.largeMin = mins[h.index]
	} else if h.id != "" && h.id != "min" && h.id != "custom" {
		a.largeSort = h.id
	}
	a.redraw()
}

func (a *app) toggleTree(path string) {
	if a.treeOpen == nil {
		a.treeOpen = map[string]bool{}
	}
	a.treeOpen[path] = !a.treeOpen[path]
	a.selected = path
	a.redraw()
}

func (a *app) selectTree(path string) {
	if a.treeOpen == nil {
		a.treeOpen = map[string]bool{}
	}
	a.selected = path
	a.hideTip()
	n := a.space.Tree
	for n != nil {
		a.treeOpen[n.Path] = true
		var next *space.Node
		for _, child := range n.Children {
			if path == child.Path || hasPrefixPath(path, child.Path) {
				next = child
				break
			}
		}
		n = next
	}
	a.revealTreeRow(path)
	a.redraw()
}

func hasPrefixPath(full, prefix string) bool {
	if len(full) < len(prefix) {
		return false
	}
	if !strings.EqualFold(full[:len(prefix)], prefix) {
		return false
	}
	if len(full) == len(prefix) {
		return true
	}
	c := full[len(prefix)]
	return c == '\\' || c == '/'
}

func (a *app) recyclePath(path string) {
	if a.page == ui.PageDupes {
		if err := dupes.CanRemove(a.dupes.Groups, path); err != nil {
			win.MessageBox(a.win, u16(err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
			return
		}
	}
	if space.HighRiskDelete(path) {
		win.MessageBox(a.win, u16("系统文件、程序目录、驱动或虚拟磁盘不能从这里删除。"), u16(appinfo.Name), win.MB_ICONWARNING)
		return
	}
	if win.MessageBox(a.win, u16("将把该文件移到回收站：\n\n"+path+"\n\n不会永久删除。仍然继续？"), u16("移到回收站"), win.MB_YESNO|win.MB_ICONWARNING|win.MB_DEFBUTTON2) != win.IDYES {
		return
	}
	if win.MessageBox(a.win, u16("再次确认：只移到回收站，并且这不是垃圾清理。继续？"), u16("再次确认"), win.MB_YESNO|win.MB_ICONWARNING|win.MB_DEFBUTTON2) != win.IDYES {
		return
	}
	if err := syswin.RecycleFile(path); err != nil {
		win.MessageBox(a.win, u16(err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
		return
	}
	a.removeSpaceFile(path)
	a.removeDupeFile(path)
	a.setStatus("已移到回收站。")
	a.redraw()
}

func (a *app) removeSpaceFile(path string) {
	filter := func(list []space.File) []space.File {
		var out []space.File
		for _, file := range list {
			if !strings.EqualFold(file.Path, path) {
				out = append(out, file)
			}
		}
		return out
	}
	a.space.Large = filter(a.space.Large)
	a.space.TopFiles = filter(a.space.TopFiles)
}

func (a *app) removeDupeFile(path string) {
	var groups []dupes.Group
	for _, g := range a.dupes.Groups {
		var files []dupes.Item
		for _, file := range g.Files {
			if !strings.EqualFold(file.Path, path) {
				files = append(files, file)
			}
		}
		if len(files) >= 2 {
			g.Files = files
			groups = append(groups, g)
		}
	}
	a.dupes.Groups = groups
}

func (a *app) uninstallApp(index int) {
	if index < 0 || index >= len(a.apps.Apps) {
		return
	}
	item := a.apps.Apps[index]
	exe, args, err := apps.UninstallCommand(item)
	if err != nil {
		win.MessageBox(a.win, u16(err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
		return
	}
	if win.MessageBox(a.win, u16("将启动系统登记的卸载程序：\n\n"+item.Name+"\n"+exe+" "+args+"\n\nCClear 不会直接删除安装目录。"), u16("卸载软件"), win.MB_YESNO|win.MB_ICONWARNING|win.MB_DEFBUTTON2) != win.IDYES {
		return
	}
	if err := syswin.RunCommand(exe, args); err != nil {
		win.MessageBox(a.win, u16(err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
	}
}

func (a *app) pruneSnapshots() {
	dir := snapshot.Dir(prefBase())
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
	a.snaps = nil
	a.setStatus("已删除本机保存的扫描快照。")
	a.redraw()
}

func (a *app) reloadSnaps() {
	list, err := snapshot.Load(prefBase())
	if err != nil {
		return
	}
	a.snaps = list
}

func (a *app) checkedReports() []clean.Report {
	var out []clean.Report
	for _, rep := range a.reports {
		if a.choices[rep.Category.ID] {
			out = append(out, rep)
		}
	}
	return out
}

func (a *app) showDetailID(id string) {
	for i, rep := range a.reports {
		if rep.Category.ID == id {
			a.showDetailAt(i)
			return
		}
	}
}

func (a *app) showDetailAt(idx int) {
	if idx < 0 || idx >= len(a.reports) {
		return
	}
	a.detailIndex = idx
	a.detailPage = 0
	if a.detail == 0 || !isWindow(a.detail) {
		a.detail = win.CreateWindowEx(0, u16(classDetail), u16("清理详情"),
			win.WS_OVERLAPPEDWINDOW|win.WS_CLIPCHILDREN,
			win.CW_USEDEFAULT, win.CW_USEDEFAULT, 760, 520,
			a.win, 0, win.GetModuleHandle(nil), nil)
		edit := win.CreateWindowEx(win.WS_EX_CLIENTEDGE, u16("EDIT"), nil,
			win.WS_CHILD|win.WS_VISIBLE|win.WS_VSCROLL|win.ES_MULTILINE|win.ES_READONLY|win.ES_AUTOVSCROLL,
			0, 0, 10, 10, a.detail, 1, win.GetModuleHandle(nil), nil)
		a.applyType(edit, ui.TypePath)
		detailEdit = edit
		detailPrev = win.CreateWindowEx(0, u16("BUTTON"), u16("上一页"),
			win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP,
			0, 0, 10, 10, a.detail, win.HMENU(idPagePrev), win.GetModuleHandle(nil), nil)
		detailNext = win.CreateWindowEx(0, u16("BUTTON"), u16("下一页"),
			win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP,
			0, 0, 10, 10, a.detail, win.HMENU(idPageNext), win.GetModuleHandle(nil), nil)
		a.applyType(detailPrev, ui.TypeButton)
		a.applyType(detailNext, ui.TypeButton)
	}
	a.showDetailPage()
	win.ShowWindow(a.detail, win.SW_SHOW)
	win.SetForegroundWindow(a.detail)
	layoutDetail(a.detail)
}

func (a *app) turnPage(delta int) {
	if a.detailIndex < 0 || a.detailIndex >= len(a.reports) {
		return
	}
	a.detailPage += delta
	a.showDetailPage()
}

func (a *app) showDetailPage() {
	if a.detailIndex < 0 || a.detailIndex >= len(a.reports) {
		return
	}
	text, pages := preview(a.reports[a.detailIndex], a.detailPage)
	if a.detailPage >= pages {
		a.detailPage = pages - 1
	}
	if a.detailPage < 0 {
		a.detailPage = 0
	}
	text, pages = preview(a.reports[a.detailIndex], a.detailPage)
	setText(detailEdit, text)
	setText(a.detail, a.reports[a.detailIndex].Category.Name)
	if detailPrev != 0 {
		win.EnableWindow(detailPrev, a.detailPage > 0)
	}
	if detailNext != 0 {
		win.EnableWindow(detailNext, a.detailPage+1 < pages)
	}
}

func (a *app) releaseReports() {
	for i := range a.reports {
		a.reports[i].Files = nil
		a.reports[i].Dirs = nil
	}
	a.pending = nil
}

func (a *app) elevate() {
	if a.busy() {
		a.stopWork()
	}
	syswin.ReleaseSingleInstance()
	if err := syswin.RelaunchElevated(); err != nil {
		if ok, acquireErr := syswin.AcquireSingleInstance(); acquireErr != nil || !ok {
			win.DestroyWindow(a.win)
			return
		}
		win.MessageBox(a.win, u16(err.Error()), u16(appinfo.Name), win.MB_ICONWARNING)
		return
	}
	win.DestroyWindow(a.win)
}

func (a *app) finish(gen int, msg uint32, reports []clean.Report, note string, cancelled bool) {
	a.mu.Lock()
	if gen != a.generation {
		a.mu.Unlock()
		return
	}
	a.pending = reports
	a.pendingNote = note
	a.pendingGen = gen
	a.pendingCancel = cancelled
	a.phase = phaseReady
	a.cancel = nil
	a.mu.Unlock()
	hwnd := win.HWND(a.hwnd.Load())
	if hwnd != 0 {
		win.PostMessage(hwnd, msg, 0, 0)
	}
}

func (a *app) stopWork() {
	a.mu.Lock()
	cancel := a.cancel
	a.cancel = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *app) busy() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.phase == phaseScan || a.phase == phaseClean || a.phase == phaseAnalyze || a.phase == phaseDupes || a.phase == phaseApps || a.phase == phaseSys
}

func (a *app) setStatus(text string) {
	a.mu.Lock()
	a.statusText = text
	a.mu.Unlock()
}

func (a *app) scanLine() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.progTotal <= 0 {
		return "正在扫描…"
	}
	found := ui.Headline(a.scanning, true)
	reclaim := clean.FormatBytes(found.Reclaim)
	if found.Unknown {
		reclaim += "，另有待系统分析"
	}
	return fmt.Sprintf("正在扫描 %s（%d/%d）。已发现预计可释放 %s。", a.scanName, a.progDone, a.progTotal, reclaim)
}

func (a *app) cleanLine() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	total := a.progTotal
	if total <= 0 {
		total = 1
	}
	return fmt.Sprintf("正在清理 %s。进度 %d/%d。进度来自实际任务，不是估计百分比。", a.cleanName, a.progDone, total)
}

func (a *app) refreshDisk() {
	total, free, err := syswin.Disk(a.drive)
	if err != nil {
		return
	}
	a.total, a.free = total, free
}

func (a *app) reloadHistory() {
	list, err := history.Load(logDir(), 30)
	if err != nil {
		return
	}
	a.history = list
}

func (a *app) createFonts() {
	a.rebuildTypekit()
	a.applyControlFonts()
}

func (a *app) scale(px int) int {
	if a.dpi == 0 {
		return px
	}
	return px * a.dpi / 96
}

func visibleCategories(drive string) []clean.Category {
	env := clean.Env{
		SystemDrive:  drive,
		SystemRoot:   os.Getenv("SystemRoot"),
		LocalAppData: os.Getenv("LOCALAPPDATA"),
		ProgramData:  os.Getenv("ProgramData"),
		Temp:         os.TempDir(),
		UserProfile:  os.Getenv("USERPROFILE"),
	}
	var out []clean.Category
	for _, cat := range clean.Categories(env) {
		if cat.Visible() {
			out = append(out, cat)
		}
	}
	return out
}

func confirmText(reports []clean.Report) string {
	var b strings.Builder
	b.WriteString("将直接删除以下内容，不会进入回收站：\r\n\r\n")
	var total int64
	var unknown, system bool
	for _, rep := range reports {
		fmt.Fprintf(&b, "• %s（%s）· %s\r\n  %s\r\n  %s\r\n", rep.Category.Name, ui.SizeText(rep), ui.RiskText(rep.Category.Risk), ui.RiskReason(rep.Category), ui.Recoverable(rep.Category))
		if rep.Category.Group == "system" {
			system = true
		}
		if rep.Unknown {
			unknown = true
		} else if rep.Bytes > 0 {
			total += rep.Bytes
		}
	}
	fmt.Fprintf(&b, "\r\n预计可释放约 %s。这是占用空间估计，不是已经释放的空间。", clean.FormatBytes(total))
	if unknown {
		b.WriteString("组件存储显示为待系统分析，不会当成 0。")
	}
	if system {
		b.WriteString("其中包含系统级清理，稍后还会单独确认。")
	}
	b.WriteString("\r\n正在使用的文件会自动跳过。确定继续？")
	return b.String()
}

func resultText(logical, estimated int64, deleted, skipped, failed int, notes []string, logPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "清理完成\r\n已删除 %d 个\r\n已跳过 %d 个\r\n删除失败 %d 个\r\n", deleted, skipped, failed)
	fmt.Fprintf(&b, "已删除文件的逻辑大小 %s\r\n预计占用空间 %s\r\n逻辑大小不是实际释放的磁盘空间。\r\n", clean.FormatBytes(logical), clean.FormatBytes(estimated))
	if len(notes) > 0 {
		b.WriteString("\r\n")
		limit := len(notes)
		if limit > 5 {
			limit = 5
		}
		for _, note := range notes[:limit] {
			b.WriteString(note)
			b.WriteString("\r\n")
		}
	}
	if logical == 0 && deleted == 0 && skipped > 0 {
		b.WriteString("\r\n没有删掉文件。可能需要管理员权限，或这些文件正在被程序使用。\r\n")
	}
	if logPath != "" {
		b.WriteString("\r\n日志：")
		b.WriteString(logPath)
	}
	return b.String()
}

func preview(rep clean.Report, page int) (string, int) {
	if rep.Category.Special == "recycle" {
		return fmt.Sprintf("将清空 %s 上的回收站，约 %s，%d 个项目。\r\n\r\n为避免打开回收站里的快捷方式，这里不逐个列出文件，清理时调用 Windows 自己的清空功能。", rep.Category.Name, clean.FormatBytes(rep.Bytes), rep.Count), 1
	}
	if rep.Category.Special == "dism" || rep.Category.Special == "windowsold" {
		text := rep.Category.Description
		if rep.Category.Warning != "" {
			text += "\r\n\r\n" + rep.Category.Warning
		}
		if rep.Note != "" {
			text += "\r\n\r\n" + rep.Note
		}
		return text, 1
	}
	files := append([]clean.FileHit(nil), rep.Files...)
	sortBySize(files)
	view, pages, page := clean.DetailPage(files, page)
	var b strings.Builder
	fmt.Fprintf(&b, "%s\r\n%s\r\n\r\n候选 %d 个文件，逻辑大小 %s，预计可释放 %s。\r\n第 %d/%d 页。\r\n", rep.Category.Name, rep.Category.Description, rep.Count, clean.FormatBytes(rep.Logical), ui.SizeText(rep), page+1, pages)
	if rep.Truncated {
		fmt.Fprintf(&b, "预览只保留前 %d 个文件。清理时会分批重新核对身份，不会跳过安全检查。\r\n", len(files))
	}
	if rep.Category.Warning != "" {
		fmt.Fprintf(&b, "\r\n注意：%s\r\n", rep.Category.Warning)
	}
	if len(view) > 0 {
		b.WriteString("\r\n这一页的文件：\r\n")
	}
	for _, file := range view {
		fmt.Fprintf(&b, "%s    %s\r\n", clean.FormatBytes(file.Size), file.Path)
	}
	if rep.SkippedRecent > 0 {
		hours := int(rep.Category.MinAge.Hours())
		if hours <= 0 {
			hours = 72
		}
		fmt.Fprintf(&b, "\r\n另有 %d 个最近 %d 小时内修改过的临时文件会保留。\r\n", rep.SkippedRecent, hours)
	}
	if rep.SkippedIdentity > 0 {
		fmt.Fprintf(&b, "\r\n另有 %d 个文件因为无法确认身份被排除，不会删除。\r\n", rep.SkippedIdentity)
	}
	for _, err := range rep.Errors {
		fmt.Fprintf(&b, "\r\n%s\r\n", err)
	}
	return b.String(), pages
}

func sortBySize(files []clean.FileHit) {
	for i := 1; i < len(files); i++ {
		item := files[i]
		j := i
		for j > 0 && files[j-1].Size < item.Size {
			files[j] = files[j-1]
			j--
		}
		files[j] = item
	}
}

func prefBase() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return os.TempDir()
	}
	return base
}

func logDir() string {
	base := prefBase()
	return filepath.Join(base, "CClear", "logs")
}

func newLog() (*os.File, string, error) {
	dir := logDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, time.Now().Format("20060102-150405")+".log")
	keep := 2
	if uiPtr != nil && uiPtr.logKeepMB > 0 {
		keep = prefs.ClampLogKeepMB(uiPtr.logKeepMB)
	}
	_ = clean.PruneLogs(dir, int64(keep)<<20)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	return file, path, err
}

func u16(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

func rgb(r, g, b byte) win.COLORREF {
	return win.COLORREF(r) | win.COLORREF(g)<<8 | win.COLORREF(b)<<16
}

func rect(l, t, r, b int) win.RECT {
	return win.RECT{Left: int32(l), Top: int32(t), Right: int32(r), Bottom: int32(b)}
}
