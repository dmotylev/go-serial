//
// Copyright 2014-2026 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package serial

import (
	"regexp"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

const devFolder = "/dev"

var osPortFilter = regexp.MustCompile(`^(cu|tty)\..*`)

const (
	ioctlTcgetattr = unix.TIOCGETA
	ioctlTcsetattr = unix.TIOCSETA
	ioctlTcflsh    = unix.TIOCFLUSH
	ioctlTioccbrk  = unix.TIOCCBRK
	ioctlTiocsbrk  = unix.TIOCSBRK
)

func setTermSettingsBaudrate(speed int, settings *unix.Termios) (error, bool) {
	baudrate, ok := baudrateMap[speed]
	if !ok {
		return nil, true
	}
	settings.Ispeed = toTermiosSpeedType(baudrate)
	settings.Ospeed = toTermiosSpeedType(baudrate)
	return nil, false
}

func (port *unixPort) setSpecialBaudrate(speed uint32) error {
	const kIOSSIOSPEED = 0x80045402
	return unix.IoctlSetPointerInt(port.handle, kIOSSIOSPEED, int(speed))
}

func (port *unixPort) ResetInputBuffer() error {
	port.closeLock.RLock()
	defer port.closeLock.RUnlock()
	if atomic.LoadUint32(&port.opened) != 1 {
		return &PortError{code: PortClosed}
	}

	return unix.IoctlSetPointerInt(port.handle, ioctlTcflsh, unix.TCIFLUSH)
}

func (port *unixPort) ResetOutputBuffer() error {
	port.closeLock.RLock()
	defer port.closeLock.RUnlock()
	if atomic.LoadUint32(&port.opened) != 1 {
		return &PortError{code: PortClosed}
	}

	return unix.IoctlSetPointerInt(port.handle, ioctlTcflsh, unix.TCOFLUSH)
}
