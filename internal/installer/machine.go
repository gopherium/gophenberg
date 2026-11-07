// SPDX-License-Identifier: Apache-2.0

// Package installer sets up Gophenberg on a Linux server, which it reaches only through one Machine.
package installer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
)

// Program is an outside program a step starts: arguments, OS user, folder, settings, input, shown line and output copy.
type Program struct {
	Name   string
	Args   []string
	User   string
	Dir    string
	Env    []string
	Stdin  string
	Shown  string
	Output io.Writer
}

// Machine is everything outside its own process the installer reaches, so that a test can hand in a fake server.
type Machine struct {
	Run          func(ctx context.Context, program Program) (string, error)
	Root         string
	Lstat        func(path string) (fs.FileInfo, error)
	Free         func(path string) (uint64, error)
	Listen       func(network, address string) (net.Listener, error)
	ListenPacket func(network, address string) (net.PacketConn, error)
	Fetch        func(request *http.Request) (*http.Response, error)
	FetchHTTPS   func(request *http.Request, roots *x509.CertPool) (*http.Response, error)
	Resolve      func(ctx context.Context, network, host string) ([]netip.Addr, error)
	Addresses    func() ([]net.Addr, error)
	Terminal     func() (Prompt, error)
	Executable   func() (string, error)
	LookupUser   func(name string) (*user.User, error)
	Euid         func() int
}

// RealMachine returns this server as the installer reaches it for real, fetching through no proxy and no redirect.
func RealMachine() Machine {
	fetch := &http.Client{Transport: &http.Transport{}, CheckRedirect: answerAsIs}
	return Machine{
		Run: run, Root: "/", Lstat: os.Lstat, Free: free, Listen: net.Listen, ListenPacket: net.ListenPacket,
		Fetch: fetch.Do, FetchHTTPS: fetchHTTPS, Resolve: net.DefaultResolver.LookupNetIP,
		Addresses: net.InterfaceAddrs, Terminal: openTerminal, Executable: os.Executable, LookupUser: user.Lookup,
		Euid: os.Geteuid,
	}
}

// fetchHTTPS fetches request over TLS 1.2 or newer trusting roots, the system's when nil, answering a redirect as is.
func fetchHTTPS(request *http.Request, roots *x509.CertPool) (*http.Response, error) {
	client := &http.Client{CheckRedirect: answerAsIs, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, DisableKeepAlives: true,
	}}
	return client.Do(request)
}

// answerAsIs makes a fetch answer a redirect as it is, so a page comes from the address asked for and no other.
func answerAsIs(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

// absentAsNil returns value, or the zero value and no error for a file that is not there, and any other err wrapped.
func absentAsNil[T any](value T, err error) (T, error) {
	var zero T
	if errors.Is(err, fs.ErrNotExist) {
		return zero, nil
	}
	if err != nil {
		return zero, fmt.Errorf("installer: %w", err)
	}
	return value, nil
}

// read returns the file at path, named from the top of the server, nil when no file is there.
func (m Machine) read(path string) ([]byte, error) {
	return absentAsNil(os.ReadFile(filepath.Join(m.Root, path)))
}

// lstat describes the file at path, named from the top of the server, not a link's target, nil when nothing is there.
func (m Machine) lstat(path string) (fs.FileInfo, error) {
	return absentAsNil(m.Lstat(filepath.Join(m.Root, path)))
}

// there reports whether anything, a link that leads nowhere included, is at path, named from the top of the server.
func (m Machine) there(path string) bool {
	info, _ := m.lstat(path)
	return info != nil
}

// isFolder reports whether a real folder, not a link to one, is at path, named from the top of the server.
func (m Machine) isFolder(path string) bool {
	info, _ := m.lstat(path)
	return info != nil && info.IsDir()
}

// empty reports whether the folder at path, named from the top of the server, holds nothing.
func (m Machine) empty(path string) (bool, error) {
	entries, err := os.ReadDir(filepath.Join(m.Root, path))
	if err != nil {
		return false, fmt.Errorf("installer: %w", err)
	}
	return len(entries) == 0, nil
}

// freeUnder returns the nearest folder of path that exists, from the top of the server, and the bytes free there.
func (m Machine) freeUnder(path string) (string, uint64, error) {
	for {
		available, err := m.Free(filepath.Join(m.Root, path))
		if !errors.Is(err, fs.ErrNotExist) || path == "/" {
			return path, available, err
		}
		path = filepath.Dir(path)
	}
}

// free returns the bytes a user who is not root may still write on the file system that holds path.
func free(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, &fs.PathError{Op: "statfs", Path: path, Err: err}
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

// openTerminal opens the terminal of the session that started the installer, still there when standard input is a pipe.
func openTerminal() (Prompt, error) {
	return promptOn(os.OpenFile("/dev/tty", os.O_RDWR, 0))
}
