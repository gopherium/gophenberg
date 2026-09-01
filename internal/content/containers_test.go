// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// holding returns a container field of the kind carrying the sub fields.
func holding(
	t *testing.T, kind content.FieldKind, key string, settings map[string]any, subs ...content.Field,
) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: "post", Key: key, Label: key, Kind: kind, Settings: settings,
	})
	if err != nil {
		t.Fatalf("NewField(%s) error = %v, want nil", kind, err)
	}
	built.Fields = subs
	return built
}

// leaf returns an ordinary sub field of the kind carrying the settings.
func leaf(t *testing.T, kind content.FieldKind, key string, settings map[string]any) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: "post", Key: key, Label: key, Kind: kind, Settings: settings,
	})
	if err != nil {
		t.Fatalf("NewField(%s) error = %v, want nil", kind, err)
	}
	return built
}

func TestSectionTakesTheValuesItsSubFieldsDeclare(t *testing.T) {
	t.Parallel()

	author := holding(t, content.FieldKindSection, "author", nil,
		leaf(t, content.FieldKindText, "name", nil))

	held := content.Values{"author": map[string]any{"name": "Maria Perez"}}

	if err := held.Validate([]content.Field{author}); err != nil {
		t.Errorf("Validate() error = %v, want the section taken", err)
	}
}

func TestSectionRefusesASubFieldItDoesNotDeclare(t *testing.T) {
	t.Parallel()

	author := holding(t, content.FieldKindSection, "author", nil,
		leaf(t, content.FieldKindText, "name", nil))

	held := content.Values{"author": map[string]any{"nickname": "Kip"}}

	if err := held.Validate([]content.Field{author}); !errors.Is(err, content.ErrUnknownField) {
		t.Errorf("Validate() error = %v, want %v", err, content.ErrUnknownField)
	}
}

func TestSectionRefusesAValueThatIsNotAnObject(t *testing.T) {
	t.Parallel()

	author := holding(t, content.FieldKindSection, "author", nil,
		leaf(t, content.FieldKindText, "name", nil))

	held := content.Values{"author": "Maria Perez"}

	if err := held.Validate([]content.Field{author}); !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("Validate() error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestSectionCarriesTheBoundsItsSubFieldsHold(t *testing.T) {
	t.Parallel()

	author := holding(t, content.FieldKindSection, "author", nil,
		leaf(t, content.FieldKindNumber, "rating", map[string]any{"min": float64(1), "max": float64(10)}))

	held := content.Values{"author": map[string]any{"rating": float64(50)}}

	if err := held.Validate([]content.Field{author}); !errors.Is(err, content.ErrFieldBounds) {
		t.Errorf("Validate() error = %v, want %v", err, content.ErrFieldBounds)
	}
	if err := held.ValidateShape([]content.Field{author}); err != nil {
		t.Errorf("ValidateShape() error = %v, want the buffer to park it", err)
	}
}

func TestRepeaterTakesTheRowsItsSubFieldsDeclare(t *testing.T) {
	t.Parallel()

	team := holding(t, content.FieldKindRepeater, "team", nil,
		leaf(t, content.FieldKindText, "name", nil))

	held := content.Values{"team": []any{
		map[string]any{"name": "Maria Perez"},
		map[string]any{"name": "Kip"},
	}}

	if err := held.Validate([]content.Field{team}); err != nil {
		t.Errorf("Validate() error = %v, want the rows taken", err)
	}
}

func TestRepeaterRefusesARowThatIsNotAnObject(t *testing.T) {
	t.Parallel()

	team := holding(t, content.FieldKindRepeater, "team", nil,
		leaf(t, content.FieldKindText, "name", nil))

	held := content.Values{"team": []any{"Maria Perez"}}

	if err := held.Validate([]content.Field{team}); !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("Validate() error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestRepeaterRefusesFewerRowsThanItAsksFor(t *testing.T) {
	t.Parallel()

	team := holding(t, content.FieldKindRepeater, "team", map[string]any{"min": float64(2)},
		leaf(t, content.FieldKindText, "name", nil))

	held := content.Values{"team": []any{map[string]any{"name": "Maria Perez"}}}

	if err := held.Validate([]content.Field{team}); !errors.Is(err, content.ErrFieldBounds) {
		t.Errorf("Validate() error = %v, want %v", err, content.ErrFieldBounds)
	}
}

func TestRepeaterRefusesMoreRowsThanItTakes(t *testing.T) {
	t.Parallel()

	team := holding(t, content.FieldKindRepeater, "team", map[string]any{"max": float64(1)},
		leaf(t, content.FieldKindText, "name", nil))

	held := content.Values{"team": []any{
		map[string]any{"name": "Maria Perez"},
		map[string]any{"name": "Kip"},
	}}

	if err := held.Validate([]content.Field{team}); !errors.Is(err, content.ErrFieldBounds) {
		t.Errorf("Validate() error = %v, want %v", err, content.ErrFieldBounds)
	}
}

func TestRepeaterTakesTwoRowsThatReadAlike(t *testing.T) {
	t.Parallel()

	team := holding(t, content.FieldKindRepeater, "team", nil,
		leaf(t, content.FieldKindText, "name", nil))

	held := content.Values{"team": []any{
		map[string]any{"name": "Maria Perez"},
		map[string]any{"name": "Maria Perez"},
	}}

	if err := held.Validate([]content.Field{team}); err != nil {
		t.Errorf("Validate() error = %v, want two alike rows taken", err)
	}
}

func TestRepeaterRowHoldsASectionOfItsOwn(t *testing.T) {
	t.Parallel()

	team := holding(t, content.FieldKindRepeater, "team", nil,
		holding(t, content.FieldKindSection, "contact", nil,
			leaf(t, content.FieldKindText, "phone", nil)))

	held := content.Values{"team": []any{
		map[string]any{"contact": map[string]any{"phone": "184467235"}},
	}}

	if err := held.Validate([]content.Field{team}); err != nil {
		t.Errorf("Validate() error = %v, want the nested section taken", err)
	}
}

func TestRepeaterRefusesAValueThatIsNotRows(t *testing.T) {
	t.Parallel()

	team := holding(t, content.FieldKindRepeater, "team", nil,
		leaf(t, content.FieldKindText, "name", nil))

	held := content.Values{"team": map[string]any{"name": "Maria Perez"}}

	if err := held.Validate([]content.Field{team}); !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("Validate() error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestRepeaterBufferParksRowsTheBoundsRefuse(t *testing.T) {
	t.Parallel()

	team := holding(t, content.FieldKindRepeater, "team", map[string]any{"min": float64(2)},
		leaf(t, content.FieldKindText, "name", nil))

	held := content.Values{"team": []any{map[string]any{"name": "Maria Perez"}}}

	if err := held.ValidateShape([]content.Field{team}); err != nil {
		t.Errorf("ValidateShape() error = %v, want the buffer to park the rows", err)
	}
}

func TestContainerTakesASubFieldItHolds(t *testing.T) {
	t.Parallel()

	inside := content.Field{TypeKey: "post", Key: "name", Label: "Name", Kind: content.FieldKindText}

	built, err := content.NewSubField(inside, content.FieldKindRepeater)

	if err != nil {
		t.Fatalf("NewSubField(text under repeater) error = %v, want nil", err)
	}
	if built.Key != "name" || built.Kind != content.FieldKindText {
		t.Errorf("built = %+v, want the text sub field", built)
	}
}

func TestContainerRefusesARelationInside(t *testing.T) {
	t.Parallel()

	inside := content.Field{
		TypeKey: "post", Key: "wrote", Label: "Wrote",
		Kind: content.FieldKindRelation, RelatesTo: "post",
	}

	_, err := content.NewSubField(inside, content.FieldKindSection)

	if !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("NewSubField(relation) error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestOnlyAContainerTakesSubFields(t *testing.T) {
	t.Parallel()

	inside := content.Field{TypeKey: "post", Key: "name", Label: "Name", Kind: content.FieldKindText}

	_, err := content.NewSubField(inside, content.FieldKindText)

	if !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("NewSubField(under text) error = %v, want %v", err, content.ErrFieldShape)
	}
}
