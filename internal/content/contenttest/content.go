// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// trashedSlug matches the slug a trashed "Hello World" post carries.
var trashedSlug = regexp.MustCompile(`^hello-world-trashed-[a-z0-9]{8}$`)

// contentCases are the cases over creating, reading, editing, trashing and deleting one item.
var contentCases = []Case{
	{"CreateAndReadBack", createAndReadBack},
	{"CreateSuffixesTakenSlugs", createSuffixesTakenSlugs},
	{"CreatesPastTheSuffixesItTries", createsPastTheSuffixesItTries},
	{"CreateSuffixesUnderConcurrency", createSuffixesUnderConcurrency},
	{"CreateRejectsAnUnknownAuthor", createRejectsAnUnknownAuthor},
	{"ByIDReportsMissingContent", byIDReportsMissingContent},
	{"UpdateChangesEditableFields", updateChangesEditableFields},
	{"UpdateSuffixesTakenSlugs", updateSuffixesTakenSlugs},
	{"UpdateReportsMissingContent", updateReportsMissingContent},
	{"UpdateReportsConflictingUpdates", updateReportsConflictingUpdates},
	{"UpdateReturnsAVersionTheNextUpdateAccepts", updateReturnsAVersionTheNextUpdateAccepts},
	{"UpdateWithASnapshotReportsMissingPosts", updateWithASnapshotReportsMissingPosts},
	{"TrashRenamesTheSlug", trashRenamesTheSlug},
	{"TrashReportsMissingPosts", trashReportsMissingPosts},
	{"RestoreRecoversTheOriginalSlug", restoreRecoversTheOriginalSlug},
	{"RestoreKeepsTheRenamedSlugWhenTheOriginalIsTaken", restoreKeepsTheRenamedSlugWhenTheOriginalIsTaken},
	{"RestoreRefusesAnItemOutOfTheTrash", restoreRefusesAnItemOutOfTheTrash},
	{"RestoreReportsMissingPosts", restoreReportsMissingPosts},
	{"DeleteRemovesTheContent", deleteRemovesTheContent},
	{"DeleteReportsMissingPosts", deleteReportsMissingPosts},
}

// createAndReadBack stores a draft and reads it back by id.
func createAndReadBack(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	p := MustPost(t, "Hello World", author)

	created, err := s.Content.Create(t.Context(), p)

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Slug != "hello-world" {
		t.Errorf("Slug = %q, want %q", created.Slug, "hello-world")
	}
	if created.Status != content.StatusDraft {
		t.Errorf("Status = %q, want %q", created.Status, content.StatusDraft)
	}

	got, err := s.Content.ByID(t.Context(), p.ID)

	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if got.ID != p.ID || got.Title != "Hello World" || got.AuthorID != author {
		t.Errorf("ByID() = %+v, want the created post", got)
	}
	if got.PublishedAt != nil {
		t.Errorf("PublishedAt = %v, want nil on a draft", got.PublishedAt)
	}
	if got.CreatedAt.Location() != time.UTC || got.UpdatedAt.Location() != time.UTC {
		t.Errorf("timestamps carry location %v, want UTC", got.CreatedAt.Location())
	}
}

// createSuffixesTakenSlugs numbers the slug of each post that repeats a title.
func createSuffixesTakenSlugs(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Content, "Hello World", author)

	second := MustCreate(t, s.Content, "Hello World", author)
	third := MustCreate(t, s.Content, "Hello World", author)

	if second.Slug != "hello-world-2" {
		t.Errorf("second Slug = %q, want %q", second.Slug, "hello-world-2")
	}
	if third.Slug != "hello-world-3" {
		t.Errorf("third Slug = %q, want %q", third.Slug, "hello-world-3")
	}
}

// createsPastTheSuffixesItTries keeps every slug distinct after the numbered suffixes run out.
func createsPastTheSuffixesItTries(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	slugs := make(map[string]bool)

	const past = 25

	for range past {
		created := MustCreate(t, s.Content, "", author)
		if slugs[created.Slug] {
			t.Fatalf("Slug = %q, want one no other post holds", created.Slug)
		}
		slugs[created.Slug] = true
	}

	if len(slugs) != past {
		t.Errorf("distinct slugs = %d, want %d", len(slugs), past)
	}
}

// createSuffixesUnderConcurrency gives two posts stored at once two distinct slugs.
func createSuffixesUnderConcurrency(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	posts := []content.Content{MustPost(t, "Race", author), MustPost(t, "Race", author)}
	slugs := make([]string, len(posts))
	errs := make([]error, len(posts))

	var wg sync.WaitGroup
	for i, p := range posts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			created, err := s.Content.Create(t.Context(), p)
			slugs[i], errs[i] = created.Slug, err
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent Create(%d) error = %v, want nil", i, err)
		}
	}
	if slugs[0] == slugs[1] {
		t.Errorf("both posts stored slug %q, want distinct slugs", slugs[0])
	}
}

// createRejectsAnUnknownAuthor refuses a post whose author holds no account.
func createRejectsAnUnknownAuthor(t *testing.T, s Stores) {
	orphan := MustPost(t, "Orphan", uuid.Must(uuid.NewV7()))

	_, err := s.Content.Create(t.Context(), orphan)

	if err == nil {
		t.Fatal("Create() error = nil, want a foreign key failure")
	}
}

// byIDReportsMissingContent answers not found for an id nothing holds.
func byIDReportsMissingContent(t *testing.T, s Stores) {
	_, err := s.Content.ByID(t.Context(), uuid.Must(uuid.NewV7()))

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("ByID() error = %v, want %v", err, content.ErrNotFound)
	}
}

// updateChangesEditableFields stores the edited title, body, excerpt and publication.
func updateChangesEditableFields(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Draft Title", author)
	edited := created
	edited.Title = "Published Title"
	edited.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	edited.Excerpt = "Summary"
	published := time.Now().UTC().Truncate(time.Microsecond)
	edited.Status = content.StatusPublished
	edited.PublishedAt = &published
	edited.UpdatedAt = published

	updated, err := s.Content.Update(t.Context(), edited, created.UpdatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Title != "Published Title" || updated.Excerpt != "Summary" {
		t.Errorf("Update() = %+v, want the edited title and excerpt", updated)
	}
	if updated.Status != content.StatusPublished {
		t.Errorf("Status = %q, want %q", updated.Status, content.StatusPublished)
	}
	if updated.PublishedAt == nil || !updated.PublishedAt.Equal(published) {
		t.Errorf("PublishedAt = %v, want %v", updated.PublishedAt, published)
	}
}

// updateSuffixesTakenSlugs numbers a slug an edit moves onto another post's.
func updateSuffixesTakenSlugs(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Content, "Taken Slug", author)
	other := MustCreate(t, s.Content, "Other Slug", author)
	edited := other
	edited.Slug = "taken-slug"

	updated, err := s.Content.Update(t.Context(), edited, other.UpdatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Slug != "taken-slug-2" {
		t.Errorf("Slug = %q, want %q", updated.Slug, "taken-slug-2")
	}
}

// updateReportsMissingContent answers not found for a post never stored.
func updateReportsMissingContent(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	missing := MustPost(t, "Missing", author)

	_, err := s.Content.Update(t.Context(), missing, missing.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrNotFound)
	}
}

// updateReportsConflictingUpdates refuses a write made from a stale version and keeps the first.
func updateReportsConflictingUpdates(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Contended", author)
	_, err := s.Content.Update(t.Context(), EditTitle(created, "First Writer"), created.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("first Update() error = %v, want nil", err)
	}

	_, err = s.Content.Update(t.Context(), EditTitle(created, "Second Writer"), created.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("Update() with a stale token error = %v, want %v", err, content.ErrConflict)
	}
	current, byIDErr := s.Content.ByID(t.Context(), created.ID)
	if byIDErr != nil {
		t.Fatalf("ByID() error = %v, want nil", byIDErr)
	}
	if current.Title != "First Writer" {
		t.Errorf("Title = %q, want the first write kept", current.Title)
	}
}

// updateReturnsAVersionTheNextUpdateAccepts reports the stored version, so a chained write succeeds.
func updateReturnsAVersionTheNextUpdateAccepts(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Chained Writes", author)
	first := created
	first.Title, first.UpdatedAt = "First Edit", time.Now().UTC()
	written, err := s.Content.Update(t.Context(), first, created.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("first Update() error = %v, want nil", err)
	}
	stored, err := s.Content.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if !written.UpdatedAt.Equal(stored.UpdatedAt) {
		t.Fatalf("Update() version = %v, want the stored %v", written.UpdatedAt, stored.UpdatedAt)
	}

	second := written
	second.Title, second.UpdatedAt = "Second Edit", time.Now().UTC()
	if _, err := s.Content.Update(t.Context(), second, written.UpdatedAt, nil, 0); err != nil {
		t.Errorf("second Update() error = %v, want the reported version accepted", err)
	}
}

// updateWithASnapshotReportsMissingPosts answers not found and stores no revision for a missing post.
func updateWithASnapshotReportsMissingPosts(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	missing := MustPost(t, "Missing", author)

	_, err := s.Content.Update(t.Context(), missing, missing.UpdatedAt, MustSnapshot(t, missing, author), 0)

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrNotFound)
	}
	revisions, revErr := s.Content.Revisions(t.Context(), missing.ID)
	if revErr != nil {
		t.Fatalf("Revisions() error = %v, want nil", revErr)
	}
	if len(revisions) != 0 {
		t.Errorf("revisions = %d, want none stored for the missing post", len(revisions))
	}
}

// trashRenamesTheSlug moves a post to the trash under a suffixed slug.
func trashRenamesTheSlug(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Hello World", author)

	trashed, err := s.Content.Trash(t.Context(), created.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	if trashed.Status != content.StatusTrash {
		t.Errorf("Status = %q, want %q", trashed.Status, content.StatusTrash)
	}
	if !trashedSlug.MatchString(trashed.Slug) {
		t.Errorf("Slug = %q, want it to match %v", trashed.Slug, trashedSlug)
	}
}

// trashReportsMissingPosts answers not found for an id nothing holds.
func trashReportsMissingPosts(t *testing.T, s Stores) {
	_, err := s.Content.Trash(t.Context(), uuid.Must(uuid.NewV7()), time.Now().UTC())

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("Trash() error = %v, want %v", err, content.ErrNotFound)
	}
}

// restoreRecoversTheOriginalSlug brings a trashed post back as a draft under its own slug.
func restoreRecoversTheOriginalSlug(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Hello World", author)
	if _, err := s.Content.Trash(t.Context(), created.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	restored, err := s.Content.Restore(t.Context(), created.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	if restored.Status != content.StatusDraft {
		t.Errorf("Status = %q, want %q", restored.Status, content.StatusDraft)
	}
	if restored.Slug != "hello-world" {
		t.Errorf("Slug = %q, want the original %q", restored.Slug, "hello-world")
	}
}

// restoreKeepsTheRenamedSlugWhenTheOriginalIsTaken restores under the trashed slug once another post took the original.
func restoreKeepsTheRenamedSlugWhenTheOriginalIsTaken(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Hello World", author)
	trashed, err := s.Content.Trash(t.Context(), created.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	replacement := MustPost(t, "Hello World", author)
	if _, err := s.Content.Create(t.Context(), replacement); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	restored, err := s.Content.Restore(t.Context(), created.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	if restored.Slug != trashed.Slug {
		t.Errorf("Slug = %q, want the renamed %q kept", restored.Slug, trashed.Slug)
	}
}

// restoreRefusesAnItemOutOfTheTrash refuses to restore a published post and leaves it published.
func restoreRefusesAnItemOutOfTheTrash(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Hello World", author)
	published := created
	stamp := time.Now().UTC().Truncate(time.Microsecond)
	published.Status, published.PublishedAt, published.UpdatedAt = content.StatusPublished, &stamp, stamp
	if _, err := s.Content.Update(t.Context(), published, created.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("publishing the post: %v", err)
	}

	_, err := s.Content.Restore(t.Context(), created.ID, time.Now().UTC())

	var refused *content.Error
	if !errors.As(err, &refused) || err != error(refused) || refused.Code != "restore_not_trashed" {
		t.Errorf("Restore() error = %v, want the bare restore_not_trashed refusal", err)
	}
	held, err := s.Content.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if held.Status != content.StatusPublished {
		t.Errorf("Status = %q, want the published post left published", held.Status)
	}
}

// restoreReportsMissingPosts answers not found for an id nothing holds.
func restoreReportsMissingPosts(t *testing.T, s Stores) {
	_, err := s.Content.Restore(t.Context(), uuid.Must(uuid.NewV7()), time.Now().UTC())

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("Restore() error = %v, want %v", err, content.ErrNotFound)
	}
}

// deleteRemovesTheContent removes a post for good.
func deleteRemovesTheContent(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Doomed", author)

	if err := s.Content.Delete(t.Context(), created.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	if _, err := s.Content.ByID(t.Context(), created.ID); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("ByID() after delete error = %v, want %v", err, content.ErrNotFound)
	}
}

// deleteReportsMissingPosts answers not found for an id nothing holds.
func deleteReportsMissingPosts(t *testing.T, s Stores) {
	err := s.Content.Delete(t.Context(), uuid.Must(uuid.NewV7()))

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("Delete() error = %v, want %v", err, content.ErrNotFound)
	}
}
