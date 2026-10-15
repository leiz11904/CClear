package history

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Record is one cleanup log summarized for the history page.
type Record struct {
	When      time.Time
	File      string
	Version   string
	Deleted   int
	Skipped   int
	Failed    int
	Logical   string
	Estimated string
	Items     []string
}

// Load reads the newest logs from dir. Paths inside logs are already redacted
// by the cleaner; this function does not add new file paths to the record list.
func Load(dir string, limit int) ([]Record, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Slice(names, func(i, j int) bool { return names[i] > names[j] })
	if limit > 0 && len(names) > limit {
		names = names[:limit]
	}
	out := make([]Record, 0, len(names))
	for _, name := range names {
		rec, err := parse(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		rec.File = name
		out = append(out, rec)
	}
	return out, nil
}

func parse(path string) (Record, error) {
	file, err := os.Open(path)
	if err != nil {
		return Record{}, err
	}
	defer file.Close()
	var rec Record
	if info, err := file.Stat(); err == nil {
		rec.When = info.ModTime()
	}
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(line, "CClear ") && rec.Version == "" {
			rec.Version = strings.TrimSpace(strings.TrimPrefix(line, "CClear "))
		}
		if strings.HasPrefix(line, "任务 ") {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(strings.TrimPrefix(line, "任务 "))); err == nil {
				rec.When = t
			}
		}
		if strings.HasPrefix(line, "---- ") && strings.Contains(line, " ----") {
			name := strings.TrimSuffix(strings.TrimPrefix(line, "---- "), " ----")
			if i := strings.Index(name, " ("); i > 0 {
				name = name[:i]
			}
			rec.Items = append(rec.Items, name)
		}
		if strings.Contains(line, "删除") && strings.Contains(line, "跳过") {
			rec.Logical, rec.Estimated = sizes(line)
			rec.Deleted += numberAfter(line, "删除")
			rec.Skipped += numberAfter(line, "跳过")
			rec.Failed += numberAfter(line, "失败")
		}
	}
	return rec, scan.Err()
}

func sizes(line string) (logical, estimated string) {
	logical = fieldAfter(line, "逻辑大小")
	estimated = fieldAfter(line, "预计占用")
	return logical, estimated
}

func fieldAfter(line, key string) string {
	i := strings.Index(line, key)
	if i < 0 {
		return ""
	}
	rest := strings.TrimSpace(line[i+len(key):])
	rest = strings.TrimLeft(rest, " ：:")
	if j := strings.IndexAny(rest, "，,"); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}

func numberAfter(line, key string) int {
	i := strings.Index(line, key)
	if i < 0 {
		return 0
	}
	rest := strings.TrimSpace(line[i+len(key):])
	rest = strings.TrimLeft(rest, " ：:")
	n := 0
	for _, r := range rest {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// FormatWhen renders a timestamp for the list.
func FormatWhen(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}
