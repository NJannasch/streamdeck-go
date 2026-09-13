# Library usage

This guide covers the decisions an application normally needs to make around
the API. The runnable programs under [`examples`](examples) cover individual
operations.

## Open and close a deck

`Open` selects the first supported device. Always close the deck so another
application can open it:

```go
deck, err := streamdeck.Open()
if err != nil {
	return err
}
defer deck.Close()
```

Opening is exclusive. Close the Elgato Stream Deck application and any other
controller before opening the same hardware.

If more than one deck may be attached, enumerate them and select by stable
serial number:

```go
devices, err := streamdeck.Devices()
if err != nil {
	return err
}
for _, device := range devices {
	fmt.Printf("%s: %s\n", device.Serial, device.Model.Name)
}

deck, err := streamdeck.OpenBySerial(serial)
```

`OpenDevice` accepts an entry returned by `Devices` when selecting by path is
more useful. A path identifies the current attachment and can change after the
device is reconnected; use the serial number for saved configuration.

## Handle input and disconnects

An opened deck has one background USB reader. Applications should continuously
drain every event type they use:

```go
for {
	select {
	case event, ok := <-deck.Events():
		if !ok {
			return nil
		}
		fmt.Printf("key=%d pressed=%t\n", event.Key, event.Pressed)
	case event, ok := <-deck.DialEvents():
		if !ok {
			return nil
		}
		fmt.Printf("dial=%+v\n", event)
	case event, ok := <-deck.TouchEvents():
		if !ok {
			return nil
		}
		fmt.Printf("touch=%+v\n", event)
	case err, ok := <-deck.Errors():
		if ok && err != nil {
			return err
		}
	}
}
```

Key indexes are zero based. Key 0 is at the top left, then indexes increase
left-to-right and top-to-bottom. Dial indexes are also zero based. Touch
coordinates use display pixels.

Input channels have bounded buffers. Events may be dropped if an application
does not drain them quickly enough. `Errors` receives a terminal background
read error, while `Done` closes whenever the reader has stopped, including a
normal `Close`. Output methods serialize USB writes internally.

Use `WatchDevices` when only attachment notifications are needed. Use
`Reconnect` for a long-running controller that should reopen the same serial
number after a cable disconnect. See [`examples/hotplug`](examples/hotplug) and
[`examples/reconnect`](examples/reconnect).

## Check model capabilities

`deck.Info().Model` describes the controls and display surfaces available on
the opened hardware. Check these fields before presenting model-specific
features:

| Check | Available operation |
| --- | --- |
| `model.HasDisplay()` | Key images and colors |
| `model.DialCount > 0` | Dial events |
| `model.TouchscreenWidth > 0` | Touch events and touchscreen images |
| `model.ScreenWidth > 0` | Neo status screen |
| `model.LCDWidth > 0` | Full physical LCD upload |
| `model.ProductID == streamdeck.StreamDeckStudio.ProductID` | Studio encoder LEDs |

Methods still validate the model and return a descriptive error when hardware
does not support an operation. The fields are useful for building a UI without
probing by failure.

## Display images

`SetKeyImage` accepts any Go `image.Image`, resizes it, applies the device's
orientation, and encodes it in the required JPEG or BMP format. Common file
formats become available by importing their decoders before calling
`image.Decode`:

```go
import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)
```

The display helpers serve different physical surfaces:

| Method | Surface |
| --- | --- |
| `SetKeyImage` | One key |
| `SetPanelImage` | One logical image divided across all keys |
| `SetWindowImage` | Complete Plus touchscreen or Neo status window |
| `SetTouchscreenImage` | Pixel-aligned region of a Plus touchscreen |
| `SetScreenImage` | Neo status screen |
| `SetLCDImage` | Physical LCD behind the keys on supported devices |

`SetPanelImage` preserves the physical gaps between keys. `SetLCDImage` writes
the continuous LCD surface and is available only where the firmware exposes
that command.

For animation, `PlayGIF` is the convenient path. For repeated or high-rate
playback, call `PrepareGIF` once and reuse the returned native frames through
`PlayAnimation`. `SetKeyImageData` similarly accepts already transformed and
encoded device-native bytes.

## Settings and stored backgrounds

Brightness values are clamped to 0 through 100. A zero sleep duration disables
idle sleep. `SleepTimeout`, `SetSleepTimeout`, and `UnitInfo` are firmware
features and return an error on protocols that do not expose them.

`StoreBackground` and `ShowBackground` address firmware storage on supported
Classic and XL devices. The reported `UnitInfo.LCDGalleryImages` value indicates
the available capacity when the firmware supplies it.

## Platform setup

The library uses a pure-Go HID transport and builds with `CGO_ENABLED=0` on
Linux, Windows, and macOS. Linux also supports ARM and ARM64.

Linux desktop sessions normally need a udev rule granting access to Elgato USB
devices:

```udev
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", TAG+="uaccess"
```

Save it as `/etc/udev/rules.d/60-streamdeck.rules`, reload the udev rules, and
reconnect the device. Windows and macOS do not normally need an additional
device rule.

Direct HID control does not provide the official desktop application's
profiles, actions, Marketplace plugins, or service integrations. See
[`CAPABILITIES.md`](CAPABILITIES.md) for that boundary and the model validation
status.
