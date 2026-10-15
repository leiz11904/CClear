//go:build windows

package main

import (
	"fmt"
	"time"

	"cclear/internal/clean"
	"cclear/internal/snapshot"
	"cclear/internal/space"
	"github.com/lxn/win"
)

func (a *app) paintAnalyze(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	win.SetTextColor(hdc, colorText)
	draw(hdc, "磁盘分析", rect(x, y, right, y+a.scale(32)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(40)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "只显示真实占用。大文件不会被当成垃圾。硬链接按实际分配去重。", rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	y += a.scale(28)
	if a.phase == phaseAnalyze {
		win.SetTextColor(hdc, colorPrimary)
		draw(hdc, a.statusText, rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		y += a.scale(32)
	}
	if !a.spaceOK {
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "还没有分析结果。点击底部“开始分析”扫描系统盘。自定义大小筛选尚未提供，因此没有该项按钮。", rect(x, y, right, y+a.scale(48)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		win.SelectObject(hdc, old)
		return y - origin + a.scale(48)
	}
	rep := a.space
	card := rect(x, y, right, y+a.scale(120))
	roundFill(hdc, card, 12, colorCard, colorLine)
	win.SetTextColor(hdc, colorText)
	draw(hdc, fmt.Sprintf("逻辑大小 %s    实际占用 %s    无法统计 %d 处", clean.FormatBytes(rep.Logical), clean.FormatBytes(rep.Allocated), int(rep.Uncounted)), rect(x+a.scale(20), y+a.scale(16), right-a.scale(16), y+a.scale(40)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, fmt.Sprintf("文件 %d · 目录 %d · 硬链接去重 %d · 跳过重解析点 %d", rep.Files, rep.Dirs, rep.HardLinks, rep.ReparseSkipped), rect(x+a.scale(20), y+a.scale(44), right-a.scale(16), y+a.scale(68)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	draw(hdc, rep.Note, rect(x+a.scale(20), y+a.scale(72), right-a.scale(16), y+a.scale(108)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(136)

	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "空间矩形图", rect(x, y, right, y+a.scale(26)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(30)
	mapH := a.scale(220)
	mapRC := rect(x, y, right, y+mapH)
	roundFill(hdc, mapRC, 8, colorLine, colorLine)
	nodes := treemapNodes(rep.Tree, a.selected)
	cells := space.LayoutTreemap(nodes, float64(mapRC.Left), float64(mapRC.Top), float64(mapRC.Right-mapRC.Left), float64(mapRC.Bottom-mapRC.Top))
	for i, cell := range cells {
		cr := rect(int(cell.X), int(cell.Y), int(cell.X+cell.W), int(cell.Y+cell.H))
		roundFill(hdc, cr, 2, mapColors[i%len(mapColors)], colorWhite)
		if cell.W > 48 && cell.H > 18 {
			win.SetTextColor(hdc, colorText)
			draw(hdc, cell.Name, cr, win.DT_CENTER|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		}
		a.hits = append(a.hits, hit{kind: hitMap, id: cell.Path, rc: cr})
	}
	y += mapH + a.scale(16)

	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "目录树", rect(x, y, right, y+a.scale(26)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(28)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	y = a.paintTree(hdc, rc, rep.Tree, 0, x, y, right, rep.Allocated)

	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "文件类型", rect(x, y, right, y+a.scale(26)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(28)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	for i, st := range rep.Types {
		if i >= 20 {
			break
		}
		if a.inView(rc, y, a.scale(22)) {
			win.SetTextColor(hdc, colorText)
			draw(hdc, fmt.Sprintf("%s    %s    %d 个    %s", st.Ext, clean.FormatBytes(st.Allocated), st.Files, percent(st.Allocated, rep.Allocated)), rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		}
		y += a.scale(22)
	}
	y += a.scale(12)

	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "最大目录（Top 100）", rect(x, y, right, y+a.scale(26)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(28)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	for _, dir := range rep.TopDirs {
		h := a.scale(44)
		if a.inView(rc, y, h) {
			row := rect(x, y, right, y+h)
			roundFill(hdc, row, 8, colorCard, colorLine)
			win.SetTextColor(hdc, colorText)
			draw(hdc, fmt.Sprintf("%s    %s    %s", dir.Name, clean.FormatBytes(dir.Allocated), percent(dir.Allocated, rep.Allocated)), rect(x+a.scale(12), y, right-a.scale(90), y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
			win.SetTextColor(hdc, colorMuted)
			draw(hdc, dir.Path, rect(x+a.scale(12), y+a.scale(22), right-a.scale(90), y+h), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
			btn := rect(right-a.scale(80), y+a.scale(8), right-a.scale(12), y+a.scale(36))
			paintGhostBtn(hdc, btn, "打开", true)
			a.hits = append(a.hits, hit{kind: hitReveal, id: dir.Path, rc: btn})
		}
		y += h + a.scale(6)
	}
	y += a.scale(8)

	y = a.paintLarge(hdc, rc, x, y, right)
	win.SelectObject(hdc, old)
	return y - origin + a.scale(24)
}

func (a *app) paintLarge(hdc win.HDC, rc win.RECT, x, y, right int) int {
	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "大文件", rect(x, y, right, y+a.scale(26)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(30)
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
		w := a.scale(100)
		btn := rect(fx, y, fx+w, y+a.scale(32))
		if a.largeMin == f.min {
			paintPrimaryBtn(hdc, btn, f.label, true)
		} else {
			paintGhostBtn(hdc, btn, f.label, true)
		}
		a.hits = append(a.hits, hit{kind: hitFilter, index: i, id: "min", rc: btn})
		fx += w + a.scale(8)
	}
	custom := rect(fx, y, fx+a.scale(100), y+a.scale(32))
	customOn := a.largeMin != 100<<20 && a.largeMin != 500<<20 && a.largeMin != 1<<30 && a.largeMin != 5<<30
	if customOn {
		paintPrimaryBtn(hdc, custom, "自定义", true)
	} else {
		paintGhostBtn(hdc, custom, "自定义", true)
	}
	a.hits = append(a.hits, hit{kind: hitFilter, id: "custom", rc: custom})
	y += a.scale(40)
	sorts := []struct{ id, label string }{{"size", "按大小"}, {"type", "按类型"}, {"time", "按时间"}, {"path", "按路径"}}
	fx = x
	for _, s := range sorts {
		w := a.scale(80)
		btn := rect(fx, y, fx+w, y+a.scale(28))
		if a.largeSort == s.id {
			paintPrimaryBtn(hdc, btn, s.label, true)
		} else {
			paintGhostBtn(hdc, btn, s.label, true)
		}
		a.hits = append(a.hits, hit{kind: hitFilter, id: s.id, rc: btn})
		fx += w + a.scale(8)
	}
	y += a.scale(40)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "自定义筛选作用于本次扫描保留的最大 400 个文件。数字是实际占用，不是逻辑大小。系统文件和虚拟磁盘禁止删除。", rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	y += a.scale(28)
	files := append([]space.File(nil), space.FilterLarge(a.space.Large, a.largeMin)...)
	space.SortFiles(files, a.largeSort)
	if len(files) == 0 {
		draw(hdc, "当前筛选下没有大文件。", rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		return y + a.scale(28)
	}
	limit := 100
	if len(files) < limit {
		limit = len(files)
	}
	for _, file := range files[:limit] {
		h := a.scale(110)
		if a.inView(rc, y, h) {
			row := rect(x, y, right, y+h)
			roundFill(hdc, row, 8, colorCard, colorLine)
			win.SetTextColor(hdc, colorText)
			draw(hdc, file.Name, rect(x+a.scale(12), y+a.scale(8), right-a.scale(12), y+a.scale(30)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
			win.SetTextColor(hdc, colorMuted)
			mod := time.Unix(file.ModUnix, 0).Format("2006-01-02 15:04")
			draw(hdc, fmt.Sprintf("%s    占用 %s    逻辑 %s    %s    %s", file.Ext, clean.FormatBytes(file.Allocated), clean.FormatBytes(file.Logical), percent(file.Allocated, a.space.Allocated), mod), rect(x+a.scale(12), y+a.scale(30), right-a.scale(12), y+a.scale(52)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
			draw(hdc, file.Path, rect(x+a.scale(12), y+a.scale(52), right-a.scale(12), y+a.scale(74)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
			bx := x + a.scale(12)
			for _, item := range []struct {
				kind  int
				label string
			}{
				{hitOpen, "查看"},
				{hitReveal, "打开位置"},
				{hitCopy, "复制路径"},
				{hitProps, "属性"},
				{hitRecycle, "回收站"},
			} {
				btn := rect(bx, y+a.scale(78), bx+a.scale(72), y+h-a.scale(8))
				paintGhostBtn(hdc, btn, item.label, true)
				a.hits = append(a.hits, hit{kind: item.kind, id: file.Path, rc: btn})
				bx += a.scale(80)
			}
		}
		y += h + a.scale(8)
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
		if a.selected == n.Path {
			roundFill(hdc, rect(indent, y, right, y+h), 6, colorNavPick, colorNavPick)
		}
		win.SetTextColor(hdc, colorText)
		draw(hdc, label, rect(indent, y, right-a.scale(160), y+h), win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		draw(hdc, clean.FormatBytes(n.Allocated)+"  "+percent(n.Allocated, total), rect(right-a.scale(150), y, right-a.scale(8), y+h), win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
		a.hits = append(a.hits, hit{kind: hitTree, id: n.Path, rc: rect(indent, y, right, y+h)})
	}
	y += h
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
	win.SetTextColor(hdc, colorText)
	draw(hdc, "重复文件", rect(x, y, right, y+a.scale(32)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(40)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "按大小和内容哈希判断。硬链接不算副本。删除前必须至少保留一份，系统文件和虚拟磁盘禁止删除。", rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	y += a.scale(28)
	if a.phase == phaseDupes {
		win.SetTextColor(hdc, colorPrimary)
		draw(hdc, a.statusText, rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		y += a.scale(32)
	}
	if !a.dupesOK {
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, "还没有重复文件结果。点击底部“查找重复”。此页不提供删除按钮。", rect(x, y, right, y+a.scale(48)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		win.SelectObject(hdc, old)
		return y - origin + a.scale(48)
	}
	rep := a.dupes
	draw(hdc, fmt.Sprintf("%s  组数 %d · 文件 %d · 可节省约 %s", rep.Note, len(rep.Groups), rep.Files, clean.FormatBytes(rep.Bytes)), rect(x, y, right, y+a.scale(40)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(48)
	if len(rep.Groups) == 0 {
		draw(hdc, "没有发现内容相同的文件。", rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		win.SelectObject(hdc, old)
		return y - origin + a.scale(48)
	}
	limit := 40
	if len(rep.Groups) < limit {
		limit = len(rep.Groups)
	}
	for gi, g := range rep.Groups[:limit] {
		win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
		win.SetTextColor(hdc, colorText)
		if a.inView(rc, y, a.scale(24)) {
			draw(hdc, fmt.Sprintf("组 %d · %s · SHA256 %s", gi+1, clean.FormatBytes(g.Size), shortHash(g.Hash)), rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		}
		y += a.scale(28)
		win.SelectObject(hdc, win.HGDIOBJ(a.font))
		for _, file := range g.Files {
			h := a.scale(64)
			if a.inView(rc, y, h) {
				row := rect(x, y, right, y+h)
				roundFill(hdc, row, 8, colorCard, colorLine)
				win.SetTextColor(hdc, colorText)
				draw(hdc, file.Path, rect(x+a.scale(12), y+a.scale(6), right-a.scale(260), y+a.scale(28)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
				bx := right - a.scale(250)
				open := rect(bx, y+a.scale(16), bx+a.scale(72), y+a.scale(44))
				paintGhostBtn(hdc, open, "打开位置", true)
				a.hits = append(a.hits, hit{kind: hitReveal, id: file.Path, rc: open})
				copyBtn := rect(bx+a.scale(80), y+a.scale(16), bx+a.scale(152), y+a.scale(44))
				paintGhostBtn(hdc, copyBtn, "复制路径", true)
				a.hits = append(a.hits, hit{kind: hitCopy, id: file.Path, rc: copyBtn})
				rec := rect(bx+a.scale(160), y+a.scale(16), bx+a.scale(232), y+a.scale(44))
				paintGhostBtn(hdc, rec, "回收站", true)
				a.hits = append(a.hits, hit{kind: hitRecycle, id: file.Path, rc: rec})
			}
			y += h + a.scale(6)
		}
		y += a.scale(8)
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
	win.SetTextColor(hdc, colorText)
	draw(hdc, "软件管理", rect(x, y, right, y+a.scale(32)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(40)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "来自 Windows 卸载登记。不会测量或删除安装目录。", rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
	y += a.scale(28)
	if !a.appsOK {
		draw(hdc, "还没有软件列表。点击底部“刷新软件列表”。", rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		win.SelectObject(hdc, old)
		return y - origin + a.scale(48)
	}
	draw(hdc, a.apps.Note, rect(x, y, right, y+a.scale(48)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(56)
	if len(a.apps.Apps) == 0 {
		draw(hdc, "没有读到带卸载入口的软件。", rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		win.SelectObject(hdc, old)
		return y - origin + a.scale(48)
	}
	for i, app := range a.apps.Apps {
		h := a.scale(96)
		if !a.inView(rc, y, h) {
			y += h + a.scale(8)
			continue
		}
		row := rect(x, y, right, y+h)
		roundFill(hdc, row, 10, colorCard, colorLine)
		win.SetTextColor(hdc, colorText)
		size := "系统未登记大小"
		if app.EstimatedOK {
			size = "登记大小 " + clean.FormatBytes(app.Estimated)
		}
		draw(hdc, app.Name, rect(x+a.scale(16), y+a.scale(10), right-a.scale(120), y+a.scale(34)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		win.SetTextColor(hdc, colorMuted)
		line := app.Publisher
		if app.Version != "" {
			line += "  ·  " + app.Version
		}
		draw(hdc, line+"  ·  "+size, rect(x+a.scale(16), y+a.scale(34), right-a.scale(120), y+a.scale(56)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		loc := app.Location
		if loc == "" {
			loc = "未登记安装位置"
		}
		draw(hdc, loc, rect(x+a.scale(16), y+a.scale(56), right-a.scale(120), y+a.scale(76)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		btn := rect(right-a.scale(100), y+a.scale(30), right-a.scale(16), y+a.scale(64))
		paintGhostBtn(hdc, btn, "卸载", true)
		a.hits = append(a.hits, hit{kind: hitUninstall, index: i, rc: btn})
		if app.Location != "" {
			open := rect(x+a.scale(16), y+a.scale(76), x+a.scale(100), y+h-a.scale(6))
			paintGhostBtn(hdc, open, "打开位置", true)
			a.hits = append(a.hits, hit{kind: hitReveal, id: app.Location, rc: open})
		}
		y += h + a.scale(8)
	}
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
}

func (a *app) paintSystem(hdc win.HDC, rc win.RECT, origin int) int {
	x, y, right := a.layoutContent(rc, origin)
	old := win.SelectObject(hdc, win.HGDIOBJ(a.titleFont))
	win.SetBkMode(hdc, win.TRANSPARENT)
	win.SetTextColor(hdc, colorText)
	draw(hdc, "系统工具", rect(x, y, right, y+a.scale(32)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(40)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "只读分析。不会删除 pagefile、休眠文件、WinSxS、系统还原或 WSL 虚拟磁盘。", rect(x, y, right, y+a.scale(40)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(48)
	if !a.sysOK {
		draw(hdc, "还没有系统占用数据。点击底部“刷新系统占用”。", rect(x, y, right, y+a.scale(24)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		win.SelectObject(hdc, old)
		return y - origin + a.scale(48)
	}
	stBtn := rect(x, y, x+a.scale(140), y+a.scale(32))
	paintGhostBtn(hdc, stBtn, "存储设置", true)
	a.hits = append(a.hits, hit{kind: hitWinStorage, id: "ms-settings:storagesense", rc: stBtn})
	prot := rect(x+a.scale(152), y, x+a.scale(300), y+a.scale(32))
	paintGhostBtn(hdc, prot, "系统还原设置", true)
	a.hits = append(a.hits, hit{kind: hitWinStorage, id: "SystemPropertiesProtection.exe", rc: prot})
	y += a.scale(44)
	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "系统占用", rect(x, y, right, y+a.scale(26)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(28)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	draw(hdc, a.sys.Note, rect(x, y, right, y+a.scale(40)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(44)
	for _, item := range a.sys.Items {
		h := a.scale(72)
		row := rect(x, y, right, y+h)
		roundFill(hdc, row, 8, colorCard, colorLine)
		win.SetTextColor(hdc, colorText)
		size := "无法读取"
		if item.Bytes < 0 {
			size = "请在磁盘分析中查看目录大小"
		} else if item.Known {
			size = clean.FormatBytes(item.Bytes)
		}
		draw(hdc, item.Name+"    "+size, rect(x+a.scale(16), y+a.scale(10), right-a.scale(16), y+a.scale(34)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
		win.SetTextColor(hdc, colorMuted)
		draw(hdc, item.Note, rect(x+a.scale(16), y+a.scale(34), right-a.scale(16), y+h-a.scale(8)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		y += h + a.scale(8)
	}
	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "开发环境", rect(x, y, right, y+a.scale(26)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(28)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, a.dev.Note, rect(x, y, right, y+a.scale(48)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(52)
	if len(a.dev.Items) == 0 {
		draw(hdc, "没有识别到已安装的开发环境目录。", rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		y += a.scale(28)
	}
	for _, item := range a.dev.Items {
		h := a.scale(72)
		if a.inView(rc, y, h) {
			row := rect(x, y, right, y+h)
			roundFill(hdc, row, 8, colorCard, colorLine)
			win.SetTextColor(hdc, colorText)
			draw(hdc, item.Name+"  ·  "+item.Kind+"  ·  "+clean.FormatBytes(item.Bytes), rect(x+a.scale(16), y+a.scale(10), right-a.scale(90), y+a.scale(34)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
			win.SetTextColor(hdc, colorMuted)
			draw(hdc, item.Note+"  "+item.Path, rect(x+a.scale(16), y+a.scale(34), right-a.scale(90), y+h-a.scale(8)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
			open := rect(right-a.scale(80), y+a.scale(18), right-a.scale(12), y+a.scale(50))
			paintGhostBtn(hdc, open, "打开", true)
			a.hits = append(a.hits, hit{kind: hitReveal, id: item.Path, rc: open})
		}
		y += h + a.scale(8)
	}
	win.SelectObject(hdc, win.HGDIOBJ(a.subtitleFont))
	win.SetTextColor(hdc, colorText)
	draw(hdc, "扫描快照", rect(x, y, right, y+a.scale(26)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
	y += a.scale(28)
	win.SelectObject(hdc, win.HGDIOBJ(a.font))
	win.SetTextColor(hdc, colorMuted)
	draw(hdc, "磁盘分析完成后会保存一份摘要。最多保留 8 份，路径只记目录名。", rect(x, y, right, y+a.scale(40)), win.DT_LEFT|dtWordBreak|dtNoPrefix)
	y += a.scale(44)
	prune := rect(x, y, x+a.scale(140), y+a.scale(32))
	paintGhostBtn(hdc, prune, "清理历史快照", true)
	a.hits = append(a.hits, hit{kind: hitSnapPrune, rc: prune})
	y += a.scale(44)
	if len(a.snaps) == 0 {
		draw(hdc, "还没有快照。完成一次磁盘分析后会出现。", rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtNoPrefix)
		y += a.scale(28)
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
		for _, rec := range a.snaps {
			draw(hdc, fmt.Sprintf("%s  占用 %s  可用 %s  文件 %d", rec.When.Local().Format("2006-01-02 15:04"), clean.FormatBytes(rec.Allocated), clean.FormatBytes(int64(rec.Free)), rec.Files), rect(x, y, right, y+a.scale(22)), win.DT_LEFT|dtSingleLine|dtEndEllipsis|dtNoPrefix)
			y += a.scale(22)
		}
	}
	win.SelectObject(hdc, old)
	return y - origin + a.scale(16)
}

func (a *app) inView(rc win.RECT, y, h int) bool {
	return int32(y+h) >= rc.Top && int32(y) < rc.Bottom
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

func percent(part, total int64) string {
	if total <= 0 || part <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", float64(part)*100/float64(total))
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
