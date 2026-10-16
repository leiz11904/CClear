//go:build windows

package main

import (
	"unsafe"

	"cclear/internal/ui"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

const (
	idNavBase    = 2000
	idFootScan   = 2101
	idFootClean  = 2102
	idFootSel    = 2103
	idFootWork   = 2104
	idFootAdmin  = 2105
	idScroll     = 2300
	idCheckBase  = 3000
	idDetailBase = 4000
	idActionBase = 5000
	sbsVert      = 0x0001
	actionCap    = 4096
	bsPushButton = 0x00000000
	bsDefPush    = 0x00000001
	bsAutoCheck  = 0x00000003
	bmGetCheck   = 0x00F0
	bmSetCheck   = 0x00F1
)

type placedCheck struct {
	rc   win.RECT
	on   bool
	show bool
}

type placedBtn struct {
	rc      win.RECT
	text    string
	access  string
	show    bool
	primary bool
}

type actionPlace struct {
	rc      win.RECT
	caption string
	access  string
	show    bool
	style   int
	kind    int
	id      string
	index   int
	bg      win.COLORREF
	fg      win.COLORREF
	edge    win.COLORREF
	detail  string
	tip     string
}

type navSlot struct {
	item   ui.NavItem
	rc     win.RECT
	group  string
	groupY int
}

func setDPIAware() {
	proc := user32.NewProc("SetProcessDpiAwarenessContext")
	if proc.Find() == nil {
		ok, _, _ := proc.Call(^uintptr(3))
		if ok != 0 {
			return
		}
	}
	user32.NewProc("SetProcessDPIAware").Call()
}

func setImmersiveDark(hwnd win.HWND, dark bool) {
	if hwnd == 0 {
		return
	}
	var on uint32
	if dark {
		on = 1
	}
	procDwmSet.Call(uintptr(hwnd), 20, uintptr(unsafe.Pointer(&on)), 4)
}

func (a *app) setTheme(name string) {
	a.dark = name == "dark"
	applyPalette(ui.Named(name))
	if a.pageBrush != 0 {
		win.DeleteObject(win.HGDIOBJ(a.pageBrush))
	}
	if a.cardBrush != 0 {
		win.DeleteObject(win.HGDIOBJ(a.cardBrush))
	}
	a.pageBrush = solidBrush(colorBg)
	a.cardBrush = solidBrush(colorCard)
	setImmersiveDark(a.win, a.dark)
	a.persistChoices()
	a.redraw()
}

func (a *app) syncDPI() {
	if a.win == 0 {
		return
	}
	proc := user32.NewProc("GetDpiForWindow")
	if proc.Find() != nil {
		return
	}
	d, _, _ := proc.Call(uintptr(a.win))
	if d == 0 {
		return
	}
	if int(d) != a.dpi {
		a.dpi = int(d)
		a.createFonts()
	}
}

func (a *app) applyControlFonts() {
	for _, hwnd := range a.navBtns {
		a.applyType(hwnd, ui.TypeNav)
	}
	for _, hwnd := range []win.HWND{a.btnScan, a.btnClean, a.btnWork, a.btnAdmin} {
		a.applyType(hwnd, ui.TypeButton)
	}
	for _, hwnd := range a.checkBtns {
		a.applyType(hwnd, ui.TypeItem)
	}
	for _, hwnd := range a.detailBtns {
		a.applyType(hwnd, ui.TypeButton)
	}
	for _, hwnd := range a.actBtns {
		a.applyType(hwnd, ui.TypeButton)
	}
}

func createBtn(parent win.HWND, id int, style uint32, label string) win.HWND {
	hwnd := win.CreateWindowEx(0, u16("BUTTON"), u16(label),
		win.WS_CHILD|win.WS_TABSTOP|win.WS_CLIPSIBLINGS|style,
		0, 0, 10, 10, parent, win.HMENU(id), win.GetModuleHandle(nil), nil)
	prepareControl(hwnd)
	return hwnd
}

func (a *app) ensureControls() {
	if a.win == 0 {
		return
	}
	if len(a.navBtns) == 0 {
		items := ui.NavItems()
		a.navBtns = make([]win.HWND, len(items))
		for i, item := range items {
			hwnd := createBtn(a.win, idNavBase+item.Page, bsPushButton, item.Title)
			a.applyType(hwnd, ui.TypeNav)
			a.navBtns[i] = hwnd
		}
	}
	if a.btnScan == 0 {
		a.btnScan = createBtn(a.win, idFootScan, bsDefPush, "开始扫描")
		a.btnClean = createBtn(a.win, idFootClean, bsPushButton, "开始清理")
		a.btnWork = createBtn(a.win, idFootWork, bsPushButton, "开始分析")
		a.btnAdmin = createBtn(a.win, idFootAdmin, bsPushButton, "以管理员运行")
		a.applyControlFonts()
	}
}

func (a *app) placeBtn(hwnd win.HWND, rc win.RECT, label string, show, enabled bool) {
	if hwnd == 0 {
		return
	}
	if !show {
		win.ShowWindow(hwnd, win.SW_HIDE)
		return
	}
	setText(hwnd, label)
	kind := faceSecondary
	if hwnd == a.btnClean || hwnd == a.btnWork || (hwnd == a.btnScan && a.scanPrimary()) {
		kind = facePrimary
	}
	setFace(hwnd, ctrlFace{kind: kind, caption: label})
	win.EnableWindow(hwnd, enabled)
	win.MoveWindow(hwnd, rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, false)
	win.ShowWindow(hwnd, win.SW_SHOW)
}

func (a *app) scanPrimary() bool {
	return a.page == ui.PageOverview || !a.scanned || a.phase == phaseScan || a.phase == phaseClean
}

func (a *app) navSlots(nav win.RECT) []navSlot {
	var slots []navSlot
	y := int(nav.Top) + a.scale(72)
	for _, g := range ui.NavGroups() {
		groupY := y
		y += a.scale(20)
		for i, item := range g.Items {
			rc := rect(int(nav.Left)+a.scale(10), y, int(nav.Right)-a.scale(10), y+a.scale(36))
			name := ""
			gy := 0
			if i == 0 {
				name = g.Title
				gy = groupY
			}
			slots = append(slots, navSlot{item: item, rc: rc, group: name, groupY: gy})
			y += a.scale(40)
		}
		y += a.scale(8)
	}
	settings := ui.SettingsItem()
	sy := int(nav.Bottom) - a.scale(68)
	slots = append(slots, navSlot{item: settings, rc: rect(int(nav.Left)+a.scale(10), sy, int(nav.Right)-a.scale(10), sy+a.scale(36))})
	return slots
}

func (a *app) layoutNative(client win.RECT) {
	a.ensureControls()
	nav := client
	nav.Right = int32(a.scale(ui.NavWidth))
	items := ui.NavItems()
	slots := a.navSlots(nav)
	byPage := map[int]win.RECT{}
	for _, slot := range slots {
		byPage[slot.item.Page] = slot.rc
	}
	for i, item := range items {
		if i >= len(a.navBtns) {
			break
		}
		rc, ok := byPage[item.Page]
		if !ok {
			continue
		}
		current := a.page == item.Page
		a.placeBtn(a.navBtns[i], rc, ui.NavAccessName(item.Title, current), true, true)
		setFace(a.navBtns[i], ctrlFace{kind: faceNav, caption: item.Title, icon: item.Icon, selected: current, bg: colorNav})
	}
	busy := a.busy()
	pad := a.scale(16)
	btnH := a.scale(36)
	footTop := int(client.Bottom) - a.scale(ui.FooterH)
	fy := footTop + (a.scale(ui.FooterH)-btnH)/2
	fx := int(nav.Right) + pad
	scanLabel := "开始扫描"
	if a.scanned {
		scanLabel = "重新扫描"
	}
	if busy && a.phase == phaseScan {
		scanLabel = "取消扫描"
	}
	if busy && a.phase == phaseClean {
		scanLabel = "取消清理"
	}
	count, reclaim, unknown, _, _ := ui.SelectedSummary(a.reports, a.choices)
	cleanLabel := ui.CleanActionLabel(count, reclaim, unknown)
	a.placeBtn(a.btnScan, rect(0, 0, 0, 0), scanLabel, false, true)
	a.placeBtn(a.btnClean, rect(0, 0, 0, 0), cleanLabel, false, true)
	a.placeBtn(a.btnWork, rect(0, 0, 0, 0), "", false, true)
	a.placeBtn(a.btnAdmin, rect(0, 0, 0, 0), "以管理员运行", false, true)
	right := int(client.Right) - pad
	switch ui.FooterMode(a.page) {
	case ui.FooterOverview:
		sw := a.btnW(scanLabel)
		a.placeBtn(a.btnScan, rect(right-sw, fy, right, fy+btnH), scanLabel, true, true)
		if !a.elevated {
			aw := a.btnW("以管理员运行")
			a.placeBtn(a.btnAdmin, rect(right-sw-a.scale(12)-aw, fy, right-sw-a.scale(12), fy+btnH), "以管理员运行", true, !busy)
		}
	case ui.FooterClean:
		if busy && (a.phase == phaseScan || a.phase == phaseClean) {
			sw := a.btnW(scanLabel)
			a.placeBtn(a.btnScan, rect(right-sw, fy, right, fy+btnH), scanLabel, true, true)
		} else if !a.scanned {
			sw := a.btnW("开始扫描")
			a.placeBtn(a.btnScan, rect(right-sw, fy, right, fy+btnH), "开始扫描", true, true)
		} else {
			cw := a.btnW(cleanLabel)
			sw := a.btnW(scanLabel)
			a.placeBtn(a.btnClean, rect(right-cw, fy, right, fy+btnH), cleanLabel, true, !busy && count > 0)
			a.placeBtn(a.btnScan, rect(right-cw-a.scale(12)-sw, fy, right-cw-a.scale(12), fy+btnH), scanLabel, true, true)
		}
	case ui.FooterAnalyze:
		if a.page == ui.PageLarge {
			label := "开始分析"
			if a.phase == phaseAnalyze {
				label = "取消分析"
			}
			a.placeBtn(a.btnWork, rect(fx, fy, fx+a.btnW(label), fy+btnH), label, true, true)
		}
	case ui.FooterDupes:
		label := "查找重复"
		if a.phase == phaseDupes {
			label = "取消查找"
		}
		a.placeBtn(a.btnWork, rect(fx, fy, fx+a.btnW(label), fy+btnH), label, true, true)
	case ui.FooterApps:
		a.placeBtn(a.btnWork, rect(fx, fy, fx+a.btnW("刷新软件列表"), fy+btnH), "刷新软件列表", true, !busy)
	case ui.FooterSystem:
		a.placeBtn(a.btnWork, rect(fx, fy, fx+a.btnW("刷新系统占用"), fy+btnH), "刷新系统占用", true, !busy)
	}
	footerH := int32(a.scale(ui.FooterH))
	scrollW := int32(a.scale(ui.ScrollBarW))
	hostH := client.Bottom - footerH
	if hostH < 0 {
		hostH = 0
	}
	a.viewH = int(hostH)
	if a.host != 0 {
		win.MoveWindow(a.host, nav.Right, 0, client.Right-nav.Right-scrollW, hostH, false)
	}
	if a.scrollBar != 0 {
		win.MoveWindow(a.scrollBar, client.Right-scrollW, 0, scrollW, hostH, false)
	}
}

func (a *app) btnW(label string) int {
	n := len([]rune(label))
	w := n*14 + 36
	if w < 108 {
		w = 108
	}
	if w > 220 {
		w = 220
	}
	return a.scale(w)
}

func (a *app) layoutChecks(client win.RECT) {
	need := 0
	if a.page == ui.PageClean && a.scanned {
		need = len(a.reports)
	}
	if a.page == ui.PageSettings {
		need = len(a.settingCats)
	}
	for len(a.checkBtns) < need {
		id := idCheckBase + len(a.checkBtns)
		hwnd := createBtn(a.host, id, bsAutoCheck, "")
		a.applyType(hwnd, ui.TypeItem)
		a.checkBtns = append(a.checkBtns, hwnd)
	}
	if a.page == ui.PageClean && a.scanned {
		for len(a.detailBtns) < len(a.reports) {
			id := idDetailBase + len(a.detailBtns)
			hwnd := createBtn(a.host, id, bsPushButton, "详情")
			a.applyType(hwnd, ui.TypeButton)
			a.detailBtns = append(a.detailBtns, hwnd)
		}
	}
	for i, hwnd := range a.checkBtns {
		if i >= need || i >= len(a.checkLaid) || !a.checkLaid[i].show {
			win.ShowWindow(hwnd, win.SW_HIDE)
		}
	}
	wantDetail := a.page == ui.PageClean && a.scanned
	for i, hwnd := range a.detailBtns {
		if !wantDetail || i >= len(a.detailLaid) || !a.detailLaid[i].show {
			win.ShowWindow(hwnd, win.SW_HIDE)
		}
	}
	_ = client
}

func (a *app) placeCheck(i int, rc win.RECT, on, show bool) {
	for len(a.checkLaid) <= i {
		a.checkLaid = append(a.checkLaid, placedCheck{})
	}
	a.checkLaid[i] = placedCheck{rc: rc, on: on, show: show}
	a.applyCheck(i, a.checkLaid[i])
}

func (a *app) flushChecks() {
	for i, p := range a.checkLaid {
		a.applyCheck(i, p)
	}
}

func (a *app) applyCheck(i int, p placedCheck) {
	if i < 0 || i >= len(a.checkBtns) {
		return
	}
	hwnd := a.checkBtns[i]
	if hwnd == 0 {
		return
	}
	if !p.show {
		win.ShowWindow(hwnd, win.SW_HIDE)
		return
	}
	name := ""
	advanced := false
	recommend := false
	if a.page == ui.PageClean && i < len(a.reports) {
		cat := a.reports[i].Category
		name = cat.Name
		advanced = ui.CleanSection(cat) == ui.SectionAdvanced
		recommend = ui.CleanSection(cat) == ui.SectionRecommend
	}
	if a.page == ui.PageSettings && i < len(a.settingCats) {
		cat := a.settingCats[i]
		name = cat.Name
		advanced = ui.CleanSection(cat) == ui.SectionAdvanced
	}
	caption := name
	access := name
	if name != "" {
		risk := ""
		if a.page == ui.PageClean && i < len(a.reports) {
			risk = ui.RiskText(a.reports[i].Category.Risk)
		}
		if a.page == ui.PageSettings && i < len(a.settingCats) {
			risk = ui.RiskText(a.settingCats[i].Risk)
		}
		access = ui.ChoiceAccessName(name, risk, advanced)
		if a.page == ui.PageSettings {
			caption = access
		}
		setText(hwnd, access)
	}
	bg := colorCard
	if advanced {
		bg = colorAdvBg
	} else if recommend {
		bg = colorRecBg
	}
	setFace(hwnd, ctrlFace{kind: faceCheck, caption: caption, bg: bg})
	cur := win.SendMessage(hwnd, bmGetCheck, 0, 0) != 0
	if cur != p.on {
		state := uintptr(0)
		if p.on {
			state = 1
		}
		win.SendMessage(hwnd, bmSetCheck, state, 0)
	}
	win.MoveWindow(hwnd, p.rc.Left, p.rc.Top, p.rc.Right-p.rc.Left, p.rc.Bottom-p.rc.Top, false)
	win.ShowWindow(hwnd, win.SW_SHOW)
}

func (a *app) placeDetail(i int, rc win.RECT, text string, show bool) {
	for len(a.detailLaid) <= i {
		a.detailLaid = append(a.detailLaid, placedBtn{})
	}
	a.detailLaid[i] = placedBtn{rc: rc, text: text, show: show}
	a.applyDetail(i, a.detailLaid[i])
}

func (a *app) flushDetails() {
	for i, p := range a.detailLaid {
		a.applyDetail(i, p)
	}
}

func (a *app) applyDetail(i int, p placedBtn) {
	if i < 0 || i >= len(a.detailBtns) {
		return
	}
	hwnd := a.detailBtns[i]
	if hwnd == 0 {
		return
	}
	if !p.show {
		win.ShowWindow(hwnd, win.SW_HIDE)
		return
	}
	name := ""
	if i < len(a.reports) {
		name = a.reports[i].Category.Name
	}
	access := ui.DetailAccessName(name, p.text == "收起")
	setText(hwnd, access)
	setFace(hwnd, ctrlFace{kind: faceSecondary, caption: p.text})
	win.MoveWindow(hwnd, p.rc.Left, p.rc.Top, p.rc.Right-p.rc.Left, p.rc.Bottom-p.rc.Top, false)
	win.ShowWindow(hwnd, win.SW_SHOW)
}

func (a *app) isCheck(hwnd win.HWND) bool {
	for _, h := range a.checkBtns {
		if h == hwnd {
			return true
		}
	}
	return false
}

func (a *app) eachShell(fn func(win.HWND)) {
	for _, h := range a.navBtns {
		fn(h)
	}
	for _, h := range []win.HWND{a.btnScan, a.btnClean, a.btnWork, a.btnAdmin, a.scrollBar} {
		fn(h)
	}
}

func (a *app) eachPage(fn func(win.HWND)) {
	for _, h := range a.checkBtns {
		fn(h)
	}
	for _, h := range a.detailBtns {
		fn(h)
	}
	for _, h := range a.actBtns {
		fn(h)
	}
}

func excludeList(hdc win.HDC, parent win.HWND, each func(func(win.HWND))) {
	if hdc == 0 || parent == 0 {
		return
	}
	excl := gdi32.NewProc("ExcludeClipRect")
	each(func(hwnd win.HWND) {
		if hwnd == 0 || !win.IsWindowVisible(hwnd) {
			return
		}
		var wr win.RECT
		win.GetWindowRect(hwnd, &wr)
		tl := win.POINT{X: wr.Left, Y: wr.Top}
		br := win.POINT{X: wr.Right, Y: wr.Bottom}
		user32.NewProc("ScreenToClient").Call(uintptr(parent), uintptr(unsafe.Pointer(&tl)))
		user32.NewProc("ScreenToClient").Call(uintptr(parent), uintptr(unsafe.Pointer(&br)))
		excl.Call(uintptr(hdc), uintptr(tl.X), uintptr(tl.Y), uintptr(br.X), uintptr(br.Y))
	})
}

func (a *app) excludeShell(hdc win.HDC) {
	excludeList(hdc, a.win, a.eachShell)
	if a.host != 0 && win.IsWindowVisible(a.host) {
		excludeList(hdc, a.win, func(fn func(win.HWND)) { fn(a.host) })
	}
}

func (a *app) excludePage(hdc win.HDC) {
	excludeList(hdc, a.host, a.eachPage)
}

func refreshList(each func(func(win.HWND))) {
	each(func(hwnd win.HWND) {
		if hwnd == 0 || !win.IsWindowVisible(hwnd) {
			return
		}
		win.InvalidateRect(hwnd, nil, false)
		win.UpdateWindow(hwnd)
	})
}

func (a *app) refreshShell() {
	refreshList(a.eachShell)
}

func (a *app) refreshPage() {
	refreshList(a.eachPage)
}

func (a *app) hasNativeNav() bool {
	return len(a.navBtns) > 0 && a.navBtns[0] != 0
}

func (a *app) hasNativeFooter() bool {
	return a.btnScan != 0
}

func (a *app) hasNativeCheck(i int) bool {
	return i >= 0 && i < len(a.checkBtns) && a.checkBtns[i] != 0
}

func windowText(hwnd win.HWND) string {
	var buf [128]uint16
	n, _, _ := user32.NewProc("GetWindowTextW").Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

func (a *app) onCommand(wParam, lParam uintptr) {
	id := int(uint16(wParam))
	notify := uint16(wParam >> 16)
	if notify == 6 { // BN_SETFOCUS
		a.revealControl(win.HWND(lParam))
		return
	}
	if notify != 0 {
		return
	}
	switch {
	case id >= idNavBase && id < idNavBase+32:
		a.setPage(id - idNavBase)
	case id == idFootScan:
		a.onClickHit(hit{kind: hitScan})
	case id == idFootClean:
		a.onClickHit(hit{kind: hitClean})
	case id == idFootSel:
		a.onClickHit(hit{kind: hitSelect})
	case id == idFootAdmin:
		a.onClickHit(hit{kind: hitElevate})
	case id == idFootWork:
		a.clickWork()
	case id >= idCheckBase && id < idCheckBase+512:
		a.clickCheck(id - idCheckBase)
	case id >= idDetailBase && id < idDetailBase+512:
		a.clickDetail(id - idDetailBase)
	case id >= idActionBase && id < idActionBase+actionCap:
		a.runAction(id - idActionBase)
	}
}

func (a *app) runAction(index int) {
	if index < 0 || index >= len(a.actLaid) {
		return
	}
	p := a.actLaid[index]
	a.onClickHit(hit{kind: p.kind, index: p.index, id: p.id})
}

func (a *app) placeAction(rc win.RECT, caption, access string, kind, index int, id string, style int) {
	if len(a.actLaid) >= actionCap || rc.Right <= rc.Left || rc.Bottom <= rc.Top {
		return
	}
	if access == "" {
		access = caption
	}
	a.actLaid = append(a.actLaid, actionPlace{
		rc: rc, caption: caption, access: access, show: true, style: style, kind: kind, id: id, index: index,
	})
}

func (a *app) flushActions() {
	parent := a.host
	if parent == 0 {
		return
	}
	for len(a.actBtns) < len(a.actLaid) {
		hwnd := createBtn(parent, idActionBase+len(a.actBtns), bsPushButton, "")
		a.applyType(hwnd, ui.TypeButton)
		a.actBtns = append(a.actBtns, hwnd)
	}
	for i, hwnd := range a.actBtns {
		if hwnd == 0 {
			continue
		}
		if i >= len(a.actLaid) || !a.actLaid[i].show {
			win.ShowWindow(hwnd, win.SW_HIDE)
			continue
		}
		p := a.actLaid[i]
		setText(hwnd, p.access)
		setFace(hwnd, ctrlFace{
			kind: p.style, caption: p.caption, bg: p.bg, fg: p.fg, edge: p.edge, detail: p.detail, tip: p.tip,
		})
		win.EnableWindow(hwnd, true)
		win.MoveWindow(hwnd, p.rc.Left, p.rc.Top, p.rc.Right-p.rc.Left, p.rc.Bottom-p.rc.Top, false)
		win.ShowWindow(hwnd, win.SW_SHOW)
	}
}

func contentProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	if uiPtr == nil {
		return win.DefWindowProc(win.HWND(hwnd), uint32(msg), wParam, lParam)
	}
	return uiPtr.hostProc(win.HWND(hwnd), uint32(msg), wParam, lParam)
}

func (a *app) hostProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_PAINT:
		a.paintHost()
		return 0
	case win.WM_COMMAND:
		a.onCommand(wParam, lParam)
		return 0
	case win.WM_MOUSEWHEEL:
		a.onWheel(int16(wParam >> 16))
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func (a *app) revealControl(hwnd win.HWND) {
	if hwnd == 0 || a.host == 0 || win.GetParent(hwnd) != a.host {
		return
	}
	var wr win.RECT
	win.GetWindowRect(hwnd, &wr)
	top := win.POINT{X: wr.Left, Y: wr.Top}
	bot := win.POINT{X: wr.Left, Y: wr.Bottom}
	screenToClient(a.host, &top)
	screenToClient(a.host, &bot)
	margin := int32(a.scale(8))
	var client win.RECT
	win.GetClientRect(a.host, &client)
	delta := 0
	if top.Y < margin {
		delta = int(top.Y - margin)
	} else if bot.Y > client.Bottom-margin {
		delta = int(bot.Y - (client.Bottom - margin))
	}
	if delta != 0 {
		a.scrollBy(delta)
	}
}

func screenToClient(hwnd win.HWND, pt *win.POINT) {
	user32.NewProc("ScreenToClient").Call(uintptr(hwnd), uintptr(unsafe.Pointer(pt)))
}

type scrollInfo struct {
	cbSize    uint32
	fMask     uint32
	nMin      int32
	nMax      int32
	nPage     uint32
	nPos      int32
	nTrackPos int32
}

var (
	procSetScrollInfo = user32.NewProc("SetScrollInfo")
	procGetScrollInfo = user32.NewProc("GetScrollInfo")
)

func (a *app) updateScrollBar() {
	if a.scrollBar == 0 {
		return
	}
	view := a.viewH
	if view < 1 {
		view = 1
	}
	max := a.contentH
	if max < 1 {
		max = 1
	}
	info := scrollInfo{
		cbSize: uint32(unsafe.Sizeof(scrollInfo{})),
		fMask:  0x17,
		nMin:   0,
		nMax:   int32(max - 1),
		nPage:  uint32(view),
		nPos:   int32(a.scroll),
	}
	procSetScrollInfo.Call(uintptr(a.scrollBar), 2, uintptr(unsafe.Pointer(&info)), 1)
	if a.contentH <= a.viewH {
		win.ShowWindow(a.scrollBar, win.SW_HIDE)
		return
	}
	win.ShowWindow(a.scrollBar, win.SW_SHOW)
}

func (a *app) onVScroll(wParam uintptr) {
	code := int(wParam & 0xffff)
	line := a.scale(48)
	view := a.viewH
	if view < line {
		view = line
	}
	switch code {
	case 0:
		a.scrollBy(-line)
	case 1:
		a.scrollBy(line)
	case 2:
		a.scrollBy(-view)
	case 3:
		a.scrollBy(view)
	case 4, 5:
		info := scrollInfo{cbSize: uint32(unsafe.Sizeof(scrollInfo{})), fMask: 0x10}
		procGetScrollInfo.Call(uintptr(a.scrollBar), 2, uintptr(unsafe.Pointer(&info)))
		a.scrollTo(int(info.nTrackPos))
	case 6:
		a.scrollTo(0)
	case 7:
		a.scrollTo(a.contentH)
	}
}

func (a *app) clickWork() {
	switch a.page {
	case ui.PageAnalyze, ui.PageLarge:
		a.onClickHit(hit{kind: hitAnalyze})
	case ui.PageDupes:
		a.onClickHit(hit{kind: hitDupes})
	case ui.PageApps:
		a.onClickHit(hit{kind: hitApps})
	case ui.PageSystem:
		a.onClickHit(hit{kind: hitSys})
	}
}

func (a *app) clickCheck(index int) {
	if index < 0 || index >= len(a.checkBtns) {
		return
	}
	on := win.SendMessage(a.checkBtns[index], bmGetCheck, 0, 0) != 0
	id := ""
	if a.page == ui.PageClean && index < len(a.reports) {
		id = a.reports[index].Category.ID
	}
	if a.page == ui.PageSettings && index < len(a.settingCats) {
		id = a.settingCats[index].ID
	}
	if id == "" {
		return
	}
	if a.choices == nil {
		a.choices = map[string]bool{}
	}
	a.choices[id] = on
	a.persistChoices()
	a.redraw()
}

func (a *app) clickDetail(index int) {
	if index < 0 || index >= len(a.reports) {
		return
	}
	id := a.reports[index].Category.ID
	if a.expanded == nil {
		a.expanded = map[string]bool{}
	}
	a.expanded[id] = !a.expanded[id]
	a.redraw()
}
