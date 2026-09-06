// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// backlinksField returns a backlinks field naming the source the settings carry.
func backlinksField(settings map[string]any) content.Field {
	return content.Field{
		TypeKey: "post", Key: "linked-from", Label: "Linked from",
		Kind: content.FieldKindBacklinks, Settings: settings,
	}
}

// namingSource returns the settings naming a relation field the demo groups declare.
func namingSource() map[string]any {
	return map[string]any{
		content.SettingSourceGroup: "cars",
		content.SettingSourceField: []any{"maker"},
	}
}

func TestNewFieldAcceptsTheBacklinksKind(t *testing.T) {
	t.Parallel()

	built, err := content.NewField(backlinksField(namingSource()))

	if err != nil {
		t.Fatalf("NewField(backlinks) error = %v, want nil", err)
	}
	if built.Kind != content.FieldKindBacklinks {
		t.Errorf("Kind = %q, want %q", built.Kind, content.FieldKindBacklinks)
	}
}

func TestBacklinksInsideAContainerIsRefused(t *testing.T) {
	t.Parallel()

	for _, parent := range []content.FieldKind{
		content.FieldKindSection, content.FieldKindRepeater, content.FieldKindLayout,
	} {
		_, err := content.NewSubField(backlinksField(namingSource()), parent)

		if !errors.Is(err, content.ErrFieldShape) || codeOf(err) != "field_backlinks_inside" {
			t.Errorf("NewSubField(backlinks under %s) error = %v, want field_backlinks_inside", parent, err)
		}
	}
}

func TestBacklinksTakesOnlyItsSourceSettings(t *testing.T) {
	t.Parallel()

	held := content.ValidateSettings(content.FieldKindBacklinks, namingSource())

	if held != nil {
		t.Errorf("ValidateSettings(backlinks) error = %v, want nil", held)
	}
}

func TestBacklinksRefusesASettingItDoesNotTake(t *testing.T) {
	t.Parallel()

	err := content.ValidateSettings(content.FieldKindBacklinks, map[string]any{content.SettingMax: 3.0})

	if codeOf(err) != "setting_unknown" {
		t.Errorf("ValidateSettings() error = %v, want setting_unknown", err)
	}
}

func TestBacklinksRefusesASourceOfTheWrongShape(t *testing.T) {
	t.Parallel()

	for name, settings := range map[string]map[string]any{
		"a group that is not a key":  {content.SettingSourceGroup: 3.0},
		"a field that is not a path": {content.SettingSourceField: "maker"},
		"a path holding no segment":  {content.SettingSourceField: []any{}},
		"a segment that is not a key": {
			content.SettingSourceField: []any{"maker", 3.0},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := content.ValidateSettings(content.FieldKindBacklinks, settings)

			if codeOf(err) != "setting_shape" {
				t.Errorf("ValidateSettings() error = %v, want setting_shape", err)
			}
		})
	}
}

func TestBacklinksTakesNoSubmittedValue(t *testing.T) {
	t.Parallel()

	built, err := content.NewField(backlinksField(namingSource()))
	if err != nil {
		t.Fatalf("NewField(backlinks) error = %v, want nil", err)
	}
	held := content.Values{"linked-from": []any{"019fb000-0000-7000-8000-000000000001"}}

	if code := codeOf(held.Validate([]content.Field{built})); code != "field_shape_value" {
		t.Errorf("Validate() error code = %q, want field_shape_value", code)
	}
}

func TestSplitValuesRefusesABacklinksValue(t *testing.T) {
	t.Parallel()

	built, err := content.NewField(backlinksField(namingSource()))
	if err != nil {
		t.Fatalf("NewField(backlinks) error = %v, want nil", err)
	}

	_, _, err = content.SplitValues(
		content.Values{"linked-from": []any{"019fb000-0000-7000-8000-000000000001"}},
		[]content.Field{built},
	)

	if codeOf(err) != "field_shape_value" {
		t.Errorf("SplitValues() error = %v, want field_shape_value", err)
	}
}
