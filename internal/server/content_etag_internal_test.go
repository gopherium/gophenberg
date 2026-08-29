// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestContentETagRefusesAnAnswerItCannotEncode(t *testing.T) {
	t.Parallel()

	answer := resolvedAddress{
		Kind: string(content.KindItem),
		Item: &publishedDetail{Fields: content.Values{"rating": math.NaN()}},
	}

	etag, err := contentETag(answer)

	if etag != "" {
		t.Errorf("contentETag() = %q, want no validator", etag)
	}
	var refused *json.UnsupportedValueError
	if !errors.As(err, &refused) {
		t.Fatalf("contentETag() error = %v, want the encoder error wrapped", err)
	}
	if got := err.Error(); !strings.HasPrefix(got, "server: encode content etag: ") {
		t.Errorf("contentETag() error = %q, want the failure named as encode content etag", got)
	}
}

func TestContentETagStandsForTheAnswerItEncodes(t *testing.T) {
	t.Parallel()

	answer := resolvedAddress{
		Kind: string(content.KindItem),
		Item: &publishedDetail{Fields: content.Values{"rating": 5}},
	}

	etag, err := contentETag(answer)

	if err != nil {
		t.Fatalf("contentETag() error = %v, want no error", err)
	}
	if len(etag) != 34 || !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("contentETag() = %q, want a quoted validator of 32 hex characters", etag)
	}
}
