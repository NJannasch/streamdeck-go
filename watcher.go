package streamdeck

import (
	"context"
	"time"
)

// DeviceChangeKind identifies a connected or disconnected device.
type DeviceChangeKind uint8

const (
	DeviceConnected DeviceChangeKind = iota + 1
	DeviceDisconnected
)

// DeviceChange is emitted by WatchDevices. Existing devices are reported as
// connected when the watcher starts.
type DeviceChange struct {
	Kind DeviceChangeKind
	Info DeviceInfo
	Err  error
}

// Connection is emitted by Reconnect whenever a matching device is opened or
// an open attempt fails. A non-nil Deck replaces the previous connection.
type Connection struct {
	Deck *Deck
	Err  error
}

// Reconnect keeps opening a device after disconnects. An empty serial selects
// the first supported deck. Callers use each emitted Deck until its Errors
// channel closes, then wait for the next Connection.
func Reconnect(ctx context.Context, serial string, interval time.Duration) <-chan Connection {
	if interval <= 0 {
		interval = time.Second
	}
	connections := make(chan Connection, 4)
	go func() {
		defer close(connections)
		for {
			var deck *Deck
			var err error
			if serial == "" {
				deck, err = Open()
			} else {
				deck, err = OpenBySerial(serial)
			}
			if err == nil {
				select {
				case connections <- Connection{Deck: deck}:
				case <-ctx.Done():
					_ = deck.Close()
					return
				}
				select {
				case <-ctx.Done():
					_ = deck.Close()
					return
				case <-deck.Done():
					_ = deck.Close()
				}
			} else if err != ErrNoDevice {
				select {
				case connections <- Connection{Err: err}:
				case <-ctx.Done():
					return
				}
			}
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case <-timer.C:
			}
		}
	}()
	return connections
}

// WatchDevices polls HID discovery and reports hot-plug changes until ctx ends.
func WatchDevices(ctx context.Context, interval time.Duration) <-chan DeviceChange {
	if interval <= 0 {
		interval = time.Second
	}
	changes := make(chan DeviceChange, 16)
	go func() {
		defer close(changes)
		known := make(map[string]DeviceInfo)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			devices, err := Devices()
			if err != nil {
				if !sendChange(ctx, changes, DeviceChange{Err: err}) {
					return
				}
			} else {
				current := make(map[string]DeviceInfo, len(devices))
				for _, info := range devices {
					current[info.Path] = info
					if _, ok := known[info.Path]; !ok && !sendChange(ctx, changes, DeviceChange{Kind: DeviceConnected, Info: info}) {
						return
					}
				}
				for path, info := range known {
					if _, ok := current[path]; !ok && !sendChange(ctx, changes, DeviceChange{Kind: DeviceDisconnected, Info: info}) {
						return
					}
				}
				known = current
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return changes
}

func sendChange(ctx context.Context, output chan<- DeviceChange, change DeviceChange) bool {
	select {
	case output <- change:
		return true
	case <-ctx.Done():
		return false
	}
}
