// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

func TestTheStoreHoldsAChoiceField(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	group, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	declared := fieldOn(t, "", "style", content.FieldKindChoice, "")
	declared.Settings = map[string]any{
		"choices": []any{
			map[string]any{"value": "ipa", "label": "IPA"},
			map[string]any{"value": "stout", "label": "Stout"},
		},
		"presentation": "radio",
	}

	created, err := store.CreateFieldInGroup(t.Context(), group.ID, declared)

	if err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	if created.Kind != content.FieldKindChoice {
		t.Errorf("Kind = %q, want %q stored", created.Kind, content.FieldKindChoice)
	}
	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := groupOf(groups, group.ID)
	if !found || len(held.Fields) != 1 {
		t.Fatalf("groups = %+v, want the stored field listed", groups)
	}
	if held.Fields[0].Kind != content.FieldKindChoice {
		t.Errorf("listed kind = %q, want %q read back", held.Fields[0].Kind, content.FieldKindChoice)
	}
	if !reflect.DeepEqual(held.Fields[0].Settings, declared.Settings) {
		t.Errorf("listed settings = %v, want %v read back", held.Fields[0].Settings, declared.Settings)
	}
}

func TestTheStoreNamesTheTargetAFieldPointsAtInVain(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	group, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	declared := fieldOn(t, "", "engine", content.FieldKindRelation, "nosuchtype")

	_, err = store.CreateFieldInGroup(t.Context(), group.ID, declared)

	if !errors.Is(err, content.ErrTargetUnknown) {
		t.Errorf("CreateFieldInGroup() error = %v, want %v", err, content.ErrTargetUnknown)
	}
}

func TestTheStoreHoldsAManyMedia(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	group, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	declared := fieldOn(t, "", "gallery", content.FieldKindMedia, "")
	declared.Many = true

	created, err := store.CreateFieldInGroup(t.Context(), group.ID, declared)

	if err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	if !created.Many {
		t.Errorf("Many = false, want the media field holding many stored")
	}
	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := groupOf(groups, group.ID)
	if !found || len(held.Fields) != 1 || !held.Fields[0].Many {
		t.Fatalf("groups = %+v, want the many media listed", groups)
	}
}

func TestRollingBackPastOpenKindsDropsWhatTheOldCheckRefuses(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	group, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	for _, declared := range []content.Field{
		fieldOn(t, "", "style", content.FieldKindChoice, ""),
		fieldOn(t, "", "subtitle", content.FieldKindText, ""),
	} {
		if _, err := store.CreateFieldInGroup(t.Context(), group.ID, declared); err != nil {
			t.Fatalf("CreateFieldInGroup(%s) error = %v, want nil", declared.Key, err)
		}
	}
	gallery := fieldOn(t, "", "gallery", content.FieldKindMedia, "")
	gallery.Many = true
	if _, err := store.CreateFieldInGroup(t.Context(), group.ID, gallery); err != nil {
		t.Fatalf("CreateFieldInGroup(gallery) error = %v, want nil", err)
	}
	url := pool.Config().ConnString()

	if err := postgres.MigrateDownTo(t.Context(), url, 14); err != nil {
		t.Fatalf("MigrateDownTo(14) error = %v, want nil", err)
	}
	if err := postgres.Migrate(t.Context(), url); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := groupOf(groups, group.ID)
	if !found {
		t.Fatalf("groups = %+v, want the group standing after the walk", groups)
	}
	keys := make(map[string]content.Field, len(held.Fields))
	for _, f := range held.Fields {
		keys[f.Key] = f
	}
	if _, stands := keys["style"]; stands {
		t.Errorf("fields = %v, want the choice dropped by the rollback", keys)
	}
	if _, stands := keys["subtitle"]; !stands {
		t.Errorf("fields = %v, want the text field standing", keys)
	}
	if kept, stands := keys["gallery"]; !stands || kept.Many {
		t.Errorf("fields = %v, want the gallery standing and holding one", keys)
	}
}

func TestContentStoreSnapshotsAChoiceAndAListValue(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareField(t, pool, "style", content.FieldKindChoice)
	types := postgres.NewTypeStore(pool)
	gallery := fieldOn(t, "post", "gallery", content.FieldKindMedia, "")
	gallery.Many = true
	if _, err := types.CreateField(t.Context(), gallery); err != nil {
		t.Fatalf("declaring the gallery field: %v, want nil", err)
	}
	post := mustPost(t, "Hello world", author)
	post.Fields = content.Values{"style": "ipa", "gallery": []any{float64(1), float64(2)}}

	created, err := store.Create(t.Context(), post)

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Fields["style"] != "ipa" {
		t.Errorf("Fields = %v, want the choice value kept", created.Fields)
	}
	listed, ok := created.Fields["gallery"].([]any)
	if !ok || len(listed) != 2 {
		t.Errorf("Fields = %v, want the media list kept", created.Fields)
	}
}
