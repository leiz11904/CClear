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
)

type hit struct {
	kind  int
	index int
	id    string
	rc    win.RECT
}

func (a *app) paintAll(hdc win.HDC, client win.RECT) {
	a.hits = a.hits[:0]
	a.checkLaid = a.checkLaid[:0]
	a.detailLaid = a.detailLaid[:0]
	a.selShow = false
	fill(hdc, client, a.pageBrush)
	navW := int32(a.scale(ui.NavWidth))
	nav := client
	nav.Right = navW
	roundFill(hdc, nav, 0, colorNav, colorNav)
	hline(hdc, int(navW), int(client.Right), int(client.Bottom)-a.scale(ui.FooterH), colorLine)
	a.paintNav(hdc, nav)
	content := client
	content.Left = navW
	content.Bottom -= int32(a.scale(ui.FooterH))
	a.paintPage(hdc, content)
	foot := client
	foot.Left = navW
	foot.Top = client.Bottom - int32(a.scale(ui.FooterH))
	roundFill(hdc, foot, 0, colorFooter, colorFooter)
	hline(hdc, int(foot.Left), int(foot.Right), int(foot.Top), colorLine)
	a.paintFooter(hdc, foot)
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
	draw(hdc, title, rect(x, y, right, y+titleH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	y += titleH + a.scale(8)
	a.useType(hdc, ui.TypePageHint)
	win.SetTextColor(hdc, colorMuted)
	hintH := a.typeH(ui.TypePageHint) * 2
	draw(hdc, hint, rect(x, y, right, y+hintH), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	return y + hintH + a.scale(12)
}

func (a *app) paintNav(hdc win.HDC, rc win.RECT) {
	win.SetBkMode(hdc, win.TRANSPARENT)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.fontOf(ui.TypeSection)))
	win.SetTextColor(hdc, colorText)
	draw(hdc, appinfo.Name, rect(int(rc.Left)+a.scale(16), int(rc.Top)+a.scale(12), int(rc.Right)-a.scale(12), int(rc.Top)+a.scale(34)), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	if !appinfo.WindowsNativeVerified {
		paintBadge(hdc, rect(int(rc.Right)-a.scale(60), int(rc.Top)+a.scale(14), int(rc.Right)-a.scale(12), int(rc.Top)+a.scale(32)), "测试版", colorPrimary, colorPrimarySoft)
	}
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorCaption)
	draw(hdc, "C 盘清理", rect(int(rc.Left)+a.scale(16), int(rc.Top)+a.scale(34), int(rc.Right)-a.scale(12), int(rc.Top)+a.scale(34)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	for _, slot := range a.navSlots(rc) {
		if slot.group != "" {
			a.useType(hdc, ui.TypeCaption)
			win.SetTextColor(hdc, colorCaption)
			draw(hdc, slot.group, rect(int(rc.Left)+a.scale(16), slot.groupY, int(rc.Right)-a.scale(12), slot.groupY+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		}
		if a.hasNativeNav() {
			continue
		}
		if a.page == slot.item.Page {
			roundFill(hdc, slot.rc, 8, colorNavPick, colorNavPick)
			bar := slot.rc
			bar.Right = bar.Left + int32(a.scale(3))
			roundFill(hdc, bar, 2, colorPrimary, colorPrimary)
			win.SetTextColor(hdc, colorPrimary)
		} else {
			win.SetTextColor(hdc, colorText)
		}
		a.useType(hdc, ui.TypeNav)
		draw(hdc, slot.item.Title, rect(int(slot.rc.Left)+a.scale(36), int(slot.rc.Top), int(slot.rc.Right)-a.scale(8), int(slot.rc.Bottom)), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
		paintIcon(hdc, rect(int(slot.rc.Left)+a.scale(10), int(slot.rc.Top)+a.scale(8), int(slot.rc.Left)+a.scale(30), int(slot.rc.Bottom)-a.scale(8)), slot.item.Icon, colorText)
		a.hits = append(a.hits, hit{kind: hitNav, index: slot.item.Page, rc: slot.rc})
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
	if a.page == ui.PageClean && a.hasNativeFooter() {
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
	if a.hasNativeFooter() {
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
	if a.contentH < view {
		a.contentH = view
	}
	if a.scroll > a.contentH-view {
		a.scroll = a.contentH - view
	}
	if a.scroll < 0 {
		a.scroll = 0
	}
	a.paintScroll(hdc, rc, view)
}

func (a *app) paintScroll(hdc win.HDC, rc win.RECT, view int) {
	if a.contentH <= view {
		return
	}
	track := win.RECT{Left: rc.Right - int32(a.scale(8)), Top: rc.Top, Right: rc.Right - int32(a.scale(2)), Bottom: rc.Bottom}
	roundFill(hdc, track, 4, colorLine, colorLine)
	thumbH := view * view / a.contentH
	if thumbH < a.scale(32) {
		thumbH = a.scale(32)
	}
	max := a.contentH - view
	thumbY := int(rc.Top)
	if max > 0 {
		thumbY += a.scroll * (view - thumbH) / max
	}
	thumb := win.RECT{Left: track.Left, Top: int32(thumbY), Right: track.Right, Bottom: int32(thumbY + thumbH)}
	roundFill(hdc, thumb, 4, colorMuted, colorMuted)
	a.hits = append(a.hits, hit{kind: hitScroll, rc: track})
}

func (a *app) paintOverview(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "总览", "看清 C 盘占用，再决定清理哪些已核对的缓存。文档、桌面和下载不会进入清理列表。")

	used := uint64(0)
	ratio := 0.0
	if a.total > 0 {
		if a.free < a.total {
			used = a.total - a.free
		}
		ratio = float64(used) / float64(a.total)
	}
	card := rect(x, y, right, y+a.scale(148))
	roundFill(hdc, card, ui.Radius, colorCard, colorLine)
	a.useType(hdc, ui.TypeSection)
	win.SetTextColor(hdc, colorText)
	draw(hdc, ui.FormatDrive(a.drive), rect(x+a.scale(20), y+a.scale(16), right-a.scale(20), y+a.scale(16)+a.typeH(ui.TypeSection)), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	paintBar(hdc, rect(x+a.scale(20), y+a.scale(52), right-a.scale(20), y+a.scale(64)), ratio, colorPrimary, colorLine)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, fmt.Sprintf("已用 %s · 剩余 %s · 总共 %s", dashBytes(int64(used)), dashBytes(int64(a.free)), dashBytes(int64(a.total))), rect(x+a.scale(20), y+a.scale(76), right-a.scale(20), y+a.scale(100)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	if a.elevated {
		paintBadge(hdc, rect(right-a.scale(108), y+a.scale(16), right-a.scale(20), y+a.scale(40)), "已是管理员", colorSuccess, colorRecBg)
	}
	y += a.scale(164)

	head := ui.Headline(a.reports, a.scanned)
	metrics := []struct {
		label, value string
		metric       bool
	}{
		{"预计可释放", ui.ReclaimValue(head), true},
		{"待系统分析", ui.PendingAnalysis(head), false},
		{"最近清理", a.lastCleanLabel(), false},
		{"最大占用目录", a.topFolderLabel(), false},
	}
	w := (right - x - a.scale(24)) / 4
	for i, m := range metrics {
		mx := x + i*(w+a.scale(8))
		box := rect(mx, y, mx+w, y+a.scale(88))
		roundFill(hdc, box, ui.Radius, colorCard, colorLine)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		draw(hdc, m.label, rect(mx+a.scale(12), y+a.scale(10), mx+w-a.scale(10), y+a.scale(10)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		role := ui.TypeItem
		if m.metric {
			role = ui.TypeMetric
		}
		a.useType(hdc, role)
		win.SetTextColor(hdc, colorText)
		draw(hdc, m.value, rect(mx+a.scale(12), y+a.scale(34), mx+w-a.scale(10), y+a.scale(80)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	}
	y += a.scale(104)

	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "推荐操作", rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(32)
	actions := []struct {
		title, body string
		kind        int
	}{
		{"开始扫描", "找出可安全清理的缓存和临时文件。", hitScan},
		{"查看磁盘分析", "只读查看目录占用，不会删除文件。", hitAnalyze},
	}
	if !a.elevated {
		actions = append(actions, struct {
			title, body string
			kind        int
		}{"以管理员运行", "部分系统缓存需要管理员权限。", hitElevate})
	}
	for _, act := range actions {
		box := rect(x, y, right, y+a.scale(72))
		roundFill(hdc, box, ui.Radius, colorCard, colorLine)
		win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
		win.SetTextColor(hdc, colorText)
		draw(hdc, act.title, rect(x+a.scale(18), y+a.scale(12), right-a.scale(18), y+a.scale(36)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		win.SelectObject(hdc, win.HGDIOBJ(a.font))
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, act.body, rect(x+a.scale(18), y+a.scale(38), right-a.scale(18), y+a.scale(60)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		a.hits = append(a.hits, hit{kind: act.kind, rc: box})
		y += a.scale(80)
	}
	if a.phase == phaseScan {
		win.SetTextColor(hdc, colorPrimary)
		draw(hdc, a.scanLine(), rect(x, y, right, y+a.scale(40)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		y += a.scale(48)
	}
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
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

func (a *app) paintCards(hdc win.HDC, rc win.RECT, origin int, list []clean.Report) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "垃圾清理", "先扫描，再勾选，确认风险后才会删除。删除不进回收站。")

	head := ui.Headline(list, a.scanned)
	stat := rect(x, y, right, y+a.scale(92))
	roundFill(hdc, stat, ui.Radius, colorCard, colorLine)
	highN := ui.CountHighRisk(list)
	cells := []struct {
		k, v, extra string
		metric      bool
		warn        bool
	}{
		{"上次扫描", ui.FormatScanAt(a.scanAt), "", false, false},
		{"可清理空间", ui.ReclaimValue(head), "", true, false},
		{"待系统分析", ui.PendingAnalysis(head), "", false, false},
		{"高级项目", ui.HighRiskValue(highN), ui.HighRiskExplain(), true, highN > 0},
	}
	cw := (right - x) / 4
	for i, c := range cells {
		cx := x + i*cw
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		if c.warn {
			win.SetTextColor(hdc, colorMid)
		}
		draw(hdc, c.k, rect(cx+a.scale(14), y+a.scale(10), cx+cw-a.scale(8), y+a.scale(10)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		role := ui.TypeItem
		if c.metric && (i == 1 || (c.warn && highN > 0)) {
			role = ui.TypeMetric
		}
		if i == 0 {
			role = ui.TypeItem
		}
		a.useType(hdc, role)
		win.SetTextColor(hdc, colorText)
		if c.warn {
			win.SetTextColor(hdc, colorMid)
		}
		valueBottom := y + a.scale(58)
		if c.extra != "" {
			valueBottom = y + a.scale(52)
		}
		draw(hdc, c.v, rect(cx+a.scale(14), y+a.scale(32), cx+cw-a.scale(8), valueBottom), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		if c.extra != "" {
			a.useType(hdc, ui.TypeCaption)
			win.SetTextColor(hdc, colorMid)
			draw(hdc, c.extra, rect(cx+a.scale(14), y+a.scale(56), cx+cw-a.scale(8), y+a.scale(82)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		}
	}
	y += a.scale(108)

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
		box := rect(x, y, right, y+a.scale(120))
		roundFill(hdc, box, ui.Radius, colorCard, colorLine)
		win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
		win.SetTextColor(hdc, colorText)
		draw(hdc, "还没有扫描结果", rect(x+a.scale(24), y+a.scale(24), right-a.scale(24), y+a.scale(52)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		win.SelectObject(hdc, win.HGDIOBJ(a.font))
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "点击底部“开始扫描”，会列出真实的可释放空间。", rect(x+a.scale(24), y+a.scale(56), right-a.scale(24), y+a.scale(88)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		win.SelectObject(hdc, old)
		return y + a.scale(136) - origin
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
	draw(hdc, "清理完成 · 实际可用空间变化 "+freed, rect(x+a.scale(16), y+a.scale(14), right-a.scale(16), y+a.scale(40)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, fmt.Sprintf("删除 %d · 跳过 %d · 失败 %d · 逻辑大小 %s", a.cleanOut.deleted, a.cleanOut.skipped, a.cleanOut.failed, clean.FormatBytes(a.cleanOut.logical)), rect(x+a.scale(16), y+a.scale(44), right-a.scale(16), y+a.scale(68)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	note := clean.SpaceNote()
	if a.cleanOut.restart {
		note = "建议稍后重启，以便完成系统组件相关清理。"
	}
	draw(hdc, note, rect(x+a.scale(16), y+a.scale(72), right-a.scale(120), y+a.scale(112)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	det := rect(right-a.scale(112), y+a.scale(16), right-a.scale(16), y+a.scale(44))
	label := "详细记录"
	if a.cleanOut.detail {
		label = "收起"
	}
	paintGhostBtn(hdc, det, label, true)
	a.hits = append(a.hits, hit{kind: hitExpand, id: "clean-result", rc: det})
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
		a.selRC = rect(right-a.scale(88), y, right, y+a.scale(28))
		a.selShow = true
		if !a.hasNativeFooter() {
			paintGhostBtn(hdc, a.selRC, a.selectLabel(), !a.busy() && a.scanned)
			a.hits = append(a.hits, hit{kind: hitSelect, rc: a.selRC})
		}
	}
	if section == ui.SectionAdvanced {
		fold := rect(right-a.scale(88), y, right, y+a.scale(28))
		label := "展开"
		if a.advancedOpen {
			label = "折叠"
		}
		paintGhostBtn(hdc, fold, label, true)
		a.hits = append(a.hits, hit{kind: hitAdvanced, rc: fold})
	}
	y += a.scale(28)
	win.SelectObject(hdc, win.HGDIOBJ(a.smallFont))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, ui.SectionHint(section), rect(x, y, right, y+a.scale(20)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	y += a.scale(24)
	if section == ui.SectionAdvanced && !a.advancedOpen {
		return y + a.scale(8)
	}
	for _, rep := range items {
		idx := ui.IndexInReports(list, rep.Category.ID)
		h := a.scale(ui.CleanCardH(section == ui.SectionAdvanced, a.expanded[rep.Category.ID]))
		bg := colorCard
		if section == ui.SectionRecommend {
			bg = colorRecBg
		}
		if section == ui.SectionAdvanced {
			bg = colorAdvBg
		}
		card := rect(x, y, right, y+h)
		roundFill(hdc, card, ui.Radius, bg, colorLine)
		nameTop := y + a.scale(8)
		nameH := a.scale(22)
		check := rect(x+a.scale(10), nameTop, right-a.scale(168), nameTop+nameH)
		on := a.choices[rep.Category.ID]
		if a.hasNativeCheck(idx) {
			a.placeCheck(idx, check, on, true)
		} else {
			box := rect(x+a.scale(12), nameTop+a.scale(2), x+a.scale(30), nameTop+a.scale(20))
			paintCheck(hdc, box, on, 4)
			a.useType(hdc, ui.TypeItem)
			win.SetTextColor(hdc, colorText)
			draw(hdc, rep.Category.Name, rect(x+a.scale(36), nameTop, right-a.scale(168), nameTop+nameH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
			a.hits = append(a.hits, hit{kind: hitCheck, id: rep.Category.ID, index: idx, rc: check})
		}
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, ui.SizeText(rep), rect(right-a.scale(160), nameTop, right-a.scale(12), nameTop+nameH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
		descTop := nameTop + nameH + a.scale(ui.ItemNameToCaptionGap())
		descH := a.typeH(ui.TypeCaption)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		desc := rep.Category.Description
		if section == ui.SectionAdvanced {
			desc = ui.RiskReason(rep.Category)
			win.SetTextColor(hdc, colorMid)
		}
		draw(hdc, desc, rect(x+a.scale(36), descTop, right-a.scale(196), descTop+descH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		fg, bgb := colorSuccess, colorRecBg
		if rep.Category.Risk == 2 || section == ui.SectionAdvanced {
			fg, bgb = colorMid, colorAdvBg
		}
		if rep.Category.Risk >= 3 {
			fg, bgb = colorHigh, colorAdvBg
		}
		badge := rect(right-a.scale(188), descTop, right-a.scale(116), descTop+descH)
		paintBadge(hdc, badge, ui.RiskText(rep.Category.Risk), fg, bgb)
		lab := "详情"
		if a.expanded[rep.Category.ID] {
			lab = "收起"
		}
		det := rect(right-a.scale(108), descTop-a.scale(2), right-a.scale(12), descTop+descH+a.scale(2))
		a.placeDetail(idx, det, lab, true)
		noteY := descTop + descH + a.scale(4)
		if section == ui.SectionAdvanced {
			a.useType(hdc, ui.TypeCaption)
			win.SetTextColor(hdc, colorMid)
			draw(hdc, ui.Recoverable(rep.Category), rect(x+a.scale(36), noteY, right-a.scale(12), noteY+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
			noteY += a.typeH(ui.TypeCaption)
		}
		if a.expanded[rep.Category.ID] {
			a.useType(hdc, ui.TypeCaption)
			win.SetTextColor(hdc, colorCaption)
			draw(hdc, cardPreviewMeta(rep), rect(x+a.scale(36), noteY, right-a.scale(12), noteY+a.typeH(ui.TypeCaption)*2), win.DT_LEFT|dtWordBreak|dtNoPrefix)
			pathY := noteY + a.typeH(ui.TypeCaption)*2 + a.scale(4)
			a.useType(hdc, ui.TypePath)
			for _, line := range cardPreviewPaths(rep) {
				draw(hdc, line, rect(x+a.scale(36), pathY, right-a.scale(12), pathY+a.typeH(ui.TypePath)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
				pathY += a.typeH(ui.TypePath)
			}
		}
		y += h + a.scale(6)
	}
	return y + a.scale(8)
}

func (a *app) paintHistory(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "清理历史", "来自本机日志摘要。不会改写历史内容，旧版本号会保留当时的记录。")
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	if len(a.history) == 0 {
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "还没有清理记录。完成一次清理后，这里会读取本机日志。", rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		win.SelectObject(hdc, old)
		return y - origin + a.scale(48)
	}
	for i, rec := range a.history {
		card := rect(x, y, right, y+a.scale(92))
		roundFill(hdc, card, ui.Radius, colorCard, colorLine)
		win.SetTextColor(hdc, colorText)
		title := history.FormatWhen(rec.When)
		if rec.Version != "" {
			title += "  ·  " + rec.Version
		}
		draw(hdc, title, rect(x+a.scale(20), y+a.scale(14), right-a.scale(20), y+a.scale(38)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		win.SetTextColor(hdc, colorMuted)
		line := fmt.Sprintf("删除 %d · 跳过 %d · 失败 %d", rec.Deleted, rec.Skipped, rec.Failed)
		if rec.Estimated != "" {
			line += " · 预计占用 " + rec.Estimated
		}
		draw(hdc, line, rect(x+a.scale(20), y+a.scale(38), right-a.scale(20), y+a.scale(60)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		draw(hdc, strings.Join(rec.Items, "、"), rect(x+a.scale(20), y+a.scale(60), right-a.scale(120), y+a.scale(82)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		open := rect(right-a.scale(100), y+a.scale(28), right-a.scale(16), y+a.scale(60))
		paintGhostBtn(hdc, open, "打开", true)
		a.hits = append(a.hits, hit{kind: hitHistory, index: i, id: rec.File, rc: open})
		y += a.scale(104)
	}
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
}

func (a *app) paintSettings(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "设置", "只保存配置。此页不会扫描或删除文件。")

	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "外观主题", rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(32)
	for i, name := range []string{"浅色", "深色"} {
		id := "light"
		if i == 1 {
			id = "dark"
		}
		box := rect(x+i*a.scale(120), y, x+i*a.scale(120)+a.scale(108), y+a.scale(36))
		on := (a.dark && id == "dark") || (!a.dark && id == "light")
		bg := colorCard
		if on {
			bg = colorPrimarySoft
		}
		roundFill(hdc, box, ui.Radius, bg, colorLine)
		win.SelectObject(hdc, win.HGDIOBJ(a.font))
		win.SetTextColor(hdc, colorText)
		draw(hdc, name, box, win.DT_CENTER|dtSingleLine|dtVCenter|dtNoPrefix)
		a.hits = append(a.hits, hit{kind: hitTheme, id: id, rc: box})
	}
	y += a.scale(52)
	y = a.settingBlock(hdc, x, y, right, "扫描规则", "普通临时文件至少保留 72 小时。文件年龄不会让其他目录进入清理范围。高风险项不能设为默认开启。")
	y = a.settingBlock(hdc, x, y, right, "排除目录", "文档、桌面、下载、照片、Program Files、System32 和 WinSxS 始终排除。本版本不能添加会扩大清理范围的自定义路径。")
	y = a.settingBlock(hdc, x, y, right, "自动检查更新", "本版本不会自动检查更新，因此没有可打开的开关。")

	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "日志保留", rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(28)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "超过上限时删除最旧的日志。不会改写日志正文里的历史版本号。", rect(x, y, right, y+a.scale(36)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(40)
	for _, mb := range []int{1, 2, 5} {
		box := rect(x, y, x+a.scale(72), y+a.scale(32))
		x += a.scale(80)
		on := a.logKeepMB == mb
		bg := colorCard
		if on {
			bg = colorPrimarySoft
		}
		roundFill(hdc, box, ui.Radius, bg, colorLine)
		win.SetTextColor(hdc, colorText)
		draw(hdc, fmt.Sprintf("%d MB", mb), box, win.DT_CENTER|dtSingleLine|dtVCenter|dtNoPrefix)
		a.hits = append(a.hits, hit{kind: hitLogKeep, index: mb, rc: box})
	}
	x, _, right = a.layoutContent(rc, origin)
	y += a.scale(48)

	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "默认扫描项目", rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(28)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "低风险项可记住勾选。高级清理项每次启动都会关闭，全选也不会打开它们。", rect(x, y, right, y+a.scale(36)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(44)
	for i, cat := range a.settingCats {
		rowH := a.scale(48)
		if ui.CleanSection(cat) == ui.SectionAdvanced {
			rowH = a.scale(64)
		}
		row := rect(x, y, right, y+rowH)
		bg := colorCard
		if ui.CleanSection(cat) == ui.SectionAdvanced {
			bg = colorAdvBg
		}
		roundFill(hdc, row, ui.Radius, bg, colorLine)
		check := rect(x+a.scale(12), y+a.scale(10), right-a.scale(12), y+a.scale(34))
		on := a.choices[cat.ID]
		label := cat.Name + " · " + ui.RiskText(cat.Risk)
		if ui.CleanSection(cat) == ui.SectionAdvanced {
			label += " · 默认关闭"
		}
		if a.hasNativeCheck(i) {
			setText(a.checkBtns[i], label)
			a.placeCheck(i, check, on, true)
		} else {
			box := rect(x+a.scale(12), y+a.scale(14), x+a.scale(32), y+a.scale(34))
			paintCheck(hdc, box, on, 4)
			win.SetTextColor(hdc, colorText)
			draw(hdc, label, rect(x+a.scale(44), y+a.scale(8), right-a.scale(12), y+a.scale(32)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
			a.hits = append(a.hits, hit{kind: hitChoice, id: cat.ID, rc: check})
		}
		if ui.CleanSection(cat) == ui.SectionAdvanced {
			win.SetTextColor(hdc, colorMuted)
			draw(hdc, ui.Recoverable(cat), rect(x+a.scale(44), y+a.scale(32), right-a.scale(12), y+a.scale(56)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		}
		y += rowH + a.scale(8)
	}
	y += a.scale(8)
	logBtn := rect(x, y, x+a.scale(128), y+a.scale(36))
	paintGhostBtn(hdc, logBtn, "打开日志目录", true)
	a.hits = append(a.hits, hit{kind: hitOpenLog, rc: logBtn})
	prune := rect(x+a.scale(140), y, x+a.scale(268), y+a.scale(36))
	paintGhostBtn(hdc, prune, "清理过期日志", true)
	a.hits = append(a.hits, hit{kind: hitPrune, rc: prune})
	y += a.scale(56)

	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "关于与诊断", rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(32)
	y = a.settingBlock(hdc, x, y, right, "本机数据位置", "日志："+clean.RedactPath(logDir())+"。快照："+clean.RedactPath(filepath.Join(prefBase(), "CClear", "snapshots"))+"。最多保留 8 份快照；目录名会脱敏。")
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	diag := ui.FormatVersion(appinfo.Version) + "。"
	if !appinfo.WindowsNativeVerified {
		diag += "Windows 原生删除验证（NTFS 联接、占用句柄、权限）尚未在本机执行。这是测试版诊断信息，不能当作生产安全验证已经通过。可执行文件默认没有 Authenticode 签名；本程序不会伪造签名，也不会绕过 SmartScreen。"
	}
	draw(hdc, diag, rect(x, y, right, y+a.scale(72)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(80)
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
}

func (a *app) settingBlock(hdc win.HDC, x, y, right int, title, body string) int {
	a.useType(hdc, ui.TypeSection)
	win.SetTextColor(hdc, colorText)
	th := a.typeH(ui.TypeSection)
	draw(hdc, title, rect(x, y, right, y+th), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	a.useType(hdc, ui.TypeBody)
	win.SetTextColor(hdc, colorMuted)
	bh := a.typeH(ui.TypeBody) * 2
	draw(hdc, body, rect(x, y+th+a.scale(6), right, y+th+a.scale(6)+bh), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	return y + th + bh + a.scale(20)
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
