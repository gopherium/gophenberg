// SPDX-License-Identifier: Apache-2.0

package server

import (
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestEveryReservedPrefixIsClosedToContentTypes(t *testing.T) {
	t.Parallel()

	prefixes := []string{"/api", adminPrefix, assetPrefix, mediaPrefix, themePrefix}

	for _, prefix := range prefixes {
		word := strings.TrimPrefix(prefix, "/")

		if !slices.Contains(content.ReservedRouteWords, word) {
			t.Errorf("the route word %q addresses %s, want the registry to refuse it", word, prefix)
		}
	}
}
