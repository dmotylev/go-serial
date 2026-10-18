[![Build Status](https://github.com/bugst/go-serial/workflows/test/badge.svg)](https://github.com/bugst/go-serial/actions?workflow=test)

# go.bug.st/serial

A cross-platform serial port library for Go.

## Documentation and examples

See the package documentation here: https://pkg.go.dev/go.bug.st/serial

## Tests

Run `go test -race ./...`. On macOS and Linux, the closed-port Drain regression uses
Python 3 from PATH to create private pseudo-terminals. It opens no physical
serial device. The test reports a skip if Python 3 is unavailable.

To run that regression explicitly, use:

```sh
go test -race -count=1 -run TestDrainClosedPortDoesNotAffectCurrentPTY -v .
```

It checks sequential calls after Close and reopen. It does not establish a bound
for a blocked Drain or concurrent Drain/Close, or qualify a USB serial driver.

## Credits

:sparkles: Thanks to all awesome [contributors]! :sparkles:

## License

This software is released under the [BSD 3-clause license].

[contributors]: https://github.com/bugst/go-serial/graphs/contributors
[BSD 3-clause license]: https://github.com/bugst/go-serial/blob/master/LICENSE

