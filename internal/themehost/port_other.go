// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package themehost

import (
	"fmt"
	"net"
)

// heldPort is a loopback port nothing listens on.
type heldPort struct {
	port int
}

// reservePort returns a loopback port nothing listens on.
func reservePort() (*heldPort, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(loopbackHost, "0"))
	if err != nil {
		return nil, fmt.Errorf("themehost: finding a free port: %w", err)
	}
	defer func() { _ = listener.Close() }()
	return &heldPort{port: listener.Addr().(*net.TCPAddr).Port}, nil
}

// release gives the port back.
func (h *heldPort) release() {}
