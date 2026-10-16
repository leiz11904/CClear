package space

import "math"

// Bubble is one circle in a space-lens layout. Area is proportional to Weight.
// X and Y are the center, in the same coordinate space as the requested box.
type Bubble struct {
	Index  int
	Weight float64
	X, Y   float64
	R      float64
}

// PackBubbles places one circle per positive weight inside the box.
// Circles do not overlap. A larger weight always gets a larger radius.
// The result is empty when the box or the weights cannot form a circle.
func PackBubbles(weights []float64, width, height float64) []Bubble {
	if width < 4 || height < 4 {
		return nil
	}
	var circles []circle
	for i, w := range weights {
		if w <= 0 || math.IsNaN(w) || math.IsInf(w, 0) {
			continue
		}
		circles = append(circles, circle{i: i, r: math.Sqrt(w) * 0.92, w: w})
	}
	if len(circles) == 0 {
		return nil
	}
	// Largest first, so the biggest directories sit near the middle.
	for i := 1; i < len(circles); i++ {
		for j := i; j > 0 && circles[j].r > circles[j-1].r; j-- {
			circles[j], circles[j-1] = circles[j-1], circles[j]
		}
	}
	placeCircles(circles)
	return fitCircles(circles, width, height)
}

type circle struct {
	i    int
	r    float64
	w    float64
	x, y float64
}

func placeCircles(cs []circle) {
	cs[0].x, cs[0].y = 0, 0
	if len(cs) == 1 {
		return
	}
	cs[1].x = cs[0].r + cs[1].r
	cs[1].y = 0
	for i := 2; i < len(cs); i++ {
		best := math.Inf(1)
		var bx, by float64
		found := false
		for j := 0; j < i; j++ {
			for k := j + 1; k < i; k++ {
				p1, p2, ok := tangent(cs[j], cs[k], cs[i].r)
				if !ok {
					continue
				}
				for _, p := range [][2]float64{p1, p2} {
					if overlaps(cs[:i], p[0], p[1], cs[i].r) {
						continue
					}
					d := p[0]*p[0] + p[1]*p[1]
					if d < best {
						best = d
						bx, by = p[0], p[1]
						found = true
					}
				}
			}
		}
		if !found {
			far := 0.0
			for _, c := range cs[:i] {
				if d := math.Hypot(c.x, c.y) + c.r; d > far {
					far = d
				}
			}
			bx = far + cs[i].r
			by = 0
		}
		cs[i].x, cs[i].y = bx, by
	}
}

func tangent(a, b circle, r float64) (p1, p2 [2]float64, ok bool) {
	dx := b.x - a.x
	dy := b.y - a.y
	d := math.Hypot(dx, dy)
	if d < 1e-6 {
		return
	}
	ra := a.r + r
	rb := b.r + r
	if d > ra+rb || d < math.Abs(ra-rb) {
		return
	}
	x := (ra*ra - rb*rb + d*d) / (2 * d)
	h2 := ra*ra - x*x
	if h2 < 0 {
		h2 = 0
	}
	h := math.Sqrt(h2)
	mx := a.x + dx*x/d
	my := a.y + dy*x/d
	rx := -dy * h / d
	ry := dx * h / d
	return [2]float64{mx + rx, my + ry}, [2]float64{mx - rx, my - ry}, true
}

func overlaps(cs []circle, x, y, r float64) bool {
	const eps = 0.05
	for _, c := range cs {
		if math.Hypot(c.x-x, c.y-y)+eps < c.r+r {
			return true
		}
	}
	return false
}

func fitCircles(cs []circle, width, height float64) []Bubble {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, c := range cs {
		if c.x-c.r < minX {
			minX = c.x - c.r
		}
		if c.y-c.r < minY {
			minY = c.y - c.r
		}
		if c.x+c.r > maxX {
			maxX = c.x + c.r
		}
		if c.y+c.r > maxY {
			maxY = c.y + c.r
		}
	}
	bw := maxX - minX
	bh := maxY - minY
	if bw < 1 {
		bw = 1
	}
	if bh < 1 {
		bh = 1
	}
	pad := math.Min(width, height) * 0.03
	if pad < 2 {
		pad = 2
	}
	scale := math.Min((width-2*pad)/bw, (height-2*pad)/bh)
	if scale <= 0 {
		return nil
	}
	cx := (minX + maxX) / 2
	cy := (minY + maxY) / 2
	out := make([]Bubble, len(cs))
	for i, c := range cs {
		out[i] = Bubble{
			Index:  c.i,
			Weight: c.w,
			X:      width/2 + (c.x-cx)*scale,
			Y:      height/2 + (c.y-cy)*scale,
			R:      c.r * scale,
		}
	}
	return out
}
