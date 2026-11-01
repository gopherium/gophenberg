// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net"
	"net/http"
	"net/netip"

	"github.com/gopherium/framework/gonsole"
)

// forwardedHeaders are the routing hints only a trusted proxy may set.
var forwardedHeaders = []string{"X-Forwarded-For", "X-Forwarded-Proto", "X-Forwarded-Host", "Forwarded"}

// socketDropped are the routing hints a request over a socket loses, its peer naming only the scheme.
var socketDropped = []string{"X-Forwarded-For", "X-Forwarded-Host", "Forwarded"}

// trustForwarded returns middleware dropping the routing hints a request's peer may not set.
func trustForwarded(trusted []string) func(http.Handler) http.Handler {
	prefixes := parsePrefixes(trusted)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, name := range droppedHeaders(r, prefixes) {
				r.Header.Del(name)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// droppedHeaders returns the routing hints the peer of r may not set.
func droppedHeaders(r *http.Request, prefixes []netip.Prefix) []string {
	switch {
	case gonsole.OverSocket(r):
		return socketDropped
	case trustedPeer(r.RemoteAddr, prefixes):
		return nil
	}
	return forwardedHeaders
}

// parsePrefixes returns the CIDR ranges named by raw.
func parsePrefixes(raw []string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(raw))
	for _, entry := range raw {
		prefixes = append(prefixes, netip.MustParsePrefix(entry))
	}
	return prefixes
}

// trustedPeer reports whether remoteAddr sits inside one of the prefixes.
func trustedPeer(remoteAddr string, prefixes []netip.Prefix) bool {
	if len(prefixes) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	peer = peer.Unmap()
	for _, prefix := range prefixes {
		if prefix.Contains(peer) {
			return true
		}
	}
	return false
}
