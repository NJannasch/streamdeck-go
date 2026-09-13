package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"

	streamdeck "github.com/NJannasch/streamdeck-go"
)

func main() {
	key := flag.Int("key", -1, "key to update; -1 updates every key")
	flag.Parse()

	deck, err := streamdeck.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer deck.Close()

	model := deck.Info().Model
	if *key >= model.KeyCount() {
		log.Fatalf("key %d is outside 0..%d", *key, model.KeyCount()-1)
	}

	first, last := *key, *key
	if *key < 0 {
		first, last = 0, model.KeyCount()-1
	}
	for index := first; index <= last; index++ {
		img := smiley(model.KeyWidth, model.KeyHeight, backgrounds[index%len(backgrounds)])
		if err := deck.SetKeyImage(index, img); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("drew smileys on keys %d..%d\n", first, last)
}

var backgrounds = []color.RGBA{
	{R: 24, G: 45, B: 86, A: 255},
	{R: 75, G: 29, B: 96, A: 255},
	{R: 12, G: 84, B: 69, A: 255},
	{R: 108, G: 45, B: 32, A: 255},
	{R: 58, G: 58, B: 64, A: 255},
}

func smiley(width, height int, background color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)

	cx, cy := width/2, height/2
	radius := min(width, height) * 2 / 5
	fillCircle(img, cx, cy, radius, color.RGBA{R: 255, G: 211, B: 51, A: 255})
	fillCircle(img, cx-radius/3, cy-radius/4, max(2, radius/10), color.Black)
	fillCircle(img, cx+radius/3, cy-radius/4, max(2, radius/10), color.Black)

	for degrees := 25; degrees <= 155; degrees++ {
		angle := float64(degrees) * math.Pi / 180
		x := cx + int(float64(radius*2/3)*math.Cos(angle))
		y := cy + int(float64(radius/2)*math.Sin(angle))
		fillCircle(img, x, y, max(1, radius/16), color.Black)
	}
	return img
}

func fillCircle(img *image.RGBA, cx, cy, radius int, value color.Color) {
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if x*x+y*y <= radius*radius {
				img.Set(cx+x, cy+y, value)
			}
		}
	}
}
