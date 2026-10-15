//go:build windows

package main

import (
	"cclear/internal/ui"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"unsafe"
)

const (
	classAsk   = "CClear.AskDialog"
	idAskYes   = 301
	idAskNo    = 302
	idAskOther = 303
)

var (
	askResult int
	askDone   bool
	askThree  bool
	askProc   = windows.NewCallback(askWndProc)
)

func (a *app) alert(title, body string) {
	a.askDialog(title, body, false, false)
}

func (a *app) askYesNo(title, body string) bool {
	return a.askDialog(title, body, true, false) == win.IDYES
}

func (a *app) askYesNoCancel(title, body string) int {
	return a.askDialog(title, body, true, true)
}

func (a *app) askDialog(title, body string, yesNo, cancel bool) int {
	inst := win.GetModuleHandle(nil)
	cls := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   askProc,
		HInstance:     inst,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
		HbrBackground: a.pageBrush,
		LpszClassName: u16(classAsk),
	}
	win.RegisterClassEx(&cls)
	w, h := a.scale(520), a.scale(400)
	askThree = cancel
	var rc win.RECT
	win.GetWindowRect(a.win, &rc)
	x := int(rc.Left) + (int(rc.Right-rc.Left)-w)/2
	y := int(rc.Top) + (int(rc.Bottom-rc.Top)-h)/2
	hwnd := win.CreateWindowEx(win.WS_EX_DLGMODALFRAME, u16(classAsk), u16(title),
		win.WS_CAPTION|win.WS_SYSMENU|win.WS_POPUP|win.WS_VISIBLE,
		int32(x), int32(y), int32(w), int32(h), a.win, 0, inst, nil)
	if hwnd == 0 {
		if yesNo {
			return int(win.MessageBox(a.win, u16(body), u16(title), win.MB_YESNO|win.MB_ICONWARNING|win.MB_DEFBUTTON2))
		}
		win.MessageBox(a.win, u16(body), u16(title), win.MB_OK|win.MB_ICONWARNING)
		return int(win.IDOK)
	}
	titleH := win.CreateWindowEx(0, u16("STATIC"), u16(title),
		win.WS_CHILD|win.WS_VISIBLE, int32(a.scale(20)), int32(a.scale(16)), int32(a.scale(440)), int32(a.scale(28)), hwnd, 0, inst, nil)
	bodyH := win.CreateWindowEx(0, u16("STATIC"), u16(body),
		win.WS_CHILD|win.WS_VISIBLE, int32(a.scale(20)), int32(a.scale(52)), int32(a.scale(480)), int32(a.scale(240)), hwnd, 0, inst, nil)
	a.applyType(titleH, ui.TypeDialogTitle)
	a.applyType(bodyH, ui.TypeBody)
	btnY := a.scale(312)
	if yesNo {
		yes := win.CreateWindowEx(0, u16("BUTTON"), u16("确定"),
			win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP|win.BS_DEFPUSHBUTTON,
			int32(a.scale(268)), int32(btnY), int32(a.scale(88)), int32(a.scale(36)), hwnd, win.HMENU(idAskYes), inst, nil)
		no := win.CreateWindowEx(0, u16("BUTTON"), u16("取消"),
			win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP,
			int32(a.scale(172)), int32(btnY), int32(a.scale(88)), int32(a.scale(36)), hwnd, win.HMENU(idAskNo), inst, nil)
		a.applyType(yes, ui.TypeButton)
		a.applyType(no, ui.TypeButton)
		if cancel {
			other := win.CreateWindowEx(0, u16("BUTTON"), u16("跳过"),
				win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP,
				int32(a.scale(76)), int32(btnY), int32(a.scale(88)), int32(a.scale(36)), hwnd, win.HMENU(idAskOther), inst, nil)
			a.applyType(other, ui.TypeButton)
		}
	} else {
		ok := win.CreateWindowEx(0, u16("BUTTON"), u16("确定"),
			win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP|win.BS_DEFPUSHBUTTON,
			int32(a.scale(372)), int32(btnY), int32(a.scale(88)), int32(a.scale(36)), hwnd, win.HMENU(idAskYes), inst, nil)
		a.applyType(ok, ui.TypeButton)
	}
	win.EnableWindow(a.win, false)
	askDone, askResult = false, win.IDCANCEL
	var msg win.MSG
	for !askDone {
		ret := win.GetMessage(&msg, 0, 0, 0)
		if ret == 0 || ret == -1 {
			break
		}
		handled, _, _ := user32.NewProc("IsDialogMessageW").Call(uintptr(hwnd), uintptr(unsafe.Pointer(&msg)))
		if handled != 0 {
			continue
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
	win.EnableWindow(a.win, true)
	win.DestroyWindow(hwnd)
	win.SetForegroundWindow(a.win)
	return askResult
}

func askWndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_COMMAND:
		switch win.LOWORD(uint32(wParam)) {
		case idAskYes:
			askResult = win.IDYES
			askDone = true
			return 0
		case idAskNo:
			if askThree {
				askResult = win.IDCANCEL
			} else {
				askResult = win.IDNO
			}
			askDone = true
			return 0
		case idAskOther:
			askResult = win.IDNO
			askDone = true
			return 0
		}
	case win.WM_CTLCOLORSTATIC:
		if uiPtr != nil {
			win.SetTextColor(win.HDC(wParam), colorText)
			win.SetBkColor(win.HDC(wParam), colorBg)
			return uintptr(uiPtr.pageBrush)
		}
	case win.WM_CLOSE:
		askResult = win.IDCANCEL
		askDone = true
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}
