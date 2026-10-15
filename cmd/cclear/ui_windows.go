//go:build windows

package main

import (
	"runtime"
	"unsafe"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

const (
	classMain   = "CClearMainWindow"
	classDetail = "CClearDetail"

	lvmDeleteAll      = win.LVM_FIRST + 9
	lvmSetColumnWidth = win.LVM_FIRST + 30

	dtSingleLine  = 0x00000020
	dtVCenter     = 0x00000004
	dtEndEllipsis = 0x00008000
	dtNoPrefix    = 0x00000800
	dtWordBreak   = 0x00000010
)

var (
	user32         = windows.NewLazySystemDLL("user32.dll")
	procFillRect   = user32.NewProc("FillRect")
	procAppendMenu = user32.NewProc("AppendMenuW")
	procIsWindow   = user32.NewProc("IsWindow")
	detailEdit     win.HWND
	detailPrev     win.HWND
	detailNext     win.HWND
	detailProcPtr  = windows.NewCallback(detailProc)
	uiPtr          *app
)

func mainWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	if uiPtr == nil {
		return win.DefWindowProc(win.HWND(hwnd), uint32(msg), wParam, lParam)
	}
	return uiPtr.wndProc(win.HWND(hwnd), uint32(msg), wParam, lParam)
}

func detailProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	window := win.HWND(hwnd)
	switch uint32(msg) {
	case win.WM_SIZE:
		layoutDetail(window)
		return 0
	case win.WM_COMMAND:
		if uiPtr != nil {
			switch uint32(wParam) & 0xffff {
			case idPagePrev:
				uiPtr.turnPage(-1)
				return 0
			case idPageNext:
				uiPtr.turnPage(1)
				return 0
			}
		}
	case win.WM_CLOSE:
		win.ShowWindow(window, win.SW_HIDE)
		return 0
	}
	return win.DefWindowProc(window, uint32(msg), wParam, lParam)
}

func isWindow(hwnd win.HWND) bool {
	r, _, _ := procIsWindow.Call(uintptr(hwnd))
	return r != 0
}

func layoutDetail(hwnd win.HWND) {
	if detailEdit == 0 {
		return
	}
	var rc win.RECT
	win.GetClientRect(hwnd, &rc)
	btnH := int32(28)
	bottom := rc.Bottom - 16
	if detailPrev != 0 {
		bottom = rc.Bottom - 12 - btnH
		win.MoveWindow(detailPrev, 8, rc.Bottom-8-btnH, 88, btnH, true)
		win.MoveWindow(detailNext, 104, rc.Bottom-8-btnH, 88, btnH, true)
	}
	win.MoveWindow(detailEdit, 8, 8, rc.Right-16, bottom-8, true)
}

func solidBrush(color win.COLORREF) win.HBRUSH {
	return win.CreateBrushIndirect(&win.LOGBRUSH{LbStyle: win.BS_SOLID, LbColor: color})
}

func fill(hdc win.HDC, rc win.RECT, brush win.HBRUSH) {
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), uintptr(brush))
}

func draw(hdc win.HDC, text string, rc win.RECT, format uint32) {
	ptr := u16(text)
	win.DrawTextEx(hdc, ptr, -1, &rc, format, nil)
	runtime.KeepAlive(ptr)
}

func appendMenu(menu win.HMENU, flags, id uintptr, text string) {
	var ptr *uint16
	if text != "" {
		ptr = u16(text)
	}
	procAppendMenu.Call(uintptr(menu), flags, id, uintptr(unsafe.Pointer(ptr)))
	runtime.KeepAlive(ptr)
}

func setText(hwnd win.HWND, text string) {
	ptr := u16(text)
	win.SendMessage(hwnd, win.WM_SETTEXT, 0, uintptr(unsafe.Pointer(ptr)))
	runtime.KeepAlive(ptr)
}

func loadAppIcon(inst win.HINSTANCE) win.HICON {
	icon := win.LoadIcon(inst, win.MAKEINTRESOURCE(1))
	if icon == 0 {
		icon = win.LoadIcon(0, win.MAKEINTRESOURCE(win.IDI_APPLICATION))
	}
	return icon
}

func insertItem(list win.HWND, index int, cols ...string) {
	first := u16(cols[0])
	item := win.LVITEM{Mask: win.LVIF_TEXT, IItem: int32(index), PszText: first}
	win.SendMessage(list, win.LVM_INSERTITEM, 0, uintptr(unsafe.Pointer(&item)))
	runtime.KeepAlive(first)
	for i := 1; i < len(cols); i++ {
		text := u16(cols[i])
		sub := win.LVITEM{ISubItem: int32(i), PszText: text}
		win.SendMessage(list, win.LVM_SETITEMTEXT, uintptr(index), uintptr(unsafe.Pointer(&sub)))
		runtime.KeepAlive(text)
	}
}

func setChecked(list win.HWND, index int, on bool) {
	state := uint32(1)
	if on {
		state = 2
	}
	item := win.LVITEM{StateMask: win.LVIS_STATEIMAGEMASK, State: state << 12}
	win.SendMessage(list, win.LVM_SETITEMSTATE, uintptr(index), uintptr(unsafe.Pointer(&item)))
}

func itemChecked(list win.HWND, index int) bool {
	state := win.SendMessage(list, win.LVM_GETITEMSTATE, uintptr(index), win.LVIS_STATEIMAGEMASK)
	return (state&win.LVIS_STATEIMAGEMASK)>>12 == 2
}

func selectedIndex(list win.HWND) int {
	idx := int32(win.SendMessage(list, win.LVM_GETNEXTITEM, ^uintptr(0), win.LVNI_SELECTED))
	if idx < 0 {
		return -1
	}
	return int(idx)
}

func setColumnWidth(list win.HWND, index, width int) {
	win.SendMessage(list, lvmSetColumnWidth, uintptr(index), uintptr(width))
}
