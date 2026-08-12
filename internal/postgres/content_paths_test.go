// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// pageType returns a hierarchical page type answering under pages.
func pageType() content.Type {
	t := postType()
	t.Key, t.SingularLabel, t.PluralLabel = "page", "Page", "Pages"
	t.RouteWord, t.Hierarchical, t.Default = "pages", true, false
	return t
}

// newNestingStore returns a store whose database knows the hierarchical page type.
func newNestingStore(t *testing.T) (*postgres.ContentStore, uuid.UUID) {
	t.Helper()
	store, author, pool := newContentStoreWithPool(t)
	if _, err := postgres.NewTypeStore(pool).Create(t.Context(), pageType()); err != nil {
		t.Fatalf("registering the page type: %v", err)
	}
	return store, author
}

// mustNest stores a page under the parent and returns it.
func mustNest(
	t *testing.T, store *postgres.ContentStore, parent *content.Content, title string, author uuid.UUID,
) content.Content {
	t.Helper()
	built, err := content.New(pageType(), parent, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	stored, err := store.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return stored
}

// addressOf returns the address the store holds for the item.
func addressOf(t *testing.T, store *postgres.ContentStore, id uuid.UUID) string {
	t.Helper()
	stored, err := store.ByID(t.Context(), id)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	return stored.Path
}

func TestContentStoreAddressesNestedContent(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)

	about := mustNest(t, store, nil, "About", author)
	team := mustNest(t, store, &about, "Team", author)

	if about.Path != "pages/about" {
		t.Errorf("about path = %q, want it under the route word", about.Path)
	}
	if team.Path != "pages/about/team" {
		t.Errorf("team path = %q, want the ancestor chain", team.Path)
	}
	if team.ParentID == nil || *team.ParentID != about.ID {
		t.Errorf("team parent = %v, want %v", team.ParentID, about.ID)
	}
}

func TestContentStoreLetsSiblingsOfDifferentParentsShareASlug(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	careers := mustNest(t, store, nil, "Careers", author)

	first := mustNest(t, store, &about, "Team", author)
	second := mustNest(t, store, &careers, "Team", author)

	if first.Slug != "team" || second.Slug != "team" {
		t.Errorf("slugs = %q and %q, want both to keep the stem", first.Slug, second.Slug)
	}
	if first.Path == second.Path {
		t.Errorf("both teams answer at %q, want distinct addresses", first.Path)
	}
}

func TestContentStoreSuffixesASlugTakenBesideASibling(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	mustNest(t, store, &about, "Team", author)

	second := mustNest(t, store, &about, "Team", author)

	if second.Slug != "team-2" {
		t.Errorf("slug = %q, want the sibling suffix", second.Slug)
	}
	if second.Path != "pages/about/team-2" {
		t.Errorf("path = %q, want the suffix carried into the address", second.Path)
	}
}

func TestContentStoreCarriesDescendantsWhenAParentIsRenamed(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	team := mustNest(t, store, &about, "Team", author)
	deep := mustNest(t, store, &team, "Maria Perez", author)
	renamed, err := about.Rename("company")
	if err != nil {
		t.Fatalf("Rename error = %v, want nil", err)
	}
	renamed.UpdatedAt = about.UpdatedAt.Add(time.Second)

	moved, err := store.Update(t.Context(), renamed, about.UpdatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if moved.Path != "pages/company" {
		t.Errorf("parent path = %q, want the rename applied", moved.Path)
	}
	if got := addressOf(t, store, team.ID); got != "pages/company/team" {
		t.Errorf("child path = %q, want it carried along", got)
	}
	if got := addressOf(t, store, deep.ID); got != "pages/company/team/maria-perez" {
		t.Errorf("grandchild path = %q, want the whole subtree carried", got)
	}
}

func TestContentStoreCarriesDescendantsWhenAParentMoves(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	team := mustNest(t, store, &about, "Team", author)
	company := mustNest(t, store, nil, "Company", author)
	moved, err := content.Reparent(pageType(), about, &company, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = about.UpdatedAt.Add(time.Second)

	if _, err := store.Update(t.Context(), moved, about.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	if got := addressOf(t, store, about.ID); got != "pages/company/about" {
		t.Errorf("moved path = %q, want it under its new parent", got)
	}
	if got := addressOf(t, store, team.ID); got != "pages/company/about/team" {
		t.Errorf("child path = %q, want the subtree carried", got)
	}
}

func TestContentStoreSwapsTwoAddressesInOneTransaction(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	first := mustNest(t, store, nil, "First", author)
	second := mustNest(t, store, nil, "Second", author)
	taking, err := first.Rename(second.Slug)
	if err != nil {
		t.Fatalf("Rename error = %v, want nil", err)
	}
	taking.UpdatedAt = first.UpdatedAt.Add(time.Second)

	_, err = store.Update(t.Context(), taking, first.UpdatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want the suffix to settle the clash", err)
	}
	if got := addressOf(t, store, first.ID); got != "pages/second-2" {
		t.Errorf("path = %q, want the taken address suffixed", got)
	}
}

func TestContentStoreKeepsAParentThatHoldsChildren(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	mustNest(t, store, &about, "Team", author)

	_, err := store.Trash(t.Context(), about.ID, time.Now().UTC())

	if !errors.Is(err, content.ErrHoldsChildren) {
		t.Errorf("Trash() error = %v, want %v", err, content.ErrHoldsChildren)
	}
}

func TestContentStoreFreesAnAddressWhenContentIsTrashed(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)

	trashed, err := store.Trash(t.Context(), about.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	if trashed.Path == "pages/about" {
		t.Errorf("path = %q, want the address freed", trashed.Path)
	}
	replacement := mustNest(t, store, nil, "About", author)
	if replacement.Path != "pages/about" {
		t.Errorf("replacement path = %q, want the freed address reused", replacement.Path)
	}
}

func TestContentStoreRestoresAnAddressWithItsSlug(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	if _, err := store.Trash(t.Context(), about.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	restored, err := store.Restore(t.Context(), about.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	if restored.Slug != "about" || restored.Path != "pages/about" {
		t.Errorf("restored = %q at %q, want the original name and address", restored.Slug, restored.Path)
	}
}

func TestContentStoreServesPublishedContentByAddress(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	published := about
	published.Status = content.StatusPublished
	at := time.Now().UTC()
	published.PublishedAt, published.UpdatedAt = &at, at
	if _, err := store.Update(t.Context(), published, about.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("publishing: %v, want nil", err)
	}

	found, err := store.PublishedByPath(t.Context(), "pages/about")

	if err != nil {
		t.Fatalf("PublishedByPath() error = %v, want nil", err)
	}
	if found.ID != about.ID {
		t.Errorf("PublishedByPath() = %v, want the published page", found.ID)
	}
	if _, err := store.PublishedByPath(t.Context(), "pages/nowhere"); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedByPath(missing) error = %v, want %v", err, content.ErrNotFound)
	}
}

func TestContentStoreCountsChildren(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	mustNest(t, store, &about, "Team", author)
	mustNest(t, store, &about, "History", author)

	held, err := store.Children(t.Context(), about.ID)

	if err != nil {
		t.Fatalf("Children() error = %v, want nil", err)
	}
	if held != 2 {
		t.Errorf("Children() = %d, want 2", held)
	}
}

func TestDepthMeasuresHowFarContentNestsBelow(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	team := mustNest(t, store, &about, "Team", author)
	mustNest(t, store, &team, "Crew", author)
	alone := mustNest(t, store, nil, "Careers", author)

	deep, err := store.Depth(t.Context(), about.ID)
	if err != nil {
		t.Fatalf("Depth error = %v, want nil", err)
	}
	flat, err := store.Depth(t.Context(), alone.ID)
	if err != nil {
		t.Fatalf("Depth error = %v, want nil", err)
	}

	if deep != 2 {
		t.Errorf("depth below About = %d, want 2", deep)
	}
	if flat != 0 {
		t.Errorf("depth below a leaf = %d, want 0", flat)
	}
}

func TestTrashRefusesContentAlreadyOnItsWayOut(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)

	trashed, err := store.Trash(t.Context(), about.ID, about.UpdatedAt)
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	_, err = store.Trash(t.Context(), about.ID, trashed.UpdatedAt)

	if !errors.Is(err, content.ErrInvalidTransition) {
		t.Fatalf("trashing twice error = %v, want %v", err, content.ErrInvalidTransition)
	}
	if got := addressOf(t, store, about.ID); got != trashed.Path {
		t.Errorf("path = %q, want the first suffix left alone at %q", got, trashed.Path)
	}
}

func TestRestoreReturnsAnItemToTheAddressItLeft(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)

	trashed, err := store.Trash(t.Context(), about.ID, about.UpdatedAt)
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	restored, err := store.Restore(t.Context(), about.ID, trashed.UpdatedAt)
	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}

	if restored.Path != "pages/about" || restored.Slug != "about" {
		t.Errorf("restored to path %q slug %q, want the address it left", restored.Path, restored.Slug)
	}
}
