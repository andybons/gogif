package main

import (
	"bytes"
	"encoding/hex"
	"image"
	"image/png"
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

func TestWrite(t *testing.T) {
	m, err := readPng("teapool.png")
	if err != nil {
		t.Errorf("readPng: expected no error, but got %q\n", err.Error())
	}
	var buf bytes.Buffer
	err = Encode(&buf, m, &Options{Quantizer: &MedianCutQuantizer{}})
	if err != nil {
		t.Errorf("Encode: expected no error, but got %q\n", err.Error())
	}

	t.Log(hex.Dump(buf.Bytes()))
}
