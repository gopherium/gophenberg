// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// nestingCases are the cases over the parent a write files an item under.
var nestingCases = []Case{
	{"CreateRefusesAParentOnceTheTypeStoppedNesting", createRefusesAParentOnceTheTypeStoppedNesting},
	{"UpdateRefusesAParentOnceTheTypeStoppedNesting", updateRefusesAParentOnceTheTypeStoppedNesting},
	{"CreateRefusesAParentInTheTrash", createRefusesAParentInTheTrash},
	{"UpdateRefusesToMoveUnderAParentInTheTrash", updateRefusesToMoveUnderAParentInTheTrash},
	{"UpdateRefusesToMoveBetweenParentsUnderOneInTheTrash", updateRefusesToMoveBetweenParentsUnderOneInTheTrash},
	{"UpdateRefusesToMoveAnItemUnderOneItHolds", updateRefusesToMoveAnItemUnderOneItHolds},
	{"AStaleEditAfterTheItemMovedReportsAConflict", aStaleEditAfterTheItemMovedReportsAConflict},
	{"MovingAnItemDeletedForGoodReportsItMissing", movingAnItemDeletedForGoodReportsItMissing},
	{"CreateRefusesAParentDeletedForGood", createRefusesAParentDeletedForGood},
}

// createRefusesAParentOnceTheTypeStoppedNesting refuses a child filed under a parent once the type stopped nesting.
func createRefusesAParentOnceTheTypeStoppedNesting(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	if _, err := s.Types.Update(t.Context(), FlatPage()); err != nil {
		t.Fatalf("Update() error = %v, want the empty type flattened", err)
	}

	_, err := s.Content.Create(t.Context(), StalePage(t, &about, "Team", author))

	if !errors.Is(err, content.ErrNotHierarchical) {
		t.Errorf("Create() error = %v, want %v", err, content.ErrNotHierarchical)
	}
}

// updateRefusesAParentOnceTheTypeStoppedNesting refuses a move under a parent once the type stopped nesting.
func updateRefusesAParentOnceTheTypeStoppedNesting(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, nil, "Team", author)
	if _, err := s.Types.Update(t.Context(), FlatPage()); err != nil {
		t.Fatalf("Update() error = %v, want the flat type stored", err)
	}
	moved, err := content.Reparent(PageType(), team, &about, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = team.UpdatedAt.Add(time.Second)

	_, err = s.Content.Update(t.Context(), moved, team.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrNotHierarchical) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrNotHierarchical)
	}
}

// createRefusesAParentInTheTrash refuses a child filed under a trashed parent with the bare sentinel.
func createRefusesAParentInTheTrash(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	if _, err := s.Content.Trash(t.Context(), about.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	_, err := s.Content.Create(t.Context(), StalePage(t, &about, "Team", author))

	if err == nil || err.Error() != content.ErrParentTrashed.Error() {
		t.Errorf("Create() error = %v, want the bare %v", err, content.ErrParentTrashed)
	}
}

// updateRefusesToMoveUnderAParentInTheTrash refuses to move a top-level item under a trashed parent.
func updateRefusesToMoveUnderAParentInTheTrash(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, nil, "Team", author)
	if _, err := s.Content.Trash(t.Context(), about.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	moved, err := content.Reparent(PageType(), team, &about, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = team.UpdatedAt.Add(time.Second)

	_, err = s.Content.Update(t.Context(), moved, team.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrParentTrashed) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrParentTrashed)
	}
}

// updateRefusesToMoveBetweenParentsUnderOneInTheTrash refuses to move a child from its parent to a trashed one.
func updateRefusesToMoveBetweenParentsUnderOneInTheTrash(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	archive := MustNest(t, s.Content, nil, "Archive", author)
	team := MustNest(t, s.Content, &about, "Team", author)
	if _, err := s.Content.Trash(t.Context(), archive.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	moved, err := content.Reparent(PageType(), team, &archive, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = team.UpdatedAt.Add(time.Second)

	_, err = s.Content.Update(t.Context(), moved, team.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrParentTrashed) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrParentTrashed)
	}
}

// updateRefusesToMoveAnItemUnderOneItHolds refuses to file an item under its own child with the bare sentinel.
func updateRefusesToMoveAnItemUnderOneItHolds(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, nil, "Team", author)
	filed, err := content.Reparent(PageType(), team, &about, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	filed.UpdatedAt = team.UpdatedAt.Add(time.Second)
	if _, err := s.Content.Update(t.Context(), filed, team.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("filing Team under About: %v", err)
	}
	moved, err := content.Reparent(PageType(), about, &team, 0)
	if err != nil {
		t.Fatalf("Reparent() on the stale Team error = %v, want nil", err)
	}
	moved.UpdatedAt = about.UpdatedAt.Add(time.Second)

	_, err = s.Content.Update(t.Context(), moved, about.UpdatedAt, nil, 0)

	if err == nil || err.Error() != content.ErrCycle.Error() {
		t.Errorf("Update() error = %v, want the bare %v", err, content.ErrCycle)
	}
}

// aStaleEditAfterTheItemMovedReportsAConflict reports an edit from a copy read before the item moved as a conflict.
func aStaleEditAfterTheItemMovedReportsAConflict(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, &about, "Team", author)
	lifted, err := content.Reparent(PageType(), team, nil, 0)
	if err != nil {
		t.Fatalf("Reparent() to the top error = %v, want nil", err)
	}
	lifted.UpdatedAt = team.UpdatedAt.Add(time.Second)
	lifted, err = s.Content.Update(t.Context(), lifted, team.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("moving Team to the top: %v", err)
	}
	filed, err := content.Reparent(PageType(), about, &lifted, 0)
	if err != nil {
		t.Fatalf("Reparent() under Team error = %v, want nil", err)
	}
	filed.UpdatedAt = about.UpdatedAt.Add(time.Second)
	if _, err := s.Content.Update(t.Context(), filed, about.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("filing About under Team: %v", err)
	}
	edited := team
	edited.Title = "Team retitled"
	edited.UpdatedAt = team.UpdatedAt.Add(2 * time.Second)

	_, err = s.Content.Update(t.Context(), edited, team.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("Update() error = %v, want the stale copy reported as %v", err, content.ErrConflict)
	}
}

// movingAnItemDeletedForGoodReportsItMissing reports a move of an item deleted for good as missing.
func movingAnItemDeletedForGoodReportsItMissing(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, nil, "Team", author)
	if err := s.Content.Delete(t.Context(), team.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	moved, err := content.Reparent(PageType(), team, &about, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = team.UpdatedAt.Add(time.Second)

	_, err = s.Content.Update(t.Context(), moved, team.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrNotFound)
	}
}

// createRefusesAParentDeletedForGood refuses a child filed under a parent deleted for good with the bare sentinel.
func createRefusesAParentDeletedForGood(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	if err := s.Content.Delete(t.Context(), about.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	_, err := s.Content.Create(t.Context(), StalePage(t, &about, "Orphan", author))

	if err == nil || err.Error() != content.ErrParentType.Error() {
		t.Errorf("Create() error = %v, want the bare %v", err, content.ErrParentType)
	}
}
