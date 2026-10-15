package space

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxTopFiles  = 100
	maxLargeKeep = 400
	maxTopDirs   = 100
	maxTypeKeep  = 40
	// DefaultLargeMin is the UI default filter, not a scan-time floor.
	DefaultLargeMin = 100 << 20
)

// File is one observed file. It is never a junk-cleanup candidate.
type File struct {
	Path      string
	Name      string
	Logical   int64
	Allocated int64
	ModUnix   int64
	Ext       string
	Links     uint32
}

// Dir is a directory with aggregated unique allocated size.
type Dir struct {
	Path      string
	Name      string
	Logical   int64
	Allocated int64
	Files     int
	Dirs      int
}

// TypeStat is occupancy by extension.
type TypeStat struct {
	Ext       string
	Files     int
	Allocated int64
}

// Node is one directory in the expandable tree.
type Node struct {
	Path      string
	Name      string
	Logical   int64
	Allocated int64
	Files     int
	Children  []*Node
}

// Report is a read-only occupancy map of one root.
type Report struct {
	Root           string
	Started        time.Time
	Finished       time.Time
	Files          int
	Dirs           int
	Logical        int64
	Allocated      int64
	Uncounted      int64
	HardLinks      int
	ReparseSkipped int
	TopFiles       []File
	Large          []File
	TopDirs        []Dir
	Types          []TypeStat
	Tree           *Node
	Errors         []string
	Cancelled      bool
	Note           string
}

type dirAcc struct {
	path      string
	name      string
	parent    string
	logical   int64
	allocated int64
	files     int
	dirs      int
}

// Scan walks root with ordinary directory APIs. It does not follow reparse
// points and does not delete anything.
func Scan(ctx context.Context, root string, onProgress func(files int, where string)) Report {
	rep := Report{
		Root:    root,
		Started: time.Now(),
		Note:    "只分析占用，不会把这些文件当作垃圾删除。",
	}
	info, err := os.Lstat(root)
	if err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		rep.Finished = time.Now()
		return rep
	}
	if info.Mode()&os.ModeSymlink != 0 {
		rep.Errors = append(rep.Errors, "已跳过符号链接根目录")
		rep.Finished = time.Now()
		return rep
	}

	dirs := map[string]*dirAcc{}
	var ensure func(string) *dirAcc
	ensure = func(path string) *dirAcc {
		path = filepath.Clean(path)
		if d, ok := dirs[path]; ok {
			return d
		}
		d := &dirAcc{path: path, name: filepath.Base(path), parent: filepath.Dir(path)}
		if strings.EqualFold(path, filepath.Clean(root)) {
			d.parent = ""
			d.name = path
		}
		dirs[path] = d
		if d.parent != "" && d.parent != path {
			p := ensure(d.parent)
			p.dirs++
		}
		return d
	}
	ensure(root)

	seen := map[[2]uint64]bool{}
	large := fileHeap{}
	var types = map[string]TypeStat{}
	var mu sync.Mutex
	seenFiles := 0

	addFile := func(path string, info fs.FileInfo) {
		meta := readMeta(path, info)
		if meta.Reparse {
			mu.Lock()
			rep.ReparseSkipped++
			mu.Unlock()
			return
		}
		id := [2]uint64{meta.Volume, meta.FileID}
		unique := true
		if meta.HasID && meta.Links > 1 {
			mu.Lock()
			if seen[id] {
				unique = false
				rep.HardLinks++
			} else {
				seen[id] = true
			}
			mu.Unlock()
		}
		alloc := int64(0)
		logical := int64(0)
		if unique {
			alloc = meta.Allocated
			if alloc <= 0 {
				alloc = meta.Logical
			}
			logical = meta.Logical
		}
		parent := ensure(filepath.Dir(path))
		mu.Lock()
		parent.logical += logical
		parent.allocated += alloc
		parent.files++
		for p := parent.parent; p != "" && p != parent.path; {
			d := dirs[p]
			if d == nil {
				break
			}
			d.logical += logical
			d.allocated += alloc
			p = d.parent
		}
		rep.Files++
		rep.Logical += logical
		rep.Allocated += alloc
		ext := typeKey(path)
		st := types[ext]
		st.Ext = ext
		st.Files++
		st.Allocated += alloc
		types[ext] = st
		if unique && alloc > 0 {
			keepLargest(&large, File{
				Path: path, Name: filepath.Base(path), Logical: meta.Logical,
				Allocated: alloc, ModUnix: info.ModTime().Unix(), Ext: ext, Links: meta.Links,
			}, maxLargeKeep)
		}
		seenFiles++
		n := seenFiles
		mu.Unlock()
		if onProgress != nil && n%300 == 0 {
			onProgress(n, path)
		}
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			mu.Lock()
			rep.Uncounted++
			if len(rep.Errors) < 12 {
				rep.Errors = append(rep.Errors, walkErr.Error())
			}
			mu.Unlock()
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			mu.Lock()
			rep.ReparseSkipped++
			mu.Unlock()
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if skipName(d.Name()) && !strings.EqualFold(path, root) {
				return filepath.SkipDir
			}
			ensure(path)
			mu.Lock()
			rep.Dirs++
			mu.Unlock()
			return nil
		}
		info, err := d.Info()
		if err != nil {
			mu.Lock()
			rep.Uncounted++
			mu.Unlock()
			return nil
		}
		addFile(path, info)
		return nil
	})
	if err != nil && ctx.Err() == nil {
		rep.Errors = append(rep.Errors, err.Error())
	}
	if ctx.Err() != nil {
		rep.Cancelled = true
		rep.Note = "扫描已取消。已统计的部分仍只用于展示占用。"
	}

	kept := dumpLargest(large)
	rep.Tree = buildTree(root, dirs)
	rep.TopDirs = topDirs(dirs, maxTopDirs)
	rep.TopFiles = topOf(kept, maxTopFiles)
	rep.Large = kept
	rep.Types = typeList(types)
	rep.Finished = time.Now()
	return rep
}

func skipName(name string) bool {
	switch strings.ToLower(name) {
	case "$recycle.bin", "system volume information":
		return true
	default:
		return false
	}
}

func typeKey(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return "无扩展名"
	}
	return ext
}

func topOf(list []File, n int) []File {
	if len(list) <= n {
		return append([]File(nil), list...)
	}
	return append([]File(nil), list[:n]...)
}

func topDirs(dirs map[string]*dirAcc, n int) []Dir {
	out := make([]Dir, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, Dir{Path: d.path, Name: d.name, Logical: d.logical, Allocated: d.allocated, Files: d.files, Dirs: d.dirs})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Allocated > out[j].Allocated })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func typeList(m map[string]TypeStat) []TypeStat {
	out := make([]TypeStat, 0, len(m))
	for _, st := range m {
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Allocated > out[j].Allocated })
	if len(out) > maxTypeKeep {
		out = out[:maxTypeKeep]
	}
	return out
}

func buildTree(root string, dirs map[string]*dirAcc) *Node {
	nodes := map[string]*Node{}
	for path, d := range dirs {
		nodes[path] = &Node{Path: d.path, Name: d.name, Logical: d.logical, Allocated: d.allocated, Files: d.files}
	}
	var rootNode *Node
	for path, d := range dirs {
		n := nodes[path]
		if d.parent == "" || nodes[d.parent] == nil {
			rootNode = n
			continue
		}
		nodes[d.parent].Children = append(nodes[d.parent].Children, n)
	}
	if rootNode == nil {
		rootNode = nodes[filepath.Clean(root)]
	}
	var sortNodes func(*Node)
	sortNodes = func(n *Node) {
		if n == nil {
			return
		}
		sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Allocated > n.Children[j].Allocated })
		if len(n.Children) > 40 {
			n.Children = n.Children[:40]
		}
		for _, c := range n.Children {
			sortNodes(c)
		}
	}
	sortNodes(rootNode)
	return rootNode
}

// FilterLarge returns files at or above minBytes.
func FilterLarge(files []File, minBytes int64) []File {
	if minBytes <= 0 {
		return files
	}
	var out []File
	for _, file := range files {
		if file.Allocated >= minBytes || file.Logical >= minBytes {
			out = append(out, file)
		}
	}
	return out
}

func SortFiles(files []File, by string) {
	sort.Slice(files, func(i, j int) bool {
		switch by {
		case "type":
			if files[i].Ext == files[j].Ext {
				return files[i].Allocated > files[j].Allocated
			}
			return files[i].Ext < files[j].Ext
		case "time":
			return files[i].ModUnix > files[j].ModUnix
		case "path":
			return strings.ToLower(files[i].Path) < strings.ToLower(files[j].Path)
		default:
			return files[i].Allocated > files[j].Allocated
		}
	})
}

// HighRiskDelete reports files that this tool will not send to the recycle bin.
func HighRiskDelete(path string) bool {
	lower := strings.ToLower(strings.ReplaceAll(path, "/", `\`))
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "pagefile.sys", "hiberfil.sys", "swapfile.sys", "bootmgr":
		return true
	}
	if strings.HasSuffix(lower, ".vhdx") || strings.HasSuffix(lower, ".vhd") || strings.HasSuffix(lower, ".avhdx") || strings.HasSuffix(lower, ".vdi") || strings.HasSuffix(lower, ".sys") || strings.HasSuffix(lower, ".mdf") || strings.HasSuffix(lower, ".ldf") {
		return true
	}
	blocked := []string{`\windows\system32\`, `\windows\syswow64\`, `\windows\winsxs\`, `\program files\`, `\program files (x86)\`, `\system volume information\`}
	for _, item := range blocked {
		if strings.Contains(lower, item) {
			return true
		}
	}
	return false
}
