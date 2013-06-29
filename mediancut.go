package main

import (
	"container/heap"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"sort"
)

const (
	numDimensions = 3
)

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

type point struct {
	x [numDimensions]int
}

type block struct {
	minCorner, maxCorner point
	points               []point
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

func NewBlock(p []point) *block {
	b := &block{points: p}
	for i := 0; i < numDimensions; i++ {
		b.minCorner.x[i] = 0
		b.maxCorner.x[i] = 0xFF
	}
	return b
}

func (b *block) longestSideIndex() int {
	m := b.maxCorner.x[0] - b.minCorner.x[0]
	maxIndex := 0
	for i := 1; i < numDimensions; i++ {
		diff := b.maxCorner.x[i] - b.minCorner.x[i]
		if diff > m {
			m = diff
			maxIndex = i
		}
	}
	return maxIndex
}

func (b *block) longestSideLength() int {
	i := b.longestSideIndex()
	return b.maxCorner.x[i] - b.minCorner.x[i]
}

func (b *block) shrink() {
	for j := 0; j < numDimensions; j++ {
		b.minCorner.x[j] = b.points[0].x[j]
		b.maxCorner.x[j] = b.points[0].x[j]
	}
	for i := 1; i < len(b.points); i++ {
		for j := 0; j < numDimensions; j++ {
			b.minCorner.x[j] = min(b.minCorner.x[j], b.points[i].x[j])
			b.maxCorner.x[j] = max(b.maxCorner.x[j], b.points[i].x[j])
		}
	}
}

type By func(p1, p2 *point) bool

func (by By) Sort(points []point) {
	ps := &pointSorter{
		points: points,
		by:     by,
	}
	sort.Sort(ps)
}

type pointSorter struct {
	points []point
	by     func(p1, p2 *point) bool
}

func (ps *pointSorter) Len() int {
	return len(ps.points)
}

func (ps *pointSorter) Swap(i, j int) {
	ps.points[i], ps.points[j] = ps.points[j], ps.points[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (ps *pointSorter) Less(i, j int) bool {
	return ps.by(&ps.points[i], &ps.points[j])
}

// A PriorityQueue implements heap.Interface and holds Blocks.
type PriorityQueue []*block

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].longestSideLength() > pq[j].longestSideLength()
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*block)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) Top() interface{} {
	n := len(*pq)
	if n == 0 {
		return nil
	}
	return (*pq)[n-1]
}

type Quantizer interface {
	Quantize(m image.Image, numColor int) (image.PalettedImage, error)
}

type MedianCutQuantizer struct{}

func (q *MedianCutQuantizer) Quantize(m image.Image, numColor int) (*image.Paletted, error) {
	bounds := m.Bounds()
	points := make([]point, bounds.Dx()*bounds.Dy())
	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := m.At(x, y).RGBA()
			points[i].x[0] = int(r)
			points[i].x[1] = int(g)
			points[i].x[2] = int(b)
			i++
		}
	}
	initialBlock := NewBlock(points)
	initialBlock.shrink()
	pq := &PriorityQueue{}
	heap.Init(pq)
	heap.Push(pq, initialBlock)

	for pq.Len() < numColor && len(pq.Top().(*block).points) > 1 {
		longestBlock := heap.Pop(pq).(*block)
		points := longestBlock.points

		// selection-sort would be much more efficient than a full-sort, here.
		func(li int) {
			By(func(p1, p2 *point) bool { return p1.x[li] < p2.x[li] }).Sort(points)
		}(longestBlock.longestSideIndex())
		median := len(points) / 2
		block1 := NewBlock(points[0:median])
		block2 := NewBlock(points[median:])
		block1.shrink()
		block2.shrink()
		heap.Push(pq, block1)
		heap.Push(pq, block2)
	}

	results := make([]point, numColor)
	for n := 0; pq.Len() > 0; n++ {
		block := heap.Pop(pq).(*block)
		sum := make([]int, numDimensions)
		for i := 0; i < len(block.points); i++ {
			for j := 0; j < numDimensions; j++ {
				sum[j] += block.points[i].x[j]
			}
		}
		var avgPoint point
		for j := 0; j < numDimensions; j++ {
			avgPoint.x[j] = sum[j] / len(block.points)
		}
		results[n] = avgPoint
	}
	palette := make(color.Palette, len(results))
	for i, r := range results {
		palette[i] = color.RGBA64{
			R: uint16(r.x[0]),
			G: uint16(r.x[1]),
			B: uint16(r.x[2]),
			A: 0xFFFF,
		}
		fmt.Printf("[%d,%d,%d],\n", r.x[0]>>8, r.x[1]>>8, r.x[2]>>8)
	}
	pm := image.NewPaletted(m.Bounds(), palette)
	pm.Stride = m.Bounds().Dy()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pm.Set(x, y, m.At(x, y))
		}
	}

	return pm, nil
}

func main() {
	f, err := os.Open("wow.jpg")
	if err != nil {
		log.Fatalf("os.Open: %q", err)
	}
	defer f.Close()

	m, _, err := image.Decode(f)
	if err != nil {
		log.Fatalf("image.Decode: %q\n", err)
	}

	q := MedianCutQuantizer{}
	pImage, _ = q.Quantize(m, 256)

	http.HandleFunc("/", handleIndex)
	fmt.Println("Serving on http://locahost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

var pImage *image.Paletted

func handleIndex(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	jpeg.Encode(w, pImage, nil)
}
