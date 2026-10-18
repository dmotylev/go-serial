//go:build darwin || linux

package serial_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"go.bug.st/serial"
)

const drainPTYBroker = `
import os
import pty
import select
import sys
import time

master, slave = pty.openpty()
print(os.ttyname(slave), flush=True)
for command in sys.stdin:
    kind, count = command.strip().split()
    if kind != "read":
        raise ValueError("unexpected broker command")
    size = int(count)
    data = bytearray()
    deadline = time.monotonic() + 3
    while len(data) < size:
        remaining = deadline - time.monotonic()
        if remaining <= 0 or not select.select([master], [], [], remaining)[0]:
            raise TimeoutError("peer bytes did not arrive")
        chunk = os.read(master, size - len(data))
        if not chunk:
            raise EOFError("peer closed before expected bytes")
        data.extend(chunk)
    print(data.hex(), flush=True)
os.close(slave)
os.close(master)
`

type drainPTY struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	input  io.WriteCloser
	output *bufio.Reader
	stderr bytes.Buffer
}

func newDrainPTY(t *testing.T) (string, *drainPTY) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is unavailable for the private PTY broker")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	broker := &drainPTY{cancel: cancel}
	broker.cmd = exec.CommandContext(ctx, python, "-u", "-c", drainPTYBroker)
	broker.cmd.Stderr = &broker.stderr
	broker.input, err = broker.cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	stdout, err := broker.cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	broker.output = bufio.NewReader(stdout)
	if err := broker.cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	path, err := broker.output.ReadString('\n')
	if err != nil {
		cancel()
		_ = broker.cmd.Wait()
		t.Fatalf("private PTY path: %v; broker: %s", err, broker.stderr.String())
	}
	return strings.TrimSpace(path), broker
}

func (broker *drainPTY) close(t *testing.T) {
	t.Helper()
	_ = broker.input.Close()
	err := broker.cmd.Wait()
	broker.cancel()
	if err != nil {
		t.Errorf("private PTY broker: %v; stderr: %s", err, broker.stderr.String())
	}
}

func (broker *drainPTY) expect(t *testing.T, want string) {
	t.Helper()
	if _, err := fmt.Fprintf(broker.input, "read %d\n", len(want)); err != nil {
		t.Fatalf("request peer bytes: %v", err)
	}
	line, err := broker.output.ReadString('\n')
	if err != nil {
		t.Fatalf("read peer bytes: %v", err)
	}
	if got := strings.TrimSpace(line); got != hex.EncodeToString([]byte(want)) {
		t.Fatalf("peer bytes: got %q, want %q", got, hex.EncodeToString([]byte(want)))
	}
}

func drainWrite(t *testing.T, port serial.Port, broker *drainPTY, payload string) {
	t.Helper()
	if count, err := port.Write([]byte(payload)); err != nil || count != len(payload) {
		t.Fatalf("write %q: %d, %v", payload, count, err)
	}
	broker.expect(t, payload)
}

func assertDrainClosed(t *testing.T, port serial.Port) {
	t.Helper()
	err := port.Drain()
	var portError *serial.PortError
	if !errors.As(err, &portError) || portError.Code() != serial.PortClosed {
		t.Fatalf("closed Drain: got %v, want PortClosed", err)
	}
}

func TestDrainClosedPortDoesNotAffectCurrentPTY(t *testing.T) {
	mode := &serial.Mode{BaudRate: 115200}
	t.Run("active", func(t *testing.T) {
		path, broker := newDrainPTY(t)
		defer broker.close(t)
		port, err := serial.Open(path, mode)
		if err != nil {
			t.Fatal(err)
		}
		defer port.Close()
		drainWrite(t, port, broker, "before")
		if err := port.Drain(); err != nil {
			t.Fatalf("active Drain: %v", err)
		}
		drainWrite(t, port, broker, "after")
	})
	t.Run("closed", func(t *testing.T) {
		path, broker := newDrainPTY(t)
		defer broker.close(t)
		port, err := serial.Open(path, mode)
		if err != nil {
			t.Fatal(err)
		}
		if err := port.Close(); err != nil {
			t.Fatal(err)
		}
		assertDrainClosed(t, port)
	})
	t.Run("reopened", func(t *testing.T) {
		path, broker := newDrainPTY(t)
		defer broker.close(t)
		old, err := serial.Open(path, mode)
		if err != nil {
			t.Fatal(err)
		}
		if err := old.Close(); err != nil {
			t.Fatal(err)
		}
		current, err := serial.Open(path, mode)
		if err != nil {
			t.Fatal(err)
		}
		defer current.Close()
		drainWrite(t, current, broker, "before")
		assertDrainClosed(t, old)
		drainWrite(t, current, broker, "after")
	})
}
