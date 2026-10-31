// SPDX-License-Identifier: Apache-2.0

package publicsite_test

import (
	"net/http"
	"testing"
	"time"
)

func TestSiteReadsAPageNumberOnlyFromPlainDigits(t *testing.T) {
	t.Parallel()

	handler, _ := siteWith(
		publishedPost("A Post", "a-post", blockMarkup, time.Now().UTC()),
		storedAt("page", "Two", "pages/page/2"),
		storedAt("category", "News", "categories/news"),
	)

	for _, address := range []string{"/page/+2", "/page/\U0000FF12", "/pages/page/+2", "/categories/news/page/+2"} {
		recorder := get(t, handler, address)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("status at %q = %d, want %d", address, recorder.Code, http.StatusNotFound)
		}
	}
}
