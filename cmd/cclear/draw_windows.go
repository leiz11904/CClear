//go:build windows

package main

import (
	"math"
	"runtime"
	"unsafe"

	"cclear/internal/ui"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

var (
	gdi32           = windows.NewLazySystemDLL("gdi32.dll")
	procRoundRect   = gdi32.NewProc("RoundRect")
	procEllipse     = gdi32.NewProc("Ellipse")
	procPie         = gdi32.NewProc("Pie")
	procCreatePen   = gdi32.NewProc("CreatePen")
	procEllipticRgn = gdi32.NewProc("CreateEllipticRgn")
	procSetWindowRgn = user32.NewProc("SetWindowRgn")
	gdiplusDLL       = windows.NewLazySystemDLL("gdiplus.dll")
	procGdipStartup  = gdiplusDLL.NewProc("GdiplusStartup")
	procGdipFromHDC  = gdiplusDLL.NewProc("GdipCreateFromHDC")
	procGdipDelGraph = gdiplusDLL.NewProc("GdipDeleteGraphics")
	procGdipSmooth   = gdiplusDLL.NewProc("GdipSetSmoothingMode")
	procGdipSolid    = gdiplusDLL.NewProc("GdipCreateSolidFill")
	procGdipDelBrush = gdiplusDLL.NewProc("GdipDeleteBrush")
	procGdipFillEll  = gdiplusDLL.NewProc("GdipFillEllipseI")
	procSelectBrush = gdi32.NewProc("SelectObject")
	procMoveTo      = gdi32.NewProc("MoveToEx")
	procLineTo      = gdi32.NewProc("LineTo")
	procSelectClip  = gdi32.NewProc("SelectClipRgn")
	procTextExtent  = gdi32.NewProc("GetTextExtentPoint32W")
	dwmapi          = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSet      = dwmapi.NewProc("DwmSetWindowAttribute")
)

var (
	colorBg          win.COLORREF
	colorCard        win.COLORREF
	colorPrimary     win.COLORREF
	colorPrimarySoft win.COLORREF
	colorText        win.COLORREF
	colorMuted       win.COLORREF
	colorCaption     win.COLORREF
	colorDisabled    win.COLORREF
	colorLine        win.COLORREF
	colorNav         win.COLORREF
	colorNavPick     win.COLORREF
	colorLow         win.COLORREF
	colorMid         win.COLORREF
	colorHigh        win.COLORREF
	colorWhite       win.COLORREF
	colorFooter      win.COLORREF
	colorSuccess     win.COLORREF
	colorFocus       win.COLORREF
	colorOnPrimary   win.COLORREF
	colorAdvBg       win.COLORREF
	colorRecBg       win.COLORREF
	mapColors        []win.COLORREF
	activeTheme      ui.Theme
)

func applyPalette(t ui.Theme) {
	activeTheme = t
	colorBg = win.COLORREF(t.Bg)
	colorCard = win.COLORREF(t.Card)
	colorPrimary = win.COLORREF(t.Primary)
	colorPrimarySoft = win.COLORREF(t.PrimarySoft)
	colorText = win.COLORREF(t.Text)
	colorMuted = win.COLORREF(t.Muted)
	colorCaption = win.COLORREF(t.Caption)
	colorDisabled = win.COLORREF(t.Disabled)
	colorLine = win.COLORREF(t.Line)
	colorNav = win.COLORREF(t.Nav)
	colorNavPick = win.COLORREF(t.NavPick)
	colorLow = win.COLORREF(t.Success)
	colorMid = win.COLORREF(t.Warning)
	colorHigh = win.COLORREF(t.Danger)
	colorWhite = win.COLORREF(t.Card)
	colorFooter = win.COLORREF(t.Footer)
	colorSuccess = win.COLORREF(t.Success)
	colorFocus = win.COLORREF(t.Focus)
	colorOnPrimary = win.COLORREF(t.OnPrimary)
	colorAdvBg = win.COLORREF(t.AdvancedBg)
	colorRecBg = win.COLORREF(t.RecommendBg)
	mapColors = []win.COLORREF{win.COLORREF(t.MapA), win.COLORREF(t.MapB), win.COLORREF(t.MapC), win.COLORREF(t.MapD)}
}

func init() {
	applyPalette(ui.Light())
}

func col(c win.COLORREF) win.COLORREF { return c }

func roundFill(hdc win.HDC, rc win.RECT, radius int, fillColor, border win.COLORREF) {
	if rc.Right <= rc.Left || rc.Bottom <= rc.Top {
		return
	}
	brush := solidBrush(fillColor)
	pen := createPen(border)
	oldB, _, _ := procSelectBrush.Call(uintptr(hdc), uintptr(brush))
	oldP := win.SelectObject(hdc, win.HGDIOBJ(pen))
	procRoundRect.Call(uintptr(hdc), uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right), uintptr(rc.Bottom), uintptr(radius), uintptr(radius))
	win.SelectObject(hdc, oldP)
	procSelectBrush.Call(uintptr(hdc), oldB)
	win.DeleteObject(win.HGDIOBJ(brush))
	win.DeleteObject(win.HGDIOBJ(pen))
}

func createPen(color win.COLORREF) win.HPEN {
	p, _, _ := procCreatePen.Call(0, 1, uintptr(color))
	return win.HPEN(p)
}

func hline(hdc win.HDC, x1, x2, y int, color win.COLORREF) {
	pen := createPen(color)
	old := win.SelectObject(hdc, win.HGDIOBJ(pen))
	procMoveTo.Call(uintptr(hdc), uintptr(x1), uintptr(y), 0)
	procLineTo.Call(uintptr(hdc), uintptr(x2), uintptr(y))
	win.SelectObject(hdc, old)
	win.DeleteObject(win.HGDIOBJ(pen))
}

func vline(hdc win.HDC, x, y1, y2 int, color win.COLORREF) {
	pen := createPen(color)
	old := win.SelectObject(hdc, win.HGDIOBJ(pen))
	procMoveTo.Call(uintptr(hdc), uintptr(x), uintptr(y1), 0)
	procLineTo.Call(uintptr(hdc), uintptr(x), uintptr(y2))
	win.SelectObject(hdc, old)
	win.DeleteObject(win.HGDIOBJ(pen))
}

func shapeButton(hwnd win.HWND, round bool, w, h int32) {
	if hwnd == 0 {
		return
	}
	if !round || w < 4 || h < 4 {
		procSetWindowRgn.Call(uintptr(hwnd), 0, 0)
		return
	}
	rgn, _, _ := procEllipticRgn.Call(0, 0, uintptr(w+1), uintptr(h+1))
	procSetWindowRgn.Call(uintptr(hwnd), rgn, 0)
}

type gdipStartupInput struct {
	version           uint32
	_                 uint32
	debug             uintptr
	suppressBackground int32
	suppressCodecs    int32
}

var gdipToken uintptr

func ensureGdiplus() bool {
	if gdipToken != 0 {
		return true
	}
	in := gdipStartupInput{version: 1}
	st, _, _ := procGdipStartup.Call(uintptr(unsafe.Pointer(&gdipToken)), uintptr(unsafe.Pointer(&in)), 0)
	return st == 0 && gdipToken != 0
}

func gdipARGB(c win.COLORREF) uintptr {
	r := uint32(c) & 0xFF
	g := (uint32(c) >> 8) & 0xFF
	b := (uint32(c) >> 16) & 0xFF
	return uintptr(0xFF000000 | r<<16 | g<<8 | b)
}

func fillSmoothEllipse(hdc win.HDC, rc win.RECT, color win.COLORREF) {
	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top
	if w < 2 || h < 2 || !ensureGdiplus() {
		fillEllipse(hdc, rc, color)
		return
	}
	var g uintptr
	if st, _, _ := procGdipFromHDC.Call(uintptr(hdc), uintptr(unsafe.Pointer(&g))); st != 0 || g == 0 {
		fillEllipse(hdc, rc, color)
		return
	}
	defer procGdipDelGraph.Call(g)
	procGdipSmooth.Call(g, 4)
	var brush uintptr
	if st, _, _ := procGdipSolid.Call(gdipARGB(color), uintptr(unsafe.Pointer(&brush))); st != 0 || brush == 0 {
		fillEllipse(hdc, rc, color)
		return
	}
	defer procGdipDelBrush.Call(brush)
	procGdipFillEll.Call(g, brush, uintptr(rc.Left), uintptr(rc.Top), uintptr(w), uintptr(h))
}

func fillEllipse(hdc win.HDC, rc win.RECT, color win.COLORREF) {
	brush := solidBrush(color)
	pen := createPen(color)
	oldB, _, _ := procSelectBrush.Call(uintptr(hdc), uintptr(brush))
	oldP := win.SelectObject(hdc, win.HGDIOBJ(pen))
	procEllipse.Call(uintptr(hdc), uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right), uintptr(rc.Bottom))
	win.SelectObject(hdc, oldP)
	procSelectBrush.Call(uintptr(hdc), oldB)
	win.DeleteObject(win.HGDIOBJ(brush))
	win.DeleteObject(win.HGDIOBJ(pen))
}

func paintDonut(hdc win.HDC, rc win.RECT, ratio float64, track, used, hole win.COLORREF) {
	fillEllipse(hdc, rc, track)
	if ratio > 0 {
		if ratio > 1 {
			ratio = 1
		}
		cx := float64(rc.Left+rc.Right) / 2
		cy := float64(rc.Top+rc.Bottom) / 2
		rx := float64(rc.Right-rc.Left) / 2
		ry := float64(rc.Bottom-rc.Top) / 2
		brush := solidBrush(used)
		pen := createPen(used)
		oldB, _, _ := procSelectBrush.Call(uintptr(hdc), uintptr(brush))
		oldP := win.SelectObject(hdc, win.HGDIOBJ(pen))
		x1 := int(cx)
		y1 := int(rc.Top)
		angle := -math.Pi/2 + 2*math.Pi*ratio
		x2 := int(cx + rx*math.Cos(angle))
		y2 := int(cy + ry*math.Sin(angle))
		if ratio >= 0.999 {
			procEllipse.Call(uintptr(hdc), uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right), uintptr(rc.Bottom))
		} else {
			procPie.Call(uintptr(hdc), uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right), uintptr(rc.Bottom), uintptr(x1), uintptr(y1), uintptr(x2), uintptr(y2))
		}
		win.SelectObject(hdc, oldP)
		procSelectBrush.Call(uintptr(hdc), oldB)
		win.DeleteObject(win.HGDIOBJ(brush))
		win.DeleteObject(win.HGDIOBJ(pen))
	}
	pad := (rc.Right - rc.Left) / 4
	inner := win.RECT{Left: rc.Left + pad, Top: rc.Top + pad, Right: rc.Right - pad, Bottom: rc.Bottom - pad}
	fillEllipse(hdc, inner, hole)
}

func paintCheck(hdc win.HDC, rc win.RECT, on bool, radius int) {
	fill := colorWhite
	border := colorLine
	if on {
		fill = colorPrimary
		border = colorPrimary
	}
	roundFill(hdc, rc, radius, fill, border)
	if !on {
		return
	}
	win.SetTextColor(hdc, colorOnPrimary)
	draw(hdc, "✓", rc, win.DT_CENTER|dtSingleLine|dtVCenter|dtNoPrefix)
}

func paintPrimaryBtn(hdc win.HDC, rc win.RECT, label string, enabled bool) {
	if uiPtr != nil {
		uiPtr.useType(hdc, ui.TypeButton)
	}
	fill := colorPrimary
	fg := colorOnPrimary
	if analyzeHot() && enabled {
		fill = rgb(16, 110, 196)
	}
	if !enabled {
		fill = colorLine
		fg = colorDisabled
	}
	roundFill(hdc, rc, 4, fill, fill)
	win.SetTextColor(hdc, fg)
	draw(hdc, label, rc, win.DT_CENTER|dtSingleLine|dtVCenter|dtNoPrefix)
}

func paintGhostBtn(hdc win.HDC, rc win.RECT, label string, enabled bool) {
	if uiPtr != nil {
		uiPtr.useType(hdc, ui.TypeButton)
	}
	fg := colorText
	if !enabled {
		fg = colorDisabled
	}
	fill := colorCard
	border := colorLine
	if analyzeHot() && enabled {
		fill = colorPrimarySoft
		border = colorPrimary
	}
	roundFill(hdc, rc, 4, fill, border)
	win.SetTextColor(hdc, fg)
	draw(hdc, label, rc, win.DT_CENTER|dtSingleLine|dtVCenter|dtNoPrefix)
}

func paintFocus(hdc win.HDC, rc win.RECT) {
	hline(hdc, int(rc.Left), int(rc.Right), int(rc.Top), colorFocus)
	hline(hdc, int(rc.Left), int(rc.Right), int(rc.Bottom-1), colorFocus)
	pen := createPen(colorFocus)
	old := win.SelectObject(hdc, win.HGDIOBJ(pen))
	procMoveTo.Call(uintptr(hdc), uintptr(rc.Left), uintptr(rc.Top), 0)
	procLineTo.Call(uintptr(hdc), uintptr(rc.Left), uintptr(rc.Bottom))
	procMoveTo.Call(uintptr(hdc), uintptr(rc.Right-1), uintptr(rc.Top), 0)
	procLineTo.Call(uintptr(hdc), uintptr(rc.Right-1), uintptr(rc.Bottom))
	win.SelectObject(hdc, old)
	win.DeleteObject(win.HGDIOBJ(pen))
}

func paintBadge(hdc win.HDC, rc win.RECT, label string, fg, bg win.COLORREF) {
	if uiPtr != nil {
		uiPtr.useType(hdc, ui.TypeBadge)
	}
	roundFill(hdc, rc, 6, bg, bg)
	win.SetTextColor(hdc, fg)
	draw(hdc, label, rc, win.DT_CENTER|dtSingleLine|dtVCenter|dtNoPrefix)
}

func paintBar(hdc win.HDC, rc win.RECT, ratio float64, fill, track win.COLORREF) {
	roundFill(hdc, rc, 4, track, track)
	if ratio <= 0 {
		return
	}
	if ratio > 1 {
		ratio = 1
	}
	w := int(float64(rc.Right-rc.Left) * ratio)
	if w < 4 {
		w = 4
	}
	inner := rc
	inner.Right = inner.Left + int32(w)
	roundFill(hdc, inner, 4, fill, fill)
}

func paintIcon(hdc win.HDC, rc win.RECT, kind string, color win.COLORREF) {
	pen := createPen(color)
	old := win.SelectObject(hdc, win.HGDIOBJ(pen))
	cx := int(rc.Left+rc.Right) / 2
	cy := int(rc.Top+rc.Bottom) / 2
	s := int(rc.Right-rc.Left) / 3
	stroke := func(x1, y1, x2, y2 int) {
		procMoveTo.Call(uintptr(hdc), uintptr(x1), uintptr(y1), 0)
		procLineTo.Call(uintptr(hdc), uintptr(x2), uintptr(y2))
	}
	box := func(l, t, r, b int) {
		stroke(l, t, r, t)
		stroke(r, t, r, b)
		stroke(r, b, l, b)
		stroke(l, b, l, t)
	}
	switch kind {
	case "home":
		stroke(cx-s, cy, cx, cy-s)
		stroke(cx, cy-s, cx+s, cy)
		stroke(cx-s+2, cy, cx-s+2, cy+s)
		stroke(cx+s-2, cy, cx+s-2, cy+s)
		stroke(cx-s+2, cy+s, cx+s-2, cy+s)
	case "clean":
		box(cx-s, cy-s/2, cx+s, cy+s)
		stroke(cx-s/2, cy, cx+s/2, cy)
	case "history":
		box(cx-s, cy-s, cx+s, cy+s)
		stroke(cx, cy-s, cx, cy)
		stroke(cx, cy, cx+s/2, cy)
	case "disk":
		box(cx-s, cy-s/3, cx+s, cy+s/3)
		stroke(cx-s, cy, cx+s, cy)
	case "files":
		box(cx-s, cy-s, cx+s, cy+s)
		stroke(cx-s/2, cy-s/3, cx+s/2, cy-s/3)
		stroke(cx-s/2, cy, cx+s/2, cy)
	case "dupes":
		box(cx-s, cy-s, cx, cy)
		box(cx, cy, cx+s, cy+s)
	case "apps":
		box(cx-s, cy-s, cx-2, cy-2)
		box(cx+2, cy-s, cx+s, cy-2)
		box(cx-s, cy+2, cx-2, cy+s)
		box(cx+2, cy+2, cx+s, cy+s)
	case "system":
		box(cx-s, cy-s, cx+s, cy+s)
		stroke(cx-s, cy, cx+s, cy)
		stroke(cx, cy-s, cx, cy+s)
	default:
		box(cx-s/2, cy-s/2, cx+s/2, cy+s/2)
	}
	win.SelectObject(hdc, old)
	win.DeleteObject(win.HGDIOBJ(pen))
}

func roundWindow(hwnd win.HWND) {
	pref := uint32(2) // DWMWCP_ROUND
	procDwmSet.Call(uintptr(hwnd), 33, uintptr(unsafe.Pointer(&pref)), 4)
	runtime.KeepAlive(&pref)
}

func textWidth(hdc win.HDC, s string) int {
	if s == "" {
		return 0
	}
	buf, err := windows.UTF16FromString(s)
	if err != nil || len(buf) < 2 {
		return 0
	}
	var sz struct{ cx, cy int32 }
	procTextExtent.Call(uintptr(hdc), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&sz)))
	runtime.KeepAlive(buf)
	return int(sz.cx)
}

func analyzeHot() bool {
	return uiPtr != nil && uiPtr.page == ui.PageAnalyze && paintingBtn != 0 && uiPtr.hotBtn == paintingBtn
}

func fillColor(hdc win.HDC, rc win.RECT, color win.COLORREF) {
	brush := solidBrush(color)
	fill(hdc, rc, brush)
	win.DeleteObject(win.HGDIOBJ(brush))
}

func contains(rc win.RECT, x, y int) bool {
	return x >= int(rc.Left) && x < int(rc.Right) && y >= int(rc.Top) && y < int(rc.Bottom)
}
