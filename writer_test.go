package gogif

import (
	"bytes"
	_ "encoding/hex"
	"image"
	"image/gif"
	"image/png"
	"io/ioutil"
	"os"
	"testing"
)

func readPng(filename string) (image.Image, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func readGif(filename string) (image.Image, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return gif.Decode(f)
}

func delta(u0, u1 uint32) int64 {
	d := int64(u0) - int64(u1)
	if d < 0 {
		return -d
	}
	return d
}

// averageDelta returns the average delta in RGB space. The two images must
// have the same bounds.
func averageDelta(m0, m1 image.Image) int64 {
	b := m0.Bounds()
	var sum, n int64
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c0 := m0.At(x, y)
			c1 := m1.At(x, y)
			r0, g0, b0, _ := c0.RGBA()
			r1, g1, b1, _ := c1.RGBA()
			sum += delta(r0, r1)
			sum += delta(g0, g1)
			sum += delta(b0, b1)
			n += 3
		}
	}
	return sum / n
}

var testCase = []struct {
	filename  string
	numColor  int
	tolerance int64
}{
	{"testdata/teapool.png", 256, 24 << 8},
}

// TODO:
// zero color quantizer.
// Varying image widths and heights.
// Transparency within a gif.
// Mismatched delay and image lengths when encoding an animated gif.

func TestWriter(t *testing.T) {
	for _, tc := range testCase {
		m0, err := readPng(tc.filename)
		if err != nil {
			t.Error(tc.filename, err)
		}
		var buf bytes.Buffer
		err = Encode(&buf, m0, &Options{Quantizer: &MedianCutQuantizer{NumColor: 256}})
		if err != nil {
			t.Error(tc.filename, err)
		}
		m1, err := gif.Decode(&buf)
		if err != nil {
			t.Error(tc.filename, err)
		}
		if m0.Bounds() != m1.Bounds() {
			t.Errorf("%s, bounds differ: %v and %v", tc.filename, m0.Bounds(), m1.Bounds())
		}
		// Compare the average delta to the tolerance level.
		t.Log("avg delta:", averageDelta(m0, m1))
		if averageDelta(m0, m1) > tc.tolerance {
			t.Errorf("%s, numColor=%d: average delta is too high", tc.filename, tc.numColor)
			continue
		}
	}
}

func TestAnimatedWriter(t *testing.T) {
	f, err := os.Open("testdata/scape.gif")
	if err != nil {
		t.Errorf("os.Open: %q", err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Error(err)
	}
	var buf bytes.Buffer
	if err := EncodeAll(&buf, g); err != nil {
		t.Error(err)
	}
	ioutil.WriteFile("test3.gif", buf.Bytes(), 0660)
}

func TestWrite2(t *testing.T) {
	m, err := readPng("testdata/teapool.png")
	if err != nil {
		t.Errorf("readPng: expected no error, but got %q\n", err.Error())
	}
	var buf bytes.Buffer
	err = Encode(&buf, m, &Options{Quantizer: &MedianCutQuantizer{NumColor: 256}})
	if err != nil {
		t.Errorf("Encode: expected no error, but got %q\n", err.Error())
	}
	ioutil.WriteFile("test2.gif", buf.Bytes(), 0660)
	//t.Log("\n", hex.Dump(buf.Bytes()))
}

func TestWrite1(t *testing.T) {
	m, err := readGif("testdata/scape.gif")
	if err != nil {
		t.Errorf("readPng: expected no error, but got %q\n", err.Error())
	}
	var buf bytes.Buffer
	err = Encode(&buf, m, &Options{Quantizer: &MedianCutQuantizer{NumColor: 256}})
	if err != nil {
		t.Errorf("Encode: expected no error, but got %q\n", err.Error())
	}
	ioutil.WriteFile("test1.gif", buf.Bytes(), 0660)
	//t.Log("\n", hex.Dump(buf.Bytes()))
}
