package desk

import (
	"strings"

	"cclear/internal/space"
)

// The chart fills the map beside the directory list.
const lensW = 1000.0
const lensH = 640.0

// Volume is the live capacity of the system drive, not a scan result.
type Volume struct {
	Drive string `json:"drive"`
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
	Used  uint64 `json:"used"`
}

// Status is the scan lifecycle. It never includes cleanup or delete state.
type Status struct {
	Running   bool   `json:"running"`
	Ready     bool   `json:"ready"`
	Cancelled bool   `json:"cancelled"`
	Files     int    `json:"files"`
	Allocated int64  `json:"allocated"`
	Note      string `json:"note"`
	Error     string `json:"error"`
	Where     string `json:"where"`
}

// Crumb is one step from the scan root to the current directory.
type Crumb struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Entry is one real child directory of the current level.
type Entry struct {
	Name      string  `json:"name"`
	Path      string  `json:"path"`
	Allocated int64   `json:"allocated"`
	Files     int     `json:"files"`
	Percent   float64 `json:"percent"`
	Color     int     `json:"color"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	R         float64 `json:"r"`
	TX        float64 `json:"tx"`
	TY        float64 `json:"ty"`
	TW        float64 `json:"tw"`
	TH        float64 `json:"th"`
}

// Level is the current directory and the children the scan kept.
type Level struct {
	Path      string  `json:"path"`
	Name      string  `json:"name"`
	Allocated int64   `json:"allocated"`
	Files     int     `json:"files"`
	Parent    string  `json:"parent"`
	Capped    bool    `json:"capped"`
	Crumbs    []Crumb `json:"crumbs"`
	Entries   []Entry `json:"entries"`
}

// TypeRow is extension occupancy for the whole scan, not the current folder.
type TypeRow struct {
	Ext       string  `json:"ext"`
	Files     int     `json:"files"`
	Allocated int64   `json:"allocated"`
	Percent   float64 `json:"percent"`
}

// FileRow is one large file kept by the scan. It is not a cleanup candidate.
type FileRow struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Allocated int64  `json:"allocated"`
}

// LevelFrom builds one directory view from an existing scan tree.
func LevelFrom(tree *space.Node, path string) Level {
	out := Level{Crumbs: []Crumb{}, Entries: []Entry{}}
	if tree == nil {
		return out
	}
	chain := ancestry(tree, path)
	if len(chain) == 0 {
		chain = []*space.Node{tree}
	}
	node := chain[len(chain)-1]
	out.Path = node.Path
	out.Name = node.Name
	if out.Name == "" {
		out.Name = node.Path
	}
	out.Allocated = node.Allocated
	out.Files = node.Files
	if len(chain) > 1 {
		out.Parent = chain[len(chain)-2].Path
	}
	for _, n := range chain {
		name := n.Name
		if name == "" {
			name = n.Path
		}
		out.Crumbs = append(out.Crumbs, Crumb{Name: name, Path: n.Path})
	}
	kids := make([]*space.Node, 0, len(node.Children))
	for _, c := range node.Children {
		if c != nil {
			kids = append(kids, c)
		}
	}
	out.Capped = len(kids) >= 40
	weights := make([]float64, len(kids))
	for i, c := range kids {
		if c.Allocated > 0 {
			weights[i] = float64(c.Allocated)
		}
	}
	packed := space.PackBubbles(weights, lensW, lensH)
	bubbleAt := map[int]space.Bubble{}
	for _, b := range packed {
		bubbleAt[b.Index] = b
	}
	cells := space.LayoutTreemap(kids, 0, 0, lensW, lensH)
	cellAt := map[string]space.Cell{}
	for _, c := range cells {
		cellAt[strings.ToLower(c.Path)] = c
	}
	for i, c := range kids {
		name := c.Name
		if name == "" {
			name = c.Path
		}
		pct := 0.0
		if node.Allocated > 0 && c.Allocated > 0 {
			pct = float64(c.Allocated) / float64(node.Allocated) * 100
		}
		entry := Entry{
			Name: name, Path: c.Path, Allocated: c.Allocated, Files: c.Files,
			Percent: pct, Color: i % 8,
		}
		if b, ok := bubbleAt[i]; ok {
			entry.X, entry.Y, entry.R = b.X, b.Y, b.R
		}
		if cell, ok := cellAt[strings.ToLower(c.Path)]; ok {
			entry.TX, entry.TY, entry.TW, entry.TH = cell.X, cell.Y, cell.W, cell.H
		}
		out.Entries = append(out.Entries, entry)
	}
	return out
}

func ancestry(root *space.Node, path string) []*space.Node {
	if root == nil {
		return nil
	}
	if path == "" || strings.EqualFold(root.Path, path) {
		return []*space.Node{root}
	}
	for _, child := range root.Children {
		if child == nil {
			continue
		}
		if sub := ancestry(child, path); len(sub) > 0 {
			return append([]*space.Node{root}, sub...)
		}
	}
	return nil
}

// TypesFrom converts the scan's extension totals.
func TypesFrom(rep space.Report) []TypeRow {
	out := make([]TypeRow, 0, len(rep.Types))
	for _, st := range rep.Types {
		ext := st.Ext
		if ext == "" {
			ext = "无扩展名"
		}
		pct := 0.0
		if rep.Allocated > 0 && st.Allocated > 0 {
			pct = float64(st.Allocated) / float64(rep.Allocated) * 100
		}
		out = append(out, TypeRow{Ext: ext, Files: st.Files, Allocated: st.Allocated, Percent: pct})
	}
	return out
}

// LargeFrom returns the largest files the scan retained.
func LargeFrom(rep space.Report, limit int) []FileRow {
	if limit <= 0 || limit > len(rep.Large) {
		limit = len(rep.Large)
	}
	out := make([]FileRow, 0, limit)
	for _, f := range rep.Large[:limit] {
		out = append(out, FileRow{Name: f.Name, Path: f.Path, Allocated: f.Allocated})
	}
	return out
}
