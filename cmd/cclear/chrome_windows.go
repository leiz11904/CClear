//go:build windows

package main

import (
	"runtime"
	"strings"
	"unsafe"

	"cclear/internal/ui"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

const (
	faceSecondary = iota
	facePrimary
	faceNav
	faceCheck
	faceSelected
	faceMap
	faceRow
	faceRowPick
	faceRank
	faceQuiet
	faceBubble
	actSecondary = faceSecondary
	actPrimary   = facePrimary
	actSelected  = faceSelected
)

type ctrlFace struct {
	kind     int
	caption  string
	icon     string
	bg       win.COLORREF
	fg       win.COLORREF
	edge     win.COLORREF
	detail   string
	tip      string
	selected bool
}

var paintingBtn win.HWND

var (
	btnProcPtr      = windows.NewCallback(btnProc)
	btnOldProc      uintptr
	btnFaces        = map[win.HWND]ctrlFace{}
	uxtheme         = windows.NewLazySystemDLL("uxtheme.dll")
	procSetTheme    = uxtheme.NewProc("SetWindowTheme")
	procDrawFocus   = user32.NewProc("DrawFocusRect")
	procGetSysColor = user32.NewProc("GetSysColor")
	procSysParam    = user32.NewProc("SystemParametersInfoW")
	procTrackMouse  = user32.NewProc("TrackMouseEvent")
)

type trackMouseEvent struct {
	cbSize      uint32
	dwFlags     uint32
	hwndTrack   uintptr
	dwHoverTime uint32
	_           uint32
}

func prepareControl(hwnd win.HWND) {
	if hwnd == 0 {
		return
	}
	blank, _ := windows.UTF16PtrFromString("")
	procSetTheme.Call(uintptr(hwnd), uintptr(unsafe.Pointer(blank)), uintptr(unsafe.Pointer(blank)))
	runtime.KeepAlive(blank)
	cur := win.GetWindowLongPtr(hwnd, win.GWLP_WNDPROC)
	if cur == btnProcPtr {
		return
	}
	if btnOldProc == 0 {
		btnOldProc = cur
	}
	win.SetWindowLongPtr(hwnd, win.GWLP_WNDPROC, btnProcPtr)
}

func setFace(hwnd win.HWND, face ctrlFace) {
	if hwnd == 0 {
		return
	}
	btnFaces[hwnd] = face
	win.InvalidateRect(hwnd, nil, false)
}

func btnProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	window := win.HWND(hwnd)
	switch uint32(msg) {
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_PAINT:
		paintControl(window)
		return 0
	case win.WM_SETFOCUS, win.WM_KILLFOCUS, win.WM_ENABLE:
		ret := callBtnProc(window, uint32(msg), wParam, lParam)
		win.InvalidateRect(window, nil, false)
		return ret
	case win.WM_MOUSEMOVE:
		if uiPtr != nil {
			uiPtr.hoverControl(window, btnFaces[window])
		}
		return callBtnProc(window, uint32(msg), wParam, lParam)
	case win.WM_MOUSELEAVE:
		if uiPtr != nil {
			uiPtr.leaveControl(window)
		}
		return callBtnProc(window, uint32(msg), wParam, lParam)
	case win.WM_MOUSEWHEEL:
		if uiPtr != nil {
			uiPtr.hideTip()
		}
		parent := win.GetParent(window)
		if parent != 0 {
			win.SendMessage(parent, uint32(msg), wParam, lParam)
		}
		return 0
	}
	return callBtnProc(window, uint32(msg), wParam, lParam)
}

func callBtnProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	if btnOldProc == 0 {
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	}
	return win.CallWindowProc(btnOldProc, hwnd, msg, wParam, lParam)
}

func paintControl(hwnd win.HWND) {
	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(hwnd, &ps)
	defer win.EndPaint(hwnd, &ps)
	paintingBtn = hwnd
	defer func() { paintingBtn = 0 }()
	var rc win.RECT
	win.GetClientRect(hwnd, &rc)
	if rc.Right <= rc.Left || rc.Bottom <= rc.Top {
		return
	}
	face := btnFaces[hwnd]
	if face.caption == "" {
		face.caption = windowText(hwnd)
	}
	enabled := win.IsWindowEnabled(hwnd)
	focused := win.GetFocus() == hwnd
	hc := highContrastOn()
	switch face.kind {
	case faceNav:
		paintNavButton(hdc, rc, face, focused, hc)
	case faceCheck:
		paintCheckButton(hdc, rc, hwnd, face, focused, enabled, hc)
	case facePrimary:
		paintFaceButton(hdc, rc, face.caption, true, enabled, focused, hc)
	case faceSelected:
		paintSelectedButton(hdc, rc, face.caption, enabled, focused, hc)
	case faceMap:
		paintMapButton(hdc, rc, face, focused, hc)
	case faceRow, faceRowPick:
		paintRowButton(hdc, rc, face, focused, hc)
	case faceRank:
		paintRankButton(hdc, rc, face, focused, hc)
	default:
		paintFaceButton(hdc, rc, face.caption, false, enabled, focused, hc)
	}
}

func paintNavButton(hdc win.HDC, rc win.RECT, face ctrlFace, focused, hc bool) {
	bg := colorNav
	fg := colorText
	if face.selected {
		bg = colorNavPick
	}
	if hc {
		bg = sysColor(15) // COLOR_BTNFACE
		fg = sysColor(18) // COLOR_BTNTEXT
		if face.selected {
			bg = sysColor(13) // COLOR_HIGHLIGHT
			fg = sysColor(14) // COLOR_HIGHLIGHTTEXT
		}
	}
	fillColor(hdc, rc, colorNav)
	pill := insetRect(rc, 1)
	if hc {
		fillColor(hdc, rc, bg)
	} else {
		roundFill(hdc, pill, 6, bg, bg)
	}
	if face.selected && !hc {
		bar := pill
		bar.Left += 6
		bar.Right = bar.Left + 3
		bar.Top += 8
		bar.Bottom -= 8
		if bar.Bottom < bar.Top {
			bar.Bottom = bar.Top
		}
		roundFill(hdc, bar, 2, colorPrimary, colorPrimary)
	}
	if face.icon != "" && uiPtr != nil {
		glyph := colorText
		if face.selected && !hc {
			glyph = colorPrimary
		}
		if hc {
			glyph = fg
		}
		paintIcon(hdc, rect(int(rc.Left)+16, int(rc.Top)+8, int(rc.Left)+36, int(rc.Bottom)-8), face.icon, glyph)
	}
	if uiPtr != nil {
		uiPtr.useType(hdc, ui.TypeNav)
	}
	win.SetBkMode(hdc, win.TRANSPARENT)
	win.SetTextColor(hdc, fg)
	text := rc
	text.Left += 42
	text.Right -= 10
	draw(hdc, face.caption, text, win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	if focused {
		drawFocus(hdc, rc)
	}
	_ = bg
}

func paintFaceButton(hdc win.HDC, rc win.RECT, label string, primary, enabled, focused, hc bool) {
	if hc {
		bg := sysColor(15)
		fg := sysColor(18)
		if primary && enabled {
			bg = sysColor(13)
			fg = sysColor(14)
		}
		if !enabled {
			fg = sysColor(17)
		}
		fillColor(hdc, rc, bg)
		if uiPtr != nil {
			uiPtr.useType(hdc, ui.TypeButton)
		}
		win.SetBkMode(hdc, win.TRANSPARENT)
		win.SetTextColor(hdc, fg)
		inset := insetRect(rc, 4)
		draw(hdc, label, inset, win.DT_CENTER|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		if focused {
			drawFocus(hdc, rc)
		}
		return
	}
	if primary {
		paintPrimaryBtn(hdc, rc, label, enabled)
	} else {
		paintGhostBtn(hdc, rc, label, enabled)
	}
	if focused {
		drawFocus(hdc, rc)
	}
}

func paintSelectedButton(hdc win.HDC, rc win.RECT, label string, enabled, focused, hc bool) {
	if hc {
		paintFaceButton(hdc, rc, label, true, enabled, focused, true)
		return
	}
	if uiPtr != nil {
		uiPtr.useType(hdc, ui.TypeButton)
	}
	fg := colorText
	if !enabled {
		fg = colorDisabled
	}
	roundFill(hdc, rc, 4, colorPrimarySoft, colorPrimary)
	win.SetBkMode(hdc, win.TRANSPARENT)
	win.SetTextColor(hdc, fg)
	draw(hdc, label, insetRect(rc, 4), win.DT_CENTER|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	if focused {
		drawFocus(hdc, rc)
	}
}

func paintRowButton(hdc win.HDC, rc win.RECT, face ctrlFace, focused, hc bool) {
	bg := colorCard
	fg := colorText
	meta := colorCaption
	picked := face.kind == faceRowPick
	if picked {
		bg = colorPrimarySoft
		fg = colorText
	}
	if hc {
		bg = sysColor(15)
		fg = sysColor(18)
		meta = fg
		if picked {
			bg = sysColor(13)
			fg = sysColor(14)
			meta = fg
		}
	}
	if !hc && !picked && uiPtr != nil && uiPtr.hotBtn == paintingBtn {
		bg = rgb(236, 242, 248)
	}
	fillColor(hdc, rc, bg)
	if picked && !hc {
		bar := rc
		bar.Right = bar.Left + 3
		fillColor(hdc, bar, colorPrimary)
	}
	if uiPtr != nil {
		uiPtr.useType(hdc, ui.TypeItem)
	}
	win.SetBkMode(hdc, win.TRANSPARENT)
	name, size, _ := strings.Cut(face.caption, "\x1f")
	win.SetTextColor(hdc, fg)
	inset := int32(8)
	metaW := int32(108)
	if uiPtr != nil {
		inset = int32(uiPtr.scale(8))
		metaW = int32(uiPtr.scale(108))
	}
	width := rc.Right - rc.Left
	if limit := width * 40 / 100; limit > inset*4 && metaW > limit {
		metaW = limit
	}
	text := rc
	text.Left += inset
	if size != "" {
		text.Right -= metaW
	} else {
		text.Right -= inset
	}
	draw(hdc, name, text, win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	if size != "" {
		if uiPtr != nil {
			uiPtr.useType(hdc, ui.TypeCaption)
		}
		win.SetTextColor(hdc, meta)
		metaRC := rc
		metaRC.Left = rc.Right - metaW
		metaRC.Right -= inset
		draw(hdc, size, metaRC, win.DT_RIGHT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	}
	if focused {
		drawFocus(hdc, rc)
	}
}

func paintRankButton(hdc win.HDC, rc win.RECT, face ctrlFace, focused, hc bool) {
	bg := colorCard
	if uiPtr != nil && uiPtr.hotBtn == paintingBtn && !hc {
		bg = rgb(244, 248, 252)
	}
	if hc {
		bg = sysColor(15)
	}
	fillColor(hdc, rc, bg)
	pad := int32(10)
	if uiPtr != nil {
		pad = int32(uiPtr.scale(10))
		uiPtr.useType(hdc, ui.TypeItem)
	}
	win.SetBkMode(hdc, win.TRANSPARENT)
	fg := colorText
	if hc {
		fg = sysColor(18)
	}
	win.SetTextColor(hdc, fg)
	sizeText, _, _ := strings.Cut(face.detail, "\x1f")
	sizeW := int32(72)
	if uiPtr != nil {
		sizeW = int32(uiPtr.scale(78))
	}
	nameRC := rc
	nameRC.Left += pad
	nameRC.Right -= sizeW + pad
	draw(hdc, face.caption, nameRC, win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	if uiPtr != nil {
		uiPtr.useType(hdc, ui.TypeCaption)
	}
	win.SetTextColor(hdc, colorCaption)
	sizeRC := rc
	sizeRC.Left = rc.Right - sizeW - pad
	sizeRC.Right -= pad
	draw(hdc, sizeText, sizeRC, win.DT_RIGHT|dtSingleLine|dtVCenter|dtNoPrefix)
	if face.edge != 0 {
		dot := int32(8)
		if uiPtr != nil {
			dot = int32(uiPtr.scale(8))
		}
		dy := rc.Top + (rc.Bottom-rc.Top-dot)/2
		fillSmoothEllipse(hdc, win.RECT{Left: rc.Left + pad, Top: dy, Right: rc.Left + pad + dot, Bottom: dy + dot}, face.edge)
	}
	if focused {
		drawFocus(hdc, rc)
	}
}

func paintMapButton(hdc win.HDC, rc win.RECT, face ctrlFace, focused, hc bool) {
	bg := face.bg
	fg := face.fg
	edge := face.edge
	if bg == 0 {
		bg = colorCard
	}
	if fg == 0 {
		fg = colorText
	}
	if edge == 0 {
		edge = colorLine
	}
	if hc {
		bg = sysColor(5)
		fg = sysColor(8)
		edge = sysColor(8)
	}
	hot := uiPtr != nil && uiPtr.hotBtn == paintingBtn
	fillColor(hdc, rc, bg)
	if hot && !hc {
		inset := insetRect(rc, 2)
		hline(hdc, int(inset.Left), int(inset.Right), int(inset.Top), edge)
		hline(hdc, int(inset.Left), int(inset.Right), int(inset.Bottom-1), edge)
		vline(hdc, int(inset.Left), int(inset.Top), int(inset.Bottom), edge)
		vline(hdc, int(inset.Right-1), int(inset.Top), int(inset.Bottom), edge)
	} else if !hc {
		hline(hdc, int(rc.Left), int(rc.Right), int(rc.Top), edge)
		vline(hdc, int(rc.Left), int(rc.Top), int(rc.Bottom), edge)
	}
	pad := int32(8)
	if uiPtr != nil {
		pad = int32(uiPtr.scale(8))
		uiPtr.useType(hdc, ui.TypeItem)
	}
	win.SetBkMode(hdc, win.TRANSPARENT)
	inner := insetRect(rc, pad)
	availW := int(inner.Right - inner.Left)
	availH := int(inner.Bottom - inner.Top)
	lineH := 18
	if uiPtr != nil {
		lineH = uiPtr.typeH(ui.TypeItem)
	}
	name := face.caption
	if hc {
		if name != "" {
			win.SetTextColor(hdc, fg)
			draw(hdc, name, inner, win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
		}
	} else if name != "" && availW >= textWidth(hdc, name) && availH >= lineH {
		win.SetTextColor(hdc, fg)
		nameRC := inner
		showDetail := face.detail != "" && availH >= lineH*2 && availW >= textWidth(hdc, face.detail)
		if showDetail {
			nameRC.Bottom = nameRC.Top + int32(lineH)
		}
		draw(hdc, name, nameRC, win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
		if showDetail {
			if uiPtr != nil {
				uiPtr.useType(hdc, ui.TypeCaption)
			}
			sub := fg
			if fg == colorOnPrimary || fg == colorWhite {
				sub = rgb(214, 230, 245)
			} else {
				sub = colorCaption
			}
			win.SetTextColor(hdc, sub)
			detailRC := inner
			detailRC.Top = nameRC.Bottom
			detailRC.Bottom = detailRC.Top + int32(lineH)
			draw(hdc, face.detail, detailRC, win.DT_LEFT|dtSingleLine|dtVCenter|dtNoPrefix)
		}
	}
	if focused {
		drawFocus(hdc, rc)
	}
}

func paintCheckButton(hdc win.HDC, rc win.RECT, hwnd win.HWND, face ctrlFace, focused, enabled, hc bool) {
	bg := face.bg
	fg := colorText
	boxBorder := colorLine
	boxFill := colorCard
	if bg == 0 {
		bg = colorCard
	}
	if hc {
		bg = sysColor(5) // COLOR_WINDOW
		fg = sysColor(8) // COLOR_WINDOWTEXT
		boxBorder = sysColor(18)
		boxFill = sysColor(5)
	}
	if !enabled {
		fg = colorDisabled
		if hc {
			fg = sysColor(17)
		}
	}
	fillColor(hdc, rc, bg)
	side := int32(16)
	if rc.Bottom-rc.Top < side+4 {
		side = rc.Bottom - rc.Top - 4
	}
	if side < 10 {
		side = 10
	}
	top := rc.Top + (rc.Bottom-rc.Top-side)/2
	box := win.RECT{Left: rc.Left + 2, Top: top, Right: rc.Left + 2 + side, Bottom: top + side}
	on := win.SendMessage(hwnd, bmGetCheck, 0, 0) != 0
	if on && !hc {
		boxFill = colorPrimary
		boxBorder = colorPrimary
	}
	if on && hc {
		boxFill = sysColor(13)
		boxBorder = sysColor(13)
	}
	roundFill(hdc, box, 3, boxFill, boxBorder)
	if on {
		mark := colorOnPrimary
		if hc {
			mark = sysColor(14)
		}
		win.SetTextColor(hdc, mark)
		win.SetBkMode(hdc, win.TRANSPARENT)
		draw(hdc, "✓", box, win.DT_CENTER|dtSingleLine|dtVCenter|dtNoPrefix)
	}
	if uiPtr != nil {
		uiPtr.useType(hdc, ui.TypeItem)
	}
	win.SetBkMode(hdc, win.TRANSPARENT)
	win.SetTextColor(hdc, fg)
	text := rc
	text.Left = box.Right + 8
	text.Right -= 4
	draw(hdc, face.caption, text, win.DT_LEFT|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix)
	if focused {
		drawFocus(hdc, rc)
	}
}

func insetRect(rc win.RECT, n int32) win.RECT {
	rc.Left += n
	rc.Top += n
	rc.Right -= n
	rc.Bottom -= n
	if rc.Right < rc.Left {
		rc.Right = rc.Left
	}
	if rc.Bottom < rc.Top {
		rc.Bottom = rc.Top
	}
	return rc
}

func drawFocus(hdc win.HDC, rc win.RECT) {
	rc = insetRect(rc, 3)
	if highContrastOn() {
		procDrawFocus.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)))
		return
	}
	left, top := int(rc.Left), int(rc.Top)
	right, bottom := int(rc.Right), int(rc.Bottom)
	hline(hdc, left, right, top, colorFocus)
	hline(hdc, left, right, bottom-1, colorFocus)
	vline(hdc, left, top, bottom, colorFocus)
	vline(hdc, right-1, top, bottom, colorFocus)
}

func sysColor(id int) win.COLORREF {
	c, _, _ := procGetSysColor.Call(uintptr(id))
	return win.COLORREF(c)
}

func highContrastOn() bool {
	var info highContrastInfo
	info.cbSize = uint32(unsafe.Sizeof(info))
	r, _, _ := procSysParam.Call(0x0042, uintptr(info.cbSize), uintptr(unsafe.Pointer(&info)), 0)
	if r == 0 {
		return false
	}
	return info.dwFlags&0x1 != 0
}

type highContrastInfo struct {
	cbSize            uint32
	dwFlags           uint32
	lpszDefaultScheme *uint16
}

func (a *app) hoverControl(hwnd win.HWND, face ctrlFace) {
	if a.page != ui.PageAnalyze {
		return
	}
	if a.leaveTracked != hwnd {
		ev := trackMouseEvent{
			cbSize:    uint32(unsafe.Sizeof(trackMouseEvent{})),
			dwFlags:   0x00000002,
			hwndTrack: uintptr(hwnd),
		}
		procTrackMouse.Call(uintptr(unsafe.Pointer(&ev)))
		a.leaveTracked = hwnd
	}
	if a.hotBtn != hwnd {
		prev := a.hotBtn
		a.hotBtn = hwnd
		if prev != 0 {
			win.InvalidateRect(prev, nil, false)
		}
		win.InvalidateRect(hwnd, nil, false)
	}
	if (face.kind == faceMap || face.kind == faceRank) && face.tip != "" {
		a.showTip(hwnd, face.tip)
	} else {
		a.hideTip()
	}
}

func (a *app) leaveControl(hwnd win.HWND) {
	if a.leaveTracked == hwnd {
		a.leaveTracked = 0
	}
	if a.hotBtn == hwnd {
		a.hotBtn = 0
		win.InvalidateRect(hwnd, nil, false)
	}
	if a.tipHot == hwnd {
		a.hideTip()
	}
}

func (a *app) showTip(hwnd win.HWND, text string) {
	if a.tip == 0 || text == "" {
		return
	}
	if a.tipHot == hwnd && win.IsWindowVisible(a.tip) != false {
		return
	}
	lines := strings.Split(text, "\n")
	a.tipHot = hwnd
	a.tipLines = lines
	w := a.scale(320)
	line := a.typeH(ui.TypeCaption)
	h := a.scale(16) + line*len(lines)
	if len(lines) > 1 {
		h += line
	}
	var wr win.RECT
	win.GetWindowRect(hwnd, &wr)
	x := int(wr.Right) + a.scale(10)
	y := int(wr.Top)
	sw := int(win.GetSystemMetrics(win.SM_CXSCREEN))
	sh := int(win.GetSystemMetrics(win.SM_CYSCREEN))
	if x+w > sw-8 {
		x = int(wr.Left) - w - a.scale(10)
	}
	if y+h > sh-8 {
		y = sh - h - 8
	}
	if x < 4 {
		x = 4
	}
	if y < 4 {
		y = 4
	}
	win.SetWindowPos(a.tip, win.HWND(^uintptr(0)), int32(x), int32(y), int32(w), int32(h), 0x0010|0x0040)
	win.ShowWindow(a.tip, 4)
	win.InvalidateRect(a.tip, nil, false)
}

func (a *app) hideTip() {
	a.tipHot = 0
	a.tipLines = nil
	if a.tip != 0 {
		win.ShowWindow(a.tip, win.SW_HIDE)
	}
}

func tipProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch uint32(msg) {
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_PAINT:
		if uiPtr != nil {
			uiPtr.paintTip(win.HWND(hwnd))
		}
		return 0
	case win.WM_MOUSEMOVE:
		if uiPtr != nil {
			uiPtr.hideTip()
		}
		return 0
	}
	return win.DefWindowProc(win.HWND(hwnd), uint32(msg), wParam, lParam)
}

func (a *app) paintTip(hwnd win.HWND) {
	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(hwnd, &ps)
	defer win.EndPaint(hwnd, &ps)
	var rc win.RECT
	win.GetClientRect(hwnd, &rc)
	fill := colorCard
	border := rgb(198, 208, 216)
	if a.dark {
		border = rgb(70, 78, 86)
	}
	fillColor(hdc, rc, fill)
	hline(hdc, int(rc.Left), int(rc.Right), int(rc.Top), border)
	hline(hdc, int(rc.Left), int(rc.Right), int(rc.Bottom-1), border)
	vline(hdc, int(rc.Left), int(rc.Top), int(rc.Bottom), border)
	vline(hdc, int(rc.Right-1), int(rc.Top), int(rc.Bottom), border)
	win.SetBkMode(hdc, win.TRANSPARENT)
	x := int(rc.Left) + a.scale(12)
	y := int(rc.Top) + a.scale(8)
	right := int(rc.Right) - a.scale(12)
	for i, line := range a.tipLines {
		role := ui.TypeCaption
		color := colorMuted
		if i == 0 {
			role = ui.TypeItem
			color = colorText
		}
		a.useType(hdc, role)
		win.SetTextColor(hdc, color)
		h := a.typeH(role)
		extra := h
		if i == 1 {
			extra = h * 2
		}
		draw(hdc, line, rect(x, y, right, y+extra), win.DT_LEFT|dtWordBreak|dtNoPrefix)
		y += extra
	}
}
