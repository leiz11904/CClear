// Package winpath decides which Windows paths a cleanup tool may touch.
// Matching uses backslash rules so the same tests run on every OS.
package winpath

import (
	"errors"
	"strings"
)

var (
	ErrEmpty     = errors.New("路径为空")
	ErrUNC       = errors.New("不支持网络路径")
	ErrNotAbs    = errors.New("需要本机绝对路径")
	ErrEscape    = errors.New("路径超出盘符根目录")
	ErrOutside   = errors.New("路径不在允许的目录内")
	ErrRoot      = errors.New("不能删除目录本身")
	ErrDrive     = errors.New("只能清理系统盘")
	ErrForbidden = errors.New("该位置被保护，不能清理")
	ErrFileKind  = errors.New("这个文件类型不在清理范围内")
)

// Clean resolves "." and ".." and rejects UNC paths.
// A drive root is returned as "C:\".
func Clean(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", ErrEmpty
	}
	p := strings.ReplaceAll(path, "/", `\`)
	lower := strings.ToLower(p)
	if strings.HasPrefix(lower, `\\?\unc\`) {
		return "", ErrUNC
	}
	if strings.HasPrefix(lower, `\\?\`) {
		p = p[4:]
		lower = strings.ToLower(p)
	}
	if strings.HasPrefix(p, `\\`) {
		return "", ErrUNC
	}
	if len(p) < 2 || p[1] != ':' || !isLetter(p[0]) {
		return "", ErrNotAbs
	}
	vol := strings.ToUpper(p[:1]) + ":"
	parts := strings.Split(p[2:], `\`)
	stack := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(stack) == 0 {
				return "", ErrEscape
			}
			stack = stack[:len(stack)-1]
		default:
			if strings.ContainsRune(part, 0) {
				return "", ErrNotAbs
			}
			stack = append(stack, part)
		}
	}
	if len(stack) == 0 {
		return vol + `\`, nil
	}
	return vol + `\` + strings.Join(stack, `\`), nil
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// Drive returns "C:" for a cleaned path.
func Drive(cleaned string) string {
	if len(cleaned) < 2 || cleaned[1] != ':' {
		return ""
	}
	return strings.ToUpper(cleaned[:2])
}

// Within reports whether path is root or a descendant of root.
// Both arguments must already be cleaned.
func Within(path, root string) bool {
	if strings.EqualFold(path, root) {
		return true
	}
	prefix := root
	if !strings.HasSuffix(prefix, `\`) {
		prefix += `\`
	}
	return len(path) > len(prefix) && strings.EqualFold(path[:len(prefix)], prefix)
}

// Base returns the last element of a cleaned path.
func Base(cleaned string) string {
	if i := strings.LastIndex(cleaned, `\`); i >= 0 {
		return cleaned[i+1:]
	}
	return cleaned
}

type rule struct {
	pattern   string
	files     func(base string) bool
	dirs      bool
	allowSelf bool
}

func anyFile(string) bool { return true }

func logFile(base string) bool {
	lower := strings.ToLower(base)
	return strings.HasSuffix(lower, ".log") || strings.HasSuffix(lower, ".cab")
}

func dmpFile(base string) bool {
	return strings.HasSuffix(strings.ToLower(base), ".dmp")
}

func thumbFile(base string) bool {
	lower := strings.ToLower(base)
	if strings.HasSuffix(lower, ".db") && (strings.HasPrefix(lower, "thumbcache_") || strings.HasPrefix(lower, "iconcache_")) {
		return true
	}
	return false
}

func memoryDump(base string) bool {
	return strings.EqualFold(base, "MEMORY.DMP")
}

// rules is the only allowlist. A cleanup root must match one pattern exactly.
func rules() []rule {
	out := []rule{
		{`Windows\Temp`, anyFile, true, false},
		{`Windows\SoftwareDistribution\Download`, anyFile, true, false},
		{`Windows\Logs\CBS`, logFile, true, false},
		{`Windows\Logs\DISM`, logFile, true, false},
		{`Windows\Minidump`, dmpFile, true, false},
		{`Windows\MEMORY.DMP`, memoryDump, false, true},
		{`Windows\ServiceProfiles\NetworkService\AppData\Local\Microsoft\Windows\DeliveryOptimization\Cache`, anyFile, true, false},
		{`Windows.old`, anyFile, true, true},
		{`ProgramData\Microsoft\Windows\WER\ReportArchive`, anyFile, true, false},
		{`ProgramData\Microsoft\Windows\WER\ReportQueue`, anyFile, true, false},
		{`ProgramData\Microsoft\Windows\DeliveryOptimization\Cache`, anyFile, true, false},
		{`Users\*\AppData\Local\Temp`, anyFile, true, false},
		{`Users\*\AppData\Local\Microsoft\Windows\Explorer`, thumbFile, false, false},
		{`Users\*\AppData\Local\Microsoft\Windows\INetCache`, anyFile, true, false},
		{`Users\*\AppData\Local\Microsoft\Windows\WER\ReportArchive`, anyFile, true, false},
		{`Users\*\AppData\Local\Microsoft\Windows\WER\ReportQueue`, anyFile, true, false},
		{`Users\*\AppData\Local\CrashDumps`, dmpFile, true, false},
		{`Users\*\AppData\Local\D3DSCache`, anyFile, true, false},
		{`Users\*\AppData\Local\npm-cache`, anyFile, true, false},
		{`Users\*\AppData\Local\pip\Cache`, anyFile, true, false},
		{`Users\*\AppData\Local\NuGet\v3-cache`, anyFile, true, false},
		{`Users\*\AppData\Local\NuGet\http-cache`, anyFile, true, false},
		{`Users\*\AppData\Local\Yarn\Cache`, anyFile, true, false},
		{`Users\*\AppData\Local\go-build`, anyFile, true, false},
		{`Users\*\AppData\Local\pnpm-cache`, anyFile, true, false},
		{`Users\*\AppData\Local\NVIDIA\DXCache`, anyFile, true, false},
		{`Users\*\AppData\Local\NVIDIA\GLCache`, anyFile, true, false},
		{`Users\*\AppData\Local\NVIDIA Corporation\NV_Cache`, anyFile, true, false},
		{`Users\*\AppData\Local\AMD\DxCache`, anyFile, true, false},
		{`Users\*\AppData\Local\AMD\GLCache`, anyFile, true, false},
	}
	browsers := []string{
		`Google\Chrome`,
		`Google\Chrome Beta`,
		`Google\Chrome SxS`,
		`Microsoft\Edge`,
		`Microsoft\Edge Beta`,
		`Microsoft\Edge Dev`,
		`BraveSoftware\Brave-Browser`,
		`Chromium`,
	}
	caches := []string{
		"Cache",
		"Code Cache",
		"GPUCache",
		"DawnGraphiteCache",
		"DawnWebGPUCache",
		"ShaderCache",
		"GrShaderCache",
	}
	for _, browser := range browsers {
		for _, cache := range caches {
			out = append(out, rule{
				pattern: `Users\*\AppData\Local\` + browser + `\User Data\*\` + cache,
				files:   anyFile,
				dirs:    true,
			})
		}
		out = append(out, rule{
			pattern: `Users\*\AppData\Local\` + browser + `\User Data\*\Service Worker\CacheStorage`,
			files:   anyFile,
			dirs:    true,
		})
	}
	for _, sub := range []string{"cache2", "startupCache", "shader-cache", "thumbnails"} {
		out = append(out, rule{
			pattern: `Users\*\AppData\Local\Mozilla\Firefox\Profiles\*\` + sub,
			files:   anyFile,
			dirs:    true,
		})
	}
	for _, sub := range []string{"Cache", "GPUCache", "Code Cache"} {
		out = append(out, rule{
			pattern: `Users\*\AppData\Local\Opera Software\Opera Stable\` + sub,
			files:   anyFile,
			dirs:    true,
		})
	}
	return out
}

func matchPattern(rel, pattern string) bool {
	rp := strings.Split(rel, `\`)
	pp := strings.Split(pattern, `\`)
	if len(rp) != len(pp) {
		return false
	}
	for i := range pp {
		if pp[i] == "*" {
			if rp[i] == "" || rp[i] == "." || rp[i] == ".." {
				return false
			}
			continue
		}
		if !strings.EqualFold(rp[i], pp[i]) {
			return false
		}
	}
	return true
}

func findRule(rel string) (rule, bool) {
	for _, item := range rules() {
		if matchPattern(rel, item.pattern) {
			return item, true
		}
	}
	return rule{}, false
}

func relative(cleaned, systemDrive string) (string, error) {
	drive := strings.ToUpper(strings.TrimSuffix(systemDrive, `\`))
	if Drive(cleaned) != drive {
		return "", ErrDrive
	}
	if len(cleaned) <= 3 {
		return "", ErrForbidden
	}
	return cleaned[3:], nil
}

// ValidateRoot checks that a cleanup root is on the allowlist.
func ValidateRoot(root, systemDrive string) error {
	cleaned, err := Clean(root)
	if err != nil {
		return err
	}
	rel, err := relative(cleaned, systemDrive)
	if err != nil {
		return err
	}
	if _, ok := findRule(rel); !ok {
		return ErrForbidden
	}
	if ProtectedUserData(cleaned) || ProtectedFile(cleaned) || forbiddenPrefix(cleaned, systemDrive) {
		return ErrForbidden
	}
	return nil
}

// ValidateDelete checks one file or directory about to be removed.
func ValidateDelete(path, root, systemDrive string, isDir bool) error {
	cleaned, err := Clean(path)
	if err != nil {
		return err
	}
	rootClean, err := Clean(root)
	if err != nil {
		return err
	}
	if err := ValidateRoot(rootClean, systemDrive); err != nil {
		return err
	}
	if !Within(cleaned, rootClean) {
		return ErrOutside
	}
	rel, err := relative(rootClean, systemDrive)
	if err != nil {
		return err
	}
	item, ok := findRule(rel)
	if !ok {
		return ErrForbidden
	}
	if ProtectedUserData(cleaned) || ProtectedFile(cleaned) || forbiddenPrefix(cleaned, systemDrive) {
		return ErrForbidden
	}
	same := strings.EqualFold(cleaned, rootClean)
	if same && !item.allowSelf {
		return ErrRoot
	}
	if isDir {
		if !item.dirs {
			return ErrFileKind
		}
		return nil
	}
	if item.files == nil || !item.files(Base(cleaned)) {
		return ErrFileKind
	}
	return nil
}

// Authorize is the last check before a delete. finalPath is the path opened
// by the file handle; reparse means the object is a junction or symlink.
func Authorize(requested, finalPath, root, systemDrive string, isDir, reparse bool) error {
	if reparse {
		return errors.New("跳过联接点或快捷方式")
	}
	if err := ValidateDelete(requested, root, systemDrive, isDir); err != nil {
		return err
	}
	if finalPath == "" {
		return errors.New("无法确认文件的真实路径")
	}
	return ValidateDelete(finalPath, root, systemDrive, isDir)
}

func forbiddenPrefix(cleaned, systemDrive string) bool {
	drive := strings.ToUpper(strings.TrimSuffix(systemDrive, `\`))
	prefixes := []string{
		drive + `\Windows\System32`,
		drive + `\Windows\SysWOW64`,
		drive + `\Windows\WinSxS`,
		drive + `\Windows\SystemApps`,
		drive + `\Windows\Fonts`,
		drive + `\Windows\servicing`,
		drive + `\Windows\SystemResources`,
		drive + `\Program Files`,
		drive + `\Program Files (x86)`,
		drive + `\ProgramData\Microsoft\Windows\Start Menu`,
		drive + `\ProgramData\Microsoft\Windows Defender`,
	}
	for _, prefix := range prefixes {
		if Within(cleaned, prefix) {
			return true
		}
	}
	return false
}

// ProtectedUserData reports Documents, Desktop, Downloads, and the other
// profile folders users do not expect a cleaner to touch.
func ProtectedUserData(cleaned string) bool {
	parts := strings.Split(cleaned, `\`)
	if len(parts) < 4 || !strings.EqualFold(parts[1], "Users") {
		return false
	}
	switch strings.ToLower(parts[3]) {
	case "documents", "desktop", "downloads", "pictures", "videos", "music",
		"onedrive", "contacts", "favorites", "links", "saved games", "searches",
		"3d objects", "文档", "桌面", "下载", "图片", "视频", "音乐":
		return true
	default:
		return false
	}
}

// ProtectedFile reports live operating-system files that are never safe to delete.
// Copies inside Windows.old are leftovers from a previous installation and may be removed.
func ProtectedFile(cleaned string) bool {
	base := strings.ToLower(Base(cleaned))
	inOld := strings.Contains(strings.ToLower(cleaned), `\windows.old\`) || strings.HasSuffix(strings.ToLower(cleaned), `\windows.old`)
	switch base {
	case "pagefile.sys", "hiberfil.sys", "swapfile.sys", "bootmgr", "bootnxt":
		return !inOld && strings.Count(cleaned, `\`) == 1
	case "ntuser.dat", "ntuser.dat.log", "ntuser.dat.log1", "ntuser.dat.log2",
		"usrclass.dat", "usrclass.dat.log", "usrclass.dat.log1", "usrclass.dat.log2":
		return !inOld
	default:
		return false
	}
}
