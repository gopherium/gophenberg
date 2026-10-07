// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// blockMarkup is the serialized block content a published fixture carries.
const blockMarkup = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"

// fixtureTitle is the title every published fixture carries.
const fixtureTitle = "Hello World"

// publishedCases are the cases over finding a published item by its address.
var publishedCases = []Case{
	{"FindsAPublishedPostByAddress", findsAPublishedPostByAddress},
	{"HidesEveryStatusButPublished", hidesEveryStatusButPublished},
	{"ScopesTheLookupToTheAddress", scopesTheLookupToTheAddress},
	{"ReportsAnUnknownAddress", reportsAnUnknownAddress},
	{"HidesARestoredPostUntilItIsPublishedAgain", hidesARestoredPostUntilItIsPublishedAgain},
	{"ServesARestoredPostAtTheAddressItKept", servesARestoredPostAtTheAddressItKept},
}

// publishWithContent stores a published post titled [fixtureTitle] carrying [blockMarkup].
func publishWithContent(t *testing.T, store content.Store, author uuid.UUID) content.Content {
	t.Helper()
	published := Publish(t, store, fixtureTitle, author, time.Now().UTC())
	edited := published
	edited.Content = blockMarkup
	updated, err := store.Update(t.Context(), edited, published.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("adding content to %q: %v", fixtureTitle, err)
	}
	return updated
}

// findsAPublishedPostByAddress returns the published post at its address with its body and UTC publication.
func findsAPublishedPostByAddress(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	published := publishWithContent(t, s.Content, author)

	found, err := s.Content.PublishedByPath(t.Context(), published.Path)

	if err != nil {
		t.Fatalf("PublishedByPath() error = %v, want nil", err)
	}
	if found.ID != published.ID {
		t.Errorf("ID = %v, want %v", found.ID, published.ID)
	}
	if found.Content != blockMarkup {
		t.Errorf("Content = %q, want the stored block markup %q", found.Content, blockMarkup)
	}
	if found.PublishedAt == nil {
		t.Fatal("PublishedAt = nil, want the publication instant")
	}
	if found.PublishedAt.Location() != time.UTC {
		t.Errorf("PublishedAt location = %v, want %v", found.PublishedAt.Location(), time.UTC)
	}
}

// hidesEveryStatusButPublished answers not found at the address of an item in any status but published.
func hidesEveryStatusButPublished(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	statuses := []content.Status{
		content.StatusDraft,
		content.StatusPending,
		content.StatusPrivate,
		content.StatusScheduled,
		content.StatusTrash,
	}
	for _, status := range statuses {
		hidden := MustPost(t, "Hidden "+string(status), author)
		now := time.Now().UTC()
		hidden.Status, hidden.Content, hidden.PublishedAt = status, blockMarkup, &now
		created, err := s.Content.Create(t.Context(), hidden)
		if err != nil {
			t.Fatalf("Create(%s) error = %v, want nil", status, err)
		}
		if created.Path != "hidden-"+string(status) {
			t.Fatalf("%s Path = %q, want %q", status, created.Path, "hidden-"+string(status))
		}
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()

			_, err := s.Content.PublishedByPath(t.Context(), "hidden-"+string(status))

			if !errors.Is(err, content.ErrNotFound) {
				t.Errorf("PublishedByPath() error = %v, want %v", err, content.ErrNotFound)
			}
		})
	}
}

// scopesTheLookupToTheAddress finds a published page at its full address and nothing at its bare slug.
func scopesTheLookupToTheAddress(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	PublishItem(t, s.Content, MustNest(t, s.Content, nil, "About Us", author))

	found, err := s.Content.PublishedByPath(t.Context(), "pages/about-us")
	if err != nil {
		t.Fatalf("PublishedByPath(page) error = %v, want nil", err)
	}
	if found.Type != "page" || found.Slug != "about-us" {
		t.Errorf("found = %q %q, want the page carrying the slug", found.Type, found.Slug)
	}

	_, err = s.Content.PublishedByPath(t.Context(), "about-us")

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedByPath(post) error = %v, want %v", err, content.ErrNotFound)
	}
}

// reportsAnUnknownAddress answers not found for an address no item holds.
func reportsAnUnknownAddress(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	publishWithContent(t, s.Content, author)

	_, err := s.Content.PublishedByPath(t.Context(), "never-written")

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedByPath() error = %v, want %v", err, content.ErrNotFound)
	}
}

// hidesARestoredPostUntilItIsPublishedAgain keeps a trashed and then restored post hidden until it is republished.
func hidesARestoredPostUntilItIsPublishedAgain(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	published := publishWithContent(t, s.Content, author)
	address := published.Path

	trashed, err := s.Content.Trash(t.Context(), published.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	if _, err := s.Content.PublishedByPath(t.Context(), trashed.Path); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedByPath() at the trashed address error = %v, want %v", err, content.ErrNotFound)
	}
	if _, err := s.Content.PublishedByPath(t.Context(), address); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedByPath() at the freed address error = %v, want %v", err, content.ErrNotFound)
	}

	restored, err := s.Content.Restore(t.Context(), trashed.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	if restored.Status != content.StatusDraft {
		t.Fatalf("Status after Restore() = %q, want %q", restored.Status, content.StatusDraft)
	}
	if _, err := s.Content.PublishedByPath(t.Context(), address); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedByPath() after restoring error = %v, want %v", err, content.ErrNotFound)
	}

	republished := restored
	republished.Status = content.StatusPublished
	if _, err := s.Content.Update(t.Context(), republished, restored.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("republishing: %v", err)
	}

	found, err := s.Content.PublishedByPath(t.Context(), address)

	if err != nil {
		t.Fatalf("PublishedByPath() after republishing error = %v, want nil", err)
	}
	if found.ID != published.ID {
		t.Errorf("ID = %v, want %v", found.ID, published.ID)
	}
}

// servesARestoredPostAtTheAddressItKept serves a republished post at the trashed address it kept after a collision.
func servesARestoredPostAtTheAddressItKept(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	first := publishWithContent(t, s.Content, author)
	trashed, err := s.Content.Trash(t.Context(), first.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	second := publishWithContent(t, s.Content, author)
	if second.Slug != first.Slug {
		t.Fatalf("second Slug = %q, want the freed %q", second.Slug, first.Slug)
	}

	restored, err := s.Content.Restore(t.Context(), trashed.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	republished := restored
	republished.Status = content.StatusPublished
	if _, err := s.Content.Update(t.Context(), republished, restored.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("republishing: %v", err)
	}

	found, err := s.Content.PublishedByPath(t.Context(), restored.Path)

	if err != nil {
		t.Fatalf("PublishedByPath() at the kept address error = %v, want nil", err)
	}
	if found.ID != first.ID {
		t.Errorf("ID = %v, want the restored post %v", found.ID, first.ID)
	}
	if !trashedSlug.MatchString(restored.Slug) {
		t.Errorf("restored Slug = %q, want the trash suffix the collision keeps", restored.Slug)
	}
}
