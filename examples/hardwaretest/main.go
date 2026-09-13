package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"time"

	streamdeck "github.com/NJannasch/streamdeck-go"
)

var colors = []color.RGBA{
	{R: 220, G: 45, B: 45, A: 255},
	{R: 235, G: 145, B: 35, A: 255},
	{R: 230, G: 210, B: 45, A: 255},
	{R: 50, G: 190, B: 80, A: 255},
	{R: 45, G: 130, B: 225, A: 255},
}

func main() {
	duration := flag.Duration("duration", 15*time.Second, "time to read key events")
	flag.Parse()
	deck, err := streamdeck.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer deck.Close()
	model := deck.Info().Model
	serial, err := deck.SerialNumber()
	if err != nil {
		log.Fatal(err)
	}
	firmware, err := deck.FirmwareVersion()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("opened %s serial=%s firmware=%s layout=%dx%d key=%dx%d\n", model.Name, serial, firmware, model.Columns, model.Rows, model.KeyWidth, model.KeyHeight)
	if info, err := deck.UnitInfo(); err != nil {
		fmt.Printf("unit information unavailable: %v\n", err)
	} else {
		fmt.Printf("firmware geometry: keys=%dx%d key=%dx%d lcd=%dx%d bpp=%d gallery=%d/%d demo=%d\n", info.Columns, info.Rows, info.KeyWidth, info.KeyHeight, info.LCDWidth, info.LCDHeight, info.BitsPerPixel, info.KeyGalleryImages, info.LCDGalleryImages, info.DemoFrames)
	}
	if timeout, err := deck.SleepTimeout(); err != nil {
		fmt.Printf("sleep timeout unavailable: %v\n", err)
	} else {
		fmt.Printf("sleep timeout: %s\n", timeout)
		if err := deck.SetSleepTimeout(timeout); err != nil {
			log.Fatal(err)
		}
		fmt.Println("sleep timeout setter accepted the unchanged value")
	}
	if err := deck.SetBrightness(35); err != nil {
		log.Fatal(err)
	}
	if err := deck.SetLCDImage(testCard(model.LCDWidth, model.LCDHeight, 21)); err != nil {
		log.Fatal(err)
	}
	fmt.Println("full-LCD test image displayed for 2 seconds")
	time.Sleep(2 * time.Second)
	for key := 0; key < model.KeyCount(); key++ {
		img := testCard(model.KeyWidth, model.KeyHeight, key)
		if err := deck.SetKeyImage(key, img); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("per-key test card displayed at 35%% brightness; reading keys for %s\n", *duration)
	timer := time.NewTimer(*duration)
	defer timer.Stop()
	for {
		select {
		case event := <-deck.Events():
			fmt.Printf("key=%d pressed=%t\n", event.Key, event.Pressed)
		case err := <-deck.Errors():
			if err != nil {
				log.Fatal(err)
			}
		case <-timer.C:
			fmt.Println("input test complete; leaving test card visible")
			return
		}
	}
}

func testCard(width, height, key int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	base := colors[key%len(colors)]
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value := base
			if x < 4 || y < 4 || x >= width-4 || y >= height-4 {
				value = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			}
			img.SetRGBA(x, y, value)
		}
	}
	// Encode the key number as five white/black blocks. This makes orientation
	// and logical-to-physical ordering visible without a font dependency.
	for bit := 0; bit < 5; bit++ {
		value := color.RGBA{A: 255}
		if key&(1<<bit) != 0 {
			value = color.RGBA{R: 255, G: 255, B: 255, A: 255}
		}
		x0, y0 := 8+bit*11, height/2-8
		for y := y0; y < y0+16; y++ {
			for x := x0; x < x0+8; x++ {
				img.SetRGBA(x, y, value)
			}
		}
	}
	return img
}
