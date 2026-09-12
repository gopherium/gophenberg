// SPDX-License-Identifier: Apache-2.0

package served_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/served"
)

// pointingType returns a type whose rows hold a relation beside one at the top of the group.
func pointingType() content.Type {
	return postType(
		content.Field{Key: "categories", Kind: content.FieldKindRelation, RelatesTo: "category", Many: true},
		content.Field{
			Key: "team", Kind: content.FieldKindRepeater,
			Fields: []content.Field{{
				Key: "wrote", Kind: content.FieldKindRelation, RelatesTo: content.TypePost, Many: true,
			}},
		},
	)
}

// namedInside returns the targets served under the key of the row standing at the place.
func namedInside(t *testing.T, values content.Values, at int, key string) []served.Target {
	t.Helper()
	rows, listed := values["team"].([]any)
	if !listed || at >= len(rows) {
		t.Fatalf("team = %#v, want rows the walk reached", values["team"])
	}
	row, held := rows[at].(map[string]any)
	if !held {
		t.Fatalf("row = %#v, want an object", rows[at])
	}
	named, ok := row[key].([]served.Target)
	if !ok {
		t.Fatalf("row[%s] = %#v, want the targets named", key, row[key])
	}
	return named
}

func TestValuesNamesTheTargetsARowPointsAt(t *testing.T) {
	t.Parallel()

	first, second := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	stores := served.Stores{Links: fakeLinks{targets: []content.Target{
		{ID: first, Title: "A first post", Path: "a-first-post"},
		{ID: second, Title: "A second post", Path: "a-second-post"},
	}}}
	stored := anItem(content.Values{"team": []any{
		map[string]any{"wrote": []any{second.String(), first.String()}},
		map[string]any{"wrote": []any{first.String()}},
	}})

	values, _, err := served.Values(t.Context(), stores, pointingType(), stored)

	if err != nil {
		t.Fatalf("Values() error = %v, want nil", err)
	}
	named := namedInside(t, values, 0, "wrote")
	if len(named) != 2 || named[0].ID != second.String() || named[1].ID != first.String() {
		t.Fatalf("the first row names %+v, want both targets in the order it holds them", named)
	}
	if named[0].Title != "A second post" || named[0].Path != "a-second-post" {
		t.Errorf("the target reads %+v, want it named and addressed", named[0])
	}
	if held := namedInside(t, values, 1, "wrote"); len(held) != 1 || held[0].ID != first.String() {
		t.Errorf("the second row names %+v, want the one target it holds", held)
	}
}

func TestValuesLeavesOutATargetNobodyServes(t *testing.T) {
	t.Parallel()

	served_, gone := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	stores := served.Stores{Links: fakeLinks{targets: []content.Target{
		{ID: served_, Title: "A post", Path: "a-post"},
	}}}
	stored := anItem(content.Values{
		"categories": []any{gone.String()},
		"team":       []any{map[string]any{"wrote": []any{gone.String(), served_.String()}}},
	})

	values, _, err := served.Values(t.Context(), stores, pointingType(), stored)

	if err != nil {
		t.Fatalf("Values() error = %v, want nil", err)
	}
	if _, held := values["categories"]; held {
		t.Errorf("categories = %#v, want the key left out where no target serves", values["categories"])
	}
	if named := namedInside(t, values, 0, "wrote"); len(named) != 1 || named[0].ID != served_.String() {
		t.Errorf("the row names %+v, want the target that serves alone", named)
	}
}

func TestValuesReportsTheTargetsItCannotRead(t *testing.T) {
	t.Parallel()

	stores := served.Stores{Links: fakeLinks{targetsErr: errStoreDown}}
	stored := anItem(content.Values{"categories": []any{uuid.Must(uuid.NewV7()).String()}})

	_, _, err := served.Values(t.Context(), stores, pointingType(), stored)

	if err == nil {
		t.Fatal("Values() error = nil, want the unreadable targets reported")
	}
}

func TestValuesLeavesTheStoredTargetsAsTheyWereRead(t *testing.T) {
	t.Parallel()

	target := uuid.Must(uuid.NewV7())
	stores := served.Stores{Links: fakeLinks{targets: []content.Target{{ID: target, Title: "A post"}}}}
	stored := anItem(content.Values{"team": []any{map[string]any{"wrote": []any{target.String()}}}})

	if _, _, err := served.Values(t.Context(), stores, pointingType(), stored); err != nil {
		t.Fatalf("Values() error = %v, want nil", err)
	}

	rows, _ := stored.Fields["team"].([]any)
	row, _ := rows[0].(map[string]any)
	held, listed := row["wrote"].([]any)
	if !listed || len(held) != 1 || held[0] != target.String() {
		t.Errorf("the stored row holds %#v, want the identities it was read with", row["wrote"])
	}
}
