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

func min(x, y uint32) uint32 {
	if x < y {
		return x
	}
	return y
}

func max(x, y uint32) uint32 {
	if x > y {
		return x
	}
	return y
}

type point struct {
	x [numDimensions]uint32
}

type block struct {
	minCorner, maxCorner point
	points               []point
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

func newBlock(p []point) *block {
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

func (b *block) longestSideLength() uint32 {
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

func (ps *pointSorter) Less(i, j int) bool {
	return ps.by(&ps.points[i], &ps.points[j])
}

// A PriorityQueue implements heap.Interface and holds blocks.
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
	Quantize(m image.Image, numColor int) (*image.Paletted, error)
}

type MedianCutQuantizer struct{}

func (q *MedianCutQuantizer) medianCut(points []point, numColor int) color.Palette {
	initialBlock := newBlock(points)
	initialBlock.shrink()
	pq := &PriorityQueue{}
	heap.Init(pq)
	heap.Push(pq, initialBlock)

	for pq.Len() < numColor && len(pq.Top().(*block).points) > 1 {
		longestBlock := heap.Pop(pq).(*block)
		points := longestBlock.points

		// Instead of sorting the entire slice, finding the median using an algorithm
		// like introselect would give much better performance. Do before submission.
		func(li int) {
			By(func(p1, p2 *point) bool { return p1.x[li] < p2.x[li] }).Sort(points)
		}(longestBlock.longestSideIndex())
		median := len(points) / 2
		block1 := newBlock(points[0:median])
		block2 := newBlock(points[median:])
		block1.shrink()
		block2.shrink()
		heap.Push(pq, block1)
		heap.Push(pq, block2)
	}

	palette := make(color.Palette, numColor)
	var n int
	for n = 0; pq.Len() > 0; n++ {
		block := heap.Pop(pq).(*block)
		sum := make([]uint32, numDimensions)
		for i := 0; i < len(block.points); i++ {
			for j := 0; j < numDimensions; j++ {
				sum[j] += block.points[i].x[j]
			}
		}
		var avgPoint point
		for j := 0; j < numDimensions; j++ {
			avgPoint.x[j] = sum[j] / uint32(len(block.points))
		}
		palette[n] = color.RGBA64{
			R: uint16(avgPoint.x[0]),
			G: uint16(avgPoint.x[1]),
			B: uint16(avgPoint.x[2]),
			A: 0xFFFF,
		}
	}
	// Trim to only the colors present in the image, which
	// could be less than numColor.
	return palette[:n]
}

func (q *MedianCutQuantizer) Quantize(m image.Image, numColor int) (*image.Paletted, error) {
	bounds := m.Bounds()
	points := make([]point, bounds.Dx()*bounds.Dy())
	colorSet := make(map[string]color.Color, numColor)
	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := m.At(x, y)
			r, g, b, _ := c.RGBA()
			colorSet[fmt.Sprintf("%d,%d,%d", r, g, b)] = c
			points[i].x[0] = r
			points[i].x[1] = g
			points[i].x[2] = b
			i++
		}
	}
	var palette color.Palette
	if len(colorSet) <= numColor {
		// No need to quantize since the total number of colors
		// fits within the limit.
		palette = make(color.Palette, len(colorSet))
		i := 0
		for _, c := range colorSet {
			palette[i] = c
			i++
		}
	} else {
		palette = q.medianCut(points, numColor)
	}

	pm := image.NewPaletted(m.Bounds(), palette)
	pm.Stride = m.Bounds().Dx()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pm.Set(x, y, m.At(x, y))
		}
	}

	return pm, nil
}

func main() {
	f, err := os.Open("scape.gif")
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
	fmt.Println("Serving result image at http://locahost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

var pImage *image.Paletted

func handleIndex(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "image/jpeg")
	jpeg.Encode(w, pImage, nil)
}
