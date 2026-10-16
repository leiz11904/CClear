//go:build (production || dev) && windows

package main

import (
	"errors"
	"strings"

	"cclear/internal/desk"
	"cclear/internal/syswin"
)

type Place struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func (a *App) Places() []Place {
	wanted := []Place{
		{ID: "downloads", Name: "下载"},
		{ID: "pictures", Name: "图片"},
		{ID: "videos", Name: "视频"},
		{ID: "documents", Name: "文档"},
	}
	out := make([]Place, 0, len(wanted))
	for _, item := range wanted {
		path, err := syswin.KnownFolder(item.ID)
		if err != nil || strings.TrimSpace(path) == "" {
			continue
		}
		item.Path = path
		out = append(out, item)
	}
	return out
}

func (a *App) Drives() ([]desk.Volume, error) {
	names, err := syswin.LogicalDrives()
	if err != nil {
		return nil, err
	}
	out := make([]desk.Volume, 0, len(names))
	for _, name := range names {
		drive := strings.TrimRight(name, `:\`)
		if drive == "" {
			continue
		}
		drive += ":"
		total, free, err := syswin.Disk(drive)
		row := desk.Volume{Drive: drive}
		if err == nil {
			row.Total = total
			row.Free = free
			if total > free {
				row.Used = total - free
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func (a *App) OpenSetting(id string) error {
	switch id {
	case "storage":
		return syswin.OpenSettings("ms-settings:storagesense")
	case "power":
		return syswin.OpenSettings("ms-settings:powersleep")
	case "apps":
		return syswin.OpenSettings("ms-settings:appsfeatures")
	case "update":
		return syswin.OpenSettings("ms-settings:windowsupdate")
	case "system":
		return syswin.OpenSettings("ms-settings:about")
	case "pagefile":
		return syswin.OpenPath("SystemPropertiesAdvanced.exe")
	default:
		return errors.New("没有对应的 Windows 设置入口")
	}
}
