// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrPublicURL reports a public address naming more or less than a scheme and a host.
var ErrPublicURL = errors.New("must be an http or https address naming only a host, such as https://example.com")

// ParsePublicURL returns the scheme and host of the public address raw names, or none when raw is empty.
func ParsePublicURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || !publicAddress(*parsed) {
		return nil, fmt.Errorf("%w, got %q", ErrPublicURL, raw)
	}
	return &url.URL{Scheme: parsed.Scheme, Host: strings.ToLower(parsed.Host)}, nil
}

// publicAddress reports whether the address names an http or https host and nothing after it.
func publicAddress(held url.URL) bool {
	held.Path = strings.TrimSuffix(held.Path, "/")
	return (held.Scheme == "http" || held.Scheme == "https") && held.Hostname() != "" && validOrigin(&held)
}
