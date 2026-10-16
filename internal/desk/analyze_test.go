package desk

import (
	"math"
	"testing"

	"cclear/internal/space"
)

func TestLevelFromDrillAndBubbles(t *testing.T) {
	root := &space.Node{
		Path: `C:\`, Name: `C:\`, Allocated: 100, Files: 10,
		Children: []*space.Node{
			{Path: `C:\Users`, Name: "Users", Allocated: 70, Files: 6, Children: []*space.Node{
				{Path: `C:\Users\a`, Name: "a", Allocated: 40, Files: 2},
			}},
			{Path: `C:\Windows`, Name: "Windows", Allocated: 30, Files: 4},
		},
	}
	top := LevelFrom(root, "")
	if len(top.Entries) != 2 || top.Parent != "" || len(top.Crumbs) != 1 {
		t.Fatalf("top %+v", top)
	}
	if top.Entries[0].R <= top.Entries[1].R {
		t.Fatalf("bubble size %+v", top.Entries)
	}
	for i, e := range top.Entries {
		if e.R <= 0 {
			continue
		}
		for _, o := range top.Entries[i+1:] {
			if o.R <= 0 {
				continue
			}
			d := math.Hypot(e.X-o.X, e.Y-o.Y)
			if d+0.2 < e.R+o.R {
				t.Fatalf("overlap %v %v", e, o)
			}
		}
	}
	next := LevelFrom(root, `C:\Users`)
	if next.Parent != `C:\` || len(next.Entries) != 1 || next.Entries[0].Name != "a" {
		t.Fatalf("drill %+v", next)
	}
	if len(next.Crumbs) != 2 {
		t.Fatalf("crumbs %+v", next.Crumbs)
	}
}
