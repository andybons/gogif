package main

import (
	"errors"
	"image"
	"image/gif"
	"io"
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

// Little-endian.
func writeUint16(b []byte, u uint16) {
	b[0] = byte(u)
	b[1] = byte(u >> 8)
}

type encoder struct {
	w   io.Writer
	m   image.Image
	pm  *image.Paletted
	err error
	buf [1024]byte // must be at least 768 so we can write color map
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
	e.w = w
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
	//  6      2 bytes  <Logical Screen Width>
	//  8      2 bytes  <Logical Screen Height>
	// 10      1 byte   bit 0:    Global Color Table Flag (GCTF)
	//                  bit 1..3: Color Resolution
	//                  bit 4:    Sort Flag to Global Color Table
	//                  bit 5..7: Size of Global Color Table: 2^(1+n)
	if _, e.err = io.WriteString(e.w, "GIF87a"); e.err != nil {
		return e.err
	}
	// Logical screen width and height.
	writeUint16(e.buf[:2], uint16(e.m.Bounds().Dx()))
	writeUint16(e.buf[2:4], uint16(e.m.Bounds().Dy()))
	e.write(e.buf[:4])

	// Write End of Image terminator.
	e.buf[0] = sTrailer
	e.write(e.buf[:1])

	return e.err
}
