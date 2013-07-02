package main

import (
	"bufio"
	"compress/lzw"
	"errors"
	"image"
	"image/gif"
	"io"
	"log"
)

////// ALREADY EXISTENT IN GIF PACKAGE.
// Section indicators.
const (
	sExtension       = 0x21
	sImageDescriptor = 0x2C
	sTrailer         = 0x3B
)

////// END OF ALREADY EXISTENT STUFF.

const (
	gcLabel     = 0xF9
	gcBlockSize = 0x04
)

var log2Lookup = [8]int{2, 4, 8, 16, 32, 64, 128, 256}

func log2Int256(x int) int {
	for i, v := range log2Lookup {
		if x <= v {
			return i
		}
	}
	return -1
}

// Little-endian.
func writeUint16(b []byte, u uint16) {
	b[0] = byte(u)
	b[1] = byte(u >> 8)
}

type writer interface {
	io.Writer
	io.ByteWriter
}

type encoder struct {
	w            writer
	g            *gif.GIF
	bitsPerPixel int
	err          error
	buf          [1024]byte // must be at least 768 so we can write color map
}

func newEncoder(w io.Writer) *encoder {
	var e encoder
	if ww, ok := w.(writer); ok {
		e.w = ww
	} else {
		e.w = bufio.NewWriter(w)
	}
	return &e
}

type blockWriter struct {
	w     writer
	slice []byte
	err   error
	tmp   [256]byte
}

func (b *blockWriter) Write(data []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	if len(data) == 0 {
		return 0, nil
	}
	total := 0
	for total < len(data) {
		b.slice = b.tmp[1:256]
		n := copy(b.slice, data[total:])
		total += n
		b.tmp[0] = byte(n)

		n, b.err = b.w.Write(b.tmp[:n+1])
		if b.err != nil {
			return 0, b.err
		}
	}
	return total, b.err
}

func (e *encoder) write(p []byte) {
	if e.err != nil {
		return
	}
	_, e.err = e.w.Write(p)
}

func (e *encoder) writeHeader() {
	if e.err != nil {
		return
	}
	// TODO: GIF87a could be valid depending on the features that
	// the image uses.
	_, e.err = io.WriteString(e.w, "GIF89a")
	if e.err != nil {
		return
	}

	// TODO: This bases the global color table on the first image
	// only.
	pm := e.g.Image[0]
	// Logical screen width and height.
	writeUint16(e.buf[:2], uint16(pm.Bounds().Dx()))
	writeUint16(e.buf[2:4], uint16(pm.Bounds().Dy()))
	e.write(e.buf[:4])

	// 10      1 byte   bit 0:    Global Color Table Flag (GCTF)
	//                  bit 1..3: Color Resolution
	//                  bit 4:    Sort Flag to Global Color Table
	//                  bit 5..7: Size of Global Color Table: 2^(1+n)
	// TODO: Clean this up.
	e.bitsPerPixel = log2Int256(len(pm.Palette)) + 1
	e.buf[0] = 0x80 | ((uint8(e.bitsPerPixel) - 1) << 4) | (uint8(e.bitsPerPixel) - 1)
	e.buf[1] = 0x00 // Background Color Index.
	e.buf[2] = 0x00 // Pixel Aspect Ratio.
	e.write(e.buf[:3])
	// Global Color Table.
	for i := 0; i < log2Lookup[e.bitsPerPixel-1]; i++ {
		if i < len(pm.Palette) {
			r, g, b, _ := pm.Palette[i].RGBA()
			// TODO: If the color table is written directly, then
			// the size of e.buf is superfluously large.
			e.write([]byte{
				byte(r >> 8),
				byte(g >> 8),
				byte(b >> 8),
			})
		} else {
			// Pad with black.
			e.write([]byte{0x00, 0x00, 0x00})
		}
	}

	// Add animation info if necessary.
	if len(e.g.Image) > 1 {
		e.buf[0] = 0x21 // Extention Introducer.
		e.buf[1] = 0xff // Aplication Label.
		e.buf[2] = 0x0b // Block Size.
		e.write(e.buf[:3])
		_, e.err = io.WriteString(e.w, "NETSCAPE2.0") // Application Identifier.
		if e.err != nil {
			return
		}
		e.buf[0] = 0x03 // Block Size.
		e.buf[1] = 0x01 // Sub-block Index.
		writeUint16(e.buf[2:4], uint16(e.g.LoopCount))
		e.buf[4] = 0x00 // Block Terminator.
		e.write(e.buf[:5])
	}
}

func (e *encoder) writeImageBlock(pm *image.Paletted, delay int) {
	if e.err != nil {
		return
	}

	if delay > 0 {
		e.buf[0] = sExtension  // Extension Introducer.
		e.buf[1] = gcLabel     // Graphic Control Label.
		e.buf[2] = gcBlockSize // Block Size.
		e.buf[3] = 0x00        // TODO: Transparency support.

		writeUint16(e.buf[4:6], uint16(delay)) // Delay Time (1/100ths of a second)

		e.buf[6] = 0x00 // TODO: Transparency support.
		e.buf[7] = 0x00 // Block Terminator.
		e.write(e.buf[:8])
	}
	log.Println("bounds of image:", pm.Bounds())
	e.buf[0] = sImageDescriptor
	writeUint16(e.buf[1:3], uint16(pm.Bounds().Min.X))
	writeUint16(e.buf[3:5], uint16(pm.Bounds().Min.Y))
	writeUint16(e.buf[5:7], uint16(pm.Bounds().Dx()))
	writeUint16(e.buf[7:9], uint16(pm.Bounds().Dy()))
	e.write(e.buf[:9])

	log.Println("palette:", pm.Palette)
	if len(pm.Palette) > 0 {
		size := log2Int256(len(pm.Palette)) // Size of Local Color Table: 2^(1+n).
		// Interlacing is not supported.
		e.buf[0] = 0x80 | uint8(size)
		e.write(e.buf[:1])

		// Local Color Table.
		for i := 0; i < log2Lookup[size]; i++ {
			if i < len(pm.Palette) {
				r, g, b, _ := pm.Palette[i].RGBA()
				// TODO: If the color table is written directly, then
				// the size of e.buf is superfluously large.
				e.write([]byte{
					byte(r >> 8),
					byte(g >> 8),
					byte(b >> 8),
				})
			} else {
				// Pad with black.
				e.write([]byte{0x00, 0x00, 0x00})
			}
		}
	} else {
		e.buf[0] = 0x00
		e.write(e.buf[:1])
	}

	litWidth := e.bitsPerPixel
	if litWidth < 2 {
		litWidth = 2
	}
	e.buf[0] = byte(litWidth)
	e.write(e.buf[:1]) // LZW Minimum Code Size.

	bw := &blockWriter{w: e.w}
	lzww := lzw.NewWriter(bw, lzw.LSB, litWidth)
	_, e.err = lzww.Write(pm.Pix)
	if e.err != nil {
		lzww.Close()
		return
	}
	lzww.Close()

	e.buf[0] = 0x00 // Block Terminator.
	e.write(e.buf[:1])
}

// Options are the encoding parameters.
type Options struct {
	Quantizer Quantizer
}

// EncodeAll writes the images in g to w in GIF format with the
// given loop count and delay between frames.
func EncodeAll(w io.Writer, g *gif.GIF) error {
	e := newEncoder(w)
	e.g = g
	e.writeHeader()
	for i, pm := range g.Image {
		e.writeImageBlock(pm, g.Delay[i])
	}
	e.buf[0] = sTrailer
	e.write(e.buf[:1])
	return e.err
}

// Encode writes the Image m to w in GIF format.
func Encode(w io.Writer, m image.Image, o *Options) error {
	// Check for bounds and size restrictions.
	b := m.Bounds()
	if b.Dx() >= 1<<16 || b.Dy() >= 1<<16 {
		return errors.New("gif: image is too large to encode")
	}

	e := newEncoder(w)

	var pm *image.Paletted
	pm, e.err = o.Quantizer.Quantize(m)
	if e.err != nil {
		return e.err
	}
	e.g = &gif.GIF{Image: []*image.Paletted{pm}}

	e.writeHeader()
	e.writeImageBlock(pm, 0)

	e.buf[0] = sTrailer
	e.write(e.buf[:1])
	return e.err
}
