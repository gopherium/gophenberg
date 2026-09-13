// SPDX-License-Identifier: Apache-2.0

package themehost

import (
	"errors"
	"fmt"
	"net/netip"
	"syscall"
)

// loopbackAddr is the loopback host as the four bytes a socket binds.
var loopbackAddr = netip.MustParseAddr(loopbackHost).As4()

// heldPort is a loopback port bound without listening, which the theme may still listen on.
type heldPort struct {
	fd   int
	port int
}

// reservePort returns a loopback port no other listener is handed until it is released.
func reservePort() (*heldPort, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err == nil {
		err = errors.Join(
			syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1),
			syscall.Bind(fd, &syscall.SockaddrInet4{Addr: loopbackAddr}),
		)
	}
	if err != nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("themehost: finding a free port: %w", err)
	}
	named, _ := syscall.Getsockname(fd)
	return &heldPort{fd: fd, port: named.(*syscall.SockaddrInet4).Port}, nil
}

// release gives the port back.
func (h *heldPort) release() {
	_ = syscall.Close(h.fd)
}
