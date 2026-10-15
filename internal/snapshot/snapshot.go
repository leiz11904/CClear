package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cclear/internal/space"
)

const maxKeep = 8

// Record is a compact occupancy snapshot. Full file paths are not stored.
type Record struct {
	When      time.Time        `json:"when"`
	Drive     string           `json:"drive"`
	Total     uint64           `json:"total"`
	Free      uint64           `json:"free"`
	Allocated int64            `json:"allocated"`
	Files     int              `json:"files"`
	Folders   []space.Dir      `json:"folders"`
	Types     []space.TypeStat `json:"types"`
}

// Diff is one folder size change between two snapshots.
type Diff struct {
	Name   string
	Before int64
	After  int64
	Delta  int64
}

func Dir(base string) string {
	return filepath.Join(base, "CClear", "snapshots")
}

func Save(base string, rec Record) error {
	dir := Dir(base)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	rec.Folders = redactFolders(rec.Folders)
	rec.Types = redactTypes(rec.Types)
	name := rec.When.UTC().Format("20060102-150405") + ".json"
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		return err
	}
	return prune(dir)
}

func Load(base string) ([]Record, error) {
	entries, err := os.ReadDir(Dir(base))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return names[i] > names[j] })
	var out []Record
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(Dir(base), name))
		if err != nil {
			continue
		}
		var rec Record
		if json.Unmarshal(data, &rec) != nil {
			continue
		}
		rec.Folders = redactFolders(rec.Folders)
		rec.Types = redactTypes(rec.Types)
		out = append(out, rec)
	}
	return out, nil
}

func Compare(older, newer Record) []Diff {
	before := map[string]int64{}
	for _, folder := range older.Folders {
		before[folder.Name] = folder.Allocated
	}
	seen := map[string]bool{}
	var out []Diff
	for _, folder := range newer.Folders {
		seen[folder.Name] = true
		prev := before[folder.Name]
		if folder.Allocated == prev {
			continue
		}
		out = append(out, Diff{Name: folder.Name, Before: prev, After: folder.Allocated, Delta: folder.Allocated - prev})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Delta, out[j].Delta
		if a < 0 {
			a = -a
		}
		if b < 0 {
			b = -b
		}
		return a > b
	})
	return out
}

// SanitizeDir rewrites stored snapshots so account names and full paths
// are not left on disk from older versions.
func SanitizeDir(base string) error {
	dir := Dir(base)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var rec Record
		if json.Unmarshal(data, &rec) != nil {
			continue
		}
		rec.Folders = redactFolders(rec.Folders)
		rec.Types = redactTypes(rec.Types)
		next, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			continue
		}
		if string(next) == string(data) {
			continue
		}
		if err := os.WriteFile(path, next, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func prune(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for len(names) > maxKeep {
		if err := os.Remove(filepath.Join(dir, names[0])); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}

func redactFolders(in []space.Dir) []space.Dir {
	out := make([]space.Dir, 0, len(in))
	for _, folder := range in {
		folder.Name = folderLabel(folder)
		folder.Path = ""
		out = append(out, folder)
	}
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func redactTypes(in []space.TypeStat) []space.TypeStat {
	if len(in) > 12 {
		return in[:12]
	}
	return in
}

func folderLabel(folder space.Dir) string {
	raw := folder.Path
	if raw == "" {
		raw = folder.Name
	}
	cleaned := strings.ReplaceAll(raw, `/`, `\`)
	lower := strings.ToLower(cleaned)
	if i := strings.Index(lower, `\users\`); i >= 0 {
		rest := cleaned[i+7:]
		if slash := strings.IndexAny(rest, `\`); slash >= 0 {
			return `~\` + filepath.Base(strings.ReplaceAll(rest[slash+1:], `\`, "/"))
		}
		return "~"
	}
	base := folder.Name
	if base == "" || strings.ContainsAny(base, `\ /`) {
		base = filepath.Base(strings.ReplaceAll(cleaned, `\`, "/"))
	}
	if len(base) > 40 {
		base = base[:40]
	}
	return base
}
