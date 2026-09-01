// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestNestedValuesSurviveTheRealStore(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	team := declareRepeater(t, store, "team")
	contact, err := store.CreateSubField(t.Context(), team.ID, sectionOn(t, "contact"))
	if err != nil {
		t.Fatalf("declaring the section: %v, want nil", err)
	}
	if _, err := store.CreateSubField(
		t.Context(), contact.ID, fieldOn(t, "", "phone", content.FieldKindText, "")); err != nil {
		t.Fatalf("declaring the phone: %v, want nil", err)
	}
	held := content.Values{"team": []any{
		map[string]any{"contact": map[string]any{"phone": "184467235"}},
		map[string]any{"contact": map[string]any{"phone": "184467236"}},
	}}
	plantTyped(t, pool, author, "car", "one", `{}`)

	if held := valuesHeld(t, pool); held != `{}` {
		t.Fatalf("the planted row holds %s, want nothing yet", held)
	}
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.content SET fields = $1 WHERE type = 'car'`, held); err != nil {
		t.Fatalf("storing the nested values: %v, want nil", err)
	}

	var read content.Values
	if err := pool.QueryRow(t.Context(),
		`SELECT fields FROM core.content WHERE type = 'car'`).Scan(&read); err != nil {
		t.Fatalf("reading the nested values: %v, want nil", err)
	}
	if !reflect.DeepEqual(read, held) {
		t.Errorf("read %v, want the nested values back whole", read)
	}

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	served, found := groupNumbered(groups, team.GroupID)
	if !found {
		t.Fatalf("the group %d is not served", team.GroupID)
	}
	if err := read.Validate(served.Fields); err != nil {
		t.Errorf("Validate() error = %v, want the stored values to stand against the served tree", err)
	}
}
