// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// unencodableItem returns a published item whose field value no JSON encoder accepts.
func unencodableItem(t *testing.T, slug string) content.Content {
	t.Helper()
	held := publishedFixture(t, slug, blockMarkup, time.Now().UTC())
	held.Fields = content.Values{"rating": math.NaN()}
	return held
}

// encodableItem returns a published item carrying a field value the JSON encoder accepts.
func encodableItem(t *testing.T, slug string) content.Content {
	t.Helper()
	held := publishedFixture(t, slug, blockMarkup, time.Now().UTC())
	held.Fields = content.Values{"rating": 5}
	return held
}

func TestContentAPIReportsAnItemItCannotEncode(t *testing.T) {
	t.Parallel()

	handler := contentServer(t, encodableItem(t, "an-encodable-item"), unencodableItem(t, "an-unencodable-item"))

	served := getContent(t, handler, "/api/content/v1/resolve?path=/an-encodable-item")
	refused := getContent(t, handler, "/api/content/v1/resolve?path=/an-unencodable-item")

	if served.Code != http.StatusOK {
		t.Fatalf("GET the encodable item = %d, want %d", served.Code, http.StatusOK)
	}
	if served.Header().Get("ETag") == "" {
		t.Fatal("ETag on the encodable item = empty, want a validator")
	}
	if refused.Code != http.StatusInternalServerError {
		t.Fatalf("GET the unencodable item = %d, want %d", refused.Code, http.StatusInternalServerError)
	}
	if got := refused.Header().Get("ETag"); got != "" {
		t.Errorf("ETag = %q, want no validator when the answer cannot be encoded", got)
	}
	body := refused.Body.String()
	if !strings.Contains(body, `"code":"internal"`) || !strings.Contains(body, `"error":"internal error"`) {
		t.Errorf("body = %q, want the encoding failure masked as an internal error", body)
	}
	if strings.Contains(body, "NaN") || strings.Contains(body, "rating") {
		t.Errorf("body = %q, want the refused value kept out of the answer", body)
	}
}
