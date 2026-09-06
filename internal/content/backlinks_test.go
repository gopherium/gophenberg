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

// namingType returns a location matching the one content type.
func namingType(key string) content.Rules {
	return content.Rules{{{
		Source: content.ScreenContentType, Operator: content.OperatorIs, Value: key,
	}}}
}

// sourceGroups returns the group holding a relation pointing at posts, beside the group reading it.
func sourceGroups(relatesTo string) []content.Group {
	return []content.Group{
		{
			ID: 1, Key: "cars", Title: "Cars", Active: true, Location: namingType("car"),
			Fields: []content.Field{
				{Key: "maker", Label: "Maker", Kind: content.FieldKindRelation, RelatesTo: relatesTo},
				{Key: "note", Label: "Note", Kind: content.FieldKindText},
			},
		},
	}
}

// readingGroup returns the group a backlinks field stands in, placed on posts.
func readingGroup() content.Group {
	return content.Group{ID: 2, Key: "makers", Title: "Makers", Active: true, Location: namingType(content.TypePost)}
}

func TestBacklinksReadsTheRelationItsSourceNames(t *testing.T) {
	t.Parallel()

	types := []content.Type{{Key: content.TypePost, Active: true}, {Key: "car", Active: true}}

	held, err := content.BacklinksSource(sourceGroups(content.TypePost), types, readingGroup(),
		backlinksField(namingSource()), content.DefaultParamRegistry(nil))

	if err != nil {
		t.Fatalf("BacklinksSource() error = %v, want nil", err)
	}
	if held.Key != "maker" {
		t.Errorf("source = %q, want the maker relation", held.Key)
	}
}

func TestBacklinksRefusesASourceItCannotFind(t *testing.T) {
	t.Parallel()

	types := []content.Type{{Key: content.TypePost, Active: true}, {Key: "car", Active: true}}
	for name, settings := range map[string]map[string]any{
		"a group nobody declared": {
			content.SettingSourceGroup: "vans", content.SettingSourceField: []any{"maker"},
		},
		"a field the group lacks": {
			content.SettingSourceGroup: "cars", content.SettingSourceField: []any{"driver"},
		},
		"a field that is not a relation": {
			content.SettingSourceGroup: "cars", content.SettingSourceField: []any{"note"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := content.BacklinksSource(
				sourceGroups(content.TypePost), types, readingGroup(), backlinksField(settings), content.DefaultParamRegistry(nil))

			if codeOf(err) != "backlinks_source_unknown" {
				t.Errorf("BacklinksSource() error = %v, want backlinks_source_unknown", err)
			}
		})
	}
}

func TestBacklinksRefusesASourcePointingSomewhereElse(t *testing.T) {
	t.Parallel()

	types := []content.Type{{Key: content.TypePost, Active: true}, {Key: "car", Active: true}}

	_, err := content.BacklinksSource(
		sourceGroups("car"), types, readingGroup(), backlinksField(namingSource()), content.DefaultParamRegistry(nil))

	if codeOf(err) != "backlinks_source_elsewhere" {
		t.Errorf("BacklinksSource() error = %v, want backlinks_source_elsewhere", err)
	}
}

func TestBacklinksReadsTheSourceSettingsItCarries(t *testing.T) {
	t.Parallel()

	held := backlinksField(namingSource())

	if content.SourceGroupOf(held) != "cars" {
		t.Errorf("SourceGroupOf() = %q, want cars", content.SourceGroupOf(held))
	}
	if path := content.SourceFieldOf(held); len(path) != 1 || path[0] != "maker" {
		t.Errorf("SourceFieldOf() = %v, want the one maker segment", path)
	}
}

func TestBacklinksReadsNoPathFromASegmentThatIsNotAKey(t *testing.T) {
	t.Parallel()

	held := backlinksField(map[string]any{content.SettingSourceField: []any{"maker", 3.0}})

	if path := content.SourceFieldOf(held); path != nil {
		t.Errorf("SourceFieldOf() = %v, want nothing", path)
	}
}

func TestBacklinksRefusesASourceItCannotAddress(t *testing.T) {
	t.Parallel()

	types := []content.Type{{Key: content.TypePost, Active: true}, {Key: "car", Active: true}}
	groups := []content.Group{{
		ID: 1, Key: "cars", Title: "Cars", Active: true, Location: namingType("car"),
		Fields: []content.Field{{
			Key: "details", Label: "Details", Kind: content.FieldKindSection,
			Fields: []content.Field{{Key: "note", Label: "Note", Kind: content.FieldKindText}},
		}},
	}}
	for name, settings := range map[string]map[string]any{
		"a path reaching inside a container": {
			content.SettingSourceGroup: "cars",
			content.SettingSourceField: []any{"details", "note"},
		},
		"a path reaching nothing inside a container": {
			content.SettingSourceGroup: "cars",
			content.SettingSourceField: []any{"details", "absent"},
		},
		"no group named at all": {content.SettingSourceField: []any{"maker"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := content.BacklinksSource(groups, types, readingGroup(),
				backlinksField(settings), content.DefaultParamRegistry(nil))

			if codeOf(err) != "backlinks_source_unknown" {
				t.Errorf("BacklinksSource() error = %v, want backlinks_source_unknown", err)
			}
		})
	}
}

func TestBacklinksRefusesASourcePointingAtAnUnregisteredType(t *testing.T) {
	t.Parallel()

	types := []content.Type{{Key: content.TypePost, Active: true}}

	_, err := content.BacklinksSource(sourceGroups("van"), types, readingGroup(),
		backlinksField(namingSource()), content.DefaultParamRegistry(nil))

	if codeOf(err) != "backlinks_source_elsewhere" {
		t.Errorf("BacklinksSource() error = %v, want backlinks_source_elsewhere", err)
	}
}

func TestRegistryRefusesABacklinksNamingASourceItCannotRead(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	group := groupNaming(t, registry, "Makers", namingPost())

	_, err := registry.CreateFieldInGroup(t.Context(), group.ID, backlinksField(namingSource()))

	if codeOf(err) != "backlinks_source_unknown" {
		t.Errorf("CreateFieldInGroup(backlinks) error = %v, want backlinks_source_unknown", err)
	}
}

func TestRegistryReportsTheTypesItCannotReadForABacklinks(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	group := groupNaming(t, registry, "Makers", namingPost())
	store.listErr = errStoreDown

	_, err := registry.CreateFieldInGroup(t.Context(), group.ID, backlinksField(namingSource()))

	if !errors.Is(err, errStoreDown) {
		t.Errorf("CreateFieldInGroup(backlinks) error = %v, want %v", err, errStoreDown)
	}
}

func TestRegistryRefusesABacklinksWhoseSourceMovesAway(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	group := groupNaming(t, registry, "Makers", namingPost())
	stored, err := registry.CreateFieldInGroup(t.Context(), group.ID, content.Field{
		Key: "note", Label: "Note", Kind: content.FieldKindText,
	})
	if err != nil {
		t.Fatalf("CreateFieldInGroup(text) error = %v, want nil", err)
	}
	stored.Kind = content.FieldKindBacklinks
	stored.Settings = namingSource()

	_, err = registry.UpdateFieldInGroup(t.Context(), group.ID, stored, stored.UpdatedAt)

	if err == nil {
		t.Error("UpdateFieldInGroup() error = nil, want the unreadable source reported")
	}
}

// readableSource declares a car type, a group holding a relation pointing at posts, and returns both groups.
func readableSource(t *testing.T, registry *content.Registry) (content.Group, content.Group) {
	t.Helper()
	if _, err := registry.Create(t.Context(), content.Type{
		Key: "car", SingularLabel: "Car", PluralLabel: "Cars", Active: true,
		PageKind: content.PageKindSingle, RouteWord: "cars",
	}); err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	cars, err := registry.CreateGroup(t.Context(), content.Group{
		Key: "cars", Title: "Cars", Location: namingType("car"), Active: true,
	})
	if err != nil {
		t.Fatalf("CreateGroup(cars) error = %v, want nil", err)
	}
	if _, err := registry.CreateFieldInGroup(t.Context(), cars.ID, content.Field{
		Key: "maker", Label: "Maker", Kind: content.FieldKindRelation, RelatesTo: content.TypePost,
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(maker) error = %v, want nil", err)
	}
	makers := groupNaming(t, registry, "Makers", namingPost())
	if _, err := registry.CreateFieldInGroup(
		t.Context(), makers.ID, backlinksField(namingSource())); err != nil {
		t.Fatalf("CreateFieldInGroup(backlinks) error = %v, want nil", err)
	}
	return cars, makers
}

func TestRegistryKeepsTheSourceABacklinksReads(t *testing.T) {
	t.Parallel()

	for name, run := range map[string]func(*content.Registry, content.Group) error{
		"deleting the relation": func(r *content.Registry, cars content.Group) error {
			return r.DeleteFieldInGroup(t.Context(), cars.ID, "maker")
		},
		"deleting the group holding it": func(r *content.Registry, cars content.Group) error {
			return r.DeleteGroup(t.Context(), cars.ID)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			registry := content.NewRegistry(newGroupingStore())
			cars, _ := readableSource(t, registry)

			err := run(registry, cars)

			if codeOf(err) != "field_referenced" {
				t.Errorf("error = %v, want field_referenced", err)
			}
		})
	}
}

func TestRegistryFreesARelationNoBacklinksNames(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	readableSource(t, registry)
	vans, err := registry.CreateGroup(t.Context(), content.Group{
		Key: "vans", Title: "Vans", Location: namingPost(), Active: true,
	})
	if err != nil {
		t.Fatalf("CreateGroup(vans) error = %v, want nil", err)
	}
	if _, err := registry.CreateFieldInGroup(t.Context(), vans.ID, content.Field{
		Key: "maker", Label: "Maker", Kind: content.FieldKindRelation, RelatesTo: content.TypePost,
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(maker) error = %v, want nil", err)
	}

	if err := registry.DeleteFieldInGroup(t.Context(), vans.ID, "maker"); err != nil {
		t.Errorf("DeleteFieldInGroup() error = %v, want the relation nobody reads taken away", err)
	}
}

func TestRegistryKeepsABacklinksOnTheTypeItsSourcePointsAt(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	_, makers := readableSource(t, registry)
	makers.Location = namingType("car")

	_, err := registry.UpdateGroup(t.Context(), makers)

	if codeOf(err) != "backlinks_source_elsewhere" {
		t.Errorf("UpdateGroup() error = %v, want backlinks_source_elsewhere", err)
	}
}

func TestRegistryKeepsAMovedBacklinksReadingItsSource(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	cars, makers := readableSource(t, registry)

	_, err := registry.MoveField(t.Context(), makers.ID, "linked-from", cars.ID)

	if codeOf(err) != "backlinks_source_elsewhere" {
		t.Errorf("MoveField() error = %v, want backlinks_source_elsewhere", err)
	}
}

func TestRegistryKeepsTheSourceWhereTheBacklinksLooksForIt(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	cars, makers := readableSource(t, registry)

	_, err := registry.MoveField(t.Context(), cars.ID, "maker", makers.ID)

	if codeOf(err) != "field_referenced" {
		t.Errorf("MoveField() error = %v, want field_referenced", err)
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
