//go:build darwin || linux

package serial_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.bug.st/serial"
)

func TestWritePrivatePTYTransfer(t *testing.T) {
	path, broker := newDrainPTY(t)
	defer broker.close(t)
	port, err := serial.Open(path, &serial.Mode{BaudRate: 115200})
	if err != nil {
		t.Fatal(err)
	}
	defer port.Close()
	payload := "transfer"
	if n, err := port.Write([]byte(payload)); err != nil || n != len(payload) {
		t.Fatalf("transfer write: %d, %v", n, err)
	}
	broker.expect(t, payload)
}

func TestWriteCloseCancelsBlockedWrite(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required for private PTY broker")
	}
	if os.Getenv("SERIAL_WRITE_PTY_CHILD") == "1" {
		runBlockedWriteChild(t)
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 7*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWriteCloseCancelsBlockedWrite$", "-test.v", "-test.timeout=6s")
	cmd.Env = append(os.Environ(), "SERIAL_WRITE_PTY_CHILD=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("blocked-write child exceeded its bound: %v\n%s", ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("blocked-write child failed: %v\n%s", err, output)
	}
	for _, marker := range []string{"peer-observed-positive-bytes", "writer-pending-before-close", "write-and-close-finished"} {
		if !strings.Contains(string(output), marker) {
			t.Fatalf("blocked-write child missed %q:\n%s", marker, output)
		}
	}
}

func runBlockedWriteChild(t *testing.T) {
	path, broker := newDrainPTY(t)
	defer broker.close(t)
	port, err := serial.Open(path, &serial.Mode{BaudRate: 115200})
	if err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte{0x55}, 1<<20)
	type writeResult struct {
		n   int
		err error
	}
	written := make(chan writeResult, 1)
	go func() {
		n, err := port.Write(payload)
		written <- writeResult{n, err}
	}()
	broker.expect(t, "\x55")
	t.Log("peer-observed-positive-bytes")
	select {
	case result := <-written:
		t.Fatalf("Write finished before Close: %d, %v", result.n, result.err)
	default:
		t.Log("writer-pending-before-close")
	}
	closed := make(chan error, 1)
	go func() { closed <- port.Close() }()
	var result writeResult
	select {
	case result = <-written:
	case <-time.After(2 * time.Second):
		t.Fatal("Write remained blocked after Close")
	}
	var portError *serial.PortError
	if result.n <= 0 || result.n >= len(payload) || !errors.As(result.err, &portError) || portError.Code() != serial.PortClosed {
		t.Fatalf("blocked Write result: %d, %v; want positive partial count and PortClosed", result.n, result.err)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close remained blocked after signaling Write")
	}
	t.Log("write-and-close-finished")
}
