package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"

	streamdeck "github.com/NJannasch/streamdeck-go"
)

func main() {
	key := flag.Int("key", 0, "key to update")
	panel := flag.Bool("panel", false, "tile the image across the complete key grid")
	lcd := flag.Bool("lcd", false, "display the image across the physical LCD")
	window := flag.Bool("window", false, "display the image in the Neo/Plus window")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("usage: go run ./examples/image [-key N | -panel | -lcd | -window] IMAGE")
	}

	file, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	img, format, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	deck, err := streamdeck.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer deck.Close()

	if *lcd {
		err = deck.SetLCDImage(img)
	} else if *window {
		err = deck.SetWindowImage(img)
	} else if *panel {
		err = deck.SetPanelImage(img)
	} else {
		err = deck.SetKeyImage(*key, img)
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("displayed %s image on %s\n", format, deck.Info().Model.Name)
}
