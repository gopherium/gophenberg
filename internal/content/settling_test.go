// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

const (
	leavingGroup = 1
	keptGroup    = 2
	thirdGroup   = 3
	fourthGroup  = 4
)

// storedField returns a field of the kind stored under the group, holding the fields inside.
func storedField(id, group int, key string, kind content.FieldKind, inside ...content.Field) content.Field {
	return content.Field{ID: id, GroupID: group, Key: key, Kind: kind, Fields: inside}
}

// storedRelation returns a relation field stored under the group pointing at the type.
func storedRelation(id, group int, key, relatesTo string) content.Field {
	return content.Field{ID: id, GroupID: group, Key: key, Kind: content.FieldKindRelation, RelatesTo: relatesTo}
}

// settledAround returns the changes deleting the leaving group needs beside a group holding the container.
func settledAround(container content.Field) []content.Settling {
	return content.SettledFields([]content.Group{
		{ID: leavingGroup, Fields: []content.Field{storedField(90, leavingGroup, "own", content.FieldKindText)}},
		{ID: keptGroup, Fields: []content.Field{container}},
	}, leavingGroup)
}

// assertSettled fails the test when the changes differ from the wanted ones.
func assertSettled(t *testing.T, got, want []content.Settling) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("SettledFields() = %+v, want %+v", got, want)
	}
}

func TestSettledFieldsCarryALeftoverWithNoTwin(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, leavingGroup, "name", content.FieldKindText)))

	assertSettled(t, got, []content.Settling{{ID: 11, Group: keptGroup}})
}

func TestSettledFieldsCarryWhatALeftoverHolds(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, leavingGroup, "rows", content.FieldKindRepeater,
			storedField(12, leavingGroup, "title", content.FieldKindText))))

	assertSettled(t, got, []content.Settling{{ID: 11, Group: keptGroup}, {ID: 12, Group: keptGroup}})
}

func TestSettledFieldsDropATwinWhicheverComesFirst(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(12, leavingGroup, "name", content.FieldKindText),
		storedField(11, keptGroup, "name", content.FieldKindText)))

	assertSettled(t, got, []content.Settling{{ID: 12, Kept: 11, Drop: true}})
}

func TestSettledFieldsFoldWhatOnlyATwinHoldsIntoTheTwinKept(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, keptGroup, "profile", content.FieldKindSection,
			storedField(13, keptGroup, "title", content.FieldKindText)),
		storedField(12, leavingGroup, "profile", content.FieldKindSection,
			storedField(14, leavingGroup, "bio", content.FieldKindText),
			storedField(15, leavingGroup, "title", content.FieldKindText))))

	assertSettled(t, got, []content.Settling{
		{ID: 14, Parent: 11, Group: keptGroup},
		{ID: 15, Kept: 13},
		{ID: 12, Kept: 11, Drop: true},
	})
}

func TestSettledFieldsFoldTwinsStandingInsideTwins(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, keptGroup, "profile", content.FieldKindSection,
			storedField(13, keptGroup, "address", content.FieldKindSection,
				storedField(16, keptGroup, "street", content.FieldKindText))),
		storedField(12, leavingGroup, "profile", content.FieldKindSection,
			storedField(14, leavingGroup, "address", content.FieldKindSection,
				storedField(15, leavingGroup, "city", content.FieldKindText)))))

	assertSettled(t, got, []content.Settling{
		{ID: 15, Parent: 13, Group: keptGroup},
		{ID: 14, Kept: 13},
		{ID: 12, Kept: 11, Drop: true},
	})
}

func TestSettledFieldsDropWhatATwinOfAnotherKindHolds(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, keptGroup, "profile", content.FieldKindSection),
		storedField(12, leavingGroup, "profile", content.FieldKindRepeater,
			storedField(13, leavingGroup, "bio", content.FieldKindText))))

	assertSettled(t, got, []content.Settling{{ID: 12, Drop: true}})
}

func TestSettledFieldsFoldIntoTheTwinTheContainersGroupStores(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, thirdGroup, "profile", content.FieldKindSection),
		storedField(12, keptGroup, "profile", content.FieldKindSection),
		storedField(13, leavingGroup, "profile", content.FieldKindSection,
			storedField(14, leavingGroup, "bio", content.FieldKindText))))

	assertSettled(t, got, []content.Settling{{ID: 14, Parent: 12, Group: keptGroup}, {ID: 13, Kept: 12, Drop: true}})
}

func TestSettledFieldsFoldIntoTheOlderOfTwoTwinsElsewhere(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(14, fourthGroup, "profile", content.FieldKindSection),
		storedField(11, thirdGroup, "profile", content.FieldKindSection),
		storedField(12, leavingGroup, "profile", content.FieldKindSection,
			storedField(13, leavingGroup, "bio", content.FieldKindText))))

	assertSettled(t, got, []content.Settling{{ID: 13, Parent: 11, Group: keptGroup}, {ID: 12, Kept: 11, Drop: true}})
}

func TestSettledFieldsLandTheOlderOfTwoLeftoversHeadingHome(t *testing.T) {
	t.Parallel()

	leftover := storedField(13, leavingGroup, "tag", content.FieldKindSection,
		storedField(15, leavingGroup, "label", content.FieldKindText))
	carried := storedField(14, keptGroup, "tag", content.FieldKindSection,
		storedField(16, keptGroup, "color", content.FieldKindText))
	want := []content.Settling{
		{ID: 13, Parent: 11, Group: keptGroup},
		{ID: 15, Group: keptGroup},
		{ID: 16, Parent: 13, Group: keptGroup},
		{ID: 14, Kept: 13},
		{ID: 12, Kept: 11, Drop: true},
	}

	for _, order := range [][]content.Field{{leftover, carried}, {carried, leftover}} {
		got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
			storedField(11, keptGroup, "profile", content.FieldKindSection),
			storedField(12, leavingGroup, "profile", content.FieldKindSection, order...)))

		assertSettled(t, got, want)
	}
}

func TestSettledFieldsFoldIntoTheTwinOfTheSameKind(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, keptGroup, "profile", content.FieldKindRepeater),
		storedField(12, leavingGroup, "profile", content.FieldKindSection,
			storedField(13, leavingGroup, "only", content.FieldKindText)),
		storedField(14, thirdGroup, "profile", content.FieldKindSection)))

	assertSettled(t, got, []content.Settling{{ID: 13, Parent: 14, Group: keptGroup}, {ID: 12, Kept: 14, Drop: true}})
}

func TestSettledFieldsFoldIntoALeftoverTheKeptTwinHolds(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, keptGroup, "profile", content.FieldKindSection,
			storedField(13, leavingGroup, "note", content.FieldKindSection)),
		storedField(12, leavingGroup, "profile", content.FieldKindSection,
			storedField(14, leavingGroup, "note", content.FieldKindSection,
				storedField(15, leavingGroup, "tag", content.FieldKindText)))))

	assertSettled(t, got, []content.Settling{
		{ID: 13, Group: keptGroup},
		{ID: 15, Parent: 13, Group: keptGroup},
		{ID: 14, Kept: 13},
		{ID: 12, Kept: 11, Drop: true},
	})
}

func TestSettledFieldsFoldPastALeftoverTheKeptTwinDrops(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, keptGroup, "profile", content.FieldKindSection,
			storedField(13, leavingGroup, "note", content.FieldKindSection),
			storedField(16, thirdGroup, "note", content.FieldKindSection)),
		storedField(12, leavingGroup, "profile", content.FieldKindSection,
			storedField(14, leavingGroup, "note", content.FieldKindSection,
				storedField(15, leavingGroup, "tag", content.FieldKindText)))))

	assertSettled(t, got, []content.Settling{
		{ID: 13, Kept: 16, Drop: true},
		{ID: 15, Parent: 16, Group: keptGroup},
		{ID: 14, Kept: 16},
		{ID: 12, Kept: 11, Drop: true},
	})
}

func TestSettledFieldsFoldTwinsInsideTheDroppedTwinOnce(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, keptGroup, "profile", content.FieldKindSection),
		storedField(12, leavingGroup, "profile", content.FieldKindSection,
			storedField(13, leavingGroup, "tag", content.FieldKindSection,
				storedField(15, leavingGroup, "label", content.FieldKindText)),
			storedField(14, thirdGroup, "tag", content.FieldKindSection,
				storedField(16, thirdGroup, "color", content.FieldKindText)))))

	assertSettled(t, got, []content.Settling{
		{ID: 13, Parent: 11, Group: keptGroup},
		{ID: 15, Group: keptGroup},
		{ID: 16, Parent: 13, Group: thirdGroup},
		{ID: 14, Kept: 13},
		{ID: 12, Kept: 11, Drop: true},
	})
}

func TestSettledFieldsFoldTwoTwinsIntoOneKeptFieldOnce(t *testing.T) {
	t.Parallel()

	kept := storedField(11, keptGroup, "profile", content.FieldKindSection,
		storedField(13, keptGroup, "tag", content.FieldKindSection),
		storedField(14, leavingGroup, "tag", content.FieldKindSection,
			storedField(15, leavingGroup, "label", content.FieldKindText)))
	dropped := storedField(12, leavingGroup, "profile", content.FieldKindSection,
		storedField(16, leavingGroup, "tag", content.FieldKindSection,
			storedField(17, leavingGroup, "label", content.FieldKindText)))
	want := []content.Settling{
		{ID: 15, Parent: 13, Group: keptGroup},
		{ID: 14, Kept: 13, Drop: true},
		{ID: 17, Kept: 15},
		{ID: 16, Kept: 13},
		{ID: 12, Kept: 11, Drop: true},
	}

	for _, order := range [][]content.Field{{kept, dropped}, {dropped, kept}} {
		got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection, order...))

		assertSettled(t, got, want)
	}
}

func TestSettledFieldsLandTheSiblingTheContainersGroupStoresBeforeOneElsewhere(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, keptGroup, "profile", content.FieldKindSection),
		storedField(12, leavingGroup, "profile", content.FieldKindSection,
			storedField(13, thirdGroup, "tag", content.FieldKindSection),
			storedField(14, keptGroup, "tag", content.FieldKindSection))))

	assertSettled(t, got, []content.Settling{
		{ID: 14, Parent: 11, Group: keptGroup},
		{ID: 13, Kept: 14},
		{ID: 12, Kept: 11, Drop: true},
	})
}

func TestSettledFieldsKeepTheSameFieldWhicheverGroupListsFirst(t *testing.T) {
	t.Parallel()

	leftover := storedField(13, leavingGroup, "tag", content.FieldKindSection,
		storedField(15, leavingGroup, "label", content.FieldKindText))
	elsewhere := storedField(14, thirdGroup, "tag", content.FieldKindRepeater,
		storedField(16, thirdGroup, "color", content.FieldKindText))
	want := []content.Settling{
		{ID: 13, Parent: 11, Group: keptGroup},
		{ID: 15, Group: keptGroup},
		{ID: 12, Kept: 11, Drop: true},
	}

	for _, order := range [][]content.Field{{leftover, elsewhere}, {elsewhere, leftover}} {
		got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
			storedField(11, keptGroup, "profile", content.FieldKindSection),
			storedField(12, leavingGroup, "profile", content.FieldKindSection, order...)))

		assertSettled(t, got, want)
	}
}

func TestSettledFieldsKeepTheIndexOnlyForATwinPointingAlike(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedRelation(11, keptGroup, "wrote", "category"),
		storedRelation(12, leavingGroup, "wrote", "tag")))

	assertSettled(t, got, []content.Settling{{ID: 12, Drop: true}})
}

func TestSettledFieldsLeaveTwinsOfOtherGroupsAlone(t *testing.T) {
	t.Parallel()

	got := settledAround(storedField(10, keptGroup, "author", content.FieldKindSection,
		storedField(11, keptGroup, "name", content.FieldKindText),
		storedField(12, thirdGroup, "name", content.FieldKindText)))

	assertSettled(t, got, nil)
}
