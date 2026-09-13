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

	serial, err := deck.SerialNumber()
	if err != nil {
		log.Fatal(err)
	}
	firmware, err := deck.FirmwareVersion()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s serial=%s firmware=%s\n", deck.Info().Model.Name, serial, firmware)
}
