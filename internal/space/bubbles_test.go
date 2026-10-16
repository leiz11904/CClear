package space

import (
	"math"
	"testing"
)

func TestPackBubblesNoOverlapAndInside(t *testing.T) {
	weights := []float64{70, 10, 8, 5, 3, 2, 1, 0.4}
	const w, h = 640.0, 420.0
	got := PackBubbles(weights, w, h)
	if len(got) != len(weights) {
		t.Fatalf("count %d", len(got))
	}
	for i, a := range got {
		if a.R <= 0 {
			t.Fatalf("radius %v", a)
		}
		if a.X-a.R < -0.5 || a.Y-a.R < -0.5 || a.X+a.R > w+0.5 || a.Y+a.R > h+0.5 {
			t.Fatalf("outside %+v", a)
		}
		for _, b := range got[i+1:] {
			d := math.Hypot(a.X-b.X, a.Y-b.Y)
			if d+0.2 < a.R+b.R {
				t.Fatalf("overlap d=%.2f r=%.2f+%.2f", d, a.R, b.R)
			}
		}
	}
	byIndex := map[int]Bubble{}
	for _, b := range got {
		byIndex[b.Index] = b
	}
	if byIndex[0].R <= byIndex[1].R || byIndex[1].R <= byIndex[len(weights)-1].R {
		t.Fatalf("radius not ordered by weight: %+v", byIndex)
	}
	ratio := byIndex[0].R / byIndex[1].R
	want := math.Sqrt(weights[0] / weights[1])
	if math.Abs(ratio-want) > 0.02 {
		t.Fatalf("area ratio %.3f want %.3f", ratio, want)
	}
}

func TestPackBubblesOneCentered(t *testing.T) {
	got := PackBubbles([]float64{4}, 200, 100)
	if len(got) != 1 {
		t.Fatal(got)
	}
	b := got[0]
	if math.Abs(b.X-100) > 1 || math.Abs(b.Y-50) > 1 {
		t.Fatalf("center %+v", b)
	}
	if b.R < 40 || b.R > 50 {
		t.Fatalf("radius %v", b.R)
	}
}

func TestPackBubblesSkipsEmpty(t *testing.T) {
	if PackBubbles(nil, 100, 100) != nil {
		t.Fatal("nil weights")
	}
	if PackBubbles([]float64{0, -1}, 100, 100) != nil {
		t.Fatal("non positive")
	}
	if PackBubbles([]float64{1}, 0, 10) != nil {
		t.Fatal("no box")
	}
}
