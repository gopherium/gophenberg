// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// linked returns a link field carrying the settings.
func linked(t *testing.T, settings map[string]any) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: content.TypePost, Key: "source", Label: "Source",
		Kind: content.FieldKindLink, Settings: settings,
	})
	if err != nil {
		t.Fatalf("NewField(source) error = %v, want nil", err)
	}
	return built
}

// linkValue returns the members a link holds under one key.
func linkValue(url, title string, newTab bool) map[string]any {
	return map[string]any{"url": url, "title": title, "new_tab": newTab}
}

func TestALinkHoldsAnAddressATitleAndATabChoice(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{
		"a web address":          linkValue("https://example.com/a", "A page", false),
		"a plain http address":   linkValue("http://example.com/a", "A page", true),
		"a mail address":         linkValue("mailto:maria@example.com", "Write to us", false),
		"a path on this site":    linkValue("/about", "About", false),
		"a path with a query":    linkValue("/search?q=blocks", "Search", false),
		"a title nobody wrote":   linkValue("https://example.com/a", "", false),
		"nothing filled in":      linkValue("", "", false),
		"a value the field lost": nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := content.Values{"source": value}

			if err := held.Validate([]content.Field{linked(t, nil)}); err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestALinkRefusesAShapeItDoesNotHold(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{
		"a bare address":            "https://example.com/a",
		"a list of addresses":       []any{linkValue("https://example.com/a", "A page", false)},
		"a member the link lost":    map[string]any{"url": "https://example.com/a", "title": "A page"},
		"a member the link refuses": map[string]any{"url": "/a", "title": "A", "new_tab": false, "rel": "me"},
		"an address that is a name": linkValue("gophenberg", "A page", false),
		"an address on no scheme":   linkValue("example.com/a", "A page", false),
		"an address on a scheme it refuses": linkValue(
			"javascript:alert(1)", "A page", false,
		),
		"an address that is a bare word path": linkValue("about", "About", false),
		"an address no parser can read":       linkValue("http://example.com/\x7f", "A page", false),
		"a mail address naming no mailbox":    linkValue("mailto:", "Write to us", false),
		"a title that is no text":             map[string]any{"url": "/a", "title": 1, "new_tab": false},
		"a tab choice that is no answer":      map[string]any{"url": "/a", "title": "A", "new_tab": "yes"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := content.Values{"source": value}

			err := held.Validate([]content.Field{linked(t, nil)})

			if !errors.Is(err, content.ErrFieldShape) {
				t.Fatalf("Validate() error = %v, want %v", err, content.ErrFieldShape)
			}
			if code, _ := content.CodeOf(err); code != "field_shape_kind" {
				t.Errorf("code = %q, want %q", code, "field_shape_kind")
			}
		})
	}
}

func TestALinkWithoutAnAddressIsEmpty(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		value  any
		filled bool
	}{
		"an address and a title":  {linkValue("https://example.com/a", "A page", false), true},
		"an address alone":        {linkValue("https://example.com/a", "", false), true},
		"a title without one":     {linkValue("", "A page", false), false},
		"a tab choice alone":      {linkValue("", "", true), false},
		"nothing written at all":  {linkValue("", "", false), false},
		"a value the field lost":  {nil, false},
		"members nobody supplied": {map[string]any{}, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			field := linked(t, nil)
			field.Required = true
			held := content.Values{"source": test.value}

			err := content.Filled(held, nil, []content.Field{field})

			if test.filled && err != nil {
				t.Errorf("Filled() error = %v, want the link counted as filled", err)
			}
			if !test.filled && !errors.Is(err, content.ErrFieldRequired) {
				t.Errorf("Filled() error = %v, want %v", err, content.ErrFieldRequired)
			}
		})
	}
}

func TestALinkTakesOnlyTheSettingsItReads(t *testing.T) {
	t.Parallel()

	for name, settings := range map[string]map[string]any{
		"instructions": {"instructions": "Where this points."},
		"listed":       {"listed": true},
		"conditions": {"conditions": content.ConditionsSetting(content.Rules{{{
			Source: "on-sale", Operator: content.OperatorIs, Value: "true",
		}}})},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := content.ValidateSettings(content.FieldKindLink, settings); err != nil {
				t.Errorf("ValidateSettings(%v) error = %v, want nil", settings, err)
			}
		})
	}
}

func TestALinkRefusesASettingItDoesNotRead(t *testing.T) {
	t.Parallel()

	for name, settings := range map[string]map[string]any{
		"a default":      {"default": "https://example.com/a"},
		"a longest":      {"maxlength": float64(10)},
		"a placeholder":  {"placeholder": "https://example.com"},
		"a presentation": {"presentation": "range"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := content.ValidateSettings(content.FieldKindLink, settings); err == nil {
				t.Errorf("ValidateSettings(%v) error = nil, want the setting refused", settings)
			}
		})
	}
}

func TestARuleReadsALinkOnlyForWhetherItIsFilled(t *testing.T) {
	t.Parallel()

	held := content.SourceOperators(content.FieldKindLink, false)

	want := []string{content.OperatorEmpty, content.OperatorNotEmpty}
	if len(held) != len(want) {
		t.Fatalf("SourceOperators() = %v, want %v", held, want)
	}
	for i, operator := range want {
		if held[i] != operator {
			t.Errorf("SourceOperators()[%d] = %q, want %q", i, held[i], operator)
		}
	}
}

func TestALinkStandsInsideAContainerAsPlainDataDoes(t *testing.T) {
	t.Parallel()

	for name, parent := range map[string]content.FieldKind{
		"a section":  content.FieldKindSection,
		"a repeater": content.FieldKindRepeater,
		"a layout":   content.FieldKindLayout,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held, err := content.NewSubField(content.Field{
				Key: "source", Label: "Source", Kind: content.FieldKindLink,
			}, parent)

			if err != nil {
				t.Fatalf("NewSubField() error = %v, want a link standing inside %s", err, parent)
			}
			if held.Kind != content.FieldKindLink {
				t.Errorf("Kind = %q, want %q", held.Kind, content.FieldKindLink)
			}
		})
	}
}

func TestALinkInsideARepeaterRowKeepsItsOwnShape(t *testing.T) {
	t.Parallel()

	rows, err := content.NewField(content.Field{
		TypeKey: content.TypePost, Key: "sources", Label: "Sources", Kind: content.FieldKindRepeater,
	})
	if err != nil {
		t.Fatalf("NewField(sources) error = %v, want nil", err)
	}
	inside, err := content.NewSubField(content.Field{
		Key: "source", Label: "Source", Kind: content.FieldKindLink,
	}, content.FieldKindRepeater)
	if err != nil {
		t.Fatalf("NewSubField(source) error = %v, want nil", err)
	}
	rows.Fields = []content.Field{inside}

	held := content.Values{"sources": []any{
		map[string]any{"source": linkValue("https://example.com/a", "A page", false)},
		map[string]any{"source": linkValue("/about", "About", true)},
	}}

	if err := held.Validate([]content.Field{rows}); err != nil {
		t.Errorf("Validate() error = %v, want the rows accepted", err)
	}
}
