package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	streamdeck "github.com/NJannasch/streamdeck-go"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	for change := range streamdeck.WatchDevices(ctx, time.Second) {
		if change.Err != nil {
			log.Print(change.Err)
			continue
		}
		verb := "connected"
		if change.Kind == streamdeck.DeviceDisconnected {
			verb = "disconnected"
		}
		fmt.Printf("%s: %s serial=%s path=%s\n", verb, change.Info.Model.Name, change.Info.Serial, change.Info.Path)
	}
}
