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
	idCheckBase  = 3000
	idDetailBase = 4000
	bsPushButton = 0x00000000
	bsDefPush    = 0x00000001
	bsAutoCheck  = 0x00000003
	bsOwnerDraw  = 0x0000000B
	bmGetCheck   = 0x00F0
	bmSetCheck   = 0x00F1
	odsSelected  = 0x0001
	odsDisabled  = 0x0004
	odsFocus     = 0x0010
	odsHotlight  = 0x0040
)

type placedCheck struct {
	rc   win.RECT
	on   bool
	show bool
}

type placedBtn struct {
	rc   win.RECT
	text string
	show bool
}

type navSlot struct {
	item   ui.NavItem
	rc     win.RECT
	group  string
	groupY int
}

type drawItemStruct struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   win.HWND
	HDC        win.HDC
	RcItem     win.RECT
	ItemData   uintptr
}

func mouseClient(hwnd win.HWND, lParam uintptr) (int, int) {
	var pt win.POINT
	user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&pt)))
	user32.NewProc("ScreenToClient").Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pt)))
	if pt.X != 0 || pt.Y != 0 {
		return int(pt.X), int(pt.Y)
	}
	return int(int16(lParam)), int(int16(lParam >> 16))
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
	for _, hwnd := range []win.HWND{a.btnScan, a.btnClean, a.btnSelect, a.btnWork, a.btnAdmin} {
		a.applyType(hwnd, ui.TypeButton)
	}
	for _, hwnd := range a.checkBtns {
		a.applyType(hwnd, ui.TypeItem)
	}
	for _, hwnd := range a.detailBtns {
		a.applyType(hwnd, ui.TypeButton)
	}
}

func createBtn(parent win.HWND, id int, style uint32, label string) win.HWND {
	return win.CreateWindowEx(0, u16("BUTTON"), u16(label),
		win.WS_CHILD|win.WS_TABSTOP|win.WS_CLIPSIBLINGS|style,
		0, 0, 10, 10, parent, win.HMENU(id), win.GetModuleHandle(nil), nil)
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
		a.btnSelect = createBtn(a.win, idFootSel, bsPushButton, "全选")
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
	win.EnableWindow(hwnd, enabled)
	win.MoveWindow(hwnd, rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, true)
	win.ShowWindow(hwnd, win.SW_SHOW)
}

func (a *app) navSlots(nav win.RECT) []navSlot {
	var slots []navSlot
	y := int(nav.Top) + a.scale(60)
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
		a.placeBtn(a.navBtns[i], rc, item.Title, true, true)
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
	a.placeBtn(a.btnSelect, rect(0, 0, 0, 0), a.selectLabel(), false, true)
	a.placeBtn(a.btnWork, rect(0, 0, 0, 0), "", false, true)
	a.placeBtn(a.btnAdmin, rect(0, 0, 0, 0), "以管理员运行", false, true)
	right := int(client.Right) - pad
	switch a.page {
	case ui.PageOverview:
		sw := a.btnW(scanLabel)
		a.placeBtn(a.btnScan, rect(right-sw, fy, right, fy+btnH), scanLabel, true, true)
		if !a.elevated {
			aw := a.btnW("以管理员运行")
			a.placeBtn(a.btnAdmin, rect(right-sw-a.scale(12)-aw, fy, right-sw-a.scale(12), fy+btnH), "以管理员运行", true, !busy)
		}
	case ui.PageClean:
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
		if a.selShow {
			a.placeBtn(a.btnSelect, a.selRC, a.selectLabel(), true, !busy && a.scanned)
		}
	case ui.PageAnalyze:
		label := "开始分析"
		if a.phase == phaseAnalyze {
			label = "取消分析"
		}
		a.placeBtn(a.btnWork, rect(fx, fy, fx+a.btnW(label), fy+btnH), label, true, true)
	case ui.PageDupes:
		label := "查找重复"
		if a.phase == phaseDupes {
			label = "取消查找"
		}
		a.placeBtn(a.btnWork, rect(fx, fy, fx+a.btnW(label), fy+btnH), label, true, true)
	case ui.PageApps:
		a.placeBtn(a.btnWork, rect(fx, fy, fx+a.btnW("刷新软件列表"), fy+btnH), "刷新软件列表", true, !busy)
	case ui.PageSystem:
		a.placeBtn(a.btnWork, rect(fx, fy, fx+a.btnW("刷新系统占用"), fy+btnH), "刷新系统占用", true, !busy)
	}
	a.layoutChecks(client)
	a.flushChecks()
	a.flushDetails()
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
		hwnd := createBtn(a.win, id, bsAutoCheck, "")
		a.applyType(hwnd, ui.TypeItem)
		a.checkBtns = append(a.checkBtns, hwnd)
	}
	if a.page == ui.PageClean && a.scanned {
		for len(a.detailBtns) < len(a.reports) {
			id := idDetailBase + len(a.detailBtns)
			hwnd := createBtn(a.win, id, bsPushButton, "详情")
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
	if a.page == ui.PageClean && i < len(a.reports) {
		name = a.reports[i].Category.Name
	}
	if a.page == ui.PageSettings && i < len(a.settingCats) {
		cat := a.settingCats[i]
		name = cat.Name + " · " + ui.RiskText(cat.Risk)
		if ui.CleanSection(cat) == ui.SectionAdvanced {
			name += " · 默认关闭"
		}
	}
	if name != "" {
		setText(hwnd, name)
	}
	cur := win.SendMessage(hwnd, bmGetCheck, 0, 0) != 0
	if cur != p.on {
		state := uintptr(0)
		if p.on {
			state = 1
		}
		win.SendMessage(hwnd, bmSetCheck, state, 0)
	}
	if a.obscuredByFooter(p.rc) {
		win.ShowWindow(hwnd, win.SW_HIDE)
		return
	}
	win.MoveWindow(hwnd, p.rc.Left, p.rc.Top, p.rc.Right-p.rc.Left, p.rc.Bottom-p.rc.Top, true)
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
	if !p.show || a.obscuredByFooter(p.rc) {
		win.ShowWindow(hwnd, win.SW_HIDE)
		return
	}
	setText(hwnd, p.text)
	win.MoveWindow(hwnd, p.rc.Left, p.rc.Top, p.rc.Right-p.rc.Left, p.rc.Bottom-p.rc.Top, true)
	win.ShowWindow(hwnd, win.SW_SHOW)
}

func (a *app) obscuredByFooter(rc win.RECT) bool {
	if a.win == 0 {
		return false
	}
	var client win.RECT
	win.GetClientRect(a.win, &client)
	return rc.Bottom > client.Bottom-int32(a.scale(ui.FooterH))
}

func (a *app) isCheck(hwnd win.HWND) bool {
	for _, h := range a.checkBtns {
		if h == hwnd {
			return true
		}
	}
	return false
}

func (a *app) eachNative(fn func(win.HWND)) {
	for _, h := range a.navBtns {
		fn(h)
	}
	for _, h := range a.checkBtns {
		fn(h)
	}
	for _, h := range a.detailBtns {
		fn(h)
	}
	for _, h := range []win.HWND{a.btnScan, a.btnClean, a.btnSelect, a.btnWork, a.btnAdmin} {
		fn(h)
	}
}

func (a *app) excludeChildren(hdc win.HDC) {
	if hdc == 0 || a.win == 0 {
		return
	}
	excl := gdi32.NewProc("ExcludeClipRect")
	a.eachNative(func(hwnd win.HWND) {
		if hwnd == 0 || !win.IsWindowVisible(hwnd) {
			return
		}
		var wr win.RECT
		win.GetWindowRect(hwnd, &wr)
		tl := win.POINT{X: wr.Left, Y: wr.Top}
		br := win.POINT{X: wr.Right, Y: wr.Bottom}
		user32.NewProc("ScreenToClient").Call(uintptr(a.win), uintptr(unsafe.Pointer(&tl)))
		user32.NewProc("ScreenToClient").Call(uintptr(a.win), uintptr(unsafe.Pointer(&br)))
		excl.Call(uintptr(hdc), uintptr(tl.X), uintptr(tl.Y), uintptr(br.X), uintptr(br.Y))
	})
}

func (a *app) refreshChildren() {
	a.eachNative(func(hwnd win.HWND) {
		if hwnd == 0 || !win.IsWindowVisible(hwnd) {
			return
		}
		win.InvalidateRect(hwnd, nil, true)
		win.UpdateWindow(hwnd)
	})
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

func (a *app) drawItem(lParam uintptr) {
	if lParam == 0 {
		return
	}
	dis := (*drawItemStruct)(unsafe.Pointer(lParam))
	hdc := dis.HDC
	rc := dis.RcItem
	enabled := dis.ItemState&odsDisabled == 0
	focus := dis.ItemState&odsFocus != 0
	hot := dis.ItemState&odsHotlight != 0 || dis.ItemState&odsSelected != 0
	id := int(dis.CtlID)
	label := windowText(dis.HwndItem)
	win.SetBkMode(hdc, win.TRANSPARENT)
	if id >= idNavBase && id < idNavBase+32 {
		page := id - idNavBase
		if a.page == page {
			roundFill(hdc, rc, ui.Radius, colorNavPick, colorNavPick)
			bar := rc
			bar.Right = bar.Left + 3
			roundFill(hdc, bar, 2, colorPrimary, colorPrimary)
			win.SetTextColor(hdc, colorPrimary)
		} else if hot {
			roundFill(hdc, rc, ui.Radius, colorPrimarySoft, colorPrimarySoft)
			win.SetTextColor(hdc, colorText)
		} else {
			roundFill(hdc, rc, ui.Radius, colorNav, colorNav)
			win.SetTextColor(hdc, colorText)
		}
		icon := "settings"
		for _, item := range ui.NavItems() {
			if item.Page == page {
				icon = item.Icon
				break
			}
		}
		paintIcon(hdc, rect(int(rc.Left)+8, int(rc.Top)+8, int(rc.Left)+28, int(rc.Bottom)-8), icon, colorText)
		a.useType(hdc, ui.TypeNav)
		draw(hdc, label, rect(int(rc.Left)+32, int(rc.Top), int(rc.Right)-8, int(rc.Bottom)), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
		if focus {
			paintFocus(hdc, rc)
		}
		return
	}
	primary := id == idFootScan && (a.page == ui.PageOverview || !a.scanned || a.phase == phaseScan || a.phase == phaseClean)
	if id == idFootClean || id == idFootWork {
		primary = enabled
	}
	if id == idFootWork && a.page != ui.PageAnalyze && a.page != ui.PageDupes {
		primary = enabled
	}
	if id == idFootSel || id == idFootAdmin {
		primary = false
	}
	if primary && enabled {
		paintPrimaryBtn(hdc, rc, label, true)
	} else {
		paintGhostBtn(hdc, rc, label, enabled)
	}
	if focus {
		paintFocus(hdc, rc)
	}
}

func windowText(hwnd win.HWND) string {
	var buf [128]uint16
	n, _, _ := user32.NewProc("GetWindowTextW").Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

func (a *app) onCommand(wParam uintptr) {
	id := int(uint16(wParam))
	notify := uint16(wParam >> 16)
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
	}
}

func (a *app) clickWork() {
	switch a.page {
	case ui.PageAnalyze:
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
