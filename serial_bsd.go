//
// Copyright 2014-2026 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package serial

import (
	"sync/atomic"

	"golang.org/x/sys/unix"
)

func (port *unixPort) Drain() error {
	port.closeLock.RLock()
	defer port.closeLock.RUnlock()
	if atomic.LoadUint32(&port.opened) != 1 {
		return &PortError{code: PortClosed}
	}

	return unix.IoctlSetInt(port.handle, unix.TIOCDRAIN, 0)
}
