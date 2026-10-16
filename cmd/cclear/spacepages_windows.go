//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"cclear/internal/clean"
	"cclear/internal/snapshot"
	"cclear/internal/space"
	"cclear/internal/ui"
	"github.com/lxn/win"
)

func (a *app) paintAnalyze(hdc win.HDC, rc win.RECT, origin int) int {
	pad := a.scale(16)
	x := int(rc.Left) + pad
	right := int(rc.Right) - pad
	y := origin + a.scale(12)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	if !a.dark {
		fillColor(hdc, rc, rgb(245, 247, 249))
	}

	label := "开始分析"
	if a.spaceOK {
		label = "重新分析"
	}
	if a.phase == phaseAnalyze {
		label = "取消分析"
	}
	bw := a.btnW(label)
	a.useType(hdc, ui.TypePageTitle)
	win.SetTextColor(hdc, colorText)
	titleH := a.typeH(ui.TypePageTitle)
	draw(hdc, "磁盘分析", rect(x, y, right-bw-a.scale(16), y+titleH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	a.placeAction(rect(right-bw, y, right, y+a.scale(32)), label, label, hitAnalyze, 0, "", actPrimary)
	y += a.scale(36)
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorMuted)
	hint := "先看整盘还剩多少，再进入占用最多的目录。大文件不会被当成垃圾。"
	if a.phase == phaseAnalyze && a.statusText != "" {
		hint = a.statusText
		win.SetTextColor(hdc, colorPrimary)
	} else if a.spaceOK && a.space.Note != "" {
		hint = a.space.Note
	}
	draw(hdc, hint, rect(x, y, right, y+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	y += a.typeH(ui.TypeCaption) + a.scale(10)
	y = a.paintAnalyzeOverview(hdc, x, y, right)
	gap := a.scale(12)
	viewH := int(rc.Bottom - rc.Top)
	rankHead := a.scale(52)
	tableTitle := a.scale(34)
	tableHead := a.scale(26)
	rankRow := a.scale(32)
	tableRow := a.typeH(ui.TypeItem)
	if tableRow < a.scale(28) {
		tableRow = a.scale(28)
	}
	avail := origin + viewH - y
	overhead := rankHead + gap + tableTitle + tableHead + a.scale(8)
	rows := 4
	if avail > overhead {
		rows = (avail - overhead) / (rankRow + tableRow)
	}
	if rows < 4 {
		rows = 4
	}
	splitH := rankHead + rows*rankRow + a.scale(8)
	bottomH := tableTitle + tableHead + rows*tableRow + a.scale(8)
	if extra := avail - gap - splitH - bottomH; extra > 0 {
		// Padding stays shorter than a row, so neither pane gains a partial extra item.
		splitRoom := rankRow - a.scale(8) - 1
		tableRoom := tableRow - a.scale(8) - 1
		if splitRoom < 0 {
			splitRoom = 0
		}
		if tableRoom < 0 {
			tableRoom = 0
		}
		splitPad := extra / 2
		if splitPad > splitRoom {
			splitPad = splitRoom
		}
		tablePad := extra - splitPad
		if tablePad > tableRoom {
			tablePad = tableRoom
		}
		splitH += splitPad
		bottomH += tablePad
	}
	splitBottom := y + splitH
	if splitBottom < y+a.scale(160) {
		splitBottom = y + a.scale(160)
	}
	a.paintAnalyzeSplit(hdc, x, y, right, splitBottom)
	tableTop := splitBottom + gap
	a.paintAnalyzeTable(hdc, x, tableTop, right, tableTop+bottomH)
	win.SelectObject(hdc, old)
	pageBottom := tableTop + bottomH
	if pageBottom-origin < viewH {
		return viewH
	}
	return pageBottom - origin
}

func (a *app) paintAnalyzeSplit(hdc win.HDC, x, y, right, bottom int) {
	gutter := a.scale(8)
	treeRight := x + (right-x)*42/100
	if treeRight < x+a.scale(248) {
		treeRight = x + a.scale(248)
	}
	if treeRight > right-a.scale(200) {
		treeRight = right - a.scale(200)
	}
	mapLeft := treeRight + gutter
	a.paintSoftCard(hdc, rect(x, y, treeRight, bottom))
	a.paintSoftCard(hdc, rect(mapLeft, y, right, bottom))
	headH := a.scale(52)
	bodyTop := y + headH
	if !a.spaceOK || a.space.Tree == nil {
		a.treeCount, a.treeShown = 0, 0
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "分析完成后，这里按占用从大到小列出目录。", rect(x+a.scale(16), bodyTop+a.scale(12), treeRight-a.scale(16), bottom-a.scale(12)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		draw(hdc, "每个色块表示一个目录，面积越大，占用空间越多。", rect(mapLeft+a.scale(16), bodyTop+a.scale(12), right-a.scale(16), bottom-a.scale(12)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	} else {
		a.paintAnalyzeRank(hdc, x, bodyTop, treeRight, bottom)
	}
	a.useType(hdc, ui.TypeItem)
	win.SetTextColor(hdc, colorText)
	treeTitle := "占用排行"
	if n := a.currentAnalyzeNode(); n != nil && n.Name != "" {
		treeTitle = "占用排行 · " + n.Name
	}
	if a.treeCount > a.treeShown && a.treeShown > 0 {
		last := a.treeScroll + a.treeShown
		if last > a.treeCount {
			last = a.treeCount
		}
		treeTitle = fmt.Sprintf("%s  %d–%d", treeTitle, a.treeScroll+1, last)
	}
	roundFill(hdc, rect(x+a.scale(8), y+a.scale(6), treeRight-a.scale(8), y+headH-a.scale(4)), 0, colorCard, colorCard)
	draw(hdc, treeTitle, rect(x+a.scale(16), y+a.scale(6), treeRight-a.scale(88), y+a.scale(28)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	if parent := a.analyzeParent(); parent != nil {
		a.placeAction(rect(treeRight-a.scale(76), y+a.scale(8), treeRight-a.scale(12), y+a.scale(32)), "上一级", "返回上一级", hitMap, 0, parent.Path, actSecondary)
	}
	mapTitle := "空间分布"
	if a.spaceOK {
		if n := findNode(a.space.Tree, a.selected); n != nil && n.Name != "" {
			mapTitle = "空间分布 · " + n.Name
		}
	}
	draw(hdc, mapTitle, rect(mapLeft+a.scale(12), y+a.scale(2), right-a.scale(8), y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "每个色块表示一个目录，面积越大，占用空间越多。点击可查看明细。", rect(mapLeft+a.scale(16), y+a.scale(24), right-a.scale(12), y+headH-a.scale(4)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	if !a.spaceOK || a.space.Tree == nil {
		return
	}
	mapRC := rect(mapLeft+a.scale(8), bodyTop+a.scale(6), right-a.scale(8), bottom-a.scale(8))
	gutterColor := rgb(226, 231, 236)
	if a.dark {
		gutterColor = rgb(28, 32, 36)
	}
	fillColor(hdc, mapRC, gutterColor)
	parent := a.space.Tree
	if n := findNode(a.space.Tree, a.selected); n != nil {
		parent = n
	}
	parentTotal := int64(0)
	if parent != nil {
		parentTotal = parent.Allocated
	}
	nodes := treemapNodes(a.space.Tree, a.selected)
	cells := space.LayoutTreemap(nodes, float64(mapRC.Left), float64(mapRC.Top), float64(mapRC.Right-mapRC.Left), float64(mapRC.Bottom-mapRC.Top))
	largest := 0
	for i := range cells {
		if cells[i].Allocated > cells[largest].Allocated {
			largest = i
		}
	}
	for i, cell := range cells {
		cr := rect(int(cell.X), int(cell.Y), int(cell.X+cell.W), int(cell.Y+cell.H))
		if cr.Right-cr.Left > 8 && cr.Bottom-cr.Top > 8 {
			cr = insetRect(cr, 2)
		}
		if cr.Right-cr.Left < 2 || cr.Bottom-cr.Top < 2 {
			continue
		}
		tone := a.mapTone(sizeRank(cells, i), i == largest)
		node := findNode(a.space.Tree, cell.Path)
		tip := cell.Name
		files, kids := 0, 0
		if node != nil {
			if node.Name != "" {
				tip = node.Name
			}
			files = node.Files
			kids = len(node.Children)
			tip = fmt.Sprintf("%s\n%s\n占用 %s\n占比 %s\n文件 %d · 下一级 %d", tip, node.Path, clean.FormatBytes(node.Allocated), percent(node.Allocated, parentTotal), files, kids)
		}
		a.placeAction(cr, cell.Name, ui.MapAccessName(cell.Name), hitMap, i, cell.Path, faceMap)
		if n := len(a.actLaid); n > 0 {
			a.actLaid[n-1].bg = tone.fill
			a.actLaid[n-1].fg = tone.fg
			a.actLaid[n-1].edge = tone.edge
			a.actLaid[n-1].detail = percent(cell.Allocated, parentTotal)
			a.actLaid[n-1].tip = tip
		}
	}
}

type mapTone struct {
	fill, fg, edge win.COLORREF
}

func (a *app) mapTone(index int, largest bool) mapTone {
	if a.dark {
		if largest {
			return mapTone{fill: rgb(92, 168, 214), fg: rgb(16, 20, 24), edge: rgb(210, 230, 240)}
		}
		dark := []mapTone{
			{rgb(52, 70, 86), rgb(226, 232, 236), rgb(120, 150, 170)},
			{rgb(48, 70, 68), rgb(226, 232, 230), rgb(110, 150, 146)},
			{rgb(58, 56, 74), rgb(230, 228, 236), rgb(140, 136, 160)},
			{rgb(70, 64, 56), rgb(236, 230, 222), rgb(160, 146, 128)},
			{rgb(56, 62, 68), rgb(226, 230, 232), rgb(130, 140, 148)},
		}
		return dark[index%len(dark)]
	}
	if largest {
		return mapTone{fill: rgb(176, 204, 224), fg: colorText, edge: rgb(92, 132, 164)}
	}
	light := []mapTone{
		{rgb(214, 224, 228), colorText, rgb(170, 186, 194)},
		{rgb(214, 226, 222), colorText, rgb(164, 186, 180)},
		{rgb(224, 220, 228), colorText, rgb(180, 172, 188)},
		{rgb(228, 222, 214), colorText, rgb(188, 176, 164)},
		{rgb(220, 224, 226), colorText, rgb(176, 182, 186)},
	}
	return light[index%len(light)]
}

func (a *app) paintAnalyzeTree(hdc win.HDC, x, y, right, bottom int, root *space.Node, total int64) {
	rows := flattenTree(root, 0, a.treeOpen, nil)
	rowH := a.typeH(ui.TypeItem)
	if rowH < a.scale(28) {
		rowH = a.scale(28)
	}
	shown := (bottom - y) / rowH
	if shown < 1 {
		shown = 1
	}
	a.treeCount = len(rows)
	a.treeShown = shown
	if a.treeScroll > len(rows)-shown {
		a.treeScroll = len(rows) - shown
	}
	if a.treeScroll < 0 {
		a.treeScroll = 0
	}
	end := a.treeScroll + shown
	if end > len(rows) {
		end = len(rows)
	}
	for i, row := range rows[a.treeScroll:end] {
		ry := y + i*rowH
		open := a.treeOpen[row.node.Path]
		label := row.node.Name
		if label == "" {
			label = row.node.Path
		}
		if len(row.node.Children) > 0 {
			if open {
				label = "▾  " + label
			} else {
				label = "▸  " + label
			}
		}
		style := faceRow
		if a.selected == row.node.Path {
			style = faceRowPick
		}
		depth := row.depth
		if depth > 12 {
			depth = 12
		}
		prefix := strings.Repeat("  ", depth)
		caption := prefix + label + "\x1f" + clean.FormatBytes(row.node.Allocated) + "  " + percent(row.node.Allocated, total)
		a.placeAction(rect(x+a.scale(4), ry, right-a.scale(4), ry+rowH), caption, ui.TreeAccessName(row.node.Name, len(row.node.Children) > 0, open), hitTree, 0, row.node.Path, style)
	}
}

func (a *app) paintAnalyzeOverview(hdc win.HDC, x, y, right int) int {
	h := a.scale(108)
	card := rect(x, y, right, y+h)
	a.paintSoftCard(hdc, card)
	used := uint64(0)
	if a.total > a.free {
		used = a.total - a.free
	}
	ratio := 0.0
	if a.total > 0 {
		ratio = float64(used) / float64(a.total)
	}
	donut := a.scale(64)
	drc := rect(x+a.scale(16), y+(h-donut)/2, x+a.scale(16)+donut, y+(h-donut)/2+donut)
	hole := colorCard
	paintDonut(hdc, drc, ratio, rgb(226, 232, 236), rgb(91, 141, 184), hole)
	tx := int(drc.Right) + a.scale(16)
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorCaption)
	draw(hdc, "已用空间", rect(tx, y+a.scale(16), tx+a.scale(240), y+a.scale(16)+a.typeH(ui.TypeCaption)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	a.useType(hdc, ui.TypeMetric)
	win.SetTextColor(hdc, colorText)
	draw(hdc, dashBytes(int64(used)), rect(tx, y+a.scale(34), tx+a.scale(240), y+a.scale(62)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	barLeft := tx
	barRight := right - a.scale(20)
	paintBar(hdc, rect(barLeft, y+a.scale(68), barRight, y+a.scale(74)), ratio, rgb(91, 141, 184), rgb(226, 232, 236))
	node := a.currentAnalyzeNode()
	cur := "—"
	if node != nil {
		name := node.Name
		if name == "" {
			name = node.Path
		}
		cur = name + "  " + clean.FormatBytes(node.Allocated)
	}
	files := "—"
	if a.spaceOK {
		files = fmt.Sprintf("%d", a.space.Files)
	}
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorMuted)
	meta := fmt.Sprintf("%s   ·   剩余 %s   ·   文件 %s   ·   当前目录 %s", ui.FormatDrive(a.drive), dashBytes(int64(a.free)), files, cur)
	draw(hdc, meta, rect(tx, y+a.scale(80), barRight, y+h-a.scale(8)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	return y + h + a.scale(12)
}

func (a *app) paintSoftCard(hdc win.HDC, rc win.RECT) {
	if !a.dark {
		shadow := rc
		shadow.Left += 1
		shadow.Top += 2
		shadow.Right += 1
		shadow.Bottom += 3
		roundFill(hdc, shadow, 12, rgb(228, 232, 236), rgb(228, 232, 236))
	}
	border := rgb(232, 236, 240)
	if a.dark {
		border = colorLine
	}
	roundFill(hdc, rc, 12, colorCard, border)
}

func (a *app) paintAnalyzeRank(hdc win.HDC, x, y, right, bottom int) {
	node := a.currentAnalyzeNode()
	list := a.childDirs(node)
	total := int64(0)
	if node != nil {
		total = node.Allocated
	}
	rowH := a.scale(32)
	if rowH < a.typeH(ui.TypeItem)+a.scale(8) {
		rowH = a.typeH(ui.TypeItem) + a.scale(8)
	}
	shown := (bottom - y) / rowH
	if shown < 1 {
		shown = 1
	}
	a.treeCount = len(list)
	a.treeShown = shown
	if a.treeScroll > len(list)-shown {
		a.treeScroll = len(list) - shown
	}
	if a.treeScroll < 0 {
		a.treeScroll = 0
	}
	if len(list) == 0 {
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "这个目录没有下一级。", rect(x+a.scale(16), y+a.scale(8), right-a.scale(12), bottom-a.scale(8)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		return
	}
	end := a.treeScroll + shown
	if end > len(list) {
		end = len(list)
	}
	for i, dir := range list[a.treeScroll:end] {
		ry := y + i*rowH
		ratio := 0.0
		if total > 0 && dir.Allocated > 0 {
			ratio = float64(dir.Allocated) / float64(total)
		}
		edge := rgb(186, 204, 216)
		if a.treeScroll+i == 0 {
			edge = rgb(91, 141, 184)
		}
		tip := fmt.Sprintf("%s\n%s\n占用 %s\n占比 %s\n文件 %d", dir.Name, dir.Path, clean.FormatBytes(dir.Allocated), percent(dir.Allocated, total), dir.Files)
		a.placeAction(rect(x+a.scale(8), ry, right-a.scale(8), ry+rowH-a.scale(4)), dir.Name, ui.MapAccessName(dir.Name), hitMap, 0, dir.Path, faceRank)
		if n := len(a.actLaid); n > 0 {
			a.actLaid[n-1].detail = clean.FormatBytes(dir.Allocated) + "\x1f" + fmt.Sprintf("%.4f", ratio)
			a.actLaid[n-1].edge = edge
			a.actLaid[n-1].tip = tip
		}
	}
}

func (a *app) paintAnalyzeTable(hdc win.HDC, x, y, right, bottom int) {
	a.paintSoftCard(hdc, rect(x, y, right, bottom))
	title := "目录内容"
	if n := a.currentAnalyzeNode(); n != nil {
		name := n.Name
		if name == "" {
			name = n.Path
		}
		if name != "" {
			title = name + " 的下一级"
		}
	}
	a.useType(hdc, ui.TypeItem)
	win.SetTextColor(hdc, colorText)
	draw(hdc, title, rect(x+a.scale(16), y+a.scale(8), right-a.scale(16), y+a.scale(30)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	y += a.scale(34)
	if !a.spaceOK {
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "分析完成后，这里列出当前目录里占用最多的下一级。", rect(x+a.scale(16), y, right-a.scale(16), bottom-a.scale(10)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		return
	}
	node := a.currentAnalyzeNode()
	list := a.childDirs(node)
	total := a.space.Allocated
	if node != nil && node.Allocated > 0 {
		total = node.Allocated
	}
	headH := a.scale(26)
	rowH := a.typeH(ui.TypeItem)
	if rowH < a.scale(28) {
		rowH = a.scale(28)
	}
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorCaption)
	draw(hdc, "名称", rect(x+a.scale(16), y, right-a.scale(280), y+headH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "占用", rect(right-a.scale(268), y, right-a.scale(168), y+headH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "占比", rect(right-a.scale(156), y, right-a.scale(96), y+headH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "操作", rect(right-a.scale(88), y, right-a.scale(16), y+headH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	if len(list) == 0 {
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "这个目录没有下一级。", rect(x+a.scale(16), y+headH, right-a.scale(16), bottom-a.scale(8)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		return
	}
	rows := (bottom - y - headH) / rowH
	if rows < 1 {
		rows = 1
	}
	if a.treeShown > 0 && rows > a.treeShown {
		rows = a.treeShown
	}
	start := a.treeScroll
	if start < 0 || start > len(list) {
		start = 0
	}
	list = list[start:]
	if len(list) > rows {
		list = list[:rows]
	}
	for i, dir := range list {
		ry := y + headH + i*rowH
		if i > 0 {
			hline(hdc, x+a.scale(16), right-a.scale(16), ry, rgb(240, 243, 246))
		}
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, dir.Name, rect(x+a.scale(16), ry, right-a.scale(280), ry+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		draw(hdc, clean.FormatBytes(dir.Allocated), rect(right-a.scale(268), ry, right-a.scale(168), ry+rowH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
		win.SetTextColor(hdc, colorText)
		draw(hdc, percent(dir.Allocated, total), rect(right-a.scale(156), ry, right-a.scale(100), ry+rowH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
		a.placeAction(rect(right-a.scale(84), ry+a.scale(2), right-a.scale(12), ry+rowH-a.scale(2)), "打开", ui.FileActionName("reveal", dir.Name), hitReveal, 0, dir.Path, actSecondary)
	}
}

func (a *app) paintTypeTable(hdc win.HDC, x, y, right, headH, rowH, rows int) {
	draw(hdc, "类型 · 全盘", rect(x+a.scale(12), y, x+a.scale(160), y+headH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "数量", rect(x+a.scale(168), y, right-a.scale(180), y+headH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "占用", rect(right-a.scale(168), y, right-a.scale(72), y+headH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "占比", rect(right-a.scale(64), y, right-a.scale(12), y+headH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	list := a.space.Types
	if len(list) > rows {
		list = list[:rows]
	}
	for i, st := range list {
		ry := y + headH + i*rowH
		if i > 0 {
			hline(hdc, x+a.scale(12), right-a.scale(12), ry, colorLine)
		}
		ext := st.Ext
		if ext == "" {
			ext = "无扩展名"
		}
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, ext, rect(x+a.scale(12), ry, x+a.scale(160), ry+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		draw(hdc, fmt.Sprintf("%d", st.Files), rect(x+a.scale(168), ry, right-a.scale(180), ry+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
		win.SetTextColor(hdc, colorText)
		draw(hdc, clean.FormatBytes(st.Allocated), rect(right-a.scale(168), ry, right-a.scale(72), ry+rowH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
		draw(hdc, percent(st.Allocated, a.space.Allocated), rect(right-a.scale(64), ry, right-a.scale(12), ry+rowH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	}
}

func (a *app) paintDirTable(hdc win.HDC, x, y, right, headH, rowH, rows int) {
	node := a.currentAnalyzeNode()
	list := a.childDirs(node)
	total := a.space.Allocated
	title := "目录"
	if node != nil && len(node.Children) > 0 {
		title = "目录 · 下一级"
		total = node.Allocated
	}
	draw(hdc, title, rect(x+a.scale(12), y, right-a.scale(250), y+headH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "占用", rect(right-a.scale(240), y, right-a.scale(150), y+headH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "占比", rect(right-a.scale(142), y, right-a.scale(88), y+headH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "操作", rect(right-a.scale(80), y, right-a.scale(12), y+headH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	if len(list) == 0 {
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "这个目录没有子目录。", rect(x+a.scale(12), y+headH, right-a.scale(12), y+headH+rowH*2), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		return
	}
	if len(list) > rows {
		list = list[:rows]
	}
	for i, dir := range list {
		ry := y + headH + i*rowH
		if i > 0 {
			hline(hdc, x+a.scale(12), right-a.scale(12), ry, colorLine)
		}
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, dir.Name, rect(x+a.scale(12), ry, right-a.scale(250), ry+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		draw(hdc, clean.FormatBytes(dir.Allocated), rect(right-a.scale(240), ry, right-a.scale(150), ry+rowH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
		draw(hdc, percent(dir.Allocated, total), rect(right-a.scale(142), ry, right-a.scale(88), ry+rowH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
		a.placeAction(rect(right-a.scale(80), ry+1, right-a.scale(8), ry+rowH-1), "打开", ui.FileActionName("reveal", dir.Name), hitReveal, 0, dir.Path, actSecondary)
	}
}

func (a *app) paintFileTable(hdc win.HDC, x, y, right, headH, rowH, rows int) {
	draw(hdc, "文件", rect(x+a.scale(12), y, x+a.scale(200), y+headH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "占用", rect(x+a.scale(200), y, x+a.scale(290), y+headH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "路径", rect(x+a.scale(302), y, right-a.scale(112), y+headH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "操作", rect(right-a.scale(104), y, right-a.scale(12), y+headH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	list := append([]space.File(nil), a.space.Large...)
	if n := a.currentAnalyzeNode(); n != nil && n.Path != "" {
		list = filesUnder(n.Path, list)
	}
	space.SortFiles(list, "size")
	if len(list) == 0 {
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "当前目录没有列入大文件的项目。", rect(x+a.scale(12), y+headH, right-a.scale(12), y+headH+rowH*2), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		return
	}
	if len(list) > rows {
		list = list[:rows]
	}
	for i, file := range list {
		ry := y + headH + i*rowH
		if i > 0 {
			hline(hdc, x+a.scale(12), right-a.scale(12), ry, colorLine)
		}
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, file.Name, rect(x+a.scale(12), ry, x+a.scale(192), ry+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		draw(hdc, clean.FormatBytes(file.Allocated), rect(x+a.scale(200), ry, x+a.scale(290), ry+rowH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
		draw(hdc, file.Path, rect(x+a.scale(302), ry, right-a.scale(112), ry+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		a.placeAction(rect(right-a.scale(104), ry+1, right-a.scale(8), ry+rowH-1), "打开位置", ui.FileActionName("reveal", file.Name), hitReveal, 0, file.Path, actSecondary)
	}
}

type treeFlat struct {
	node  *space.Node
	depth int
}

func flattenTree(n *space.Node, depth int, open map[string]bool, out []treeFlat) []treeFlat {
	if n == nil {
		return out
	}
	out = append(out, treeFlat{node: n, depth: depth})
	if open[n.Path] {
		for _, child := range n.Children {
			out = flattenTree(child, depth+1, open, out)
		}
	}
	return out
}

func (a *app) paintLargePage(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "大文件", "按实际占用列出较大的文件。它们不是垃圾。打开位置、复制路径和移到回收站都需要你确认，系统文件和虚拟磁盘不能删除。")
	if !a.spaceOK {
		y = a.paintEmptyState(hdc, x, y, right, "还没有大文件列表", "使用底部的“开始分析”。分析是只读的，不会把大文件放进清理列表。")
		win.SelectObject(hdc, old)
		return y + a.scale(16) - origin
	}
	y = a.paintLarge(hdc, rc, x, y, right)
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
}

func (a *app) paintLarge(hdc win.HDC, rc win.RECT, x, y, right int) int {
	filters := []struct {
		label string
		min   int64
	}{
		{"大于 100MB", 100 << 20},
		{"大于 500MB", 500 << 20},
		{"大于 1GB", 1 << 30},
		{"大于 5GB", 5 << 30},
	}
	fx := x
	for i, f := range filters {
		w := a.scale(116)
		btn := rect(fx, y, fx+w, y+a.scale(32))
		style := actSecondary
		if a.largeMin == f.min {
			style = actSelected
		}
		a.placeAction(btn, f.label, ui.FilterAccessName(f.label, a.largeMin == f.min), hitFilter, i, "min", style)
		fx += w + a.scale(8)
	}
	custom := rect(fx, y, fx+a.scale(88), y+a.scale(32))
	customOn := a.largeMin != 100<<20 && a.largeMin != 500<<20 && a.largeMin != 1<<30 && a.largeMin != 5<<30
	style := actSecondary
	if customOn {
		style = actSelected
	}
	a.placeAction(custom, "自定义", ui.FilterAccessName("自定义大小", customOn), hitFilter, 0, "custom", style)
	y += a.scale(40)
	sorts := []struct{ id, label string }{{"size", "按大小"}, {"type", "按类型"}, {"time", "按时间"}, {"path", "按路径"}}
	fx = x
	for _, s := range sorts {
		w := a.scale(80)
		btn := rect(fx, y, fx+w, y+a.scale(28))
		style := actSecondary
		if a.largeSort == s.id {
			style = actSelected
		}
		a.placeAction(btn, s.label, ui.FilterAccessName(s.label, a.largeSort == s.id), hitFilter, 0, s.id, style)
		fx += w + a.scale(8)
	}
	y += a.scale(40)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	hintH := a.typeH(ui.TypeCaption) * 2
	draw(hdc, "自定义筛选作用于本次扫描保留的最大 400 个文件。数字是实际占用，不是逻辑大小。系统文件和虚拟磁盘禁止删除。", rect(x, y, right, y+hintH), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += hintH + a.scale(8)
	files := append([]space.File(nil), space.FilterLarge(a.space.Large, a.largeMin)...)
	space.SortFiles(files, a.largeSort)
	if len(files) == 0 {
		return a.paintEmptyState(hdc, x, y, right, "当前筛选下没有大文件", "放宽大小条件，或重新分析后再看这份列表。")
	}
	limit := 100
	if len(files) < limit {
		limit = len(files)
	}
	headH := a.scale(32)
	rowH := a.scale(40)
	roundFill(hdc, rect(x, y, right, y+headH+rowH*limit), 4, colorCard, colorLine)
	pad := a.scale(12)
	opW := a.scale(72)
	dateW := a.scale(132)
	typeW := a.scale(72)
	sizeW := a.scale(88)
	nameW := a.scale(168)
	if nameW > (right-x)/3 {
		nameW = (right - x) / 3
	}
	nameR := x + pad + nameW
	sizeR := nameR + sizeW
	typeR := sizeR + typeW
	dateR := typeR + dateW
	opL := right - pad - opW
	if dateR > opL-a.scale(24) {
		dateR = opL - a.scale(24)
	}
	a.useType(hdc, ui.TypeCaption)
	win.SetTextColor(hdc, colorCaption)
	draw(hdc, "文件名", rect(x+pad, y, nameR-a.scale(8), y+headH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "大小", rect(nameR, y, sizeR-a.scale(8), y+headH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "类型", rect(sizeR+a.scale(8), y, typeR, y+headH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "修改日期", rect(typeR, y, dateR, y+headH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	draw(hdc, "位置", rect(dateR+a.scale(8), y, opL-a.scale(8), y+headH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
	y += headH
	for i, file := range files[:limit] {
		if i > 0 {
			hline(hdc, x+pad, right-pad, y, colorLine)
		}
		ext := file.Ext
		if ext == "" {
			ext = "无扩展名"
		}
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, file.Name, rect(x+pad, y, nameR-a.scale(8), y+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorText)
		draw(hdc, clean.FormatBytes(file.Allocated), rect(nameR, y, sizeR-a.scale(8), y+rowH), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
		win.SetTextColor(hdc, colorCaption)
		draw(hdc, ext, rect(sizeR+a.scale(8), y, typeR, y+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		draw(hdc, time.Unix(file.ModUnix, 0).Format("2006-01-02 15:04"), rect(typeR, y, dateR, y+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		draw(hdc, file.Path, rect(dateR+a.scale(8), y, opL-a.scale(8), y+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		btnTop := y + (rowH-a.scale(28))/2
		a.placeAction(rect(opL, btnTop, opL+opW, btnTop+a.scale(28)), "操作", "操作 "+file.Name, hitFileMenu, 0, file.Path, actSecondary)
		y += rowH
	}
	return y
}

func (a *app) paintTree(hdc win.HDC, view win.RECT, n *space.Node, depth, x, y, right int, total int64) int {
	if n == nil {
		return y
	}
	h := a.scale(32)
	if a.inView(view, y, h) {
		indent := x + depth*a.scale(16)
		open := a.treeOpen[n.Path]
		label := n.Name
		if len(n.Children) > 0 {
			if open {
				label = "▾  " + label
			} else {
				label = "▸  " + label
			}
		}
		style := actSecondary
		if a.selected == n.Path {
			style = actSelected
		}
		caption := label + "   " + clean.FormatBytes(n.Allocated) + "  " + percent(n.Allocated, total)
		a.placeAction(rect(indent, y, right, y+h), caption, ui.TreeAccessName(n.Name, len(n.Children) > 0, open), hitTree, 0, n.Path, style)
	}
	y += h + a.scale(4)
	if a.treeOpen[n.Path] {
		for _, child := range n.Children {
			y = a.paintTree(hdc, view, child, depth+1, x, y, right, total)
		}
	}
	return y
}

func (a *app) paintDupes(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "重复文件", "按大小和内容哈希判断。硬链接不算副本。删除前必须至少保留一份，系统文件和虚拟磁盘禁止删除。")
	if a.phase == phaseDupes {
		win.SetTextColor(hdc, colorPrimary)
		draw(hdc, a.statusText, rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		y += a.scale(32)
	}
	if !a.dupesOK {
		y = a.paintEmptyState(hdc, x, y, right, "还没有查找结果", "使用底部的“查找重复”。此页可以打开位置或移到回收站，不会自动删除整组。")
		win.SelectObject(hdc, old)
		return y + a.scale(16) - origin
	}
	rep := a.dupes
	sumH := a.scale(72)
	roundFill(hdc, rect(x, y, right, y+sumH), ui.Radius, colorCard, colorLine)
	sums := []struct{ k, v string }{
		{"重复组", fmt.Sprintf("%d", len(rep.Groups))},
		{"文件", fmt.Sprintf("%d", rep.Files)},
		{"可节省", clean.FormatBytes(rep.Bytes)},
	}
	sw := (right - x - a.scale(32)) / 3
	for i, s := range sums {
		sx := x + a.scale(16) + i*sw
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		draw(hdc, s.k, rect(sx, y+a.scale(12), sx+sw-a.scale(8), y+a.scale(32)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, s.v, rect(sx, y+a.scale(34), sx+sw-a.scale(8), y+a.scale(58)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	}
	y += sumH + a.scale(8)
	if rep.Note != "" {
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, rep.Note, rect(x, y, right, y+a.typeH(ui.TypeCaption)*2), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		y += a.typeH(ui.TypeCaption)*2 + a.scale(8)
	}
	if len(rep.Groups) == 0 {
		y = a.paintEmptyState(hdc, x, y, right, "没有发现重复文件", "按内容哈希比较后，没有两份完全相同、且不是硬链接的文件。")
		win.SelectObject(hdc, old)
		return y + a.scale(16) - origin
	}
	limit := 40
	if len(rep.Groups) < limit {
		limit = len(rep.Groups)
	}
	for gi, g := range rep.Groups[:limit] {
		headH := a.scale(40)
		rowH := a.scale(64)
		groupH := headH + len(g.Files)*rowH
		roundFill(hdc, rect(x, y, right, y+groupH), ui.Radius, colorCard, colorLine)
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		save := int64(0)
		if n := len(g.Files); n > 1 && g.Size > 0 {
			save = g.Size * int64(n-1)
		}
		draw(hdc, fmt.Sprintf("组 %d · %d 个文件 · 单个 %s · 可节省 %s · SHA256 %s", gi+1, len(g.Files), clean.FormatBytes(g.Size), clean.FormatBytes(save), shortHash(g.Hash)), rect(x+a.scale(16), y, right-a.scale(16), y+headH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		hline(hdc, x+a.scale(16), right-a.scale(16), y+headH, colorLine)
		y += headH
		for i, file := range g.Files {
			h := rowH
			if a.inView(rc, y, h) {
				if i > 0 {
					hline(hdc, x+a.scale(16), right-a.scale(16), y, colorLine)
				}
				name := baseName(file.Path)
				a.useType(hdc, ui.TypeItem)
				win.SetTextColor(hdc, colorText)
				draw(hdc, name, rect(x+a.scale(12), y+a.scale(8), right-a.scale(292), y+a.scale(30)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
				a.useType(hdc, ui.TypeCaption)
				win.SetTextColor(hdc, colorMuted)
				draw(hdc, file.Path, rect(x+a.scale(12), y+a.scale(32), right-a.scale(292), y+a.scale(56)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
				bx := right - a.scale(280)
				open := rect(bx, y+a.scale(16), bx+a.scale(84), y+a.scale(48))
				a.placeAction(open, "打开位置", ui.FileActionName("reveal", name), hitReveal, 0, file.Path, actSecondary)
				copyBtn := rect(bx+a.scale(92), y+a.scale(16), bx+a.scale(176), y+a.scale(48))
				a.placeAction(copyBtn, "复制路径", ui.FileActionName("copy", name), hitCopy, 0, file.Path, actSecondary)
				rec := rect(bx+a.scale(184), y+a.scale(16), bx+a.scale(268), y+a.scale(48))
				a.placeAction(rec, "回收站", ui.FileActionName("recycle", name), hitRecycle, 0, file.Path, actSecondary)
			}
			y += h
		}
		y += a.scale(12)
	}
	if len(rep.Groups) > limit {
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "仅显示前 40 组，避免列表过长。", rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		y += a.scale(24)
	}
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
}

func (a *app) paintApps(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "软件管理", "来自 Windows 卸载登记。不会测量安装目录，也不会直接删除文件。")
	if !a.appsOK {
		y = a.paintEmptyState(hdc, x, y, right, "还没有软件列表", "使用底部的“刷新软件列表”。卸载会交给软件自己的卸载程序。")
		win.SelectObject(hdc, old)
		return y + a.scale(16) - origin
	}
	if a.apps.Note != "" {
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, a.apps.Note, rect(x, y, right, y+a.typeH(ui.TypeCaption)*2), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		y += a.typeH(ui.TypeCaption)*2 + a.scale(8)
	}
	if len(a.apps.Apps) == 0 {
		y = a.paintEmptyState(hdc, x, y, right, "没有可卸载的软件", "卸载登记里没有带卸载入口的项目。")
		win.SelectObject(hdc, old)
		return y + a.scale(16) - origin
	}
	rowH := a.scale(88)
	roundFill(hdc, rect(x, y, right, y+rowH*len(a.apps.Apps)), ui.Radius, colorCard, colorLine)
	for i, app := range a.apps.Apps {
		h := rowH
		if i > 0 {
			hline(hdc, x+a.scale(16), right-a.scale(16), y, colorLine)
		}
		textRight := right - a.scale(112)
		a.useType(hdc, ui.TypeItem)
		win.SetTextColor(hdc, colorText)
		draw(hdc, app.Name, rect(x+a.scale(16), y+a.scale(10), textRight, y+a.scale(32)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		a.useType(hdc, ui.TypeCaption)
		win.SetTextColor(hdc, colorCaption)
		line := app.Publisher
		if line == "" {
			line = "未登记发布者"
		}
		if app.Version != "" {
			line += "  ·  " + app.Version
		}
		size := "占用未知"
		if app.EstimatedOK {
			size = "登记 " + clean.FormatBytes(app.Estimated)
		}
		draw(hdc, line+"  ·  "+size, rect(x+a.scale(16), y+a.scale(32), textRight, y+a.scale(52)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		win.SetTextColor(hdc, colorMuted)
		loc := app.Location
		if loc == "" {
			loc = "未登记安装位置"
		}
		draw(hdc, loc, rect(x+a.scale(16), y+a.scale(52), textRight, y+a.scale(74)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		btnTop := y + a.scale(12)
		btnH := a.scale(28)
		if app.Location == "" {
			btnTop = y + (h-btnH)/2
		}
		a.placeAction(rect(right-a.scale(96), btnTop, right-a.scale(12), btnTop+btnH), "卸载", "卸载 "+app.Name, hitUninstall, i, app.Name, actSecondary)
		if app.Location != "" {
			openTop := y + a.scale(48)
			a.placeAction(rect(right-a.scale(96), openTop, right-a.scale(12), openTop+btnH), "位置", ui.FileActionName("reveal", app.Name), hitReveal, 0, app.Location, actSecondary)
		}
		y += h
	}
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
}

func (a *app) paintSystem(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	y = a.paintPageHeader(hdc, x, y, right, "系统工具", "只读查看系统占用。不会删除 pagefile、休眠文件、WinSxS、系统还原或 WSL 虚拟磁盘。")
	if !a.sysOK {
		y = a.paintEmptyState(hdc, x, y, right, "还没有系统占用数据", "使用底部的“刷新系统占用”。这里只展示系统报告的大小，不直接删除。")
		win.SelectObject(hdc, old)
		return y + a.scale(16) - origin
	}
	stBtn := rect(x, y, x+a.scale(140), y+a.scale(32))
	a.placeAction(stBtn, "存储设置", "打开 Windows 存储设置", hitWinStorage, 0, "ms-settings:storagesense", actSecondary)
	prot := rect(x+a.scale(152), y, x+a.scale(320), y+a.scale(32))
	a.placeAction(prot, "系统还原设置", "打开系统还原设置", hitWinStorage, 0, "SystemPropertiesProtection.exe", actSecondary)
	y += a.scale(44)
	y = a.paintSectionLabel(hdc, x, y, right, "系统占用")
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	draw(hdc, a.sys.Note, rect(x, y, right, y+a.scale(40)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(44)
	if len(a.sys.Items) > 0 {
		rowH := a.scale(72)
		roundFill(hdc, rect(x, y, right, y+rowH*len(a.sys.Items)), ui.Radius, colorCard, colorLine)
	}
	for i, item := range a.sys.Items {
		h := a.scale(72)
		if i > 0 {
			hline(hdc, x+a.scale(16), right-a.scale(16), y, colorLine)
		}
		win.SetTextColor(hdc, colorText)
		size := item.Status
		if size == "" {
			size = "未知"
		}
		if item.Known && item.Bytes >= 0 {
			size = clean.FormatBytes(item.Bytes)
		}
		a.useType(hdc, ui.TypeItem)
		draw(hdc, item.Name, rect(x+a.scale(16), y+a.scale(10), right-a.scale(168), y+a.scale(32)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		draw(hdc, size, rect(right-a.scale(156), y+a.scale(10), right-a.scale(16), y+a.scale(32)), win.DT_RIGHT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		win.SetTextColor(hdc, colorMuted)
		a.useType(hdc, ui.TypeCaption)
		draw(hdc, item.Note, rect(x+a.scale(16), y+a.scale(36), right-a.scale(16), y+h-a.scale(8)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		y += h
	}
	y = a.paintSectionLabel(hdc, x, y, right, "开发环境")
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, a.dev.Note, rect(x, y, right, y+a.scale(48)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(52)
	if len(a.dev.Items) == 0 {
		y = a.paintEmptyState(hdc, x, y, right, "没有识别到开发环境", "未发现常见的开发工具缓存或 SDK 目录。")
		y += a.scale(12)
	}
	if len(a.dev.Items) > 0 {
		rowH := a.scale(72)
		roundFill(hdc, rect(x, y, right, y+rowH*len(a.dev.Items)), ui.Radius, colorCard, colorLine)
	}
	for i, item := range a.dev.Items {
		h := a.scale(72)
		if a.inView(rc, y, h) {
			if i > 0 {
				hline(hdc, x+a.scale(16), right-a.scale(16), y, colorLine)
			}
			a.useType(hdc, ui.TypeItem)
			win.SetTextColor(hdc, colorText)
			draw(hdc, item.Name+"  ·  "+item.Kind, rect(x+a.scale(16), y+a.scale(10), right-a.scale(228), y+a.scale(32)), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
			draw(hdc, clean.FormatBytes(item.Bytes), rect(right-a.scale(216), y+a.scale(10), right-a.scale(104), y+a.scale(32)), win.DT_RIGHT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
			win.SetTextColor(hdc, colorMuted)
			a.useType(hdc, ui.TypeCaption)
			draw(hdc, item.Note+"  "+item.Path, rect(x+a.scale(16), y+a.scale(36), right-a.scale(104), y+h-a.scale(8)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
			open := rect(right-a.scale(88), y+a.scale(18), right-a.scale(12), y+a.scale(50))
			a.placeAction(open, "打开", ui.FileActionName("reveal", item.Name), hitReveal, 0, item.Path, actSecondary)
		}
		y += h
	}
	y = a.paintSectionLabel(hdc, x, y, right, "扫描快照")
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "磁盘分析完成后会保存一份摘要。最多保留 8 份，路径只记目录名。", rect(x, y, right, y+a.scale(40)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(44)
	prune := rect(x, y, x+a.scale(148), y+a.scale(32))
	a.placeAction(prune, "清理历史快照", "清理历史快照", hitSnapPrune, 0, "", actSecondary)
	y += a.scale(44)
	if len(a.snaps) == 0 {
		y = a.paintEmptyState(hdc, x, y, right, "还没有快照", "完成一次磁盘分析后，这里保留最多 8 份摘要。")
		y += a.scale(8)
	} else {
		if len(a.snaps) >= 2 {
			diff := snapshot.Compare(a.snaps[1], a.snaps[0])
			win.SetTextColor(hdc, colorText)
			draw(hdc, "最近两次变化", rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
			y += a.scale(26)
			if len(diff) == 0 {
				win.SetTextColor(hdc, colorMuted)
				draw(hdc, "主要目录占用没有变化。", rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
				y += a.scale(22)
			}
			for _, d := range diff {
				sign := "+"
				if d.Delta < 0 {
					sign = ""
				}
				win.SetTextColor(hdc, colorText)
				draw(hdc, fmt.Sprintf("%s  %s%s", d.Name, sign, clean.FormatBytes(d.Delta)), rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
				y += a.scale(22)
			}
			y += a.scale(8)
		}
		rowH := a.scale(40)
		roundFill(hdc, rect(x, y, right, y+rowH*len(a.snaps)), ui.Radius, colorCard, colorLine)
		for i, rec := range a.snaps {
			if i > 0 {
				hline(hdc, x+a.scale(16), right-a.scale(16), y, colorLine)
			}
			a.useType(hdc, ui.TypeItem)
			win.SetTextColor(hdc, colorText)
			draw(hdc, rec.When.Local().Format("2006-01-02 15:04"), rect(x+a.scale(12), y, x+a.scale(180), y+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
			a.useType(hdc, ui.TypeCaption)
			win.SetTextColor(hdc, colorCaption)
			draw(hdc, "占用 "+clean.FormatBytes(rec.Allocated)+"    可用 "+clean.FormatBytes(int64(rec.Free))+"    文件 "+fmt.Sprintf("%d", rec.Files), rect(x+a.scale(188), y, right-a.scale(12), y+rowH), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
			y += rowH
		}
	}
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
}

func (a *app) inView(rc win.RECT, y, h int) bool {
	// Every action stays a real control, including rows above or below the
	// viewport, so Tab can reach them. The content host clips the pixels.
	_ = rc
	_ = y
	_ = h
	return true
}

func treemapNodes(root *space.Node, selected string) []*space.Node {
	if root == nil {
		return nil
	}
	n := findNode(root, selected)
	if n != nil && len(n.Children) > 0 {
		return n.Children
	}
	return root.Children
}

func findNode(n *space.Node, path string) *space.Node {
	if n == nil || path == "" {
		return n
	}
	if n.Path == path {
		return n
	}
	for _, child := range n.Children {
		if got := findNode(child, path); got != nil && got.Path == path {
			return got
		}
	}
	return nil
}

func (a *app) analyzeParent() *space.Node {
	n := a.currentAnalyzeNode()
	if n == nil || a.space.Tree == nil || strings.EqualFold(n.Path, a.space.Tree.Path) {
		return nil
	}
	return parentOf(a.space.Tree, n.Path)
}

func parentOf(root *space.Node, path string) *space.Node {
	if root == nil {
		return nil
	}
	for _, child := range root.Children {
		if child == nil {
			continue
		}
		if strings.EqualFold(child.Path, path) {
			return root
		}
		if found := parentOf(child, path); found != nil {
			return found
		}
	}
	return nil
}

func (a *app) currentAnalyzeNode() *space.Node {
	if !a.spaceOK || a.space.Tree == nil {
		return nil
	}
	if a.selected == "" {
		return a.space.Tree
	}
	if n := findNode(a.space.Tree, a.selected); n != nil {
		return n
	}
	return a.space.Tree
}

func (a *app) childDirs(n *space.Node) []space.Dir {
	if n == nil {
		return append([]space.Dir(nil), a.space.TopDirs...)
	}
	list := make([]space.Dir, 0, len(n.Children))
	for _, c := range n.Children {
		if c == nil {
			continue
		}
		name := c.Name
		if name == "" {
			name = c.Path
		}
		list = append(list, space.Dir{Path: c.Path, Name: name, Allocated: c.Allocated, Files: c.Files})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Allocated > list[j].Allocated })
	return list
}

func filesUnder(root string, list []space.File) []space.File {
	var out []space.File
	for _, f := range list {
		if hasPrefixPath(f.Path, root) {
			out = append(out, f)
		}
	}
	return out
}

func sizeRank(cells []space.Cell, index int) int {
	rank := 0
	for i := range cells {
		if cells[i].Allocated > cells[index].Allocated {
			rank++
		}
	}
	return rank
}

func percent(part, total int64) string {
	if total <= 0 || part <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", float64(part)*100/float64(total))
}

func baseName(path string) string {
	i := strings.LastIndexAny(path, `/\`)
	if i >= 0 && i+1 < len(path) {
		return path[i+1:]
	}
	if path == "" {
		return "该文件"
	}
	return path
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
