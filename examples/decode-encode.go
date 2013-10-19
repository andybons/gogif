package main

import (
	"bytes"
	"image/gif"
	"io/ioutil"
	"log"
	"os"

	"github.com/andybons/gogif"
)

func main() {
	decodeThenEncode("scape.gif")
	decodeThenEncode("blob.gif")
	decodeThenEncode("shapes.gif")
	decodeThenEncode("shipit.gif")
}

func decodeThenEncode(filename string) {
	f, err := os.Open("testdata/" + filename)
	if err != nil {
		log.Fatalf("os.Open: %q", err)
	}
	defer f.Close()

	g, err := gif.DecodeAll(f)
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	if err := gogif.EncodeAll(&buf, g); err != nil {
		log.Fatal(err)
	}
	log.Println("Writing", filename+".out.gif")
	ioutil.WriteFile(filename+".out.gif", buf.Bytes(), 0660)
}
