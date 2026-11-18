// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/gouncer"
	authkitsqlite "github.com/gopherium/gouncer/authkit/sqlite"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/seed"
	"github.com/gopherium/gophenberg/internal/sqlite"
)

// emptySQLiteFile returns the path of a new empty SQLite file, alone in a folder of its own.
func emptySQLiteFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "site.db")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("creating the empty file: %v", err)
	}
	return path
}

// folderNames returns the name of every entry in the folder.
func folderNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the folder: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestCheckConnectsToNothingOnAnEmptySQLiteFile(t *testing.T) {
	t.Parallel()

	path := emptySQLiteFile(t)
	env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": sqlitePrefix + path})

	got := testkit.Run(t, Program(env, keeping(&keeper{})), "", "check")

	if got.Code != gonsole.ExitDone || got.Stdout != "settings, plugins and command names are valid\n" {
		t.Fatalf("check = %d, stdout %q, stderr %q, want 0 and every setting valid", got.Code, got.Stdout, got.Stderr)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("reading the file: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("the file holds %d bytes after check, want 0", info.Size())
	}
	if names := folderNames(t, filepath.Dir(path)); !slices.Equal(names, []string{"site.db"}) {
		t.Errorf("the folder holds %v after check, want only site.db with no -wal file", names)
	}
}

func TestCheckNamesAMissingSQLiteFileByItsPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "site.db")
	env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": sqlitePrefix + path})

	got := testkit.Run(t, Program(env, noPlugins), "", "check")

	if want := sqlite.ErrNoFile.Error() + ": " + path; got.Code != gonsole.ExitFailed ||
		!strings.Contains(got.Stderr, want) {
		t.Errorf("check = %d with stderr %q, want 1 and %q", got.Code, got.Stderr, want)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the file after check: %v, want it still missing", err)
	}
}

func TestMigrateNamesTheDatabaseAddressForABadSQLitePath(t *testing.T) {
	t.Parallel()

	for _, address := range []string{"sqlite:site.db", "sqlite://x"} {
		t.Run(address, func(t *testing.T) {
			t.Parallel()

			env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": address})

			got := testkit.Run(t, Program(env, noPlugins), "", "migrate")

			if got.Code != gonsole.ExitFailed || !strings.Contains(got.Stderr, "GOPHENBERG_DATABASE_URL:") ||
				strings.Contains(got.Stderr, "BaseFolder") {
				t.Errorf("migrate = %d with stderr %q, want 1 and GOPHENBERG_DATABASE_URL named, never BaseFolder",
					got.Code, got.Stderr)
			}
		})
	}
}

func TestAPluginCommandNamesABadSQLiteSettingAsTheHandleOpens(t *testing.T) {
	t.Parallel()

	env := testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL":       sqlitePrefix + emptySQLiteFile(t),
		"GOPHENBERG_SQLITE_SYNCHRONOUS": "off",
	})

	got := testkit.Run(t, Program(env, keeping(&keeper{})), "", "keeper:show")

	if got.Code != gonsole.ExitFailed || !strings.Contains(got.Stderr, "GOPHENBERG_SQLITE_SYNCHRONOUS") {
		t.Errorf("keeper:show = %d with stderr %q, want 1 and GOPHENBERG_SQLITE_SYNCHRONOUS named",
			got.Code, got.Stderr)
	}
}

func TestDescribingThePluginsOnSQLiteOpensNoDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "site.db")
	call := gonsole.Call{
		Env:      settingsEnv(testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": sqlitePrefix + path})),
		Describe: true,
	}

	loaded, err := loadPlugins(binary{plugins: keeping(&keeper{})}, goncierge.New(), &sitePool{})(t.Context(), call)

	if err != nil {
		t.Fatalf("loadPlugins() error = %v, want the plugins described with no database opened", err)
	}
	if err := loaded.Release(t.Context()); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
}

func TestEachEngineLendsTheMediaLibraryOnlyUnderAMediaFolder(t *testing.T) {
	t.Parallel()

	for _, address := range []string{unreachableDatabaseURL, sqliteAddress} {
		for mediaDir, want := range map[string]bool{"": false, t.TempDir(): true} {
			built, err := openStores(composeConfig{databaseURL: address, fieldDepth: 4, mediaDir: mediaDir})
			if err != nil {
				t.Fatalf("openStores(%q) error = %v, want nil", address, err)
			}
			built.release()

			if got := built.library != nil; got != want {
				t.Errorf("openStores(%q) with the media folder %q lends a library %v, want %v",
					address, mediaDir, got, want)
			}
		}
	}
}

func TestTheSQLiteSiteKeepsEveryStoreItBuildsInTheFile(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	built := sqliteStores(composeConfig{handle: openSQLiteFile(t, path), fieldDepth: 4, mediaDir: t.TempDir()})

	if err := built.settings.Save(t.Context(), map[string]string{"blogname": "Synthetic Site"}); err != nil {
		t.Fatalf("saving a site setting: %v", err)
	}
	value, found, err := sqlite.NewSettingStore(openSQLiteFile(t, path)).Lookup(t.Context(), "blogname")
	if err != nil || !found || value != "Synthetic Site" {
		t.Errorf("the file holds blogname = %q, %v, %v, want the value the site saved", value, found, err)
	}
	if _, found, err := built.readers.Lookup(t.Context(), uuid.Nil, "theme"); err != nil || found {
		t.Errorf("the reader setting lookup = %v, %v, want nothing found in a new file", found, err)
	}
	file := "2026/10/sunrise.jpg"
	if saved, err := built.saved(t.Context(), []string{file}); err != nil || saved[file] {
		t.Errorf("saved() = %v, %v, want no file saved in a new file", saved, err)
	}
	if built.library == nil || built.registry == nil {
		t.Errorf("library %v and registry %v, want both built under a media folder", built.library, built.registry)
	}
}

func TestTheSQLiteSiteKeepsTheAccountsItCreatesInTheFile(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	built := sqliteStores(composeConfig{handle: openSQLiteFile(t, path), fieldDepth: 4})
	if built.users == nil {
		t.Fatal("the SQLite site holds no user store, want one over the file")
	}
	created := gouncer.User{
		ID: uuid.Must(uuid.NewV7()), Email: seed.AuthorEmail, Name: seed.AuthorName, PasswordHash: "hash",
		CreatedAt: accountsAdded,
	}

	if err := built.users.CreateUser(t.Context(), created); err != nil {
		t.Fatalf("creating an account through the site's user store: %v", err)
	}

	read, err := authkitsqlite.NewUserStore(openSQLiteFile(t, path)).UserByEmail(t.Context(), created.Email)
	if err != nil || read.ID != created.ID || read.Name != created.Name {
		t.Errorf("a second handle on the file reads %s named %q (%v), want the account %s the site created",
			read.ID, read.Name, err, created.ID)
	}
}

func TestTheSQLiteSiteKeepsTheTypesAndContentItStoresInTheFile(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	built := sqliteStores(composeConfig{handle: openSQLiteFile(t, path), fieldDepth: 4})
	if built.content == nil || built.types == nil {
		t.Fatalf("the SQLite site holds a content store %t and a type store %t, want both over the file",
			built.content != nil, built.types != nil)
	}

	book := registeredBook(t, built.registry)
	stored := storedBook(t, built, book, "Dune")

	second := openSQLiteFile(t, path)
	held, err := sqlite.NewTypeStore(second).ByKey(t.Context(), book.Key)
	if err != nil || held.RouteWord != "books" {
		t.Errorf("a second handle on the file reads the book type under %q (%v), want the route word books "+
			"the site's registry stored", held.RouteWord, err)
	}
	read, err := sqlite.NewContentStore(second).ByID(t.Context(), stored.ID)
	if err != nil || read.Path != "books/dune" {
		t.Errorf("a second handle on the file reads the book at %q (%v), want books/dune the site's content store "+
			"stored", read.Path, err)
	}
}

// registeredBook returns the book type after storing it through the registry.
func registeredBook(t *testing.T, registry *content.Registry) content.Type {
	t.Helper()
	book, err := content.NewType("book", "Book", "Books", "books")
	if err != nil {
		t.Fatalf("NewType() error = %v, want a valid type", err)
	}
	created, err := registry.Create(t.Context(), book)
	if err != nil {
		t.Fatalf("creating the book type through the site's registry: %v", err)
	}
	return created
}

// storedBook returns the book titled title after storing it and its author through the stores of built.
func storedBook(t *testing.T, built site, book content.Type, title string) content.Content {
	t.Helper()
	author := gouncer.User{
		ID: uuid.Must(uuid.NewV7()), Email: seed.AuthorEmail, Name: seed.AuthorName, PasswordHash: "hash",
		CreatedAt: accountsAdded,
	}
	if err := built.users.CreateUser(t.Context(), author); err != nil {
		t.Fatalf("creating the author through the site's user store: %v", err)
	}
	item, err := content.New(book, nil, title, author.ID)
	if err != nil {
		t.Fatalf("New() error = %v, want a draft book", err)
	}
	stored, err := built.content.Create(t.Context(), item)
	if err != nil {
		t.Fatalf("storing %q through the site's content store: %v", title, err)
	}
	return stored
}

func TestAPostgreSQLProgramOpensNoHandle(t *testing.T) {
	t.Parallel()

	program := Program(testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL}), noPlugins)

	if program.Open != nil {
		t.Error("Open is set on a PostgreSQL address, want nil so the steps keep their address")
	}
}
