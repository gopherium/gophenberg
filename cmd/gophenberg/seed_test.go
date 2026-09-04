// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gouncer"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/seed"
)

func TestSeedGivesTheAdminTheAdminRole(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("seedDemoData() error = %v, want nil", err)
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()

	held, err := authkitpg.NewUserStore(pool).UserByEmail(t.Context(), seed.AdminEmail)

	if err != nil {
		t.Fatalf("UserByEmail() error = %v, want nil", err)
	}
	if held.Role != role.Admin {
		t.Errorf("role = %q, want the seeded administrator to hold %q", held.Role, role.Admin)
	}
}

func TestSeedCarriesAnAdminAcrossFromBeforeRolesExisted(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := authkitpg.Migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("migrating the auth schema: %v", err)
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	users := authkitpg.NewUserStore(pool)
	before, err := gouncer.NewUser(seed.AdminEmail, seed.AdminName, seed.AdminPassword)
	if err != nil {
		t.Fatalf("NewUser() error = %v, want nil", err)
	}
	if before.Role != "" {
		t.Fatalf("Role = %q, want an account holding none before the seed runs", before.Role)
	}
	if err := users.CreateUser(t.Context(), before); err != nil {
		t.Fatalf("storing the account that predates roles: %v", err)
	}

	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("seedDemoData() error = %v, want nil", err)
	}

	held, err := users.UserByEmail(t.Context(), seed.AdminEmail)
	if err != nil {
		t.Fatalf("UserByEmail() error = %v, want nil", err)
	}
	if held.Role != role.Admin {
		t.Errorf("role = %q, want the roleless administrator carried across to %q", held.Role, role.Admin)
	}
	if held.PasswordHash != before.PasswordHash {
		t.Error("password hash changed, want the repair to touch only the role")
	}
}

func TestSeedStoresAnAccountUnderEveryRole(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	var stdout strings.Builder
	if err := seedDemoData(t.Context(), testGetenv(env), &stdout); err != nil {
		t.Fatalf("seedDemoData() error = %v, want nil", err)
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	users := authkitpg.NewUserStore(pool)

	for _, email := range []string{seed.EditorEmail, seed.AuthorEmail} {
		if !strings.Contains(stdout.String(), email) {
			t.Errorf("output = %q, want it to name the seeded %q", stdout.String(), email)
		}
	}
	for email, want := range map[string]string{
		seed.AdminEmail:  role.Admin,
		seed.EditorEmail: role.Editor,
		seed.AuthorEmail: role.Author,
	} {
		held, err := users.UserByEmail(t.Context(), email)
		if err != nil {
			t.Fatalf("UserByEmail(%q) error = %v, want nil", email, err)
		}
		if held.Role != want {
			t.Errorf("%q holds %q, want %q", email, held.Role, want)
		}
	}
}

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
func seededCounts(t *testing.T, databaseURL string) map[content.Status]int {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	counts, err := postgres.NewContentStore(pool).Counts(t.Context(), content.TypePost)
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

	if err := seedDemoData(t.Context(), testGetenv(env), &stdout); err != nil {
		t.Fatalf("seedDemoData() error = %v, want nil", err)
	}

	counts := seededCounts(t, databaseURL)
	want := map[content.Status]int{
		content.StatusPublished: 3,
		content.StatusDraft:     1,
		content.StatusPending:   1,
		content.StatusTrash:     1,
	}
	for status, total := range want {
		if counts[status] != total {
			t.Errorf("counts[%q] = %d, want %d", status, counts[status], total)
		}
	}
	if !strings.Contains(stdout.String(), seed.AdminEmail) {
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
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("first seedDemoData() error = %v, want nil", err)
	}
	first := seededCounts(t, databaseURL)
	var stdout strings.Builder

	if err := seedDemoData(t.Context(), testGetenv(env), &stdout); err != nil {
		t.Fatalf("second seedDemoData() error = %v, want nil", err)
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

func TestSeedStoresTheDemoMediaWhenADirectoryIsConfigured(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	dir := t.TempDir()
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL, "GOPHENBERG_MEDIA_DIR": dir}

	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("seedDemoData() error = %v, want nil", err)
	}

	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	items, total, err := postgres.NewMediaStore(pool).List(t.Context(), media.Filter{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 3 {
		t.Fatalf("stored media = %d, want the three demo pictures", total)
	}
	for _, item := range items {
		if item.AltText == "" {
			t.Errorf("%q carries no alt text, want every demo picture described", item.Title)
		}
		if len(item.Sizes) == 0 {
			t.Errorf("%q carries no renditions, want them derived by the pipeline", item.Title)
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(item.File))); err != nil {
			t.Errorf("the file of %q is missing: %v", item.Title, err)
		}
	}
}

func TestSeedReportsAFailingMediaDirectory(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("in the way"), 0o644); err != nil {
		t.Fatalf("planting the blocking file: %v", err)
	}
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL, "GOPHENBERG_MEDIA_DIR": blocked}

	err := seedDemoData(t.Context(), testGetenv(env), io.Discard)

	if err == nil {
		t.Error("seedDemoData() into a blocked media directory error = nil, want a failure")
	}
}

func TestSeedStoresBlockContent(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("seedDemoData() error = %v, want nil", err)
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()

	posts, _, err := postgres.NewContentStore(pool).List(
		t.Context(), content.Filter{Type: content.TypePost, Status: content.StatusPublished, Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if len(posts) != 3 {
		t.Fatalf("published posts = %d, want 3", len(posts))
	}
	stored, err := postgres.NewContentStore(pool).ByID(t.Context(), posts[0].ID)
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

			if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err == nil {
				t.Fatal("seedDemoData() error = nil, want a failure")
			}
		})
	}
}

func TestSeedReportsMigrationFailures(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	execSQL(t, databaseURL, "CREATE SCHEMA core")

	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}

	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err == nil {
		t.Error("seed() over a conflicting schema error = nil, want a failure")
	}
}

func TestSeedReportsAdminFailures(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("first seedDemoData() error = %v, want nil", err)
	}
	execSQL(t, databaseURL, "DROP TABLE auth.users CASCADE")

	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err == nil {
		t.Error("seed() without the users table error = nil, want a failure")
	}
}

func TestSeedReportsContentFailures(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("first seedDemoData() error = %v, want nil", err)
	}
	execSQL(t, databaseURL, "DROP TABLE core.content CASCADE")

	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err == nil {
		t.Error("seed() without the content table error = nil, want a failure")
	}
}

// seededTypes returns the registry the seeding left behind.
func seededTypes(t *testing.T, databaseURL string) []content.Type {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	registered, err := postgres.NewTypeStore(pool).List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	return registered
}

// seededAddress returns the address the seeding stored content at.
func seededAddress(t *testing.T, databaseURL, title string) string {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	var path string
	row := pool.QueryRow(t.Context(), `SELECT path FROM core.content WHERE title = $1`, title)
	if err := row.Scan(&path); err != nil {
		t.Fatalf("reading the address of %q: %v", title, err)
	}
	return path
}

func TestSeedRegistersThePageType(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	var stdout strings.Builder

	if err := seedDemoData(t.Context(), testGetenv(env), &stdout); err != nil {
		t.Fatalf("seedDemoData() error = %v, want nil", err)
	}

	registered := seededTypes(t, databaseURL)
	var page content.Type
	for _, listed := range registered {
		if listed.Key == "page" {
			page = listed
		}
	}
	if page.Key != "page" {
		t.Fatalf("registry = %v, want a page type beside the built-in post type", registered)
	}
	if page.RouteWord != "pages" || !page.Hierarchical {
		t.Errorf("page = %+v, want it nesting under pages", page)
	}
	if page.Default {
		t.Error("the page type carries the default flag, want the post type to keep the root")
	}
}

func TestSeedStoresAPageHoldingAChild(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	var stdout strings.Builder

	if err := seedDemoData(t.Context(), testGetenv(env), &stdout); err != nil {
		t.Fatalf("seedDemoData() error = %v, want nil", err)
	}

	if got := seededAddress(t, databaseURL, "About"); got != "pages/about" {
		t.Errorf("About answers at %q, want %q", got, "pages/about")
	}
	if got := seededAddress(t, databaseURL, "Team"); got != "pages/about/team" {
		t.Errorf("Team answers at %q, want %q", got, "pages/about/team")
	}
}

// refusingUserStore is a user store that refuses to store an account.
type refusingUserStore struct {
	gouncer.Store
}

// UserByEmail reports that no account carries the email.
func (refusingUserStore) UserByEmail(context.Context, string) (gouncer.User, error) {
	return gouncer.User{}, gouncer.ErrUserNotFound
}

// CreateUser refuses to store the account.
func (refusingUserStore) CreateUser(context.Context, gouncer.User) error {
	return context.DeadlineExceeded
}

func TestSeedNamesTheRoleItCouldNotStore(t *testing.T) {
	t.Parallel()

	err := seedAccountsWithRoles(t.Context(), refusingUserStore{})

	if err == nil {
		t.Fatal("seedAccountsWithRoles() error = nil, want a failure")
	}
	if !strings.Contains(err.Error(), role.Editor) {
		t.Errorf("error = %v, want it to name the role it was seeding", err)
	}
}

func TestSeedReportsContentItCannotRegister(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("first seedDemoData() error = %v, want nil", err)
	}
	execSQL(t, databaseURL, "DROP TABLE core.content_types CASCADE")

	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("opening the pool: %v", err)
	}
	defer pool.Close()

	if err := seedDemoContent(t.Context(), pool, authkitpg.NewUserStore(pool)); err == nil {
		t.Error("seedDemoContent() error = nil, want the missing registry reported")
	}
}

func TestSeedReportsAnAccountItCannotStore(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("first seedDemoData() error = %v, want nil", err)
	}
	execSQL(t, databaseURL, "DELETE FROM auth.users WHERE email = '"+seed.EditorEmail+"'")
	execSQL(t, databaseURL,
		"ALTER TABLE auth.users ADD CONSTRAINT no_editor CHECK (email <> '"+seed.EditorEmail+"')")

	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err == nil {
		t.Error("seedDemoData() error = nil, want the refused account reported")
	}
}

func TestSeedReportsCategoriesItCannotStore(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("first seedDemoData() error = %v, want nil", err)
	}
	execSQL(t, databaseURL, "DELETE FROM core.content WHERE type = 'category'")
	execSQL(t, databaseURL,
		"ALTER TABLE core.content ADD CONSTRAINT no_categories CHECK (type <> 'category')")

	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("opening the pool: %v", err)
	}
	defer pool.Close()

	if err := seedDemoContent(t.Context(), pool, authkitpg.NewUserStore(pool)); err == nil {
		t.Error("seedDemoContent() error = nil, want the refused category reported")
	}
}

func TestSeedReportsPagesItCannotStore(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("first seedDemoData() error = %v, want nil", err)
	}
	execSQL(t, databaseURL, "DELETE FROM core.content WHERE type = 'page'")
	execSQL(t, databaseURL, "ALTER TABLE core.content ADD CONSTRAINT no_pages CHECK (type <> 'page')")

	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("opening the pool: %v", err)
	}
	defer pool.Close()

	if err := seedDemoContent(t.Context(), pool, authkitpg.NewUserStore(pool)); err == nil {
		t.Error("seedDemoContent() error = nil, want the refused page reported")
	}
}

func TestSeedReportsContainersItCannotDeclare(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("first seedDemoData() error = %v, want nil", err)
	}
	execSQL(t, databaseURL, "DELETE FROM core.content_fields WHERE key IN ('team', 'name', 'role')")
	execSQL(t, databaseURL,
		"ALTER TABLE core.content_fields ADD CONSTRAINT no_team CHECK (key <> 'team')")

	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("opening the pool: %v", err)
	}
	defer pool.Close()

	if err := seedDemoContent(t.Context(), pool, authkitpg.NewUserStore(pool)); err == nil {
		t.Error("seedDemoContent() error = nil, want the refused container reported")
	}
}
