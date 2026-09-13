# Repository instructions

These instructions apply to the complete repository.

## Project boundaries

- Keep the library CGO-free and usable with `CGO_ENABLED=0` on Linux, Windows,
  and macOS. Preserve Linux ARM and ARM64 support.
- Maintain compatibility with Go 1.22, the version declared in `go.mod`.
- Keep this module focused on direct USB HID control. Profiles, process
  launching, keyboard injection, Marketplace plugins, and service integrations
  belong in applications built on top of the library.
- Avoid new dependencies unless they provide clear value across supported
  platforms and are compatible with the MIT-licensed distribution.

## Code map

- `model.go` contains supported product IDs, geometry, and protocol traits.
- `discovery.go` owns enumeration and exclusive device opening.
- `transport.go` isolates the HID dependency.
- `deck.go` implements reports, events, settings, and display commands.
- `image.go` transforms images and constructs device packets.
- `animation.go` prepares and plays reusable native animation frames.
- `watcher.go` implements hot-plug watching and reconnect behavior.
- `deck_test.go` contains transport fakes and packet-level tests.
- `examples/` contains small programs intended for users to run directly.

## Implementation rules

- Treat the exported `Model` values as immutable descriptors. Identify hardware
  by USB product ID rather than display name.
- Keep public key and dial indexes zero based. Key zero is top-left; ordering is
  left-to-right and then top-to-bottom.
- Route output reports through the existing serialized write helpers. Preserve
  idempotent `Close`, terminal `Errors`, and `Done` channel behavior.
- Validate indexes, dimensions, nil images or colors, and model capabilities
  before sending packets. Wrap transport errors with the operation being
  performed.
- Base new protocol bytes on public documentation or captured behavior from the
  exact device. Record uncertain assumptions in tests or capability docs.
- Add focused packet-level tests for protocol changes. Do not claim physical
  validation unless the behavior was observed on that model.

## Verification

Run the standard checks after Go changes:

```sh
make check
```

Run the Docker matrix when changing platform code, dependencies, build tags, or
the HID transport:

```sh
make verify
```

The GitHub Actions workflow is the final check for Go 1.22 and the current Go
version across Linux AMD64/ARM/ARM64, Windows AMD64/ARM64, and macOS
AMD64/ARM64.

Do not run examples that open or modify a connected Stream Deck unless the task
explicitly calls for hardware testing. `examples/hardwaretest` changes
brightness, sleep settings, and displayed images. Never include device serial
numbers or unrelated USB captures in source, fixtures, logs, or reports.

## Documentation

- Document every exported identifier and update package comments when lifecycle
  or concurrency behavior changes.
- Update `README.md` for installation and common examples, `USAGE.md` for API
  guidance, and `CAPABILITIES.md` for feature or hardware-support changes.
- Add or update a runnable example when a new public feature needs more than a
  short package comment to use correctly.
- Keep the MIT license section at the bottom of `README.md` and maintain
  `THIRD_PARTY_NOTICES.md` when dependency licenses or versions change.
