package space

// Cell is one rectangle in a treemap. Area is proportional to Allocated.
type Cell struct {
	Path       string
	Name       string
	Allocated  int64
	X, Y, W, H float64
}

// LayoutTreemap packs nodes into a rectangle using a slice-and-dice layout.
func LayoutTreemap(nodes []*Node, x, y, w, h float64) []Cell {
	var items []Cell
	var total int64
	for _, n := range nodes {
		if n != nil && n.Allocated > 0 {
			items = append(items, Cell{Path: n.Path, Name: n.Name, Allocated: n.Allocated})
			total += n.Allocated
		}
	}
	if total <= 0 || w <= 0 || h <= 0 || len(items) == 0 {
		return nil
	}
	return split(items, total, x, y, w, h, true)
}

func split(items []Cell, total int64, x, y, w, h float64, vertical bool) []Cell {
	if len(items) == 0 || total <= 0 {
		return nil
	}
	if len(items) == 1 {
		items[0].X, items[0].Y, items[0].W, items[0].H = x, y, w, h
		return items
	}
	var left []Cell
	var sum int64
	half := total / 2
	i := 0
	for i < len(items)-1 && sum+items[i].Allocated <= half {
		sum += items[i].Allocated
		left = append(left, items[i])
		i++
	}
	if len(left) == 0 {
		sum = items[0].Allocated
		left = items[:1]
		i = 1
	}
	right := items[i:]
	rest := total - sum
	if vertical {
		lw := w * float64(sum) / float64(total)
		return append(split(left, sum, x, y, lw, h, false), split(right, rest, x+lw, y, w-lw, h, false)...)
	}
	lh := h * float64(sum) / float64(total)
	return append(split(left, sum, x, y, w, lh, true), split(right, rest, x, y+lh, w, h-lh, true)...)
}

func HitCell(cells []Cell, px, py float64) *Cell {
	for i := range cells {
		c := &cells[i]
		if px >= c.X && py >= c.Y && px < c.X+c.W && py < c.Y+c.H {
			return c
		}
	}
	return nil
}
