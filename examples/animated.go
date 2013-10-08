package main

import (
	"image"
	"image/gif"
	"log"
	"os"

	"github.com/andybons/gogif"
	"github.com/nfnt/resize"
)

func main() {
	f, err := os.Open("testdata/shapes.gif")
	if err != nil {
		log.Fatal(err.Error())
	}
	defer f.Close()

	im, err := gif.DecodeAll(f)
	if err != nil {
		log.Fatal(err.Error())
	}

	for i, frame := range im.Image {
		im.Image[i] = ImageToPaletted(ProcessImage(frame))
	}

	out, err := os.Create("shapes.out.gif")
	if err != nil {
		log.Fatal(err.Error())
	}

	defer out.Close()

	gogif.EncodeAll(out, im)
}

func ProcessImage(img image.Image) image.Image {
	return resize.Resize(200, 0, img, resize.Bilinear)
}

// Converts an image to an image.Paletted with 256 colors.
func ImageToPaletted(img image.Image) *image.Paletted {
	pm, ok := img.(*image.Paletted)
	if !ok {
		b := img.Bounds()
		pm = image.NewPaletted(b, nil)
		q := &gogif.MedianCutQuantizer{NumColor: 256}
		q.Quantize(pm, b, img, image.ZP)
	}
	return pm
}
