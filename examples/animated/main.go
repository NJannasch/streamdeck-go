package main

import (
	"context"
	"flag"
	"image/gif"
	"log"
	"os"
	"os/signal"
)

import streamdeck "github.com/NJannasch/streamdeck-go"

func main() {
	key := flag.Int("key", 0, "key to animate")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("usage: go run ./examples/animated [-key N] ANIMATION.gif")
	}
	file, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	animation, err := gif.DecodeAll(file)
	if err != nil {
		log.Fatal(err)
	}
	deck, err := streamdeck.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer deck.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := deck.PlayGIF(ctx, *key, animation); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
