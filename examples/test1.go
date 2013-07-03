package main

import (
	"fmt"
	"gogif"
	"image"
	"image/gif"
	"log"
	"net/http"
	"os"
)

func main() {
	f, err := os.Open("testdata/scape.gif")
	if err != nil {
		log.Fatalf("os.Open: %q", err)
	}
	defer f.Close()

	g, err := gif.DecodeAll(f)
	if err != nil {
		log.Fatalf("gif.DecodeAll: %q\n", err)
	}
	log.Printf("num images: %d, delay: %v, loopcount: %d", len(g.Image), g.Delay, g.LoopCount)

	m = g.Image[0]

	http.HandleFunc("/", handleIndex)
	fmt.Println("Serving result image at http://locahost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

var m image.Image

func handleIndex(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "image/gif")
	gogif.Encode(w, m, nil)
}
