// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// filterable returns one field of every kind a filter reads, one it does not, and a container holding a text.
func filterable() []content.Field {
	return []content.Field{
		{Key: "note", Kind: content.FieldKindText},
		{Key: "price", Kind: content.FieldKindNumber},
		{Key: "on-sale", Kind: content.FieldKindBoolean},
		{Key: "since", Kind: content.FieldKindDate},
		{Key: "colour", Kind: content.FieldKindChoice},
		{Key: "tags", Kind: content.FieldKindChoice, Settings: map[string]any{content.SettingMultiple: true}},
		{Key: "cover", Kind: content.FieldKindMedia},
		{Key: "crew", Kind: content.FieldKindRepeater, Fields: []content.Field{{Key: "name", Kind: content.FieldKindText}}},
	}
}

func TestParseFieldFilterCoercesEachTermByItsKind(t *testing.T) {
	t.Parallel()

	query := url.Values{
		"field[note]": {"half price"}, "field[price]": {"10"}, "field[on-sale]": {"true"},
		"field[since]": {"2026-09-05"}, "field[colour]": {"red"}, "field[tags]": {"red"}, "page": {"2"},
	}

	terms, err := content.ParseFieldFilter(query, filterable())

	if err != nil {
		t.Fatalf("ParseFieldFilter() = %v, want every term read", err)
	}
	want := map[string]any{
		"note": "half price", "price": 10.0, "on-sale": true, "since": "2026-09-05", "colour": "red", "tags": []any{"red"},
	}
	if !reflect.DeepEqual(terms, want) {
		t.Errorf("terms = %#v, want %#v", terms, want)
	}
}

func TestParseFieldFilterReadsNothingWhenNoTermIsNamed(t *testing.T) {
	t.Parallel()

	terms, err := content.ParseFieldFilter(url.Values{"page": {"2"}, "type": {"post"}}, filterable())

	if err != nil {
		t.Fatalf("ParseFieldFilter() = %v, want nothing refused", err)
	}
	if terms != nil {
		t.Errorf("terms = %#v, want none", terms)
	}
}

func TestParseFieldFilterRefusesATermItCannotRead(t *testing.T) {
	t.Parallel()

	cases := map[string]url.Values{
		"a key named twice":           {"field[note]": {"a", "b"}},
		"a key the type lacks":        {"field[missing]": {"a"}},
		"a key inside a container":    {"field[name]": {"a"}},
		"a kind no filter reads":      {"field[cover]": {"a"}},
		"a number written in words":   {"field[price]": {"ten"}},
		"a number that is not one":    {"field[price]": {"NaN"}},
		"a number without a bound":    {"field[price]": {"Inf"}},
		"a boolean that is a word":    {"field[on-sale]": {"yes"}},
		"a date outside the calendar": {"field[since]": {"2026-13-01"}},
	}
	for name, query := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			terms, err := content.ParseFieldFilter(query, filterable())

			if !errors.Is(err, content.ErrFieldFilterInvalid) {
				t.Errorf("ParseFieldFilter() = %v, want %v", err, content.ErrFieldFilterInvalid)
			}
			if terms != nil {
				t.Errorf("terms = %#v, want none beside a refusal", terms)
			}
		})
	}
}

func TestListedValuesKeepsTheShownValuesUnderListedFields(t *testing.T) {
	t.Parallel()

	fields := []content.Field{
		{Key: "note", Kind: content.FieldKindText, Settings: map[string]any{content.SettingListed: true}},
		{Key: "price", Kind: content.FieldKindNumber},
		{Key: "on-sale", Kind: content.FieldKindBoolean, Settings: map[string]any{content.SettingListed: true}},
		{Key: "sale-note", Kind: content.FieldKindText, Settings: map[string]any{
			content.SettingListed:     true,
			content.SettingConditions: shownWhen("on-sale", content.OperatorIs, "true"),
		}},
		{Key: "since", Kind: content.FieldKindDate, Settings: map[string]any{content.SettingListed: true}},
	}
	values := content.Values{"note": "half price", "price": 10.0, "on-sale": false, "sale-note": "frozen"}

	listed := content.ListedValues(fields, values)

	want := content.Values{"note": "half price", "on-sale": false}
	if !reflect.DeepEqual(listed, want) {
		t.Errorf("ListedValues() = %#v, want %#v", listed, want)
	}
}

func TestListedValuesAnswersAnEmptyObjectWhenNothingIsListed(t *testing.T) {
	t.Parallel()

	listed := content.ListedValues(filterable(), content.Values{"note": "half price"})

	if !reflect.DeepEqual(listed, content.Values{}) {
		t.Errorf("ListedValues() = %#v, want an empty object", listed)
	}
}

func TestNamesFieldFilterReadsWhetherAQueryCarriesATerm(t *testing.T) {
	t.Parallel()

	for name, held := range map[string]bool{
		"field[price]": true, "field[]": true, "price": false, "page": false, "field": false,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if named := content.NamesFieldFilter(url.Values{name: {"10"}}); named != held {
				t.Errorf("NamesFieldFilter(%q) = %v, want %v", name, named, held)
			}
		})
	}
}
