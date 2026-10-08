// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"reflect"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// typeFieldCases are the cases over deleting a field definition along with the values it held.
var typeFieldCases = []Case{
	{"DeleteFieldInGroupSweepsRevisionValues", deleteFieldInGroupSweepsRevisionValues},
}

// deleteFieldInGroupSweepsRevisionValues sweeps a deleted field's value from the item and its revision alike.
func deleteFieldInGroupSweepsRevisionValues(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	declared := DeclareTypedField(t, s.Types, "post", "color")
	DeclareTypedField(t, s.Types, "post", "other")
	planted, revision := TypedHolding(t, s.Types, s.Content, "post", "Planted", author,
		content.Values{"color": "red", "other": float64(1)})

	if err := s.Types.DeleteFieldInGroup(t.Context(), declared.GroupID, "color", nil); err != nil {
		t.Fatalf("DeleteFieldInGroup() error = %v, want nil", err)
	}

	held, err := s.Types.ByKey(t.Context(), "post")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if slices.ContainsFunc(held.Fields, func(f content.Field) bool { return f.Key == "color" }) {
		t.Errorf("ByKey() fields = %+v, want the definition gone", held.Fields)
	}
	surviving := content.Values{"other": float64(1)}
	if got := ValuesOf(t, s.Content, planted.ID); !reflect.DeepEqual(got, surviving) {
		t.Errorf("the content row holds %v, want only the surviving key", got)
	}
	if got := RevisionValuesOf(t, s.Content, revision); !reflect.DeepEqual(got, surviving) {
		t.Errorf("the revision holds %v, want the key swept from snapshots too", got)
	}
}
