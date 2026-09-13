# streamdeck-go

`streamdeck-go` is an independent, unofficial project. It is not affiliated
with, endorsed by, or sponsored by Elgato or Corsair. Stream Deck, Elgato, and
Corsair are trademarks of their respective owners.

`streamdeck-go` is a CGO-free Go library for controlling Elgato Stream Deck
hardware directly over USB HID on Linux, Windows, and macOS. Linux includes ARM
and ARM64.

The API supports the Original, Mini, 15-key Classic/MK.2, XL, Neo, Pedal, Plus,
Plus XL, Studio, and known module variants. It covers key, dial, and touch
input; native images and GIF animation; brightness and sleep settings;
full LCD and window images; firmware information; hot-plug watching; and
reconnection. See [CAPABILITIES.md](CAPABILITIES.md) for the detailed boundary
between direct hardware control and Elgato's desktop/plugin platform.

## Install

```sh
go get github.com/NJannasch/streamdeck-go
```

The library requires Go 1.22 or newer.

The API is currently pre-v1 and may change while additional physical models
are validated. Confirm the final module path before the first public release.

The project is available under the [MIT License](LICENSE). Third-party
dependency attributions are listed in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

See [USAGE.md](USAGE.md) for device selection, event handling, reconnects,
capability checks, images, settings, and platform setup. The precise hardware
and desktop-software boundary is documented in
[CAPABILITIES.md](CAPABILITIES.md).

## Example

```go
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

	if err := deck.SetBrightness(30); err != nil {
		log.Fatal(err)
	}

	for event := range deck.Events() {
		fmt.Printf("key %d pressed=%t\n", event.Key, event.Pressed)
	}
}
```

List attached supported devices without opening them:

```sh
go run ./examples/list
```

Open the first device and read its firmware information:

```sh
go run ./examples/deviceinfo
```

Draw generated smiley faces on all keys, or only key 4:

```sh
go run ./examples/smiley
go run ./examples/smiley -key 4
```

Display a PNG, JPEG, or GIF frame on one key, or tile it across the panel:

```sh
go run ./examples/image -key 0 ./photo.png
go run ./examples/image -panel ./wallpaper.jpg
go run ./examples/image -lcd ./wallpaper.jpg
go run ./examples/image -window ./status.png
```

Play an animated GIF until it completes or Ctrl-C is pressed:

```sh
go run ./examples/animated -key 0 ./animation.gif
```

Print keys, dials, and touch gestures, or watch USB hot-plug changes:

```sh
go run ./examples/controls
go run ./examples/hotplug
```

Keep a long-running controller attached to the same serial after disconnects:

```sh
go run ./examples/reconnect -serial SERIAL_NUMBER
```

Run the MK.2 hardware diagnostic (changes brightness and leaves a generated
test card visible):

```sh
go run ./examples/hardwaretest
```

For high-rate updates, call `PrepareGIF` once and replay its native frames with
`PlayAnimation`, or pass already transformed and encoded JPEG/BMP bytes to
`SetKeyImageData`.

Only one application can own a Stream Deck at a time. Close Elgato Stream Deck
software and other controllers before opening it.

### Linux permissions

Grant the logged-in desktop user access to Elgato HID devices:

```udev
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", TAG+="uaccess"
```

Place that rule in `/etc/udev/rules.d/60-streamdeck.rules`, reload the rules,
and reconnect the device.

## Verify in Docker

The Docker build tests the Go 1.22 minimum, then runs race detection, vet, and
CGO-disabled compile checks with Go 1.27 for Linux ARM, Linux ARM64, Windows
AMD64/ARM64, and macOS AMD64/ARM64:

```sh
docker build -t streamdeck-go-check .
```

The same check is available as `make verify`; `make check` runs tests, race
detection, and vet on the host.

Once the repository is hosted on GitHub, the CI workflow runs tests with Go
1.22 and Go 1.27 and builds every package for Linux AMD64/ARM/ARM64, Windows
AMD64/ARM64, and macOS AMD64/ARM64. It runs on pushes, pull requests, and manual
dispatches.

Docker verifies builds but does not access the USB device. Hardware tests should
run on the host or in a container with the relevant `/dev/hidraw*` device passed
through.

## Hardware validation

The Stream Deck MK.2 (`0fd9:0080`) has been discovered, opened, and queried on
real Linux hardware. Other model protocols are implemented from Elgato's HID
documentation and the Python library's device definitions and are covered by
packet-level tests, but still need confirmation on their physical devices.
