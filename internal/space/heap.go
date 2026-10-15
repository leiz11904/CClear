package space

import "container/heap"

type fileHeap []File

func (h fileHeap) Len() int            { return len(h) }
func (h fileHeap) Less(i, j int) bool  { return h[i].Allocated < h[j].Allocated }
func (h fileHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *fileHeap) Push(x interface{}) { *h = append(*h, x.(File)) }
func (h *fileHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func keepLargest(h *fileHeap, item File, max int) {
	if max <= 0 {
		return
	}
	if h.Len() < max {
		heap.Push(h, item)
		return
	}
	if item.Allocated > (*h)[0].Allocated {
		(*h)[0] = item
		heap.Fix(h, 0)
	}
}

func dumpLargest(h fileHeap) []File {
	out := append([]File(nil), h...)
	SortFiles(out, "size")
	return out
}
