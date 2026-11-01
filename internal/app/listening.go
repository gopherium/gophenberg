// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gopherium/framework/gonsole"
)

// defaultListening is where a site listens when no setting names a place.
var defaultListening = gonsole.Listening{Network: "tcp", Address: "localhost:8081"}

// listeningFrom returns where the site listens, refusing a socket beside what reaches the site over TCP.
func listeningFrom(env gonsole.Env, settings runConfig) (gonsole.Listening, error) {
	listening, err := env.Listening(defaultListening)
	if err != nil {
		return gonsole.Listening{}, err
	}
	if listening.Network != "unix" {
		return listening, nil
	}
	return listening, errors.Join(
		needsTCP(env, "a theme reaches", "THEMES_DIR", settings.themesDir),
		needsTCP(env, "the first start of an update checks", "INSTALL_ROOT", settings.install.Root),
	)
}

// needsTCP returns an error when the setting holds a value that what names reaches the site with over TCP alone.
func needsTCP(env gonsole.Env, what, setting, value string) error {
	if value == "" {
		return nil
	}
	return fmt.Errorf("%s: %s the site over TCP, so leave %s empty", env.Key("SOCKET"), what, env.Key(setting))
}

// apiAddress returns the address a theme on this host reaches the site at, localhost for a bare port.
func apiAddress(address string) string {
	if strings.HasPrefix(address, ":") {
		return "localhost" + address
	}
	return address
}
