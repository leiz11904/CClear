//go:build windows

package main

import (
	"cclear/internal/space"
	"cclear/internal/ui"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"unsafe"
)

const (
	classPrompt = "CClear.SizePrompt"
	idPromptOK  = 201
	idPromptNo  = 202
)

var (
	promptEdit win.HWND
	promptDone bool
	promptOK   bool
	promptText string
	promptProc = windows.NewCallback(promptWndProc)
)

func promptSizeMB(owner win.HWND, dpi int) (int64, bool) {
	inst := win.GetModuleHandle(nil)
	cls := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   promptProc,
		HInstance:     inst,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
		HbrBackground: win.HBRUSH(win.COLOR_WINDOW + 1),
		LpszClassName: u16(classPrompt),
	}
	win.RegisterClassEx(&cls)
	scale := func(px int) int {
		if dpi == 0 {
			return px
		}
		return px * dpi / 96
	}
	w, h := scale(360), scale(160)
	var rc win.RECT
	win.GetWindowRect(owner, &rc)
	x := int(rc.Left) + (int(rc.Right-rc.Left)-w)/2
	y := int(rc.Top) + (int(rc.Bottom-rc.Top)-h)/2
	hwnd := win.CreateWindowEx(win.WS_EX_DLGMODALFRAME, u16(classPrompt), u16("自定义大小（MB）"),
		win.WS_CAPTION|win.WS_SYSMENU|win.WS_POPUP|win.WS_VISIBLE,
		int32(x), int32(y), int32(w), int32(h), owner, 0, inst, nil)
	if hwnd == 0 {
		return 0, false
	}
	hint := win.CreateWindowEx(0, u16("STATIC"), u16("输入正整数，单位 MB。只筛选本次扫描保留的最大文件列表。"),
		win.WS_CHILD|win.WS_VISIBLE, int32(scale(16)), int32(scale(12)), int32(scale(320)), int32(scale(40)), hwnd, 0, inst, nil)
	promptEdit = win.CreateWindowEx(win.WS_EX_CLIENTEDGE, u16("EDIT"), u16(""),
		win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP, int32(scale(16)), int32(scale(56)), int32(scale(320)), int32(scale(32)), hwnd, 0, inst, nil)
	ok := win.CreateWindowEx(0, u16("BUTTON"), u16("确定"),
		win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP|win.BS_DEFPUSHBUTTON, int32(scale(150)), int32(scale(100)), int32(scale(88)), int32(scale(36)), hwnd, win.HMENU(idPromptOK), inst, nil)
	no := win.CreateWindowEx(0, u16("BUTTON"), u16("取消"),
		win.WS_CHILD|win.WS_VISIBLE|win.WS_TABSTOP, int32(scale(248)), int32(scale(100)), int32(scale(88)), int32(scale(36)), hwnd, win.HMENU(idPromptNo), inst, nil)
	if uiPtr != nil {
		uiPtr.applyType(hint, ui.TypeBody)
		uiPtr.applyType(promptEdit, ui.TypeBody)
		uiPtr.applyType(ok, ui.TypeButton)
		uiPtr.applyType(no, ui.TypeButton)
	}
	win.SetFocus(promptEdit)
	win.EnableWindow(owner, false)
	promptDone, promptOK, promptText = false, false, ""
	var msg win.MSG
	for !promptDone {
		ret := win.GetMessage(&msg, 0, 0, 0)
		if ret == 0 || ret == -1 {
			break
		}
		if msg.Message == win.WM_KEYDOWN && msg.WParam == win.VK_RETURN {
			promptAccept(hwnd)
			continue
		}
		if msg.Message == win.WM_KEYDOWN && msg.WParam == win.VK_ESCAPE {
			promptDone = true
			continue
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
	win.EnableWindow(owner, true)
	win.DestroyWindow(hwnd)
	win.SetForegroundWindow(owner)
	if !promptOK {
		return 0, false
	}
	n, err := space.ParseSizeMB(promptText)
	if err != nil {
		win.MessageBox(owner, u16(err.Error()), u16("自定义大小"), win.MB_ICONWARNING)
		return 0, false
	}
	return n, true
}

func promptAccept(hwnd win.HWND) {
	var buf [64]uint16
	user32 := windows.NewLazySystemDLL("user32.dll")
	n, _, _ := user32.NewProc("GetWindowTextW").Call(uintptr(promptEdit), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	promptText = windows.UTF16ToString(buf[:n])
	promptOK = true
	promptDone = true
}

func promptWndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_COMMAND:
		switch win.LOWORD(uint32(wParam)) {
		case idPromptOK:
			promptAccept(hwnd)
			return 0
		case idPromptNo:
			promptDone = true
			return 0
		}
	case win.WM_CLOSE:
		promptDone = true
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}
