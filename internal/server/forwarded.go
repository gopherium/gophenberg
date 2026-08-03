// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net"
	"net/http"
	"net/netip"
)

// forwardedHeaders are the routing hints only a trusted proxy may set.
var forwardedHeaders = []string{"X-Forwarded-For", "X-Forwarded-Proto", "X-Forwarded-Host", "Forwarded"}

// trustForwarded returns middleware dropping [forwardedHeaders] set by peers outside trusted.
func trustForwarded(trusted []string) func(http.Handler) http.Handler {
	prefixes := parsePrefixes(trusted)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !trustedPeer(r.RemoteAddr, prefixes) {
				for _, name := range forwardedHeaders {
					r.Header.Del(name)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
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
