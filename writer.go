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

// Masks etc.
const (
	// Fields.
	fColorMapFollows = 1 << 7

	// Image fields.
	ifLocalColorTable = 1 << 7
	ifInterlace       = 1 << 6
	ifPixelSizeMask   = 7

	// Graphic control flags.
	gcTransparentColorSet = 1 << 0
)

////// END OF ALREADY EXISTENT STUFF.

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
	m            image.Image
	pm           *image.Paletted
	bitsPerPixel int
	err          error
	buf          [1024]byte // must be at least 768 so we can write color map
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
	log.Println("writing:", len(data))
	total := 0
	for total < len(data) {
		b.slice = b.tmp[1:256]
		n := copy(b.slice, data[total:])
		log.Println("Bytes copied:", n)
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

// Options are the encoding parameters.
type Options struct {
	Quantizer Quantizer
}

func EncodeAll(w io.Writer, m *gif.GIF, o *Options) error {
	return nil
}

// Encode writes the Image m to w in GIF format.
func Encode(w io.Writer, m image.Image, o *Options) error {
	// Check for bounds and size restrictions.
	b := m.Bounds()
	if b.Dx() >= 1<<16 || b.Dy() >= 1<<16 {
		return errors.New("gif: image is too large to encode")
	}
	var e encoder

	if ww, ok := w.(writer); ok {
		e.w = ww
	} else {
		e.w = bufio.NewWriter(w)
	}
	e.m = m

	e.pm, e.err = o.Quantizer.Quantize(m, 256)
	if e.err != nil {
		return e.err
	}

	// GIF87a:

	// GIF Header
	// Image Block
	// Trailer

	//  0      3 bytes  "GIF"
	//  3      3 bytes  "87a" or "89a"
	if _, e.err = io.WriteString(e.w, "GIF89a"); e.err != nil {
		return e.err
	}
	//  6      2 bytes  <Logical Screen Width>
	//  8      2 bytes  <Logical Screen Height>
	// Logical screen width and height.
	writeUint16(e.buf[:2], uint16(e.m.Bounds().Dx()))
	writeUint16(e.buf[2:4], uint16(e.m.Bounds().Dy()))
	e.write(e.buf[:4])

	// 10      1 byte   bit 0:    Global Color Table Flag (GCTF)
	//                  bit 1..3: Color Resolution
	//                  bit 4:    Sort Flag to Global Color Table
	//                  bit 5..7: Size of Global Color Table: 2^(1+n)
	log.Println("Len of palette:", len(e.pm.Palette))
	e.bitsPerPixel = log2Int256(len(e.pm.Palette)) + 1
	log.Println("Bits per pixel", e.bitsPerPixel)
	e.buf[0] = 0x80 | ((uint8(e.bitsPerPixel) - 1) << 4) | (uint8(e.bitsPerPixel) - 1)
	// 11      1 byte   <Background Color Index>
	e.buf[1] = 0x00
	// 12      1 byte   <Pixel Aspect Ratio>
	e.buf[2] = 0x00
	e.write(e.buf[:3])
	// 13      ? bytes  <Global Color Table(0..255 x 3 bytes) if GCTF is one>
	// Global Color Table.
	for i := 0; i < log2Lookup[e.bitsPerPixel-1]; i++ {
		if i < len(e.pm.Palette) {
			r, g, b, _ := e.pm.Palette[i].RGBA()
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

	// Offset   Length   Contents
	//   0      1 byte   Image Separator (0x2c)
	//   1      2 bytes  Image Left Position
	//   3      2 bytes  Image Top Position
	//   5      2 bytes  Image Width
	//   7      2 bytes  Image Height
	e.buf[0] = sImageDescriptor
	writeUint16(e.buf[1:3], uint16(e.m.Bounds().Min.X))
	writeUint16(e.buf[3:5], uint16(e.m.Bounds().Min.Y))
	writeUint16(e.buf[5:7], uint16(e.m.Bounds().Dx()))
	writeUint16(e.buf[7:9], uint16(e.m.Bounds().Dy()))

	//   8      1 byte   bit 0:    Local Color Table Flag (LCTF)
	//                   bit 1:    Interlace Flag
	//                   bit 2:    Sort Flag
	//                   bit 2..3: Reserved
	//                   bit 4..7: Size of Local Color Table: 2^(1+n)
	//          ? bytes  Local Color Table(0..255 x 3 bytes) if LCTF is one
	e.buf[9] = 0x00
	e.write(e.buf[:10])

	//          1 byte   LZW Minimum Code Size
	litWidth := e.bitsPerPixel
	if litWidth < 2 {
		litWidth = 2
	}
	e.buf[0] = byte(litWidth)
	e.write(e.buf[:1])

	// [ // Blocks
	//          1 byte   Block Size (s)
	//         (s)bytes  Image Data
	// ]*
	bw := &blockWriter{w: e.w}
	lzww := lzw.NewWriter(bw, lzw.LSB, litWidth)
	if _, err := lzww.Write(e.pm.Pix); err != nil {
		return err
	}
	lzww.Close()

	//          1 byte   Block Terminator(0x00)
	e.buf[0] = 0x00
	e.write(e.buf[:1])

	//         1 bytes  <Trailer> (0x3b)
	e.buf[0] = sTrailer
	e.write(e.buf[:1])
	log.Println("DONE")
	return e.err
}
