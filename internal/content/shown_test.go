// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// switchedOn returns a boolean field and a field of the kind shown only while the switch holds.
func switchedOn(kind content.FieldKind, key string) []content.Field {
	return []content.Field{
		sourceField("on-sale", content.FieldKindBoolean, nil),
		{
			Key: key, Label: "A Reader", Kind: kind, Required: true,
			Settings: map[string]any{content.SettingConditions: shownWhen("on-sale", content.OperatorIs, "true")},
		},
	}
}

func TestFilledLetsAHiddenRequiredFieldStandEmpty(t *testing.T) {
	t.Parallel()

	fields := switchedOn(content.FieldKindText, "sale-note")

	err := content.Filled(content.Values{"on-sale": false}, content.Relations{}, fields)

	if err != nil {
		t.Errorf("Filled() = %v, want a hidden required field left alone", err)
	}
}

func TestFilledStillRequiresAShownField(t *testing.T) {
	t.Parallel()

	fields := switchedOn(content.FieldKindText, "sale-note")

	err := content.Filled(content.Values{"on-sale": true}, content.Relations{}, fields)

	if !errors.Is(err, content.ErrFieldRequired) {
		t.Errorf("Filled() = %v, want %v", err, content.ErrFieldRequired)
	}
}

func TestFilledLeavesTheFieldsInsideAHiddenContainerAlone(t *testing.T) {
	t.Parallel()

	fields := switchedOn(content.FieldKindSection, "sale-details")
	fields[1].Fields = []content.Field{{
		Key: "fee", Label: "Fee", Kind: content.FieldKindNumber, Required: true,
	}}
	values := content.Values{"on-sale": false, "sale-details": map[string]any{}}

	err := content.Filled(values, content.Relations{}, fields)

	if err != nil {
		t.Errorf("Filled() = %v, want a hidden container's required fields left alone", err)
	}
}

func TestFilledRequiresTheFieldsInsideAShownContainer(t *testing.T) {
	t.Parallel()

	fields := switchedOn(content.FieldKindSection, "sale-details")
	fields[1].Required = false
	fields[1].Fields = []content.Field{{
		Key: "fee", Label: "Fee", Kind: content.FieldKindNumber, Required: true,
	}}
	values := content.Values{"on-sale": true, "sale-details": map[string]any{}}

	err := content.Filled(values, content.Relations{}, fields)

	if !errors.Is(err, content.ErrFieldRequired) {
		t.Errorf("Filled() = %v, want %v", err, content.ErrFieldRequired)
	}
}

func TestShownTakesAwayWhatTheConditionsHide(t *testing.T) {
	t.Parallel()

	fields := switchedOn(content.FieldKindText, "sale-note")
	values := content.Values{"on-sale": false, "sale-note": "half price"}

	shown := content.Shown(fields, values)

	if _, held := shown["sale-note"]; held {
		t.Errorf("Shown() = %v, want the hidden key taken away", shown)
	}
	if shown["on-sale"] != false {
		t.Errorf("Shown() = %v, want the shown key kept", shown)
	}
}

func TestShownKeepsWhatTheConditionsShow(t *testing.T) {
	t.Parallel()

	fields := switchedOn(content.FieldKindText, "sale-note")
	values := content.Values{"on-sale": true, "sale-note": "half price"}

	shown := content.Shown(fields, values)

	if shown["sale-note"] != "half price" {
		t.Errorf("Shown() = %v, want the shown key kept", shown)
	}
}

func TestShownLeavesTheValuesItReadsAlone(t *testing.T) {
	t.Parallel()

	fields := switchedOn(content.FieldKindText, "sale-note")
	values := content.Values{"on-sale": false, "sale-note": "half price"}

	content.Shown(fields, values)

	if values["sale-note"] != "half price" {
		t.Errorf("the values read = %v, want the stored map left as it was", values)
	}
}

func TestShownTakesAwayAHiddenKeyOfEveryRow(t *testing.T) {
	t.Parallel()

	fields := []content.Field{{
		Key: "crew", Label: "Crew", Kind: content.FieldKindRepeater,
		Fields: switchedOn(content.FieldKindText, "fee"),
	}}
	values := content.Values{"crew": []any{
		map[string]any{"on-sale": true, "fee": "kept"},
		map[string]any{"on-sale": false, "fee": "hidden"},
	}}

	shown := content.Shown(fields, values)

	rows, listed := shown["crew"].([]any)
	if !listed || len(rows) != 2 {
		t.Fatalf("Shown() = %v, want both rows served", shown)
	}
	if rows[0].(map[string]any)["fee"] != "kept" {
		t.Errorf("the first row = %v, want the shown value kept", rows[0])
	}
	if _, held := rows[1].(map[string]any)["fee"]; held {
		t.Errorf("the second row = %v, want the hidden value taken away", rows[1])
	}
}

func TestShownTakesAwayAHiddenKeyOfASection(t *testing.T) {
	t.Parallel()

	fields := []content.Field{{
		Key: "details", Label: "Details", Kind: content.FieldKindSection,
		Fields: switchedOn(content.FieldKindText, "fee"),
	}}
	values := content.Values{"details": map[string]any{"on-sale": false, "fee": "hidden"}}

	shown := content.Shown(fields, values)

	inside, held := shown["details"].(map[string]any)
	if !held {
		t.Fatalf("Shown() = %v, want the section served", shown)
	}
	if _, found := inside["fee"]; found {
		t.Errorf("the section = %v, want the hidden value taken away", inside)
	}
}

func TestShownLeavesAContainerItCannotReadAlone(t *testing.T) {
	t.Parallel()

	fields := []content.Field{{
		Key: "crew", Label: "Crew", Kind: content.FieldKindSection,
		Fields: switchedOn(content.FieldKindText, "fee"),
	}}
	values := content.Values{"crew": "not a container"}

	shown := content.Shown(fields, values)

	if shown["crew"] != "not a container" {
		t.Errorf("Shown() = %v, want a value it cannot read left as it was", shown)
	}
}
