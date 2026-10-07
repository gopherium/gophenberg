// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// slugCases are the cases over crowded slugs and a listing page past the offset range.
var slugCases = []Case{
	{"ListServesAPageBeyondWhatAnOffsetHolds", listServesAPageBeyondWhatAnOffsetHolds},
	{"ReportsExhaustedSlugSuffixes", reportsExhaustedSlugSuffixes},
	{"ReportsASlugTakenEvenUnderTheIdentifiedOne", reportsASlugTakenEvenUnderTheIdentifiedOne},
	{"CreatesUnderACrowdedSlugAnyway", createsUnderACrowdedSlugAnyway},
}

// listServesAPageBeyondWhatAnOffsetHolds answers an empty page with the full total far past the last row.
func listServesAPageBeyondWhatAnOffsetHolds(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Content, "Only Post", author)

	rows, total, err := s.Content.List(t.Context(), content.Filter{
		Type:    content.TypePost,
		OrderBy: content.OrderByDate,
		Order:   content.OrderDesc,
		Page:    30000000,
		PerPage: 100,
	})

	if err != nil {
		t.Fatalf("List() on a page past the offset range error = %v, want nil", err)
	}
	if len(rows) != 0 {
		t.Errorf("List() returned %d rows, want none that far out", len(rows))
	}
	if total != 1 {
		t.Errorf("total = %d, want the count of matching posts", total)
	}
}

// reportsExhaustedSlugSuffixes refuses an edit onto a slug whose numbered suffixes are all taken.
func reportsExhaustedSlugSuffixes(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	for range 20 {
		MustCreate(t, s.Content, "Crowded", author)
	}
	spare := MustCreate(t, s.Content, "Spare Title", author)
	edited := spare
	edited.Slug = "crowded"

	_, updateErr := s.Content.Update(t.Context(), edited, spare.UpdatedAt, nil, 0)

	if !errors.Is(updateErr, content.ErrSlugTaken) {
		t.Errorf("Update() error = %v, want %v", updateErr, content.ErrSlugTaken)
	}
}

// reportsASlugTakenEvenUnderTheIdentifiedOne refuses a post once its suffixes and its id slug are all taken.
func reportsASlugTakenEvenUnderTheIdentifiedOne(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	for range 20 {
		MustCreate(t, s.Content, "Crowded", author)
	}
	crowded := MustPost(t, "Crowded", author)
	decoy := "crowded-" + strings.ReplaceAll(crowded.ID.String(), "-", "")
	if held := MustCreate(t, s.Content, decoy, author); held.Slug != decoy {
		t.Fatalf("decoy Slug = %q, want %q", held.Slug, decoy)
	}

	_, createErr := s.Content.Create(t.Context(), crowded)

	if !errors.Is(createErr, content.ErrSlugTaken) {
		t.Errorf("Create() error = %v, want %v", createErr, content.ErrSlugTaken)
	}
}

// createsUnderACrowdedSlugAnyway stores a post under its id slug once the numbered suffixes run out.
func createsUnderACrowdedSlugAnyway(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	for range 20 {
		MustCreate(t, s.Content, "Crowded", author)
	}

	created, err := s.Content.Create(t.Context(), MustPost(t, "Crowded", author))

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if !strings.HasPrefix(created.Slug, "crowded-") {
		t.Errorf("Slug = %q, want one carrying the crowded stem", created.Slug)
	}
	if !strings.Contains(created.Slug, strings.ReplaceAll(created.ID.String(), "-", "")) {
		t.Errorf("Slug = %q, want it to carry the id of the post", created.Slug)
	}
}
