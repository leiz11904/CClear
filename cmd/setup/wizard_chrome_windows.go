//go:build windows

package main

import (
	"unsafe"

	"runtime"

	uitheme "cclear/internal/ui"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	odsSelected = 0x0001
	odsDisabled = 0x0004
	odsFocus    = 0x0010

	dlgcDefPushButton = 0x0010
	dlgcButton        = 0x2000

	emSetMargins  = 0x00D3
	ecLeftMargin  = 0x0001
	ecRightMargin = 0x0002

	tmeLeave = 0x00000002

	colorWindow        = 5
	colorWindowText    = 8
	colorHighlight     = 13
	colorHighlightText = 14
	colorBtnFace       = 15
	colorGrayText      = 17
	colorBtnText       = 18

	spiGetHighContrast = 0x0042
	hcfHighContrastOn  = 0x0001

	dwmwaUseImmersiveDarkMode = 20
)

type drawItem struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	_          uint32
	HwndItem   win.HWND
	HDC        win.HDC
	RcItem     win.RECT
	ItemData   uintptr
}

type trackMouse struct {
	CbSize    uint32
	Flags     uint32
	HwndTrack win.HWND
	HoverTime uint32
}

type wizButton struct {
	primary bool
	hover   bool
	orig    uintptr
}

type highContrastInfo struct {
	CbSize uint32
	Flags  uint32
	Scheme uintptr
}

var (
	wizButtons    = map[win.HWND]*wizButton{}
	wizButtonProc = windows.NewCallback(wizardButtonProc)
	gdi32         = windows.NewLazySystemDLL("gdi32.dll")
	procRoundRect = gdi32.NewProc("RoundRect")
	procCreatePen = gdi32.NewProc("CreatePen")
)

func (s *setup) loadChrome() {
	s.highContrast = highContrastOn()
	if s.highContrast {
		s.theme = uitheme.Theme{
			Name:      "contrast",
			Bg:        sysColor(colorWindow),
			Card:      sysColor(colorWindow),
			Primary:   sysColor(colorHighlight),
			Text:      sysColor(colorWindowText),
			Muted:     sysColor(colorBtnText),
			Caption:   sysColor(colorBtnText),
			Disabled:  sysColor(colorGrayText),
			Line:      sysColor(colorBtnText),
			Footer:    sysColor(colorBtnFace),
			Danger:    sysColor(colorWindowText),
			OnPrimary: sysColor(colorHighlightText),
			Focus:     sysColor(colorHighlight),
		}
		s.theme.PrimarySoft = s.theme.Bg
		return
	}
	if appsUseLightTheme() {
		s.theme = uitheme.Light()
		return
	}
	s.theme = uitheme.Dark()
}

func (s *setup) applyChrome() {
	if s.pageBrush != 0 {
		win.DeleteObject(win.HGDIOBJ(s.pageBrush))
	}
	if s.cardBrush != 0 {
		win.DeleteObject(win.HGDIOBJ(s.cardBrush))
	}
	if s.footerBrush != 0 {
		win.DeleteObject(win.HGDIOBJ(s.footerBrush))
	}
	s.pageBrush = brush(win.COLORREF(s.theme.Bg))
	s.cardBrush = brush(win.COLORREF(s.theme.Card))
	s.footerBrush = brush(win.COLORREF(s.theme.Footer))
	if s.font != 0 {
		win.DeleteObject(win.HGDIOBJ(s.font))
	}
	if s.title != 0 {
		win.DeleteObject(win.HGDIOBJ(s.title))
	}
	if s.caption != 0 {
		win.DeleteObject(win.HGDIOBJ(s.caption))
	}
	s.font = uiFontPx(s.dpi, uitheme.BodyPx, win.FW_NORMAL)
	s.title = uiFontPx(s.dpi, uitheme.TitlePx, 600)
	s.caption = uiFontPx(s.dpi, uitheme.CaptionPx, win.FW_NORMAL)
	for _, hwnd := range []win.HWND{s.body, s.license, s.dir, s.launch} {
		if hwnd != 0 {
			win.SendMessage(hwnd, win.WM_SETFONT, uintptr(s.font), 1)
		}
	}
	dark := int32(0)
	if s.theme.Name == "dark" {
		dark = 1
		for _, hwnd := range []win.HWND{s.license, s.dir, s.launch} {
			setWindowTheme(hwnd, "DarkMode_Explorer")
		}
	}
	windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute").Call(
		uintptr(s.hwnd),
		dwmwaUseImmersiveDarkMode,
		uintptr(unsafe.Pointer(&dark)),
		unsafe.Sizeof(dark),
	)
}

func (s *setup) ownButton(hwnd win.HWND, primary bool) {
	if hwnd == 0 {
		return
	}
	orig := win.SetWindowLongPtr(hwnd, win.GWLP_WNDPROC, wizButtonProc)
	wizButtons[hwnd] = &wizButton{primary: primary, orig: orig}
}

func wizardButtonProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	btn := wizButtons[win.HWND(hwnd)]
	if btn == nil {
		return win.DefWindowProc(win.HWND(hwnd), uint32(msg), wParam, lParam)
	}
	switch uint32(msg) {
	case win.WM_MOUSEMOVE:
		if !btn.hover {
			btn.hover = true
			win.InvalidateRect(win.HWND(hwnd), nil, false)
			ev := trackMouse{CbSize: uint32(unsafe.Sizeof(trackMouse{})), Flags: tmeLeave, HwndTrack: win.HWND(hwnd)}
			windows.NewLazySystemDLL("user32.dll").NewProc("TrackMouseEvent").Call(uintptr(unsafe.Pointer(&ev)))
		}
	case win.WM_MOUSELEAVE:
		btn.hover = false
		win.InvalidateRect(win.HWND(hwnd), nil, false)
	case win.WM_GETDLGCODE:
		code := win.CallWindowProc(btn.orig, win.HWND(hwnd), uint32(msg), wParam, lParam)
		if btn.primary && win.IsWindowEnabled(win.HWND(hwnd)) {
			code |= dlgcButton | dlgcDefPushButton
		}
		return code
	}
	return win.CallWindowProc(btn.orig, win.HWND(hwnd), uint32(msg), wParam, lParam)
}

func (s *setup) paintButton(di *drawItem) {
	btn := wizButtons[di.HwndItem]
	disabled := di.ItemState&odsDisabled != 0
	pressed := di.ItemState&odsSelected != 0
	focused := di.ItemState&odsFocus != 0
	primary := btn != nil && btn.primary && !disabled
	hover := btn != nil && btn.hover && !disabled && !pressed
	back := win.COLORREF(s.theme.Footer)
	if di.CtlID == idBrowse {
		back = win.COLORREF(s.theme.Bg)
	}
	plate := solidBrush(back)
	fillRect(di.HDC, di.RcItem, plate)
	win.DeleteObject(win.HGDIOBJ(plate))

	fill := s.theme.Card
	border := s.theme.Line
	text := s.theme.Text
	switch {
	case disabled:
		fill = s.theme.Bg
		text = s.theme.Disabled
	case primary && pressed:
		fill = shade(s.theme.Primary, -18)
		border = fill
		text = s.theme.OnPrimary
	case primary && hover:
		fill = shade(s.theme.Primary, 16)
		border = fill
		text = s.theme.OnPrimary
	case primary:
		fill = s.theme.Primary
		border = s.theme.Primary
		text = s.theme.OnPrimary
	case pressed || hover:
		fill = s.theme.PrimarySoft
		border = s.theme.Line
	}
	if s.highContrast && primary {
		fill = s.theme.Primary
		border = s.theme.Primary
		text = s.theme.OnPrimary
	}
	roundFill(di.HDC, di.RcItem, s.px(4), win.COLORREF(fill), win.COLORREF(border))
	if focused && !disabled {
		inset := di.RcItem
		inset.Left += int32(s.px(3))
		inset.Top += int32(s.px(3))
		inset.Right -= int32(s.px(3))
		inset.Bottom -= int32(s.px(3))
		roundFill(di.HDC, inset, s.px(2), win.COLORREF(fill), win.COLORREF(s.theme.Focus))
	}
	win.SetBkMode(di.HDC, win.TRANSPARENT)
	win.SetTextColor(di.HDC, win.COLORREF(text))
	old := win.SelectObject(di.HDC, win.HGDIOBJ(s.font))
	drawText(di.HDC, windowText(di.HwndItem), di.RcItem, dtSingleLine|dtNoPrefix|dtCenter|dtVCenter)
	win.SelectObject(di.HDC, old)
}

func (s *setup) paintChrome(hdc win.HDC, client win.RECT) {
	fillRect(hdc, client, s.pageBrush)
	w := int(client.Right)
	h := int(client.Bottom)
	m := s.px(32)
	win.SetBkMode(hdc, win.TRANSPARENT)

	win.SetTextColor(hdc, win.COLORREF(s.theme.Caption))
	old := win.SelectObject(hdc, win.HGDIOBJ(s.caption))
	drawText(hdc, pageStep(s.page), box(m, s.px(28), w-m, s.px(46)), dtSingleLine|dtNoPrefix)
	win.SelectObject(hdc, win.HGDIOBJ(s.title))
	win.SetTextColor(hdc, win.COLORREF(s.theme.Text))
	drawText(hdc, pageTitle(s.page), box(m, s.px(48), w-m, s.px(80)), dtSingleLine|dtNoPrefix)
	win.SelectObject(hdc, win.HGDIOBJ(s.font))
	win.SetTextColor(hdc, win.COLORREF(s.theme.Muted))
	drawText(hdc, pageSub(s.page), box(m, s.px(84), w-m, s.px(108)), dtSingleLine|dtNoPrefix|dtEndEllipsis)
	win.SelectObject(hdc, old)

	if s.page == pageLicense || s.page == pageDir {
		var field win.RECT
		if s.page == pageLicense {
			win.GetWindowRect(s.license, &field)
		} else {
			win.GetWindowRect(s.dir, &field)
		}
		windows.NewLazySystemDLL("user32.dll").NewProc("MapWindowPoints").Call(0, uintptr(s.hwnd), uintptr(unsafe.Pointer(&field)), 2)
		field.Left -= int32(s.px(1))
		field.Top -= int32(s.px(1))
		field.Right += int32(s.px(1))
		field.Bottom += int32(s.px(1))
		roundFill(hdc, field, s.px(4), win.COLORREF(s.theme.Card), win.COLORREF(s.theme.Line))
	}

	footerTop := h - s.px(uitheme.FooterH)
	if s.noticeBody != "" {
		bar := box(m, footerTop-s.px(8)-s.noticeHeight(), w-m, footerTop-s.px(8))
		roundFill(hdc, bar, s.px(4), win.COLORREF(s.theme.Card), win.COLORREF(s.theme.Line))
		accent := bar
		accent.Left += int32(s.px(8))
		accent.Top += int32(s.px(10))
		accent.Bottom -= int32(s.px(10))
		accent.Right = accent.Left + int32(s.px(3))
		fillRect(hdc, accent, brushKeep(win.COLORREF(s.theme.Danger)))
		win.SetTextColor(hdc, win.COLORREF(s.theme.Text))
		old = win.SelectObject(hdc, win.HGDIOBJ(s.caption))
		textLeft := int(bar.Left) + s.px(16)
		textRight := int(bar.Right) - s.px(12)
		drawText(hdc, s.noticeTitle, box(textLeft, int(bar.Top)+s.px(8), textRight, int(bar.Top)+s.px(26)), dtSingleLine|dtNoPrefix|dtEndEllipsis)
		win.SelectObject(hdc, win.HGDIOBJ(s.font))
		win.SetTextColor(hdc, win.COLORREF(s.theme.Muted))
		drawText(hdc, s.noticeBody, box(textLeft, int(bar.Top)+s.px(28), textRight, int(bar.Bottom)-s.px(8)), dtNoPrefix|dtWordBreak|dtEndEllipsis)
		win.SelectObject(hdc, old)
	}
	fillRect(hdc, box(0, footerTop, w, h), s.footerBrush)
	hline(hdc, 0, w, footerTop, win.COLORREF(s.theme.Line))
}

func (s *setup) noticeHeight() int {
	if s.noticeBody == "" {
		return 0
	}
	return s.px(64)
}

func (s *setup) setNotice(title, body string) {
	s.noticeTitle = title
	s.noticeBody = body
}

func (s *setup) clearNotice() {
	s.noticeTitle = ""
	s.noticeBody = ""
}

func (s *setup) placeButtons(w, h int) {
	m := s.px(32)
	gap := s.px(uitheme.Space)
	btnH := s.px(32)
	btnW := s.px(96)
	footerTop := h - s.px(uitheme.FooterH)
	y := footerTop + (s.px(uitheme.FooterH)-btnH)/2
	right := w - m
	var shown []win.HWND
	for _, hwnd := range []win.HWND{s.next, s.back, s.cancel} {
		if hwnd != 0 && win.IsWindowVisible(hwnd) {
			shown = append(shown, hwnd)
		}
	}
	for _, hwnd := range shown {
		right -= btnW
		win.MoveWindow(hwnd, int32(right), int32(y), int32(btnW), int32(btnH), true)
		right -= gap
	}
}

func shade(c uitheme.Color, delta int) uitheme.Color {
	shift := func(v int) byte {
		v += delta
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return byte(v)
	}
	return uitheme.RGB(shift(int(c&0xFF)), shift(int((c>>8)&0xFF)), shift(int((c>>16)&0xFF)))
}

func roundFill(hdc win.HDC, rc win.RECT, radius int, fill, border win.COLORREF) {
	if rc.Right <= rc.Left || rc.Bottom <= rc.Top {
		return
	}
	b := solidBrush(fill)
	p, _, _ := procCreatePen.Call(0, 1, uintptr(border))
	oldB := win.SelectObject(hdc, win.HGDIOBJ(b))
	oldP := win.SelectObject(hdc, win.HGDIOBJ(p))
	procRoundRect.Call(uintptr(hdc), uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right), uintptr(rc.Bottom), uintptr(radius), uintptr(radius))
	win.SelectObject(hdc, oldP)
	win.SelectObject(hdc, oldB)
	win.DeleteObject(win.HGDIOBJ(b))
	win.DeleteObject(win.HGDIOBJ(p))
}

func hline(hdc win.HDC, x1, x2, y int, color win.COLORREF) {
	p, _, _ := procCreatePen.Call(0, 1, uintptr(color))
	old := win.SelectObject(hdc, win.HGDIOBJ(p))
	gdi32.NewProc("MoveToEx").Call(uintptr(hdc), uintptr(x1), uintptr(y), 0)
	gdi32.NewProc("LineTo").Call(uintptr(hdc), uintptr(x2), uintptr(y))
	win.SelectObject(hdc, old)
	win.DeleteObject(win.HGDIOBJ(p))
}

func solidBrush(color win.COLORREF) win.HBRUSH {
	return brush(color)
}

var dangerBrush win.HBRUSH
var dangerColor win.COLORREF

func brushKeep(color win.COLORREF) win.HBRUSH {
	if dangerBrush == 0 || dangerColor != color {
		if dangerBrush != 0 {
			win.DeleteObject(win.HGDIOBJ(dangerBrush))
		}
		dangerBrush = solidBrush(color)
		dangerColor = color
	}
	return dangerBrush
}

func uiFontPx(dpi, px int, weight int32) win.HFONT {
	if dpi <= 0 {
		dpi = 96
	}
	lf := win.LOGFONT{
		LfHeight:         -int32(px * dpi / 96),
		LfWeight:         weight,
		LfCharSet:        win.DEFAULT_CHARSET,
		LfOutPrecision:   4,
		LfQuality:        win.CLEARTYPE_QUALITY,
		LfPitchAndFamily: win.VARIABLE_PITCH | win.FF_SWISS,
	}
	name, err := windows.UTF16FromString("Microsoft YaHei UI")
	if err == nil {
		for i := 0; i < len(name) && i < len(lf.LfFaceName); i++ {
			lf.LfFaceName[i] = name[i]
		}
	}
	return win.CreateFontIndirect(&lf)
}

func setEditMargins(hwnd win.HWND, px int) {
	win.SendMessage(hwnd, emSetMargins, ecLeftMargin|ecRightMargin, uintptr(px)<<16|uintptr(px))
}

func setWindowTheme(hwnd win.HWND, theme string) {
	if hwnd == 0 {
		return
	}
	name := utf(theme)
	windows.NewLazySystemDLL("uxtheme.dll").NewProc("SetWindowTheme").Call(uintptr(hwnd), uintptr(unsafe.Pointer(name)), 0)
	runtime.KeepAlive(name)
}

func highContrastOn() bool {
	info := highContrastInfo{CbSize: uint32(unsafe.Sizeof(highContrastInfo{}))}
	r, _, _ := windows.NewLazySystemDLL("user32.dll").NewProc("SystemParametersInfoW").Call(
		spiGetHighContrast,
		uintptr(info.CbSize),
		uintptr(unsafe.Pointer(&info)),
		0,
	)
	return r != 0 && info.Flags&hcfHighContrastOn != 0
}

func appsUseLightTheme() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return true
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return true
	}
	return v != 0
}

func sysColor(index int) uitheme.Color {
	c, _, _ := windows.NewLazySystemDLL("user32.dll").NewProc("GetSysColor").Call(uintptr(index))
	return uitheme.Color(uint32(c))
}

func pageStep(page int) string {
	switch page {
	case pageLicense:
		return "步骤 2 / 4"
	case pageDir:
		return "步骤 3 / 4"
	case pageInstall:
		return "步骤 4 / 4"
	case pageFinish:
		return "已完成"
	default:
		return "步骤 1 / 4"
	}
}
