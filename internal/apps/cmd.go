package apps

import (
	"errors"
	"strings"
)

// UninstallCommand returns the registered uninstaller. It never points at the install directory.
func UninstallCommand(app App) (exe, args string, err error) {
	if strings.TrimSpace(app.Uninstall) == "" {
		return "", "", errors.New("没有登记卸载程序")
	}
	exe, args = splitCmd(app.Uninstall)
	lower := strings.ToLower(exe)
	blocked := []string{"cmd.exe", "powershell", "pwsh.exe", "wscript", "cscript", "mshta"}
	for _, item := range blocked {
		if strings.Contains(lower, item) {
			return "", "", errors.New("拒绝执行脚本卸载命令")
		}
	}
	if exe == "" {
		return "", "", errors.New("无法解析卸载命令")
	}
	return exe, args, nil
}

func splitCmd(s string) (exe, args string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if s[0] == '"' {
		end := strings.Index(s[1:], `"`)
		if end >= 0 {
			return s[1 : 1+end], strings.TrimSpace(s[2+end:])
		}
	}
	i := strings.IndexByte(s, ' ')
	if i < 0 {
		return s, ""
	}
	return s[:i], strings.TrimSpace(s[i+1:])
}
