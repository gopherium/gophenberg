// SPDX-License-Identifier: Apache-2.0

package app

import (
	"testing"

	"github.com/gopherium/framework/gonsole/testkit"
)

func TestASiteThatNamesNoPlaceListensOnLocalhost(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	if settings.listening.Network != "tcp" || settings.listening.Address != "localhost:8081" {
		t.Errorf("listening = %+v, want TCP on localhost:8081", settings.listening)
	}
}

func TestAThemeReachesASiteOnABarePortOverLocalhost(t *testing.T) {
	t.Parallel()

	for address, want := range map[string]string{
		":3000":          "localhost:3000",
		"127.0.0.1:8081": "127.0.0.1:8081",
		"localhost:8081": "localhost:8081",
	} {
		if got := apiAddress(address); got != want {
			t.Errorf("apiAddress(%q) = %q, want %q", address, got, want)
		}
	}
}
