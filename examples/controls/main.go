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
	for {
		select {
		case event, ok := <-deck.Events():
			if !ok {
				return
			}
			fmt.Printf("key: %+v\n", event)
		case event := <-deck.DialEvents():
			fmt.Printf("dial: %+v\n", event)
		case event := <-deck.TouchEvents():
			fmt.Printf("touch: %+v\n", event)
		case err, ok := <-deck.Errors():
			if ok && err != nil {
				log.Fatal(err)
			}
		}
	}
}
