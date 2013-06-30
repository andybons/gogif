package main

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

func TestPalettes(t *testing.T) {

}

func TestWrite(t *testing.T) {
	m, err := readGif("scape.gif")
	if err != nil {
		t.Errorf("readGif: expected no error, but got %q\n", err.Error())
	}
	var buf bytes.Buffer
	err = Encode(&buf, m, &Options{Quantizer: &MedianCutQuantizer{}})
	if err != nil {
		t.Errorf("Encode: expected no error, but got %q\n", err.Error())
	}
	ioutil.WriteFile("test1.gif", buf.Bytes(), 0660)
	//t.Log("\n", hex.Dump(buf.Bytes()))
}

func TestWrite2(t *testing.T) {
	m, err := readPng("teapool.png")
	if err != nil {
		t.Errorf("readPng: expected no error, but got %q\n", err.Error())
	}
	var buf bytes.Buffer
	err = Encode(&buf, m, &Options{Quantizer: &MedianCutQuantizer{}})
	if err != nil {
		t.Errorf("Encode: expected no error, but got %q\n", err.Error())
	}
	ioutil.WriteFile("test2.gif", buf.Bytes(), 0660)
	//t.Log("\n", hex.Dump(buf.Bytes()))
}
