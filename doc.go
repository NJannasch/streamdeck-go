// Package streamdeck controls Elgato Stream Deck hardware directly over USB
// HID without requiring CGO or the Elgato desktop software.
//
// It supports the Original, Mini, Classic/MK.2, XL, Neo, Pedal, Plus, Plus XL,
// Studio, and known module variants. Call Devices to discover hardware, then
// Open, OpenBySerial, or OpenDevice to obtain an exclusively locked Deck.
//
// A Deck starts its input reader when opened. Drain the event channels while
// it is in use, inspect Errors for a terminal USB read failure, and call Close
// to release the device. Done closes when the input reader has stopped.
// Display and settings methods serialize their USB writes, so independent
// goroutines may update different controls.
//
// Key and dial indexes are zero based. Key zero is the top-left key and indexes
// proceed from left to right, then top to bottom. Model fields describe which
// optional controls and displays are available. Unsupported model-specific
// operations return an error.
//
// Only one process can own a device at a time. The Elgato desktop application
// must release a Stream Deck before this package can open it.
package streamdeck
