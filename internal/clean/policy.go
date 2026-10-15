package clean

import (
	"os"
	"path/filepath"
	"strings"
)

// MatchIdentity reports whether a file still matches the snapshot taken at scan time.
// A size change always fails. Timestamp comparisons allow a two-second filesystem tolerance.
func MatchIdentity(size, mod, currentSize, currentMod int64) bool {
	if size != currentSize {
		return false
	}
	if mod == 0 || currentMod == 0 {
		return true
	}
	delta := mod - currentMod
	if delta < 0 {
		delta = -delta
	}
	return delta <= 2
}

// Reclaimable is the space a delete can be expected to return.
// Extra hard links keep the data alive, so they contribute nothing.
func Reclaimable(logical, allocated int64, links uint32) int64 {
	if links > 1 || logical < 0 {
		return 0
	}
	if allocated > 0 {
		return allocated
	}
	return logical
}

// UpdateShouldSkip reports that update-related cleanup must wait.
func UpdateShouldSkip(rebootPending, installBusy bool) bool {
	return rebootPending || installBusy
}

// UpdateServiceBusy is true unless the service is known to be stopped.
// A query failure is treated as busy so cleanup does not continue.
func UpdateServiceBusy(queryFailed, stopped bool) bool {
	return queryFailed || !stopped
}

// DISMArgs returns the supported component-store commands.
// ResetBase is intentionally absent.
func DISMArgs(phase string) []string {
	switch phase {
	case "analyze":
		return []string{"/Online", "/Cleanup-Image", "/AnalyzeComponentStore"}
	case "cleanup":
		return []string{"/Online", "/Cleanup-Image", "/StartComponentCleanup", "/NoRestart"}
	default:
		return nil
	}
}

// OldWindowsPlan is the Disk Cleanup invocation for a previous Windows installation.
// The sage number is private to CClear so other cleanup handlers are not selected.
func OldWindowsPlan() (handler, valueName string, args []string) {
	return "Previous Installations", "StateFlags64721", []string{"/sagerun:64721"}
}

// RedactPath hides the account name in a filesystem path.
func RedactPath(path string) string {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '\\' || r == '/' })
	for i := 0; i+1 < len(parts); i++ {
		if strings.EqualFold(parts[i], "Users") && parts[i+1] != "" && parts[i+1] != "~" {
			parts[i+1] = "~"
		}
	}
	if strings.Contains(path, `\`) {
		return strings.Join(parts, `\`)
	}
	return strings.Join(parts, string(os.PathSeparator))
}

// RedactText replaces account folders inside free-form log lines.
func RedactText(s string) string {
	s = redactUsers(s, `\`)
	s = redactUsers(s, `/`)
	return s
}

func redactUsers(s, sep string) string {
	lower := strings.ToLower(s)
	token := "users" + sep
	var b strings.Builder
	i := 0
	for {
		idx := strings.Index(lower[i:], token)
		if idx < 0 {
			b.WriteString(s[i:])
			return b.String()
		}
		idx += i
		start := idx + len(token)
		b.WriteString(s[i:start])
		j := start
		for j < len(s) && s[j] != '\\' && s[j] != '/' && s[j] != ' ' && s[j] != '\t' && s[j] != '\r' {
			j++
		}
		if start < j && s[start:j] != "~" {
			b.WriteString("~")
		} else {
			b.WriteString(s[start:j])
		}
		i = j
	}
}

// SpaceNote explains why free-space change may not equal the file-size estimate.
func SpaceNote() string {
	return "实际可用空间的变化可能和预计值不同，因为系统或其他程序也在读写磁盘。"
}

// GroupLabel is the name shown for a rule group.
func GroupLabel(group string) string {
	switch group {
	case "deep":
		return "深度清理"
	case "system":
		return "系统清理"
	case "recycle":
		return "回收站"
	default:
		return "常规清理"
	}
}

// DetailPage returns one page of an already capped preview list.
func DetailPage(files []FileHit, page int) (view []FileHit, pages, pageOut int) {
	if len(files) == 0 {
		return nil, 1, 0
	}
	pages = (len(files) + DetailPageSize - 1) / DetailPageSize
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = pages - 1
	}
	start := page * DetailPageSize
	end := start + DetailPageSize
	if end > len(files) {
		end = len(files)
	}
	return files[start:end], pages, page
}

// RiskLabel is the short risk name shown in the list.
func RiskLabel(risk int) string {
	switch risk {
	case 3:
		return "高"
	case 2:
		return "中"
	default:
		return "低"
	}
}

// PruneLogs deletes the oldest log files until the directory is within maxBytes.
func PruneLogs(dir string, maxBytes int64) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type item struct {
		path string
		size int64
	}
	var files []item
	var total int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, item{path: filepath.Join(dir, entry.Name()), size: info.Size()})
		total += info.Size()
	}
	for total > maxBytes && len(files) > 1 {
		oldest := 0
		for i := 1; i < len(files); i++ {
			if files[i].path < files[oldest].path {
				oldest = i
			}
		}
		if err := os.Remove(files[oldest].path); err != nil && !os.IsNotExist(err) {
			return err
		}
		total -= files[oldest].size
		files = append(files[:oldest], files[oldest+1:]...)
	}
	return nil
}

// SanitizeLogDir rewrites existing logs so account names are not left in plain text.
func SanitizeLogDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		next := RedactText(string(data))
		if next == string(data) {
			continue
		}
		if err := os.WriteFile(path, []byte(next), 0o600); err != nil {
			return err
		}
	}
	return nil
}
