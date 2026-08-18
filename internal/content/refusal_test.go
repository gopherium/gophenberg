// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestRefusalKeepsTheSentinelFindable(t *testing.T) {
	t.Parallel()

	refused := content.Refuse(content.ErrUnknownField, "field_unknown",
		"content: unknown field: color", content.Details{"field": "color"})

	if !errors.Is(refused, content.ErrUnknownField) {
		t.Errorf("errors.Is() = false, want the sentinel still findable through the carrier")
	}
}

func TestRefusalReadsAsTheProseItReplaces(t *testing.T) {
	t.Parallel()

	refused := content.Refuse(content.ErrUnknownField, "field_unknown",
		"content: unknown field: color", content.Details{"field": "color"})

	if got := refused.Error(); got != "content: unknown field: color" {
		t.Errorf("Error() = %q, want the message a log and a catalogless client already read", got)
	}
}

func TestRefusalCarriesItsDetails(t *testing.T) {
	t.Parallel()

	refused := content.Refuse(content.ErrFieldShape, "field_shape_kind", "content: field shape", content.Details{
		"field": "doors", "kind": "number",
	})

	held, ok := content.DetailsOf(refused)
	if !ok {
		t.Fatalf("DetailsOf() ok = false, want the details readable")
	}
	if held["field"] != "doors" || held["kind"] != "number" {
		t.Errorf("details = %v, want the dynamic parts as data", held)
	}
}

func TestDetailsOfReportsAPlainError(t *testing.T) {
	t.Parallel()

	if _, ok := content.DetailsOf(content.ErrUnknownField); ok {
		t.Errorf("DetailsOf() ok = true, want a plain sentinel to carry none")
	}
}

func TestDetailsOfReadsThroughAWrap(t *testing.T) {
	t.Parallel()

	refused := content.Refuse(content.ErrUnknownField, "field_unknown",
		"content: unknown field: color", content.Details{"field": "color"})

	held, ok := content.DetailsOf(fmt.Errorf("server: saving the item: %w", refused))

	if !ok || held["field"] != "color" {
		t.Errorf("DetailsOf() = %v, %v, want the details found through a wrapping error", held, ok)
	}
}

func TestCodeOfNamesTheCondition(t *testing.T) {
	t.Parallel()

	refused := content.Refuse(content.ErrFieldShape, "field_shape_kind", "content: field shape", nil)

	code, ok := content.CodeOf(refused)

	if !ok || code != "field_shape_kind" {
		t.Errorf("CodeOf() = %q, %v, want the site's own condition named", code, ok)
	}
}

func TestCodeOfReportsAPlainError(t *testing.T) {
	t.Parallel()

	if _, ok := content.CodeOf(content.ErrUnknownField); ok {
		t.Errorf("CodeOf() ok = true, want a plain sentinel to name none")
	}
}
