// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"errors"
	"regexp"
	"strconv"
	"syscall"
	"testing"
)

// startingLine matches the port a start line names.
var startingLine = regexp.MustCompile(`msg="theme starting" theme=\S+ port=(\d+)`)

// startingPort returns the port the first start line names, or zero before any start is logged.
func startingPort(logs string) int {
	found := startingLine.FindStringSubmatch(logs)
	if found == nil {
		return 0
	}
	port, _ := strconv.Atoi(found[1])
	return port
}

// bindUnshared binds a fresh loopback socket to the port without SO_REUSEADDR and returns what the bind met.
func bindUnshared(t *testing.T, port int) error {
	t.Helper()

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("opening a socket: %v", err)
	}
	defer func() { _ = syscall.Close(fd) }()
	return syscall.Bind(fd, &syscall.SockaddrInet4{Port: port, Addr: [4]byte{127, 0, 0, 1}})
}

func TestSupervisorKeepsThePortTakenWhileTheThemeHasNotBoundIt(t *testing.T) {
	t.Parallel()

	_, logs := startSupervisor(t, "deaf", nil)
	var port int
	waitFor(t, "the attempt to name the port it gave the theme", func() bool {
		port = startingPort(logs.String())
		return port != 0
	})

	err := bindUnshared(t, port)

	if !errors.Is(err, syscall.EADDRINUSE) {
		t.Errorf("binding port %d = %v, want it held for the theme so no other listener is handed it", port, err)
	}
}
