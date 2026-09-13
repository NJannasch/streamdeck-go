# Contributing

Bug reports, hardware validation, documentation fixes, and focused pull
requests are welcome. Search existing issues first and use the matching issue
form so device and platform details are available from the start.

For substantial API changes or new protocol commands, open an issue before
implementation. This gives maintainers and hardware owners a place to confirm
the scope and test strategy.

## Development

The module requires Go 1.22 or newer. Run the local checks before submitting a
pull request:

```sh
make check
```

`make check` runs the tests, race detector, and vet. To reproduce the complete
CGO-disabled platform build matrix in Docker, run:

```sh
make verify
```

New public behavior should have focused tests. Update package comments,
`USAGE.md`, and runnable examples when callers need new setup or usage
instructions.

## Hardware changes

Protocol changes should identify the physical model, USB product ID, firmware
version, operating system, and architecture used for testing. Describe the
exact input report or output command being implemented and link public protocol
documentation when available.

Do not include device serial numbers, access tokens, or unrelated USB captures
in issues, logs, fixtures, or pull requests. Keep packet fixtures as small as
possible and explain what each relevant byte represents.

If physical hardware was unavailable, state that clearly. Packet-level tests
are useful, but hardware validation remains necessary before a model can be
marked as verified in `CAPABILITIES.md`.

## Pull requests

Keep each pull request focused on one behavior. Explain the problem, resulting
API or protocol behavior, tests performed, and any remaining hardware coverage
limits. All contributions are accepted under the repository's MIT License.
