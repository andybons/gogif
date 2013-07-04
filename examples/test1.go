package main

import (
	"bytes"
	"gogif"
	"image"
	"image/draw"
	"image/gif"
	"image/png"
	"io/ioutil"
	"log"
	"os"
)

// clip clips r against each image's bounds (after translating into the
// destination image's co-ordinate space) and shifts the point sp by
// the same amount as the change in r.Min.
func clip(dst draw.Image, r *image.Rectangle, src image.Image, sp *image.Point) {
	orig := r.Min
	*r = r.Intersect(dst.Bounds())
	*r = r.Intersect(src.Bounds().Add(orig.Sub(*sp)))
	dx := r.Min.X - orig.X
	dy := r.Min.Y - orig.Y
	if dx == 0 && dy == 0 {
		return
	}
	(*sp).X += dx
	(*sp).Y += dy
}

func main() {
	f, err := os.Open("testdata/scape.gif")
	if err != nil {
		log.Fatalf("os.Open: %q", err)
	}
	defer f.Close()

	g, err := gif.DecodeAll(f)
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	if err := gogif.Encode(&buf, g.Image[0], nil); err != nil {
		log.Fatal(err)
	}
	ioutil.WriteFile("test1.gif", buf.Bytes(), 0660)

	buf.Reset()
	m := g.Image[0]
	draw.Draw(m, m.Bounds(), m, image.Pt(100, 100), draw.Src)

	b := m.Bounds()
	b.Min.X = 5
	b.Min.Y = 5
	p := image.Pt(40, 20)
	log.Printf("%v %v %v", m.Bounds(), b, p)
	clip(m, &b, m, &p)
	log.Printf("%v %v %v", m.Bounds(), b, p)

	if err := png.Encode(&buf, m); err != nil {
		log.Fatal(err)
	}
	ioutil.WriteFile("test2.png", buf.Bytes(), 0660)
}
