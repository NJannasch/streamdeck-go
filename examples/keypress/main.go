package main

import (
	"fmt"
	"log"

	streamdeck "github.com/NJannasch/streamdeck-go"
)

func main() {
	deck, err := streamdeck.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer deck.Close()

	fmt.Printf("opened %s (%s)\n", deck.Info().Model.Name, deck.Info().Serial)
	if err := deck.SetBrightness(30); err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case event, ok := <-deck.Events():
			if !ok {
				return
			}
			fmt.Printf("key %d pressed=%t\n", event.Key, event.Pressed)
		case err, ok := <-deck.Errors():
			if ok && err != nil {
				log.Fatal(err)
			}
		}
	}
}
