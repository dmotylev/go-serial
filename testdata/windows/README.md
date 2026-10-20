# Windows regression fixtures

`go test -run TestWindowsRegression .` on Windows tests the current checkout's Windows implementation through the public Port API using simulated Win32 calls. `go test -race ./...` propagates race instrumentation to every child suite; the ordinary suite uses no race instrumentation. The test requires the Go toolchain that built the parent test and its already-cached module dependencies. Child runs force offline module lookup and readonly module files.

The Go harness copies current non-test package source plus the unchanged go.mod/go.sum into a disposable child module, then rewrites Win32 call sites to these fakes. It excludes parent tests, so execution cannot recurse. No alternate production source or production injection seam is retained.

Three existing fixture families remain separate:

- lifetime: thirteen public-Port cases, including zero-byte completion after Close cancellation;
- completion-order: one case using a test-copy-only gate immediately before the Read completion call;
- dcb: six transaction interleavings, using a test-copy-only configuration mutex wrapper to observe contention.

The lifetime fixtures originated in vu1-dials/windows-read-retry-preparation-2026-10-20; the other families retain windows-completion-order and windows-dcb-serialization fixtures. Fixture files are embedded as text and compiled only in the disposable module. Fakes, gates and mutex instrumentation never enter the maintained library's production build. This is deterministic software regression evidence; it does not qualify native drivers or physical COM/USB devices.
