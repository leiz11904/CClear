package sysspace

import (
	"os"
	"path/filepath"
	"strings"
)

// Item is a system occupancy that must not be deleted by CClear.
type Item struct {
	Name  string
	Path  string
	Bytes int64
	Known bool
	Note  string
}

// Report is read-only system occupancy.
type Report struct {
	Items []Item
	Note  string
}

func Read(drive string) Report {
	drive = strings.TrimRight(drive, `\/`)
	rep := Report{Note: "系统文件只展示占用。CClear 不会删除 pagefile、hiberfil、WinSxS 或系统还原点。需要调整休眠或还原时，请使用 Windows 自己的设置。"}
	files := []Item{
		{Name: "虚拟内存 pagefile.sys", Path: filepath.Join(drive, "pagefile.sys"), Note: "由 Windows 管理，禁止删除。"},
		{Name: "休眠文件 hiberfil.sys", Path: filepath.Join(drive, "hiberfil.sys"), Note: "由 Windows 管理，禁止删除。"},
		{Name: "交换文件 swapfile.sys", Path: filepath.Join(drive, "swapfile.sys"), Note: "由 Windows 管理，禁止删除。"},
		{Name: "组件存储 WinSxS", Path: filepath.Join(drive, "Windows", "WinSxS"), Note: "只能通过系统 DISM 清理，且在垃圾清理里默认关闭。"},
		{Name: "系统还原与卷影 System Volume Information", Path: filepath.Join(drive, "System Volume Information"), Note: "无法直接删除。请使用系统还原设置。"},
		{Name: "Windows 更新下载", Path: filepath.Join(drive, "Windows", "SoftwareDistribution", "Download"), Note: "垃圾清理里有对应项，默认关闭。"},
	}
	for _, item := range files {
		info, err := os.Lstat(item.Path)
		if err != nil {
			item.Note += " 当前无法读取，可能需要管理员权限或不存在。"
			rep.Items = append(rep.Items, item)
			continue
		}
		item.Known = true
		if info.IsDir() {
			item.Bytes = -1
			item.Note += " 目录大小请在磁盘分析中查看，这里不直接遍历。"
		} else {
			item.Bytes = info.Size()
		}
		rep.Items = append(rep.Items, item)
	}
	rep.Items = append(rep.Items, Item{
		Name: "系统保留存储",
		Path: filepath.Join(drive, "Windows"),
		Note: "Windows 没有稳定公开的接口给出保留存储准确字节数。这里不填写估计值，也不提供删除。请使用系统存储设置查看。",
	})
	return rep
}
