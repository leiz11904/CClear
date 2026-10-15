//go:build windows

package main

import (
	"strings"
	"unsafe"

	"cclear/internal/ui"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

type typekit struct {
	dpi      int
	face     string
	pathFace string
	fonts    [ui.TypePath + 1]win.HFONT
}

func (a *app) fontOf(role ui.TypeRole) win.HFONT {
	if int(role) >= len(a.types.fonts) {
		role = ui.TypeBody
	}
	h := a.types.fonts[role]
	if h == 0 {
		return a.types.fonts[ui.TypeBody]
	}
	return h
}

func (a *app) useType(hdc win.HDC, role ui.TypeRole) {
	win.SelectObject(hdc, win.HGDIOBJ(a.fontOf(role)))
}

func (a *app) typeH(role ui.TypeRole) int {
	return a.scale(ui.LinePx(role))
}

func (a *app) applyType(hwnd win.HWND, role ui.TypeRole) {
	if hwnd == 0 {
		return
	}
	win.SendMessage(hwnd, win.WM_SETFONT, uintptr(a.fontOf(role)), 1)
}

func (tk *typekit) release() {
	seen := map[win.HFONT]bool{}
	for i, h := range tk.fonts {
		if h != 0 && !seen[h] {
			win.DeleteObject(win.HGDIOBJ(h))
			seen[h] = true
		}
		tk.fonts[i] = 0
	}
}

func (a *app) rebuildTypekit() {
	a.types.release()
	a.types.dpi = a.dpi
	a.types.face = resolveFace(ui.FontStack())
	a.types.pathFace = resolveFace(ui.PathFontStack())
	for role := ui.TypePageTitle; role <= ui.TypePath; role++ {
		spec := ui.TypeSpecOf(role)
		face := a.types.face
		if spec.Mono {
			face = a.types.pathFace
		}
		a.types.fonts[role] = createRoleFont(face, spec, a.dpi)
	}
	a.titleFont = a.fontOf(ui.TypePageTitle)
	a.subtitleFont = a.fontOf(ui.TypeSection)
	a.font = a.fontOf(ui.TypeBody)
	a.smallFont = a.fontOf(ui.TypeCaption)
}

func createRoleFont(face string, spec ui.TypeSpec, dpi int) win.HFONT {
	if dpi <= 0 {
		dpi = 96
	}
	lf := win.LOGFONT{
		LfHeight:         -int32(spec.Px * dpi / 96),
		LfWeight:         int32(spec.Weight),
		LfCharSet:        win.DEFAULT_CHARSET,
		LfOutPrecision:   4, // OUT_TT_PRECIS
		LfQuality:        win.CLEARTYPE_QUALITY,
		LfPitchAndFamily: win.VARIABLE_PITCH | win.FF_SWISS,
	}
	if spec.Mono {
		lf.LfPitchAndFamily = win.FIXED_PITCH | win.FF_MODERN
	}
	name, err := windows.UTF16FromString(face)
	if err == nil {
		copy(lf.LfFaceName[:], name)
	}
	return win.CreateFontIndirect(&lf)
}

func resolveFace(stack []string) string {
	hdc := win.CreateCompatibleDC(0)
	if hdc == 0 {
		return stack[0]
	}
	defer win.DeleteDC(hdc)
	for _, face := range stack {
		h := createRoleFont(face, ui.TypeSpec{Px: 16, Weight: ui.WeightRegular}, 96)
		if h == 0 {
			continue
		}
		old := win.SelectObject(hdc, win.HGDIOBJ(h))
		var buf [64]uint16
		n, _, _ := gdi32.NewProc("GetTextFaceW").Call(uintptr(hdc), 64, uintptr(unsafe.Pointer(&buf[0])))
		win.SelectObject(hdc, old)
		win.DeleteObject(win.HGDIOBJ(h))
		if n == 0 {
			continue
		}
		got := windows.UTF16ToString(buf[:])
		if faceMatches(got, face) {
			return face
		}
	}
	return stack[len(stack)-1]
}

func faceMatches(got, want string) bool {
	g := strings.ToLower(strings.TrimSpace(got))
	w := strings.ToLower(want)
	return g == w || strings.Contains(g, w) || strings.Contains(w, g)
}
