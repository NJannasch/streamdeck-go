package streamdeck

import (
	"errors"
	"fmt"
)

var (
	// ErrNoDevice is returned when no supported Stream Deck matches a request.
	ErrNoDevice = errors.New("streamdeck: no matching device found")
	// ErrClosed is returned when an operation is attempted on a closed deck.
	ErrClosed = errors.New("streamdeck: device is closed")
)

// DeviceInfo identifies a supported attached Stream Deck.
type DeviceInfo struct {
	Path   string
	Serial string
	Model  Model
}

// Devices returns all attached Stream Decks supported by this package.
func Devices() ([]DeviceInfo, error) {
	devices, err := enumerate()
	if err != nil {
		return nil, fmt.Errorf("streamdeck: enumerate HID devices: %w", err)
	}

	result := make([]DeviceInfo, 0, len(devices))
	for _, device := range devices {
		model, err := modelForProduct(device.ProductId())
		if err != nil {
			continue
		}
		result = append(result, DeviceInfo{
			Path:   device.Path(),
			Serial: device.SerialNumber(),
			Model:  model,
		})
	}
	return result, nil
}

// Open opens the first supported attached Stream Deck with an exclusive lock.
func Open() (*Deck, error) {
	devices, err := Devices()
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, ErrNoDevice
	}
	return OpenDevice(devices[0])
}

// OpenBySerial opens the supported Stream Deck with the requested serial.
func OpenBySerial(serial string) (*Deck, error) {
	devices, err := Devices()
	if err != nil {
		return nil, err
	}
	for _, info := range devices {
		if info.Serial == serial {
			return OpenDevice(info)
		}
	}
	return nil, ErrNoDevice
}

// OpenDevice opens a device returned by Devices with an exclusive lock.
func OpenDevice(info DeviceInfo) (*Deck, error) {
	devices, err := enumerate()
	if err != nil {
		return nil, fmt.Errorf("streamdeck: enumerate HID devices: %w", err)
	}

	for _, device := range devices {
		if device.Path() != info.Path {
			continue
		}
		model, err := modelForProduct(device.ProductId())
		if err != nil {
			return nil, err
		}
		if err := device.Open(true); err != nil {
			return nil, fmt.Errorf("streamdeck: open %s: %w", info.Path, err)
		}
		if err := resetImageStream(device, model); err != nil {
			_ = device.Close()
			return nil, fmt.Errorf("streamdeck: initialize image stream: %w", err)
		}
		return newDeck(DeviceInfo{
			Path:   device.Path(),
			Serial: device.SerialNumber(),
			Model:  model,
		}, device), nil
	}

	return nil, ErrNoDevice
}
