package main

import (
	"fmt"
	"log"

	streamdeck "github.com/NJannasch/streamdeck-go"
)

func main() {
	devices, err := streamdeck.Devices()
	if err != nil {
		log.Fatal(err)
	}
	if len(devices) == 0 {
		log.Fatal(streamdeck.ErrNoDevice)
	}
	for _, device := range devices {
		fmt.Printf("%s serial=%s path=%s keys=%d\n",
			device.Model.Name,
			device.Serial,
			device.Path,
			device.Model.KeyCount(),
		)
	}
}
