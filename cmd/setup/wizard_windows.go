//go:build windows

package main

import (
	"os"
	"runtime"
	"sync"
	"unsafe"

	"cclear/internal/appinfo"
	"cclear/internal/syswin"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

const (
	pageWelcome = iota
	pageLicense
	pageDir
	pageInstall
	pageFinish

	idBack   = 101
	idNext   = 102
	idCancel = 103
	idBrowse = 104
	idLaunch = 105
	wmDone   = win.WM_APP + 8

	dtSingleLine = 0x00000020
	dtWordBreak  = 0x00000010
	dtNoPrefix   = 0x00000800
	bmGetCheck   = 0x00F0
	bmSetCheck   = 0x00F1
	bstChecked   = 1
	wmGetText    = 13
	wmGetTextLen = 14
	bifNewStyle  = 0x0040 | 0x0001
)

type setup struct {
	hwnd        win.HWND
	body        win.HWND
	license     win.HWND
	dir         win.HWND
	browse      win.HWND
	progress    win.HWND
	launch      win.HWND
	back        win.HWND
	next        win.HWND
	cancel      win.HWND
	font        win.HFONT
	title       win.HFONT
	pageBrush   win.HBRUSH
	headBrush   win.HBRUSH
	dpi         int
	page        int
	installing  bool
	installedTo string
}

func main() {
	runtime.LockOSThread()
	_ = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	if !syswin.Windows10OrNewer() {
		win.MessageBox(0, utf("CClear 需要 Windows 10 或更高版本。"), utf(appinfo.Name), win.MB_ICONWARNING)
		return
	}
	if isUninstall(os.Args) {
		runUninstall()
		return
	}
	if isSilent(os.Args) {
		if err := install(defaultDir()); err != nil {
			os.Exit(1)
		}
		return
	}
	if err := runWizard(); err != nil {
		win.MessageBox(0, utf(err.Error()), utf("CClear 安装程序"), win.MB_ICONWARNING)
	}
}

func runUninstall() {
	if !isSilent(os.Args) {
		if win.MessageBox(0, utf("确定要卸载 CClear 吗？这不会删除你的文档。"), utf("卸载 CClear"), win.MB_YESNO|win.MB_ICONWARNING|win.MB_DEFBUTTON2) != win.IDYES {
			return
		}
	}
	if err := uninstall(); err != nil && !isSilent(os.Args) {
		win.MessageBox(0, utf(err.Error()), utf("卸载 CClear"), win.MB_ICONWARNING)
		os.Exit(1)
	}
	if !isSilent(os.Args) {
		win.MessageBox(0, utf("CClear 已卸载。"), utf("卸载 CClear"), win.MB_OK)
	}
}

func runWizard() error {
	ui = &setup{dpi: 96}
	var icc win.INITCOMMONCONTROLSEX
	icc.DwSize = uint32(unsafe.Sizeof(icc))
	icc.DwICC = win.ICC_PROGRESS_CLASS | win.ICC_STANDARD_CLASSES
	win.InitCommonControlsEx(&icc)
	inst := win.GetModuleHandle(nil)
	ui.pageBrush = brush(rgb(244, 247, 246))
	ui.headBrush = brush(rgb(15, 118, 110))
	icon := win.LoadIcon(inst, win.MAKEINTRESOURCE(1))
	class := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		Style:         win.CS_HREDRAW | win.CS_VREDRAW,
		LpfnWndProc:   windows.NewCallback(setupProc),
		HInstance:     inst,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
		HbrBackground: ui.pageBrush,
		LpszClassName: utf("CClearSetup"),
		HIcon:         icon,
		HIconSm:       icon,
	}
	if win.RegisterClassEx(&class) == 0 {
		return errRegister
	}
	ui.hwnd = win.CreateWindowEx(0, utf("CClearSetup"), utf("CClear 安装程序"),
		win.WS_OVERLAPPED|win.WS_CAPTION|win.WS_SYSMENU|win.WS_MINIMIZEBOX|win.WS_CLIPCHILDREN,
		win.CW_USEDEFAULT, win.CW_USEDEFAULT, 760, 520,
		0, 0, inst, nil)
	if ui.hwnd == 0 {
		return errRegister
	}
	if dpi := win.GetDpiForWindow(ui.hwnd); dpi != 0 {
		ui.dpi = int(dpi)
	}
	win.SetWindowPos(ui.hwnd, 0, 0, 0, int32(ui.px(760)), int32(ui.px(520)), win.SWP_NOMOVE|win.SWP_NOZORDER)
	ui.font = uiFont(ui.dpi, 10, win.FW_NORMAL)
	ui.title = uiFont(ui.dpi, 18, win.FW_SEMIBOLD)
	ui.createChildren(inst)
	ui.showPage()
	win.ShowWindow(ui.hwnd, win.SW_SHOW)
	win.UpdateWindow(ui.hwnd)
	var msg win.MSG
	for {
		ret := win.GetMessage(&msg, 0, 0, 0)
		if ret == 0 || ret == -1 {
			break
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
	return nil
}

var (
	ui          *setup
	errRegister = osError("无法创建安装窗口")
)

func setupProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	if ui == nil {
		return win.DefWindowProc(win.HWND(hwnd), uint32(msg), wParam, lParam)
	}
	return ui.proc(win.HWND(hwnd), uint32(msg), wParam, lParam)
}

func (s *setup) proc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_PAINT:
		s.paint()
		return 0
	case win.WM_SIZE:
		s.layout()
		return 0
	case win.WM_COMMAND:
		s.onCommand(uint32(wParam) & 0xFFFF)
		return 0
	case win.WM_CTLCOLORSTATIC:
		win.SetBkMode(win.HDC(wParam), win.TRANSPARENT)
		win.SetTextColor(win.HDC(wParam), rgb(51, 65, 85))
		return uintptr(s.pageBrush)
	case wmDone:
		s.installFinished()
		return 0
	case win.WM_CLOSE:
		if s.installing {
			return 0
		}
		if s.page != pageFinish {
			if win.MessageBox(hwnd, utf("确定要取消安装吗？"), utf("CClear 安装程序"), win.MB_YESNO|win.MB_ICONWARNING|win.MB_DEFBUTTON2) != win.IDYES {
				return 0
			}
		}
		win.DestroyWindow(hwnd)
		return 0
	case win.WM_DESTROY:
		win.PostQuitMessage(0)
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func (s *setup) createChildren(inst win.HINSTANCE) {
	s.body = child("STATIC", "", win.WS_CHILD|win.SS_LEFT|win.SS_NOPREFIX, 0)
	s.license = child("EDIT", licenseText, win.WS_CHILD|win.WS_VSCROLL|win.ES_MULTILINE|win.ES_READONLY|win.ES_AUTOVSCROLL|win.WS_BORDER, 0)
	s.dir = child("EDIT", defaultDir(), win.WS_CHILD|win.WS_TABSTOP|win.ES_AUTOHSCROLL|win.WS_BORDER, 0)
	s.browse = child("BUTTON", "浏览...", win.WS_CHILD|win.WS_TABSTOP, idBrowse)
	s.progress = child("msctls_progress32", "", win.WS_CHILD|win.PBS_MARQUEE, 0)
	win.SendMessage(s.progress, win.PBM_SETRANGE32, 0, 100)
	s.launch = child("BUTTON", "启动 CClear", win.WS_CHILD|win.WS_TABSTOP|win.BS_AUTOCHECKBOX, idLaunch)
	win.SendMessage(s.launch, bmSetCheck, bstChecked, 0)
	s.back = child("BUTTON", "上一步", win.WS_CHILD|win.WS_TABSTOP, idBack)
	s.next = child("BUTTON", "下一步", win.WS_CHILD|win.WS_TABSTOP|win.BS_DEFPUSHBUTTON, idNext)
	s.cancel = child("BUTTON", "取消", win.WS_CHILD|win.WS_TABSTOP, idCancel)
	for _, hwnd := range []win.HWND{s.body, s.license, s.dir, s.browse, s.launch, s.back, s.next, s.cancel} {
		win.SendMessage(hwnd, win.WM_SETFONT, uintptr(s.font), 1)
	}
	_ = inst
}

func child(class, text string, style uint32, id uintptr) win.HWND {
	return win.CreateWindowEx(0, utf(class), utf(text), style|win.WS_CLIPSIBLINGS,
		0, 0, 10, 10, ui.hwnd, win.HMENU(id), win.GetModuleHandle(nil), nil)
}

func (s *setup) onCommand(id uint32) {
	switch id {
	case idCancel:
		win.SendMessage(s.hwnd, win.WM_CLOSE, 0, 0)
	case idBack:
		if s.page > pageWelcome && s.page < pageInstall {
			s.page--
			s.showPage()
		}
	case idNext:
		s.nextPage()
	case idBrowse:
		s.pickDir()
	}
}

func (s *setup) nextPage() {
	switch s.page {
	case pageWelcome, pageLicense:
		s.page++
		s.showPage()
	case pageDir:
		dir := windowText(s.dir)
		if _, err := validInstallDir(dir); err != nil {
			win.MessageBox(s.hwnd, utf(err.Error()), utf("CClear 安装程序"), win.MB_ICONWARNING)
			return
		}
		s.installedTo = dir
		s.page = pageInstall
		s.installing = true
		s.showPage()
		win.SendMessage(s.progress, win.PBM_SETMARQUEE, 1, 30)
		go func(target string) {
			err := install(target)
			installMu.Lock()
			installResult = err
			installMu.Unlock()
			win.PostMessage(s.hwnd, wmDone, 0, 0)
		}(dir)
	case pageFinish:
		if win.SendMessage(s.launch, bmGetCheck, 0, 0) == bstChecked && s.installedTo != "" {
			_ = syswin.OpenPath(s.installedTo + `\cclear.exe`)
		}
		win.DestroyWindow(s.hwnd)
	}
}

var (
	installResult error
	installMu     sync.Mutex
)

func (s *setup) installFinished() {
	s.installing = false
	win.SendMessage(s.progress, win.PBM_SETMARQUEE, 0, 0)
	installMu.Lock()
	err := installResult
	installResult = nil
	installMu.Unlock()
	if err != nil {
		s.installedTo = ""
		win.MessageBox(s.hwnd, utf(err.Error()), utf("安装没有完成"), win.MB_ICONWARNING)
		s.page = pageDir
		s.showPage()
		return
	}
	s.page = pageFinish
	s.showPage()
}

func (s *setup) showPage() {
	show(s.body, s.page == pageWelcome || s.page == pageDir || s.page == pageInstall || s.page == pageFinish)
	show(s.license, s.page == pageLicense)
	show(s.dir, s.page == pageDir)
	show(s.browse, s.page == pageDir)
	show(s.progress, s.page == pageInstall)
	show(s.launch, s.page == pageFinish)
	show(s.back, s.page == pageLicense || s.page == pageDir)
	show(s.next, true)
	show(s.cancel, true)
	win.EnableWindow(s.back, s.page == pageLicense || s.page == pageDir)
	win.EnableWindow(s.next, s.page != pageInstall)
	win.EnableWindow(s.cancel, !s.installing)
	switch s.page {
	case pageDir:
		setText(s.next, "安装")
		setText(s.body, "选择 CClear 的安装文件夹。")
	case pageInstall:
		setText(s.next, "安装")
		setText(s.body, "正在复制文件并创建快捷方式，请稍候。")
	case pageFinish:
		setText(s.next, "完成")
		setText(s.cancel, "关闭")
		setText(s.body, "CClear 已安装。\r\n\r\n可以在开始菜单、桌面或“应用和功能”里找到它。卸载不会删除你的文档。")
	default:
		setText(s.next, "下一步")
		setText(s.cancel, "取消")
		if s.page == pageWelcome {
			setText(s.body, "此向导将把 CClear 安装到你的计算机。\r\n\r\nCClear 用于安全清理系统盘上的临时文件、缓存和回收站。它不会删除文档、桌面、下载目录，也不会卸载已安装的软件。\r\n\r\n点击“下一步”继续。")
		}
	}
	s.layout()
	win.InvalidateRect(s.hwnd, nil, true)
}

func (s *setup) pickDir() {
	path, ok := browseFolder(s.hwnd, "选择安装文件夹")
	if ok && path != "" {
		setText(s.dir, installFolder(path))
	}
}

func (s *setup) layout() {
	var client win.RECT
	win.GetClientRect(s.hwnd, &client)
	w := int(client.Right)
	h := int(client.Bottom)
	m := s.px(20)
	header := s.px(88)
	btnH := s.px(32)
	btnW := s.px(96)
	btnY := h - m - btnH
	top := header + m
	bottom := btnY - m
	bodyBottom := bottom
	switch s.page {
	case pageDir:
		bodyBottom = top + s.px(32)
	case pageInstall:
		bodyBottom = top + s.px(64)
	case pageFinish:
		bodyBottom = top + s.px(100)
	}
	win.MoveWindow(s.body, int32(m), int32(top), int32(w-m*2), int32(bodyBottom-top), true)
	win.MoveWindow(s.license, int32(m), int32(top), int32(w-m*2), int32(bottom-top), true)
	win.MoveWindow(s.dir, int32(m), int32(top+s.px(36)), int32(w-m*3-s.px(96)), int32(s.px(28)), true)
	win.MoveWindow(s.browse, int32(w-m-s.px(96)), int32(top+s.px(36)), int32(s.px(96)), int32(s.px(28)), true)
	win.MoveWindow(s.progress, int32(m), int32(top+s.px(80)), int32(w-m*2), int32(s.px(16)), true)
	win.MoveWindow(s.launch, int32(m), int32(top+s.px(110)), int32(w-m*2), int32(s.px(24)), true)
	win.MoveWindow(s.cancel, int32(w-m-btnW), int32(btnY), int32(btnW), int32(btnH), true)
	win.MoveWindow(s.next, int32(w-m*2-btnW*2), int32(btnY), int32(btnW), int32(btnH), true)
	win.MoveWindow(s.back, int32(w-m*3-btnW*3), int32(btnY), int32(btnW), int32(btnH), true)
}

func (s *setup) paint() {
	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(s.hwnd, &ps)
	defer win.EndPaint(s.hwnd, &ps)
	var client win.RECT
	win.GetClientRect(s.hwnd, &client)
	fillRect(hdc, client, s.pageBrush)
	header := client
	header.Bottom = int32(s.px(88))
	fillRect(hdc, header, s.headBrush)
	win.SetBkMode(hdc, win.TRANSPARENT)
	win.SetTextColor(hdc, rgb(255, 255, 255))
	old := win.SelectObject(hdc, win.HGDIOBJ(s.title))
	drawText(hdc, pageTitle(s.page), box(s.px(24), s.px(18), int(client.Right)-s.px(24), s.px(52)), dtSingleLine|dtNoPrefix)
	win.SelectObject(hdc, win.HGDIOBJ(s.font))
	drawText(hdc, pageSub(s.page), box(s.px(24), s.px(52), int(client.Right)-s.px(24), s.px(78)), dtSingleLine|dtNoPrefix)
	win.SelectObject(hdc, old)
}

func pageTitle(page int) string {
	switch page {
	case pageLicense:
		return "许可说明"
	case pageDir:
		return "选择安装位置"
	case pageInstall:
		return "正在安装"
	case pageFinish:
		return "安装完成"
	default:
		return "欢迎使用 CClear 安装向导"
	}
}

func pageSub(page int) string {
	switch page {
	case pageLicense:
		return "请阅读下面的说明，然后点击“下一步”。"
	case pageDir:
		return "建议安装到 Program Files。"
	case pageInstall:
		return "请不要关闭这个窗口。"
	case pageFinish:
		return "安装程序已经完成。"
	default:
		return "安全清理系统盘上的临时文件和缓存。"
	}
}

func browseFolder(owner win.HWND, title string) (string, bool) {
	var name [260]uint16
	info := browseInfo{
		HwndOwner:      uintptr(owner),
		PszDisplayName: &name[0],
		LpszTitle:      utf(title),
		UlFlags:        bifNewStyle,
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	pidl, _, _ := shell32.NewProc("SHBrowseForFolderW").Call(uintptr(unsafe.Pointer(&info)))
	if pidl == 0 {
		return "", false
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(pidl))
	var path [260]uint16
	ok, _, _ := shell32.NewProc("SHGetPathFromIDListW").Call(pidl, uintptr(unsafe.Pointer(&path[0])))
	if ok == 0 {
		return "", false
	}
	return windows.UTF16ToString(path[:]), true
}

type browseInfo struct {
	HwndOwner      uintptr
	PidlRoot       uintptr
	PszDisplayName *uint16
	LpszTitle      *uint16
	UlFlags        uint32
	Lpfn           uintptr
	LParam         uintptr
	IImage         int32
}

func windowText(hwnd win.HWND) string {
	n := win.SendMessage(hwnd, wmGetTextLen, 0, 0)
	buf := make([]uint16, n+1)
	if len(buf) == 0 {
		return ""
	}
	win.SendMessage(hwnd, wmGetText, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	return windows.UTF16ToString(buf)
}

func show(hwnd win.HWND, on bool) {
	if on {
		win.ShowWindow(hwnd, win.SW_SHOW)
		return
	}
	win.ShowWindow(hwnd, win.SW_HIDE)
}

func setText(hwnd win.HWND, text string) {
	ptr := utf(text)
	win.SendMessage(hwnd, win.WM_SETTEXT, 0, uintptr(unsafe.Pointer(ptr)))
	runtime.KeepAlive(ptr)
}

func (s *setup) px(n int) int {
	if s.dpi == 0 {
		return n
	}
	return n * s.dpi / 96
}

func utf(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

func rgb(r, g, b byte) win.COLORREF {
	return win.COLORREF(r) | win.COLORREF(g)<<8 | win.COLORREF(b)<<16
}

func box(l, t, r, b int) win.RECT {
	return win.RECT{Left: int32(l), Top: int32(t), Right: int32(r), Bottom: int32(b)}
}

func brush(color win.COLORREF) win.HBRUSH {
	return win.CreateBrushIndirect(&win.LOGBRUSH{LbStyle: win.BS_SOLID, LbColor: color})
}

func fillRect(hdc win.HDC, rc win.RECT, paint win.HBRUSH) {
	windows.NewLazySystemDLL("user32.dll").NewProc("FillRect").Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), uintptr(paint))
}

func drawText(hdc win.HDC, text string, rc win.RECT, format uint32) {
	ptr := utf(text)
	win.DrawTextEx(hdc, ptr, -1, &rc, format, nil)
	runtime.KeepAlive(ptr)
}

func uiFont(dpi, points int, weight int32) win.HFONT {
	if dpi <= 0 {
		dpi = 96
	}
	px := points * 96 / 72
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

type osError string

func (e osError) Error() string { return string(e) }

const licenseText = `CClear 软件许可

版权所有 (C) 2026 CClear

本软件按“原样”提供，用于清理 Windows 系统盘上的临时文件、缓存和回收站。

1. 清理会直接删除所选文件，不会把它们放进回收站。
2. 默认只处理临时文件、缓存、错误报告、崩溃转储和回收站。
3. 文档、桌面、下载、照片和已安装的软件不在清理范围内。
4. 带有警告的项目需要你手动勾选。
5. 作者不对因使用本软件造成的数据丢失承担责任。

点击“下一步”表示你已阅读以上说明。`
