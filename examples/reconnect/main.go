package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	streamdeck "github.com/NJannasch/streamdeck-go"
)

func main() {
	serial := flag.String("serial", "", "device serial; empty selects the first supported deck")
	interval := flag.Duration("interval", time.Second, "delay between open attempts")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Println("waiting for a Stream Deck; press Ctrl-C to stop")
	for connection := range streamdeck.Reconnect(ctx, *serial, *interval) {
		if connection.Err != nil {
			log.Printf("open: %v", connection.Err)
			continue
		}
		deck := connection.Deck
		fmt.Printf("connected: %s serial=%s\n", deck.Info().Model.Name, deck.Info().Serial)
		readDeck(ctx, deck)
		fmt.Println("disconnected; waiting to reconnect")
	}
}

func readDeck(ctx context.Context, deck *streamdeck.Deck) {
	for {
		select {
		case event, ok := <-deck.Events():
			if !ok {
				return
			}
			fmt.Printf("key=%d pressed=%t\n", event.Key, event.Pressed)
		case event, ok := <-deck.DialEvents():
			if !ok {
				return
			}
			fmt.Printf("dial=%+v\n", event)
		case event, ok := <-deck.TouchEvents():
			if !ok {
				return
			}
			fmt.Printf("touch=%+v\n", event)
		case err, ok := <-deck.Errors():
			if !ok {
				return
			}
			if err != nil {
				log.Printf("device: %v", err)
			}
		case <-deck.Done():
			return
		case <-ctx.Done():
			_ = deck.Close()
			return
		}
	}
}
