# Capability matrix

This module controls Stream Deck hardware directly over USB HID. The table
separates that job from features supplied by Elgato's desktop application and
plugin runtime.

## Direct hardware API

| Capability | streamdeck-go | Python StreamDeck library |
| --- | --- | --- |
| Discovery and exclusive open | Yes | Yes |
| Key press/release | Yes | Yes |
| Key images and solid colors | Yes | Yes |
| Pre-encoded native images | Yes | Yes |
| Panel tiling | Yes | Example/helper |
| Animated GIF playback | Yes | Example |
| Brightness, reset, serial, firmware | Yes | Yes |
| Plus/Plus XL dial input | Yes | Yes |
| Plus/Plus XL touch input and images | Yes | Yes |
| Neo touch keys and status screen | Yes | Yes |
| Studio dial input and encoder LEDs | Yes | Yes |
| Hot-plug watch and reconnect stream | Yes | Backend dependent |
| Full physical LCD upload | Yes (documented models) | No |
| Direct LCD fill and sleep timeout | Yes (main protocol) | No |
| Stored LCD backgrounds | Yes (Classic/XL) | No |
| Firmware-reported unit geometry | Yes (main protocol) | No |

Supported product families are Original, Original V2, MK.2, Mini, XL, Neo,
Pedal, Plus, Plus XL, Studio, and the known Mini/MK.2/XL module variants.
Protocols other than the connected MK.2 are covered by packet tests and should
be treated as hardware-unverified until tested on each physical model.

## Desktop software and plugin platform

The official Stream Deck application owns higher-level behavior: profiles,
pages and folders; application-based profile switching; hotkeys and text
injection; multi-actions and delays; plugin installation and the Marketplace;
accounts and third-party service integrations; audio and soundboard actions;
icons, titles, fonts, and layout editing; screensavers; and plugin settings/UI.

Those are application features rather than HID commands. They can be built on
top of this reusable module, but putting process launching, keyboard injection,
profile storage, or vendor integrations into the hardware package would couple
it to one controller application. A later application can consume key, dial,
and touch events here and implement those policies independently.

Elgato plugins communicate with the running Stream Deck desktop application
over its WebSocket SDK. Direct HID control and the official desktop application
also compete for exclusive access to the same device, so this module is not a
drop-in host for Marketplace plugins.
