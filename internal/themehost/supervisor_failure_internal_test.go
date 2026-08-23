// SPDX-License-Identifier: Apache-2.0

package themehost

import (
	"context"
	"net/http"
	"testing"
)

func TestReadyRefusesAProbeAddressThatIsNotAURL(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		probe string
	}{
		{name: "a control byte in the address", probe: "http://127.0.0.1:8080\x7f" + healthPath},
		{name: "an unclosed bracket around the host", probe: "http://[::1" + healthPath},
		{name: "a broken percent escape in the path", probe: "http://127.0.0.1:8080/%zz" + healthPath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := http.NewRequestWithContext(context.Background(), http.MethodGet, tc.probe, nil); err == nil {
				t.Fatalf("NewRequestWithContext(%q) built a request, want the address refused first", tc.probe)
			}
			if got := ready(context.Background(), tc.probe); got {
				t.Errorf("ready(%q) = true, want false for an address that is not a URL", tc.probe)
			}
		})
	}
}
