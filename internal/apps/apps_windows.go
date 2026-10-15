//go:build windows

package apps

import (
	"context"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func init() {
	ListInstalled = listInstalled
}

func listInstalled(ctx context.Context) Report {
	rep := Report{Note: "占用数字来自系统登记的 EstimatedSize，可能缺失或不准确，不能当成软件的全部磁盘占用。卸载只会启动系统登记的卸载程序，不会直接删除安装目录。"}
	keys := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	}
	seen := map[string]bool{}
	for _, item := range keys {
		if ctx.Err() != nil {
			rep.Cancelled = true
			return rep
		}
		parent, err := registry.OpenKey(item.root, item.path, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		names, err := parent.ReadSubKeyNames(-1)
		if err != nil {
			parent.Close()
			continue
		}
		for _, name := range names {
			sub, err := registry.OpenKey(parent, name, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			app := readApp(sub)
			sub.Close()
			if app.Name == "" || app.Uninstall == "" {
				continue
			}
			key := strings.ToLower(app.Name + "|" + app.Location)
			if seen[key] {
				continue
			}
			seen[key] = true
			rep.Apps = append(rep.Apps, app)
		}
		parent.Close()
	}
	sort.Slice(rep.Apps, func(i, j int) bool {
		if rep.Apps[i].Estimated == rep.Apps[j].Estimated {
			return strings.ToLower(rep.Apps[i].Name) < strings.ToLower(rep.Apps[j].Name)
		}
		return rep.Apps[i].Estimated > rep.Apps[j].Estimated
	})
	return rep
}

func readApp(key registry.Key) App {
	var app App
	app.Name = stringValue(key, "DisplayName")
	app.Version = stringValue(key, "DisplayVersion")
	app.Publisher = stringValue(key, "Publisher")
	app.Location = stringValue(key, "InstallLocation")
	app.Uninstall = stringValue(key, "UninstallString")
	syscomp, _, _ := key.GetIntegerValue("SystemComponent")
	if syscomp == 1 {
		app.Name = ""
		return app
	}
	if release, _, err := key.GetIntegerValue("EstimatedSize"); err == nil {
		app.Estimated = int64(release) * 1024
		app.EstimatedOK = true
	} else {
		app.Note = "系统没有提供安装大小"
	}
	return app
}

func stringValue(key registry.Key, name string) string {
	value, _, err := key.GetStringValue(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}
