package devspace

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Item is one recognized development-related occupancy.
type Item struct {
	ID    string
	Name  string
	Kind  string
	Path  string
	Bytes int64
	Note  string
}

// Report is read-only. No Docker or WSL cleanup commands are executed.
type Report struct {
	Items     []Item
	Note      string
	Cancelled bool
}

// Env is filled from the machine. Tests pass isolated directories.
type Env struct {
	UserProfile  string
	LocalAppData string
	UserHome     string
}

// Scan looks for known development caches and virtual disks.
func Scan(ctx context.Context, env Env) Report {
	rep := Report{Note: "只识别占用。不会删除镜像、容器、卷、虚拟磁盘或用户项目。WSL 的 VHDX 禁止直接删除。Docker 清理需要你在官方工具里手动执行。"}
	home := env.UserHome
	if home == "" {
		home = env.UserProfile
	}
	candidates := []Item{
		{ID: "docker-app", Name: "Docker Desktop 数据", Kind: "运行环境", Path: filepath.Join(env.LocalAppData, "Docker"), Note: "运行环境，不是缓存。不自动清理。"},
		{ID: "docker-wsl", Name: "Docker WSL 虚拟磁盘", Kind: "虚拟磁盘", Path: filepath.Join(env.LocalAppData, "Docker", "wsl"), Note: "VHDX，禁止直接删除。"},
		{ID: "vscode", Name: "VS Code 扩展与缓存", Kind: "缓存", Path: filepath.Join(env.UserProfile, ".vscode"), Note: "含扩展，不只是缓存。"},
		{ID: "vs", Name: "Visual Studio 安装", Kind: "SDK", Path: `C:\Program Files\Microsoft Visual Studio`, Note: "安装目录，不能当缓存删。"},
		{ID: "npm", Name: "npm 缓存", Kind: "缓存", Path: filepath.Join(env.LocalAppData, "npm-cache"), Note: "缓存。垃圾清理里另有可选规则，默认关闭。"},
		{ID: "pnpm", Name: "pnpm 缓存", Kind: "缓存", Path: filepath.Join(env.LocalAppData, "pnpm-cache"), Note: "缓存。"},
		{ID: "yarn", Name: "Yarn 缓存", Kind: "缓存", Path: filepath.Join(env.LocalAppData, "Yarn", "Cache"), Note: "缓存。"},
		{ID: "pip", Name: "pip 缓存", Kind: "缓存", Path: filepath.Join(env.LocalAppData, "pip", "Cache"), Note: "缓存。"},
		{ID: "conda", Name: "Conda 环境", Kind: "运行环境", Path: filepath.Join(home, "miniconda3"), Note: "运行环境，不是缓存。"},
		{ID: "conda2", Name: "Anaconda", Kind: "运行环境", Path: filepath.Join(home, "anaconda3"), Note: "运行环境，不是缓存。"},
		{ID: "gradle", Name: "Gradle 缓存", Kind: "缓存", Path: filepath.Join(home, ".gradle"), Note: "构建缓存。"},
		{ID: "android", Name: "Android SDK", Kind: "SDK", Path: filepath.Join(home, "AppData", "Local", "Android", "Sdk"), Note: "SDK，不是项目源码。"},
		{ID: "go-mod", Name: "Go 模块缓存", Kind: "缓存", Path: filepath.Join(home, "go", "pkg", "mod"), Note: "模块缓存。"},
		{ID: "go-build", Name: "Go 构建缓存", Kind: "缓存", Path: filepath.Join(env.LocalAppData, "go-build"), Note: "构建缓存。"},
	}
	for _, item := range wslDisks(env.LocalAppData) {
		candidates = append(candidates, item)
	}
	for _, item := range candidates {
		if ctx.Err() != nil {
			rep.Cancelled = true
			return rep
		}
		info, err := os.Lstat(item.Path)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if info.IsDir() {
			item.Bytes = dirSize(ctx, item.Path)
		} else {
			item.Bytes = info.Size()
			if strings.HasSuffix(strings.ToLower(item.Path), ".vhdx") {
				item.Kind = "虚拟磁盘"
				item.Note = "VHDX，禁止直接删除。"
			}
		}
		rep.Items = append(rep.Items, item)
	}
	return rep
}

func wslDisks(local string) []Item {
	root := filepath.Join(local, "Packages")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []Item
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.Contains(name, "canonical") && !strings.Contains(name, "linux") && !strings.Contains(name, "debian") && !strings.Contains(name, "ubuntu") && !strings.Contains(name, "suse") {
			continue
		}
		disk := filepath.Join(root, entry.Name(), "LocalState", "ext4.vhdx")
		out = append(out, Item{ID: "wsl-" + entry.Name(), Name: "WSL 虚拟磁盘", Kind: "虚拟磁盘", Path: disk, Note: "VHDX，禁止直接删除。"})
	}
	return out
}

func dirSize(ctx context.Context, root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || d == nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}
