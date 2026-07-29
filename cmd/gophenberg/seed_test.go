// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/gouncer"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// execSQL runs statement against the database at databaseURL.
func execSQL(t *testing.T, databaseURL, statement string) {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(t.Context(), statement); err != nil {
		t.Fatalf("exec %q: %v", statement, err)
	}
}

// seededCounts returns the seeded posts of each status.
func seededCounts(t *testing.T, databaseURL string) map[post.Status]int {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	counts, err := postgres.NewPostStore(pool).Counts(t.Context(), post.TypePost)
	if err != nil {
		t.Fatalf("Counts() error = %v, want nil", err)
	}
	return counts
}

func TestSeedStoresTheDemoPosts(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	var stdout strings.Builder

	if err := seed(t.Context(), testGetenv(env), &stdout); err != nil {
		t.Fatalf("seed() error = %v, want nil", err)
	}

	counts := seededCounts(t, databaseURL)
	want := map[post.Status]int{
		post.StatusPublished: 2,
		post.StatusDraft:     1,
		post.StatusPending:   1,
		post.StatusTrash:     1,
	}
	for status, total := range want {
		if counts[status] != total {
			t.Errorf("counts[%q] = %d, want %d", status, counts[status], total)
		}
	}
	if !strings.Contains(stdout.String(), seedAdminEmail) {
		t.Errorf("output = %q, want the admin credentials", stdout.String())
	}
	if !strings.Contains(stdout.String(), "development only") {
		t.Errorf("output = %q, want the development warning", stdout.String())
	}
}

func TestSeedIsIdempotent(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seed(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("first seed() error = %v, want nil", err)
	}
	first := seededCounts(t, databaseURL)
	var stdout strings.Builder

	if err := seed(t.Context(), testGetenv(env), &stdout); err != nil {
		t.Fatalf("second seed() error = %v, want nil", err)
	}

	second := seededCounts(t, databaseURL)
	for status, total := range first {
		if second[status] != total {
			t.Errorf("counts[%q] = %d after reseeding, want %d", status, second[status], total)
		}
	}
	if !strings.Contains(stdout.String(), "already exists") {
		t.Errorf("output = %q, want it to report the existing admin", stdout.String())
	}
}

func TestSeedStoresBlockContent(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seed(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("seed() error = %v, want nil", err)
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()

	posts, _, err := postgres.NewPostStore(pool).List(
		t.Context(), post.Filter{Type: post.TypePost, Status: post.StatusPublished, Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if len(posts) != 2 {
		t.Fatalf("published posts = %d, want 2", len(posts))
	}
	stored, err := postgres.NewPostStore(pool).ByID(t.Context(), posts[0].ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if !strings.Contains(stored.Content, "<!-- wp:") {
		t.Errorf("content = %q, want serialized block markup", stored.Content)
	}
	if stored.PublishedAt == nil {
		t.Error("PublishedAt = nil, want a published post to carry its date")
	}
}

func TestSeedValidatesItsEnvironment(t *testing.T) {
	t.Parallel()

	tests := map[string]map[string]string{
		"missing database url": {},
		"malformed database url": {
			"GOPHENBERG_DATABASE_URL": "not a url \x00",
		},
		"unreachable database": {
			"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		},
	}

	for testName, env := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if err := seed(t.Context(), testGetenv(env), io.Discard); err == nil {
				t.Fatal("seed() error = nil, want a failure")
			}
		})
	}
}

func TestSeedReportsMigrationFailures(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	execSQL(t, databaseURL, "CREATE SCHEMA core")

	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}

	if err := seed(t.Context(), testGetenv(env), io.Discard); err == nil {
		t.Error("seed() over a conflicting schema error = nil, want a failure")
	}
}

func TestSeedReportsAdminFailures(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seed(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("first seed() error = %v, want nil", err)
	}
	execSQL(t, databaseURL, "DROP TABLE auth.users CASCADE")

	if err := seed(t.Context(), testGetenv(env), io.Discard); err == nil {
		t.Error("seed() without the users table error = nil, want a failure")
	}
}

func TestSeedReportsPostFailures(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seed(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("first seed() error = %v, want nil", err)
	}
	execSQL(t, databaseURL, "DROP TABLE core.posts CASCADE")

	if err := seed(t.Context(), testGetenv(env), io.Discard); err == nil {
		t.Error("seed() without the posts table error = nil, want a failure")
	}
}

func TestSeedPostsReportsStoreFailures(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		store post.Store
		users gouncer.Store
	}{
		"admin lookup": {store: stubPostStore{}, users: stubUserStore{err: errStub}},
		"post lookup":  {store: stubPostStore{byIDErr: errStub}, users: stubUserStore{}},
		"build":        {store: stubPostStore{byIDErr: post.ErrNotFound}, users: stubUserStore{id: uuid.Nil}},
		"create": {
			store: stubPostStore{byIDErr: post.ErrNotFound, createErr: errStub},
			users: stubUserStore{id: uuid.New()},
		},
		"trash": {
			store: stubPostStore{byIDErr: post.ErrNotFound, trashErr: errStub},
			users: stubUserStore{id: uuid.New()},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if err := seedPosts(t.Context(), test.store, test.users); err == nil {
				t.Error("seedPosts() error = nil, want a failure")
			}
		})
	}
}

func TestStoreDemoPostRejectsAnUnknownStatus(t *testing.T) {
	t.Parallel()

	scripted := demoPost{title: "Unknown", status: post.Status("nonsense")}

	err := storeDemoPost(t.Context(), stubPostStore{}, scripted, uuid.New(), uuid.New())

	if err == nil {
		t.Error("storeDemoPost() with an unknown status error = nil, want a failure")
	}
}

// errStub is the failure reported by the seeding stubs.
var errStub = errors.New("stub failure")

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
	post.Store
	byIDErr   error
	createErr error
	trashErr  error
}

// ByID reports the scripted lookup failure.
func (s stubPostStore) ByID(_ context.Context, _ uuid.UUID) (post.Post, error) {
	return post.Post{}, s.byIDErr
}

// Create reports the scripted storage failure.
func (s stubPostStore) Create(_ context.Context, p post.Post) (post.Post, error) {
	return p, s.createErr
}

// Trash reports the scripted trashing failure.
func (s stubPostStore) Trash(_ context.Context, _ uuid.UUID, _ time.Time) (post.Post, error) {
	return post.Post{}, s.trashErr
}
