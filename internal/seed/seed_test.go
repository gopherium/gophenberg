// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content"
)

// testRegistry returns a registry holding the built-in post type.
func testRegistry() *content.Registry {
	return content.NewRegistry(stubTypeStore{})
}

// stubTypeStore answers with the built-in post type alone.
type stubTypeStore struct{}

// List returns the built-in post type.
func (stubTypeStore) List(context.Context) ([]content.Type, error) {
	return []content.Type{{
		Key: content.TypePost, SingularLabel: "Post", PluralLabel: "Posts",
		Revisions: true, RevisionCap: 100, PageKind: content.PageKindSingle,
		Default: true, Active: true, Description: "Manage the posts on this site.",
	}}, nil
}

// ByKey returns the built-in post type, or reports the key missing.
func (s stubTypeStore) ByKey(ctx context.Context, key string) (content.Type, error) {
	types, _ := s.List(ctx)
	if key == content.TypePost {
		return types[0], nil
	}
	return content.Type{}, content.ErrTypeNotFound
}

// Create refuses to register a type in a seeding run.
func (stubTypeStore) Create(context.Context, content.Type) (content.Type, error) {
	return content.Type{}, content.ErrTypeTaken
}

// Update refuses to edit a type in a seeding run.
func (stubTypeStore) Update(context.Context, content.Type) (content.Type, error) {
	return content.Type{}, content.ErrTypeNotFound
}

// Delete refuses to remove a type in a seeding run.
func (stubTypeStore) Delete(context.Context, string) error { return content.ErrTypeNotFound }

// Nested counts no nested items in a seeding run.
func (stubTypeStore) Nested(context.Context, string) (int, error) { return 0, nil }

// ReorderFields refuses to reorder fields in a seeding run.
// ListGroups returns no field groups.
func (stubTypeStore) ListGroups(context.Context) ([]content.Group, error) {
	return nil, nil
}

// CreateGroup hands the group back unstored.
func (stubTypeStore) CreateGroup(_ context.Context, g content.Group) (content.Group, error) {
	return g, nil
}

// UpdateGroup hands the group back unstored.
func (stubTypeStore) UpdateGroup(
	_ context.Context, g content.Group, _ []content.Field, _ content.Recheck,
) (content.Group, error) {
	return g, nil
}

// DeleteGroup removes no group.
func (stubTypeStore) DeleteGroup(context.Context, int, content.Recheck) error { return nil }

// DeleteFieldsOfGroup removes no field.
func (stubTypeStore) DeleteFieldsOfGroup(context.Context, int, []string, content.Recheck) error {
	return nil
}

// ReorderGroups stores no order.
func (stubTypeStore) ReorderGroups(context.Context, []int) error { return nil }

// CreateFieldInGroup hands the field back undeclared.
func (stubTypeStore) CreateFieldInGroup(
	_ context.Context, _ int, f content.Field, _ content.Recheck,
) (content.Field, error) {
	return f, nil
}

// MoveField carries no field.
func (stubTypeStore) MoveField(context.Context, int, int, int, int, content.Recheck) (content.Field, error) {
	return content.Field{}, content.ErrFieldNotFound
}

// UpdateFieldInGroup hands the field back unstored.
func (stubTypeStore) UpdateFieldInGroup(
	_ context.Context, _ int, f content.Field, _ time.Time, _ content.Recheck,
) (content.Field, error) {
	return f, nil
}

// CreateSubField hands the field back unstored.
func (stubTypeStore) CreateSubField(_ context.Context, _ int, f content.Field, _ int) (content.Field, error) {
	return f, nil
}

// DeleteSubField removes no field.
func (stubTypeStore) DeleteSubField(_ context.Context, _ int, _ content.Recheck) error {
	return nil
}

// UpdateSubField carries no edit.
func (stubTypeStore) UpdateSubField(
	_ context.Context, _ int, f content.Field, _ time.Time,
) (content.Field, error) {
	return f, nil
}

// ReorderSubFields stores no order.
func (stubTypeStore) ReorderSubFields(_ context.Context, _ int, _ []string) error {
	return nil
}

// DeleteFieldInGroup removes no field.
func (stubTypeStore) DeleteFieldInGroup(context.Context, int, string, content.Recheck) error {
	return nil
}

// ReorderFieldsInGroup stores no order.
func (stubTypeStore) ReorderFieldsInGroup(context.Context, int, []string) error { return nil }

// CreateField refuses to declare a field in a seeding run.
func (stubTypeStore) CreateField(context.Context, content.Field) (content.Field, error) {
	return content.Field{}, content.ErrTypeNotFound
}

func TestPostsReportsStoreFailures(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		store content.Store
		users gouncer.Store
	}{
		"admin lookup": {store: stubPostStore{}, users: stubUserStore{err: errStub}},
		"post lookup":  {store: stubPostStore{byIDErr: errStub}, users: stubUserStore{}},
		"build":        {store: stubPostStore{byIDErr: content.ErrNotFound}, users: stubUserStore{id: uuid.Nil}},
		"create": {
			store: stubPostStore{byIDErr: content.ErrNotFound, createErr: errStub},
			users: stubUserStore{id: uuid.New()},
		},
		"trash": {
			store: stubPostStore{byIDErr: content.ErrNotFound, trashErr: errStub},
			users: stubUserStore{id: uuid.New()},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if err := Posts(t.Context(), test.store, testRegistry(), test.users); err == nil {
				t.Error("Posts() error = nil, want a failure")
			}
		})
	}
}

func TestPostsStoresEveryScriptedPost(t *testing.T) {
	t.Parallel()

	store := &countingPostStore{}

	if err := Posts(t.Context(), store, testRegistry(), stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("Posts() error = %v, want nil", err)
	}

	if store.created != len(demoPosts()) {
		t.Errorf("created %d posts, want %d", store.created, len(demoPosts()))
	}
	if store.trashed != 2 {
		t.Errorf("trashed %d posts, want 2", store.trashed)
	}
}

func TestDemoPostsHoldTheStatusesTheListsPageThrough(t *testing.T) {
	t.Parallel()

	held := map[content.Status]int{}
	for _, scripted := range demoPosts() {
		held[scripted.status]++
	}

	want := map[content.Status]int{
		content.StatusPublished: 23, content.StatusDraft: 5, content.StatusPending: 6, content.StatusTrash: 2,
	}
	if !maps.Equal(held, want) {
		t.Errorf("statuses = %v, want %v", held, want)
	}
}

func TestDemoPostsCarryUniqueIdentitiesAndAddresses(t *testing.T) {
	t.Parallel()

	ids, slugs := map[string]bool{}, map[string]bool{}
	for _, scripted := range demoPosts() {
		slug := content.Slugify(scripted.title)
		if ids[scripted.id] || slugs[slug] {
			t.Errorf("%q repeats an identity or an address", scripted.title)
		}
		ids[scripted.id], slugs[slug] = true, true
	}
}

// rosterUserStore answers each demo account under its own identity.
type rosterUserStore struct {
	gouncer.Store
	ids map[string]uuid.UUID
}

// UserByEmail returns the account the roster holds under the email.
func (s rosterUserStore) UserByEmail(_ context.Context, email string) (gouncer.User, error) {
	return gouncer.User{ID: s.ids[email]}, nil
}

// seededPosts stores the demo posts in a counting store over the roster, answering what it stored.
func seededPosts(t *testing.T, roster rosterUserStore) []content.Content {
	t.Helper()
	store := &countingPostStore{}
	if err := Posts(t.Context(), store, testRegistry(), roster); err != nil {
		t.Fatalf("Posts() error = %v, want nil", err)
	}
	return store.stored
}

// demoRoster returns a roster holding an identity for each demo account.
func demoRoster() rosterUserStore {
	return rosterUserStore{ids: map[string]uuid.UUID{
		AdminEmail: uuid.New(), EditorEmail: uuid.New(), AuthorEmail: uuid.New(),
	}}
}

func TestPostsCreditEveryDemoAccountWithWork(t *testing.T) {
	t.Parallel()

	roster := demoRoster()

	stored := seededPosts(t, roster)

	written := map[uuid.UUID]int{}
	for _, post := range stored {
		written[post.AuthorID]++
	}
	for email, id := range roster.ids {
		if written[id] == 0 {
			t.Errorf("%s wrote nothing, want every demo account credited with posts", email)
		}
	}
	if len(written) != len(roster.ids) {
		t.Errorf("posts are credited to %d accounts, want only the %d demo ones", len(written), len(roster.ids))
	}
}

func TestPostsSpreadTheArchiveOverEighteenMonths(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC()
	stored := seededPosts(t, demoRoster())

	oldest, dated := before, 0
	for _, post := range stored {
		at := post.UpdatedAt
		if post.PublishedAt != nil {
			at = *post.PublishedAt
		}
		if at.Before(before.Add(-24 * time.Hour)) {
			dated++
			if !post.UpdatedAt.Equal(at) {
				t.Errorf("%q changed at %v but went out at %v, want one date", post.Title, post.UpdatedAt, at)
			}
		}
		if at.Before(oldest) {
			oldest = at
		}
		if post.CreatedAt.After(at) {
			t.Errorf("%q was written at %v after its date %v", post.Title, post.CreatedAt, at)
		}
	}
	if dated != 30 {
		t.Errorf("%d posts are dated in the past, want the 30 of the archive", dated)
	}
	if months := before.Sub(oldest).Hours() / 24 / 30; months < 17 || months > 18.5 {
		t.Errorf("the oldest post is %.1f months old, want the archive spread over about 18 months", months)
	}
}

func TestPostsSwitchOnTheFeaturedOnes(t *testing.T) {
	t.Parallel()

	stored := seededPosts(t, demoRoster())

	featured := 0
	for _, post := range stored {
		if post.Fields[FeaturedFieldKey] == true {
			featured++
		}
	}
	if featured != 5 {
		t.Errorf("%d posts are featured, want 5", featured)
	}
}

func TestPostsLeavesPostsItAlreadyStored(t *testing.T) {
	t.Parallel()

	store := &countingPostStore{found: true}

	if err := Posts(t.Context(), store, testRegistry(), stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("Posts() error = %v, want nil", err)
	}

	if store.created != 0 {
		t.Errorf("created %d posts, want none over a seeded database", store.created)
	}
}

func TestStoreDemoPostRejectsAnUnknownStatus(t *testing.T) {
	t.Parallel()

	scripted := demoPost{title: "Unknown", status: content.Status("nonsense")}

	postType, err := testRegistry().ByKey(t.Context(), content.TypePost)
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}

	err = storeDemoPost(t.Context(), stubPostStore{}, postType, scripted, uuid.New(), uuid.New())

	if err == nil {
		t.Error("storeDemoPost() with an unknown status error = nil, want a failure")
	}
}

// errStub is the failure reported by the seeding stubs.
var errStub = errors.New("stub failure")

// countingPostStore is a post store counting what the seeding stored.
type countingPostStore struct {
	content.Store
	found   bool
	created int
	trashed int
	stored  []content.Content
}

// ByID reports whether the post was already stored.
func (s *countingPostStore) ByID(_ context.Context, _ uuid.UUID) (content.Content, error) {
	if s.found {
		return content.Content{}, nil
	}
	return content.Content{}, content.ErrNotFound
}

// Create counts the post as stored and keeps it.
func (s *countingPostStore) Create(_ context.Context, p content.Content) (content.Content, error) {
	s.created++
	s.stored = append(s.stored, p)
	return p, nil
}

// Trash counts the post as trashed.
func (s *countingPostStore) Trash(_ context.Context, _ uuid.UUID, _ time.Time) (content.Content, error) {
	s.trashed++
	return content.Content{}, nil
}

// stubUserStore is a user store returning a scripted admin.
type stubUserStore struct {
	gouncer.Store
	id  uuid.UUID
	err error
}

// UserByEmail returns the scripted admin.
func (s stubUserStore) UserByEmail(_ context.Context, _ string) (gouncer.User, error) {
	return gouncer.User{ID: s.id}, s.err
}

// stubPostStore is a post store reporting scripted failures.
type stubPostStore struct {
	content.Store
	byIDErr   error
	createErr error
	trashErr  error
}

// ByID reports the scripted lookup failure.
func (s stubPostStore) ByID(_ context.Context, _ uuid.UUID) (content.Content, error) {
	return content.Content{}, s.byIDErr
}

// Create reports the scripted storage failure.
func (s stubPostStore) Create(_ context.Context, p content.Content) (content.Content, error) {
	return p, s.createErr
}

// Trash reports the scripted trashing failure.
func (s stubPostStore) Trash(_ context.Context, _ uuid.UUID, _ time.Time) (content.Content, error) {
	return content.Content{}, s.trashErr
}

func TestPostsReportsAMissingPostType(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(&categoryTypeStore{postless: true})

	err := Posts(t.Context(), newFilingStore(), registry, stubUserStore{id: uuid.New()})

	if err == nil {
		t.Error("Posts() error = nil, want the missing post type reported")
	}
}

// AdoptType takes a plugin's type over as the site's own.
func (stubTypeStore) AdoptType(context.Context, string) error {
	return nil
}

// AdoptGroup takes a plugin's group over as the site's own.
func (stubTypeStore) AdoptGroup(context.Context, string) error {
	return nil
}
