//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"cclear/internal/appinfo"
	"cclear/internal/clean"
	"cclear/internal/history"
	"cclear/internal/ui"
	"github.com/lxn/win"
)

const (
	hitNav = iota + 1
	hitCheck
	hitExpand
	hitDetail
	hitScan
	hitClean
	hitSelect
	hitElevate
	hitAnalyze
	hitOpenLog
	hitPrune
	hitHistory
	hitChoice
	hitScroll
	hitDupes
	hitApps
	hitSys
	hitFilter
	hitReveal
	hitCopy
	hitOpen
	hitTree
	hitMap
	hitUninstall
	hitRecycle
	hitSnapPrune
	hitWinStorage
	hitProps
	hitAdvanced
	hitTheme
	hitLogKeep
	hitGo
	hitFileMenu
	hitAnalyzeTab
)

type hit struct {
	kind  int
	index int
	id    string
	rc    win.RECT
}

func (a *app) paintShell(hdc win.HDC, client win.RECT) {
	fill(hdc, client, a.pageBrush)
	navW := int32(a.scale(ui.NavWidth))
	nav := client
	nav.Right = navW
	roundFill(hdc, nav, 0, colorNav, colorNav)
	fillColor(hdc, rect(int(nav.Right)-1, int(client.Top), int(nav.Right), int(client.Bottom)), colorLine)
	foot := client
	foot.Left = navW
	foot.Top = client.Bottom - int32(a.scale(ui.FooterH))
	roundFill(hdc, foot, 0, colorFooter, colorFooter)
	hline(hdc, int(foot.Left), int(foot.Right), int(foot.Top), colorLine)
	a.paintNav(hdc, nav)
	a.paintFooter(hdc, foot)
}

func (a *app) paintHost() {
	if a.host == 0 || a.painting {
		return
	}
	a.painting = true
	defer func() { a.painting = false }()
	var client win.RECT
	win.GetClientRect(a.host, &client)
	if client.Right <= 1 || client.Bottom <= 1 {
		return
	}
	a.actLaid = a.actLaid[:0]
	a.checkLaid = a.checkLaid[:0]
	a.detailLaid = a.detailLaid[:0]
	a.selShow = false
	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(a.host, &ps)
	mem := win.CreateCompatibleDC(hdc)
	bmp := win.CreateCompatibleBitmap(hdc, client.Right, client.Bottom)
	old := win.SelectObject(mem, win.HGDIOBJ(bmp))
	fill(mem, client, a.pageBrush)
	a.paintPage(mem, client)
	a.layoutChecks(client)
	a.flushChecks()
	a.flushDetails()
	a.flushActions()
	a.excludePage(hdc)
	win.BitBlt(hdc, 0, 0, client.Right, client.Bottom, mem, 0, 0, win.SRCCOPY)
	win.SelectObject(mem, old)
	win.DeleteObject(win.HGDIOBJ(bmp))
	win.DeleteDC(mem)
	win.EndPaint(a.host, &ps)
	a.refreshPage()
	a.updateScrollBar()
}

func (a *app) layoutContent(rc win.RECT, origin int) (x, y, right int) {
	pad := a.scale(24)
	max := a.scale(ui.ContentMax)
	inner := int(rc.Right - rc.Left)
	if inner > max+pad*2 {
		extra := inner - max - pad*2
		x = int(rc.Left) + pad + extra/2
		right = x + max
	} else {
		x = int(rc.Left) + pad
		right = int(rc.Right) - pad
	}
	y = origin + a.scale(16)
	return
}

func (a *app) paintPageHeader(hdc win.HDC, x, y, right int, title, hint string) int {
	a.useType(hdc, ui.TypePageTitle)
	win.SetTextColor(hdc, colorText)
	titleH := a.typeH(ui.TypePageTitle)
	draw(hdc, title, rect(x+a.scale(ui.TextInset), y, right-a.scale(ui.TextInset), y+titleH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	y += titleH + a.scale(6)
	a.useType(hdc, ui.TypePageHint)
	win.SetTextColor(hdc, colorMuted)
	hintH := a.typeH(ui.TypePageHint) * 2
	draw(hdc, hint, rect(x, y, right, y+hintH), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	return y + hintH + a.scale(16)
}

func (a *app) paintSectionLabel(hdc win.HDC, x, y, right int, title string) int {
	a.useType(hdc, ui.TypeSection)
	win.SetTextColor(hdc, colorText)
	h := a.typeH(ui.TypeSection)
	draw(hdc, title, rect(x, y, right, y+h), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	return y + h + a.scale(10)
}

func (a *app) paintEmptyState(hdc win.HDC, x, y, right int, title, body string) int {
	h := a.scale(104)
	roundFill(hdc, rect(x, y, right, y+h), ui.Radius, colorCard, colorLine)
	a.useType(hdc, ui.TypeSection)
	win.SetTextColor(hdc, colorText)
	draw(hdc, title, rect(x+a.scale(20), y+a.scale(20), right-a.scale(20), y+a.scale(20)+a.typeH(ui.TypeSection)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	a.useType(hdc, ui.TypeBody)
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, body, rect(x+a.scale(20), y+a.scale(52), right-a.scale(20), y+h-a.scale(16)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	return y + h
}

func (a *app) paintNav(hdc win.HDC, rc win.RECT) {
	win.SetBkMode(hdc, win.TRANSPARENT)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.fontOf(ui.TypeSection)))
	mark := rect(int(rc.Left)+a.scale(16), int(rc.Top)+a.scale(16), int(rc.Left)+a.scale(44), int(rc.Top)+a.scale(44))
	roundFill(hdc, mark, 8, colorPrimary, colorPrimary)
	win.SetTextColor(hdc, colorOnPrimary)
	draw(hdc, "C", mark, win.DT_CENTER|dtSingleLine|dtVCenter|dtNoPrefix)
	win.SetTextColor(hdc, colorText)
	draw(hdc, appinfo.Name, rect(int(rc.Left)+a.scale(52), int(rc.Top)+a.scale(16), int(rc.Right)-a.scale(12), int(rc.Top)+a.scale(44)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	for _, slot := range a.navSlots(rc) {
		if slot.group != "" {
			a.useType(hdc, ui.TypeCaption)
			win.SetTextColor(hdc, colorCaption)
			draw(hdc, slot.group, rect(int(rc.Left)+a.scale(16), slot.groupY, int(rc.Right)-a.scale(12), slot.groupY+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		}
	}
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorCaption)
	draw(hdc, ui.FormatVersion(appinfo.Version), rect(int(rc.Left)+a.scale(16), int(rc.Bottom)-a.scale(28), int(rc.Right)-a.scale(12), int(rc.Bottom)-a.scale(10)), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	win.SelectObject(hdc, old)
}

func (a *app) paintFooter(hdc win.HDC, rc win.RECT) {
	win.SetBkMode(hdc, win.TRANSPARENT)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.fontOf(ui.TypeBody)))
	pad := a.scale(16)
	y := int(rc.Top)
	x := int(rc.Left) + pad
	if a.page == ui.PageSettings || a.page == ui.PageHistory {
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "此页只用于查看或保存配置，不会删除文件。", rect(x, y, int(rc.Right)-pad, int(rc.Bottom)), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
		win.SelectObject(hdc, old)
		return
	}
	if a.page == ui.PageAnalyze {
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "分析是只读的。大文件不会进入清理列表。", rect(x, y, int(rc.Right)-pad, int(rc.Bottom)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		win.SelectObject(hdc, old)
		return
	}
	if ui.FooterMode(a.page) == ui.FooterClean {
		count, reclaim, unknown, _, _ := ui.SelectedSummary(a.reports, a.choices)
		win.SetTextColor(hdc, colorText)
		left := fmt.Sprintf("已选择 %d 项", count)
		if a.scanned {
			if unknown && reclaim == 0 {
				left += " · 预计释放待系统分析"
			} else {
				left += " · 预计释放 " + clean.FormatBytes(reclaim)
			}
		}
		draw(hdc, left, rect(x, y, x+a.scale(360), int(rc.Bottom)), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
		win.SelectObject(hdc, old)
		return
	}
	win.SelectObject(hdc, old)
}

func (a *app) paintPage(hdc win.HDC, rc win.RECT) {
	clip := win.CreateRectRgn(rc.Left, rc.Top, rc.Right, rc.Bottom)
	procSelectClip.Call(uintptr(hdc), uintptr(clip))
	defer func() {
		procSelectClip.Call(uintptr(hdc), 0)
		win.DeleteObject(win.HGDIOBJ(clip))
	}()
	origin := int(rc.Top) - a.scroll
	switch a.page {
	case ui.PageOverview:
		a.contentH = a.paintOverview(hdc, rc, origin)
	case ui.PageClean:
		a.contentH = a.paintCards(hdc, rc, origin, a.reports)
	case ui.PageAnalyze:
		a.contentH = a.paintAnalyze(hdc, rc, origin)
	case ui.PageLarge:
		a.contentH = a.paintLargePage(hdc, rc, origin)
	case ui.PageDupes:
		a.contentH = a.paintDupes(hdc, rc, origin)
	case ui.PageApps:
		a.contentH = a.paintApps(hdc, rc, origin)
	case ui.PageSystem:
		a.contentH = a.paintSystem(hdc, rc, origin)
	case ui.PageHistory:
		a.contentH = a.paintHistory(hdc, rc, origin)
	case ui.PageSettings:
		a.contentH = a.paintSettings(hdc, rc, origin)
	}
	view := int(rc.Bottom - rc.Top)
	a.viewH = view
	if a.contentH > view {
		a.contentH += a.scale(ui.ScrollTail)
	} else if a.contentH < view {
		a.contentH = view
	}
	a.clampScroll()
}

func (a *app) paintOverview(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "总览", "先看还剩多少空间。开始扫描在窗口底部，只检查已核对的缓存。大文件不会被当成垃圾。")

	used := uint64(0)
	ratio := 0.0
	if a.total > 0 {
		if a.free < a.total {
			used = a.total - a.free
		}
		ratio = float64(used) / float64(a.total)
	}
	head := ui.Headline(a.reports, a.scanned)
	panelH := a.scale(208)
	roundFill(hdc, rect(x, y, right, y+panelH), 4, colorCard, colorLine)
	d := a.scale(120)
	dx := x + a.scale(24)
	dy := y + a.scale(28)
	paintDonut(hdc, rect(dx, dy, dx+d, dy+d), ratio, colorLine, colorPrimary, colorCard)
	split := x + (right-x)*58/100
	tx := dx + d + a.scale(20)
	textRight := split - a.scale(16)
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorCaption)
	nameRight := textRight
	if a.elevated {
		nameRight = textRight - a.scale(80)
		paintBadge(hdc, rect(textRight-a.scale(72), y+a.scale(22), textRight, y+a.scale(42)), "管理员", colorSuccess, colorCard)
	}
	draw(hdc, ui.FormatDrive(a.drive), rect(tx, y+a.scale(24), nameRight, y+a.scale(24)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorCaption)
	draw(hdc, "剩余", rect(tx, y+a.scale(48), textRight, y+a.scale(48)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	a.useType(hdc, ui.TypeMetric)
	win.SetTextColor(hdc, colorText)
	draw(hdc, dashBytes(int64(a.free)), rect(tx, y+a.scale(68), textRight, y+a.scale(68)+a.typeH(ui.TypeMetric)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "已用 "+dashBytes(int64(used))+"    总共 "+dashBytes(int64(a.total)), rect(tx, y+a.scale(102), textRight, y+a.scale(122)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	paintBar(hdc, rect(tx, y+a.scale(128), textRight, y+a.scale(136)), ratio, colorPrimary, colorLine)
	a.placeAction(rect(tx, y+a.scale(156), tx+a.btnW("查看磁盘分析"), y+a.scale(188)), "查看磁盘分析", "查看磁盘分析", hitAnalyze, 0, "", actSecondary)
	vline(hdc, split, y+a.scale(20), y+panelH-a.scale(20), colorLine)
	facts := []struct{ k, v string }{
		{"可安全释放", ui.ReclaimValue(head)},
		{"上次扫描", ui.FormatScanAt(a.scanAt)},
		{"最近清理", a.lastCleanLabel()},
		{"占用最多", a.topFolderLabel()},
	}
	factY := y + a.scale(24)
	factH := (panelH - a.scale(40)) / len(facts)
	for _, fact := range facts {
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		draw(hdc, fact.k, rect(split+a.scale(20), factY, right-a.scale(20), factY+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, fact.v, rect(split+a.scale(20), factY+a.typeH(ui.TypeCaption), right-a.scale(20), factY+factH-a.scale(4)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		factY += factH
	}
	y += panelH + a.scale(16)
	y = a.paintOverviewLower(hdc, x, y, right)
	if a.phase == phaseScan {
		win.SetTextColor(hdc, colorPrimary)
		draw(hdc, a.scanLine(), rect(x, y, right, y+a.scale(40)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		y += a.scale(48)
	}
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
}

func (a *app) paintOverviewLower(hdc win.HDC, x, y, right int) int {
	gap := a.scale(12)
	colW := (right - x - gap) / 2
	leftH := a.paintRankColumn(hdc, x, y, x+colW)
	rightH := a.paintHistoryColumn(hdc, x+colW+gap, y, right)
	if rightH > leftH {
		leftH = rightH
	}
	y += leftH + a.scale(8)
	y = a.paintSectionLabel(hdc, x, y, right, "常用功能")
	bx := x
	for _, item := range []struct {
		title string
		page  int
	}{
		{"垃圾清理", ui.PageClean},
		{"磁盘分析", ui.PageAnalyze},
		{"大文件", ui.PageLarge},
		{"重复文件", ui.PageDupes},
		{"软件管理", ui.PageApps},
	} {
		w := a.btnW(item.title)
		if bx+w > right {
			break
		}
		a.placeAction(rect(bx, y, bx+w, y+a.scale(32)), item.title, "打开"+item.title, hitGo, item.page, "", actSecondary)
		bx += w + a.scale(8)
	}
	return y + a.scale(32)
}

func (a *app) paintRankColumn(hdc win.HDC, x, y, right int) int {
	start := y
	y = a.paintSectionLabel(hdc, x, y, right, "空间占用")
	h := a.scale(176)
	roundFill(hdc, rect(x, y, right, y+h), 4, colorCard, colorLine)
	if !a.spaceOK || len(a.space.TopDirs) == 0 {
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, "还没有目录排行", rect(x+a.scale(16), y+a.scale(28), right-a.scale(16), y+a.scale(52)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "完成磁盘分析后，这里显示占用最多的目录。没有分析就不会显示数字。", rect(x+a.scale(16), y+a.scale(56), right-a.scale(16), y+h-a.scale(16)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		return y + h - start
	}
	n := len(a.space.TopDirs)
	if n > 5 {
		n = 5
	}
	rowH := h / n
	for i, dir := range a.space.TopDirs[:n] {
		ry := y + i*rowH
		if i > 0 {
			hline(hdc, x+a.scale(12), right-a.scale(12), ry, colorLine)
		}
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, dir.Name, rect(x+a.scale(12), ry, right-a.scale(96), ry+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		draw(hdc, clean.FormatBytes(dir.Allocated), rect(right-a.scale(92), ry, right-a.scale(12), ry+rowH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	}
	return y + h - start
}

func (a *app) paintHistoryColumn(hdc win.HDC, x, y, right int) int {
	start := y
	y = a.paintSectionLabel(hdc, x, y, right, "最近清理")
	h := a.scale(176)
	roundFill(hdc, rect(x, y, right, y+h), 4, colorCard, colorLine)
	if len(a.history) == 0 {
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, "还没有清理记录", rect(x+a.scale(16), y+a.scale(28), right-a.scale(16), y+a.scale(52)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "完成一次清理后，这里显示释放量、成功、跳过和失败。不会编造记录。", rect(x+a.scale(16), y+a.scale(56), right-a.scale(16), y+h-a.scale(16)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		return y + h - start
	}
	n := len(a.history)
	if n > 4 {
		n = 4
	}
	rowH := h / n
	for i, rec := range a.history[:n] {
		ry := y + i*rowH
		if i > 0 {
			hline(hdc, x+a.scale(12), right-a.scale(12), ry, colorLine)
		}
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		when := history.FormatWhen(rec.When)
		draw(hdc, when, rect(x+a.scale(12), ry+a.scale(4), right-a.scale(12), ry+rowH/2), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		line := fmt.Sprintf("删除 %d · 跳过 %d · 失败 %d", rec.Deleted, rec.Skipped, rec.Failed)
		if rec.Estimated != "" {
			line = rec.Estimated + " · " + line
		}
		draw(hdc, line, rect(x+a.scale(12), ry+rowH/2, right-a.scale(12), ry+rowH-a.scale(2)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	}
	return y + h - start
}

func (a *app) lastCleanLabel() string {
	if a.cleanOut.show && a.cleanOut.diskDelta != 0 {
		return clean.FormatBytes(a.cleanOut.diskDelta)
	}
	if len(a.history) > 0 && a.history[0].Estimated != "" {
		return a.history[0].Estimated
	}
	return "尚无记录"
}

func (a *app) topFolderLabel() string {
	if a.spaceOK && len(a.space.TopDirs) > 0 {
		name := a.space.TopDirs[0].Name
		if name == "" {
			name = "系统盘目录"
		}
		return name + " · " + clean.FormatBytes(a.space.TopDirs[0].Allocated)
	}
	return "打开磁盘分析查看"
}

func (a *app) summaryColumns(inner int) (left, width [4]int) {
	dipInner := inner
	if a.dpi > 0 {
		dipInner = inner * 96 / a.dpi
	}
	left, width = ui.SummaryColumns(dipInner)
	if a.dpi > 0 && a.dpi != 96 {
		for i := range left {
			left[i] = left[i] * a.dpi / 96
			width[i] = width[i] * a.dpi / 96
		}
	}
	end := left[3] + width[3]
	if end > inner && width[3] > end-inner {
		width[3] -= end - inner
	}
	return left, width
}

func (a *app) paintCards(hdc win.HDC, rc win.RECT, origin int, list []clean.Report) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "垃圾清理", "先扫描，再勾选，确认风险后才会删除。删除不进回收站。")

	head := ui.Headline(list, a.scanned)
	highN := ui.CountHighRisk(list)
	cardH := a.scale(124)
	if highN > 0 {
		cardH += a.typeH(ui.TypeCaption)
	}
	roundFill(hdc, rect(x, y, right, y+cardH), ui.Radius, colorCard, colorLine)
	split := x + (right-x)*2/5
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorCaption)
	draw(hdc, "可清理空间", rect(x+a.scale(20), y+a.scale(18), split-a.scale(16), y+a.scale(18)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	a.useType(hdc, ui.TypeMetric)
	win.SetTextColor(hdc, colorText)
	draw(hdc, ui.ReclaimValue(head), rect(x+a.scale(20), y+a.scale(42), split-a.scale(16), y+a.scale(42)+a.typeH(ui.TypeMetric)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "上次扫描  "+ui.FormatScanAt(a.scanAt), rect(x+a.scale(20), y+a.scale(80), split-a.scale(16), y+a.scale(80)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	vline(hdc, split, y+a.scale(16), y+cardH-a.scale(16), colorLine)
	rx := split + a.scale(20)
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorCaption)
	draw(hdc, "待系统分析", rect(rx, y+a.scale(16), right-a.scale(16), y+a.scale(16)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	a.useType(hdc, ui.TypeItem)
	win.SetTextColor(hdc, colorText)
	draw(hdc, ui.PendingAnalysis(head), rect(rx, y+a.scale(36), right-a.scale(16), y+a.scale(36)+a.typeH(ui.TypeItem)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorCaption)
	if highN > 0 {
		win.SetTextColor(hdc, colorMid)
	}
	draw(hdc, "高级项目", rect(rx, y+a.scale(64), right-a.scale(16), y+a.scale(64)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	a.useType(hdc, ui.TypeItem)
	win.SetTextColor(hdc, colorText)
	if highN > 0 {
		win.SetTextColor(hdc, colorMid)
	}
	draw(hdc, ui.HighRiskValue(highN), rect(rx, y+a.scale(84), right-a.scale(16), y+a.scale(84)+a.typeH(ui.TypeItem)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	if highN > 0 {
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorMid)
		draw(hdc, ui.HighRiskExplain(), rect(rx, y+a.scale(108), right-a.scale(16), y+a.scale(108)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	}
	y += cardH + a.scale(16)

	if a.phase == phaseScan {
		y = a.paintScanProgress(hdc, x, y, right)
	}
	if a.phase == phaseClean {
		win.SelectObject(hdc, win.HGDIOBJ(a.font))
		win.SetTextColor(hdc, colorPrimary)
		draw(hdc, a.cleanLine(), rect(x, y, right, y+a.scale(40)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		if a.progTotal > 0 {
			paintBar(hdc, rect(x, y+a.scale(44), right, y+a.scale(52)), float64(a.progDone)/float64(a.progTotal), colorPrimary, colorLine)
		}
		y += a.scale(68)
	}
	if a.cleanOut.show {
		y = a.paintCleanResult(hdc, x, y, right)
	}
	if a.scanFailed {
		roundFill(hdc, rect(x, y, right, y+a.scale(72)), ui.Radius, colorAdvBg, colorHigh)
		win.SetTextColor(hdc, colorHigh)
		draw(hdc, "扫描未能完成。"+a.statusText, rect(x+a.scale(16), y+a.scale(16), right-a.scale(16), y+a.scale(56)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		y += a.scale(84)
	}

	if !a.scanned && a.phase != phaseScan {
		y = a.paintEmptyState(hdc, x, y, right, "还没有扫描结果", "使用底部的“开始扫描”。结果会按推荐、可选和高级分组，并标出真实可释放空间。")
		win.SelectObject(hdc, old)
		return y + a.scale(16) - origin
	}

	if a.scanned {
		y = a.paintCleanSection(hdc, x, y, right, list, ui.SectionRecommend)
		y = a.paintCleanSection(hdc, x, y, right, list, ui.SectionOptional)
		y = a.paintCleanSection(hdc, x, y, right, list, ui.SectionAdvanced)
	}
	win.SelectObject(hdc, old)
	return y - origin + a.scale(24)
}

func (a *app) paintScanProgress(hdc win.HDC, x, y, right int) int {
	box := rect(x, y, right, y+a.scale(96))
	roundFill(hdc, box, ui.Radius, colorCard, colorLine)
	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, a.scanLine(), rect(x+a.scale(16), y+a.scale(12), right-a.scale(16), y+a.scale(44)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	a.mu.Lock()
	done, total, scanning := a.progDone, a.progTotal, a.scanning
	a.mu.Unlock()
	found := ui.Headline(scanning, true)
	win.SelectObject(hdc, win.HGDIOBJ(a.smallFont))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, fmt.Sprintf("已发现 %d 个候选文件。剩余时间无法准确计算，因此不显示。", found.Count), rect(x+a.scale(16), y+a.scale(48), right-a.scale(16), y+a.scale(68)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	ratio := 0.0
	if total > 0 {
		ratio = float64(done) / float64(total)
	}
	paintBar(hdc, rect(x+a.scale(16), y+a.scale(74), right-a.scale(16), y+a.scale(82)), ratio, colorPrimary, colorLine)
	return y + a.scale(112)
}

func (a *app) paintCleanResult(hdc win.HDC, x, y, right int) int {
	h := a.scale(148)
	if a.cleanOut.detail {
		h = a.scale(220)
	}
	box := rect(x, y, right, y+h)
	roundFill(hdc, box, ui.Radius, colorRecBg, colorSuccess)
	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	freed := clean.FormatBytes(a.cleanOut.estimated)
	if a.cleanOut.diskDelta != 0 {
		freed = clean.FormatBytes(a.cleanOut.diskDelta)
	}
	draw(hdc, "清理完成 · 实际可用空间变化 "+freed, rect(x+a.scale(16), y+a.scale(14), right-a.scale(140), y+a.scale(40)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, fmt.Sprintf("删除 %d · 跳过 %d · 失败 %d · 逻辑大小 %s", a.cleanOut.deleted, a.cleanOut.skipped, a.cleanOut.failed, clean.FormatBytes(a.cleanOut.logical)), rect(x+a.scale(16), y+a.scale(44), right-a.scale(16), y+a.scale(68)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	note := clean.SpaceNote()
	if a.cleanOut.restart {
		note = "建议稍后重启，以便完成系统组件相关清理。"
	}
	draw(hdc, note, rect(x+a.scale(16), y+a.scale(72), right-a.scale(120), y+a.scale(112)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	det := rect(right-a.scale(128), y+a.scale(12), right-a.scale(12), y+a.scale(44))
	label := "详细记录"
	if a.cleanOut.detail {
		label = "收起"
	}
	a.placeAction(det, label, ui.ResultToggleName(a.cleanOut.detail), hitExpand, 0, "clean-result", actSecondary)
	if a.cleanOut.detail {
		draw(hdc, strings.Join(a.cleanOut.notes, " · "), rect(x+a.scale(16), y+a.scale(116), right-a.scale(16), y+h-a.scale(12)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	}
	return y + h + a.scale(16)
}

func (a *app) paintCleanSection(hdc win.HDC, x, y, right int, list []clean.Report, section int) int {
	items := ui.ReportsInSection(list, section)
	if len(items) == 0 {
		return y
	}
	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	if section == ui.SectionAdvanced {
		win.SetTextColor(hdc, colorMid)
	}
	draw(hdc, ui.SectionTitle(section), rect(x, y, right-a.scale(120), y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	if section == ui.SectionRecommend {
		a.selRC = rect(right-a.scale(108), y, right, y+a.scale(28))
		a.selShow = true
		a.placeAction(a.selRC, a.selectLabel(), a.selectLabel(), hitSelect, 0, "", actSecondary)
	}
	if section == ui.SectionAdvanced {
		fold := rect(right-a.scale(108), y, right, y+a.scale(28))
		label := "展开"
		if a.advancedOpen {
			label = "折叠"
		}
		a.placeAction(fold, label, ui.AdvancedToggleName(a.advancedOpen), hitAdvanced, 0, "", actSecondary)
	}
	y += a.scale(32)
	win.SelectObject(hdc, win.HGDIOBJ(a.smallFont))
	win.SetTextColor(hdc, colorMuted)
	hintH := a.typeH(ui.TypeCaption)
	if section == ui.SectionAdvanced {
		hintH *= 2
		win.SetTextColor(hdc, colorMid)
	}
	draw(hdc, ui.SectionHint(section), rect(x, y, right, y+hintH), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += hintH + a.scale(8)
	if section == ui.SectionAdvanced && !a.advancedOpen {
		return y + a.scale(8)
	}
	heights := make([]int, len(items))
	totalH := 0
	for i, rep := range items {
		heights[i] = a.scale(ui.CleanCardH(section == ui.SectionAdvanced, a.expanded[rep.Category.ID]))
		totalH += heights[i]
	}
	bg := colorCard
	if section == ui.SectionRecommend {
		bg = colorRecBg
	}
	if section == ui.SectionAdvanced {
		bg = colorAdvBg
	}
	roundFill(hdc, rect(x, y, right, y+totalH), ui.Radius, bg, colorLine)
	for i, rep := range items {
		if i > 0 {
			hline(hdc, x+a.scale(16), right-a.scale(16), y, colorLine)
		}
		idx := ui.IndexInReports(list, rep.Category.ID)
		advanced := section == ui.SectionAdvanced
		expanded := a.expanded[rep.Category.ID]
		pad := a.scale(8)
		nameH := a.scale(22)
		h := heights[i]
		nameTop := y + pad
		check := rect(x+pad, nameTop, right-a.scale(148), nameTop+nameH)
		a.placeCheck(idx, check, a.choices[rep.Category.ID], true)
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, ui.SizeText(rep), rect(right-a.scale(136), nameTop, right-pad, nameTop+nameH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		descTop := nameTop + nameH + a.scale(4)
		descH := a.scale(22)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		desc := rep.Category.Description
		if advanced {
			desc = ui.RiskReason(rep.Category)
			win.SetTextColor(hdc, colorMid)
		}
		draw(hdc, desc, rect(x+a.scale(40), descTop, right-a.scale(204), descTop+descH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		fg, bgb := colorSuccess, colorCard
		if rep.Category.Risk == 2 || advanced {
			fg, bgb = colorMid, colorCard
		}
		if rep.Category.Risk >= 3 {
			fg, bgb = colorHigh, colorCard
		}
		badgeH := a.scale(20)
		badgeTop := descTop + (descH-badgeH)/2
		paintBadge(hdc, rect(right-a.scale(196), badgeTop, right-a.scale(124), badgeTop+badgeH), ui.RiskText(rep.Category.Risk), fg, bgb)
		lab := "详情"
		if expanded {
			lab = "收起"
		}
		a.placeDetail(idx, rect(right-a.scale(116), descTop, right-pad, descTop+descH), lab, true)
		noteY := descTop + descH + a.scale(4)
		if advanced {
			a.useType(hdc, ui.TypeCaption)
			win.SetTextColor(hdc, colorMid)
			draw(hdc, ui.AdvancedItemNote(ui.Recoverable(rep.Category)), rect(x+a.scale(40), noteY, right-pad, noteY+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
			noteY += a.typeH(ui.TypeCaption) + a.scale(4)
		}
		if expanded {
			a.useType(hdc, ui.TypeCaption)
			win.SetTextColor(hdc, colorCaption)
			draw(hdc, cardPreviewMeta(rep), rect(x+a.scale(40), noteY, right-pad, noteY+a.typeH(ui.TypeCaption)*2), win.DT_LEFT|dtWordBreak|dtNoPrefix)
			pathY := noteY + a.typeH(ui.TypeCaption)*2 + a.scale(4)
			a.useType(hdc, ui.TypePath)
			for _, line := range cardPreviewPaths(rep) {
				draw(hdc, line, rect(x+a.scale(40), pathY, right-pad, pathY+a.typeH(ui.TypePath)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
				pathY += a.typeH(ui.TypePath)
			}
		}
		y += h
	}
	return y + a.scale(16)
}

func (a *app) paintHistory(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "清理历史", "来自本机日志摘要。不会改写历史内容，旧版本号会保留当时的记录。")
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	if len(a.history) == 0 {
		y = a.paintEmptyState(hdc, x, y, right, "还没有清理记录", "完成一次清理后，这里读取本机日志。历史正文不会被改写。")
		win.SelectObject(hdc, old)
		return y + a.scale(16) - origin
	}
	titleH := a.typeH(ui.TypeItem)
	lineH := a.typeH(ui.TypeCaption)
	rowH := a.scale(12) + titleH + a.scale(4) + lineH*2 + a.scale(12)
	roundFill(hdc, rect(x, y, right, y+rowH*len(a.history)), ui.Radius, colorCard, colorLine)
	for i, rec := range a.history {
		if i > 0 {
			hline(hdc, x+a.scale(16), right-a.scale(16), y, colorLine)
		}
		cardH := rowH
		when := history.FormatWhen(rec.When)
		btnW := a.scale(88)
		textRight := right - a.scale(16) - btnW - a.scale(12)
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		titleTop := y + a.scale(12)
		dateRight := textRight
		if rec.Version != "" {
			dateRight = x + a.scale(16) + (textRight-x-a.scale(16))*3/5
		}
		draw(hdc, when, rect(x+a.scale(16), titleTop, dateRight, titleTop+titleH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		if rec.Version != "" {
			a.useType(hdc, ui.TypeCaption)
			win.SetTextColor(hdc, colorCaption)
			draw(hdc, rec.Version, rect(dateRight+a.scale(8), titleTop, textRight, titleTop+titleH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
			a.useType(hdc, ui.TypeItem)
		}
		win.SetTextColor(hdc, colorMuted)
		a.useType(hdc, ui.TypeCaption)
		line := fmt.Sprintf("删除 %d · 跳过 %d · 失败 %d", rec.Deleted, rec.Skipped, rec.Failed)
		if rec.Estimated != "" {
			line += " · 预计占用 " + rec.Estimated
		}
		lineTop := titleTop + titleH + a.scale(4)
		draw(hdc, line, rect(x+a.scale(16), lineTop, textRight, lineTop+lineH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		draw(hdc, strings.Join(rec.Items, "、"), rect(x+a.scale(16), lineTop+lineH, textRight, lineTop+lineH*2), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		open := rect(right-a.scale(12)-btnW, y+(cardH-a.scale(32))/2, right-a.scale(12), y+(cardH-a.scale(32))/2+a.scale(32))
		a.placeAction(open, "打开", ui.HistoryOpenName(when), hitHistory, i, rec.File, actSecondary)
		y += cardH
	}
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
}

func (a *app) paintSettings(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "设置", "只保存配置。此页不会扫描或删除文件。")

	y = a.paintSectionLabel(hdc, x, y, right, "外观")
	themeCard := rect(x, y, right, y+a.scale(68))
	roundFill(hdc, themeCard, ui.Radius, colorCard, colorLine)
	for i, name := range []string{"浅色", "深色"} {
		id := "light"
		if i == 1 {
			id = "dark"
		}
		box := rect(x+a.scale(16)+i*a.scale(132), y+a.scale(16), x+a.scale(16)+i*a.scale(132)+a.scale(120), y+a.scale(52))
		on := (a.dark && id == "dark") || (!a.dark && id == "light")
		style := actSecondary
		if on {
			style = actSelected
		}
		a.placeAction(box, name, ui.ThemeAccessName(id == "dark", on), hitTheme, 0, id, style)
	}
	y += a.scale(80)
	y = a.paintSectionLabel(hdc, x, y, right, "规则")
	y = a.paintSettingGroup(hdc, x, y, right, colorCard, []settingRow{
		{"扫描规则", "普通临时文件至少保留 72 小时。文件年龄不会让其他目录进入清理范围。高风险项不能设为默认开启。"},
		{"排除目录", "文档、桌面、下载、照片、Program Files、System32 和 WinSxS 始终排除。本版本不能添加会扩大清理范围的自定义路径。"},
		{"自动检查更新", "本版本不会自动检查更新，因此没有可打开的开关。"},
	})

	y = a.paintSectionLabel(hdc, x, y, right, "日志")
	logTop := y
	logH := a.scale(16) + a.typeH(ui.TypeBody)*2 + a.scale(12) + a.scale(36) + a.scale(12) + a.scale(36) + a.scale(16)
	roundFill(hdc, rect(x, y, right, y+logH), ui.Radius, colorCard, colorLine)
	a.useType(hdc, ui.TypeBody)
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, ui.LogKeepHint(), rect(x+a.scale(16), y+a.scale(14), right-a.scale(16), y+a.scale(14)+a.typeH(ui.TypeBody)*2), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	choiceY := y + a.scale(14) + a.typeH(ui.TypeBody)*2 + a.scale(10)
	logX := x + a.scale(16)
	for _, mb := range []int{1, 2, 5} {
		box := rect(logX, choiceY, logX+a.scale(88), choiceY+a.scale(32))
		logX += a.scale(96)
		on := a.logKeepMB == mb
		style := actSecondary
		if on {
			style = actSelected
		}
		a.placeAction(box, fmt.Sprintf("%d MB", mb), ui.LogKeepAccessName(mb, on), hitLogKeep, mb, "", style)
	}
	btnY := choiceY + a.scale(44)
	hline(hdc, x+a.scale(16), right-a.scale(16), btnY-a.scale(8), colorLine)
	a.placeAction(rect(x+a.scale(16), btnY, x+a.scale(156), btnY+a.scale(32)), "打开日志目录", "打开日志目录", hitOpenLog, 0, "", actSecondary)
	a.placeAction(rect(x+a.scale(168), btnY, x+a.scale(308), btnY+a.scale(32)), "清理过期日志", "清理过期日志", hitPrune, 0, "", actSecondary)
	y = logTop + logH + a.scale(12)
	y = a.paintSectionLabel(hdc, x, y, right, "默认扫描项目")
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "低风险项可记住勾选。高级清理项每次启动都会关闭，全选也不会打开它们。", rect(x, y, right, y+a.scale(36)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(44)
	type prefRow struct {
		index int
		h     int
	}
	var plain, risky []prefRow
	for i, cat := range a.settingCats {
		row := prefRow{index: i, h: a.scale(44)}
		if ui.CleanSection(cat) == ui.SectionAdvanced {
			row.h = a.scale(44) + a.typeH(ui.TypeCaption) + a.scale(6)
			risky = append(risky, row)
		} else {
			plain = append(plain, row)
		}
	}
	paintPrefs := func(rows []prefRow, fill win.COLORREF, warn bool) {
		if len(rows) == 0 {
			return
		}
		total := 0
		for _, row := range rows {
			total += row.h
		}
		roundFill(hdc, rect(x, y, right, y+total), ui.Radius, fill, colorLine)
		for n, row := range rows {
			if n > 0 {
				hline(hdc, x+a.scale(16), right-a.scale(16), y, colorLine)
			}
			a.placeCheck(row.index, rect(x+a.scale(12), y+a.scale(6), right-a.scale(12), y+a.scale(38)), a.choices[a.settingCats[row.index].ID], true)
			if warn {
				win.SetTextColor(hdc, colorMid)
				a.useType(hdc, ui.TypeCaption)
				draw(hdc, ui.AdvancedWarning(), rect(x+a.scale(40), y+a.scale(40), right-a.scale(16), y+a.scale(40)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
			}
			y += row.h
		}
		y += a.scale(12)
	}
	paintPrefs(plain, colorCard, false)
	paintPrefs(risky, colorAdvBg, true)
	y = a.paintSectionLabel(hdc, x, y, right, "关于")
	diag := ui.FormatVersion(appinfo.Version) + "。"
	if !appinfo.WindowsNativeVerified {
		diag += "本机尚未完成 NTFS 联接、占用句柄和权限的删除验证，因此不能把当前结果当作生产安全验证。程序默认没有 Authenticode 签名，也不会伪造签名或绕过 SmartScreen。"
	}
	y = a.paintSettingGroup(hdc, x, y, right, colorCard, []settingRow{
		{"本机数据位置", "日志：" + clean.RedactPath(logDir()) + "。快照：" + clean.RedactPath(filepath.Join(prefBase(), "CClear", "snapshots")) + "。最多保留 8 份快照；目录名会脱敏。"},
		{"版本与验证", diag},
	})
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
}

type settingRow struct {
	title, body string
}

func (a *app) paintSettingGroup(hdc win.HDC, x, y, right int, fill win.COLORREF, rows []settingRow) int {
	if len(rows) == 0 {
		return y
	}
	th := a.typeH(ui.TypeSection)
	bh := a.typeH(ui.TypeBody) * 2
	rowH := a.scale(14) + th + a.scale(6) + bh + a.scale(10)
	roundFill(hdc, rect(x, y, right, y+rowH*len(rows)), ui.Radius, fill, colorLine)
	for i, row := range rows {
		ry := y + i*rowH
		if i > 0 {
			hline(hdc, x+a.scale(16), right-a.scale(16), ry, colorLine)
		}
		a.useType(hdc, ui.TypeSection)
		win.SetTextColor(hdc, colorText)
		draw(hdc, row.title, rect(x+a.scale(16), ry+a.scale(12), right-a.scale(16), ry+a.scale(12)+th), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeBody)
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, row.body, rect(x+a.scale(16), ry+a.scale(12)+th+a.scale(4), right-a.scale(16), ry+rowH-a.scale(8)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	}
	return y + rowH*len(rows) + a.scale(12)
}

func riskColor(risk int) win.COLORREF {
	switch risk {
	case 3:
		return colorHigh
	case 2:
		return colorMid
	default:
		return colorLow
	}
}

func dashBytes(n int64) string {
	if n <= 0 {
		return "—"
	}
	return clean.FormatBytes(n)
}

func cardPreviewMeta(rep clean.Report) string {
	s := fmt.Sprintf("候选 %d 个文件，逻辑大小 %s，预计可释放 %s。", rep.Count, clean.FormatBytes(rep.Logical), ui.SizeText(rep))
	if rep.Category.Warning != "" {
		s += " " + rep.Category.Warning
	}
	return s
}

func cardPreviewPaths(rep clean.Report) []string {
	limit := 3
	if len(rep.Files) < limit {
		limit = len(rep.Files)
	}
	var out []string
	for i := 0; i < limit; i++ {
		out = append(out, clean.FormatBytes(rep.Files[i].Size)+"  "+clean.RedactPath(rep.Files[i].Path))
	}
	if rep.Count > limit {
		out = append(out, "其余文件可在详情中分页查看。")
	}
	return out
}
