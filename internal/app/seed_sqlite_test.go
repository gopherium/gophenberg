// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"io"
	"maps"
	"reflect"
	"strings"
	"testing"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/seed"
)

func TestSeedStoresTheDemoDataOnTheSQLiteSiteTheRunComposed(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	composed := composedSQLiteSite(t, path)
	var first, again strings.Builder

	firstErr := composed.seed(t.Context(), seedCall(path, &first))
	againErr := composed.seed(t.Context(), seedCall(path, &again))

	if firstErr != nil || againErr != nil {
		t.Fatalf("seed() = %v, then %v, want the demo data stored twice on the SQLite site", firstErr, againErr)
	}
	if want := seedLines("created"); first.String() != want {
		t.Errorf("the first seed printed %q, want %q", first.String(), want)
	}
	if want := seedLines("kept"); again.String() != want {
		t.Errorf("the second seed printed %q, want %q", again.String(), want)
	}
	read := sqliteStores(composeConfig{handle: openSQLiteFile(t, path), fieldDepth: content.DefaultFieldDepth})
	wantDemoPosts(t, read.content)
	wantDemoPages(t, read.content)
	wantDemoTypes(t, read.types)
}

// composedSQLiteSite returns what a run's plugin registration keeps for the seed over the migrated SQLite file at path.
func composedSQLiteSite(t *testing.T, path string) *sitePool {
	t.Helper()
	composed := &sitePool{}
	call := gonsole.Call{Env: settingsEnv(migratingSQLite(path))}.WithDB(openSQLiteFile(t, path))
	loaded, err := loadPlugins(binary{plugins: noPlugins}, goncierge.New(), composed)(t.Context(), call)
	if err != nil {
		t.Fatalf("loadPlugins() error = %v, want the SQLite site composed", err)
	}
	t.Cleanup(func() { _ = loaded.Release(context.WithoutCancel(t.Context())) })
	return composed
}

// seedCall returns the call of a seed over the SQLite file at path that prints to stdout.
func seedCall(path string, stdout io.Writer) gonsole.Call {
	return gonsole.Call{Env: settingsEnv(migratingSQLite(path)), Stdout: stdout}
}

// seedLines returns what one seed prints when it reports every demo account as verb, created or kept.
func seedLines(verb string) string {
	var lines strings.Builder
	for _, email := range []string{seed.AdminEmail, seed.EditorEmail, seed.AuthorEmail, seed.MemberEmail} {
		lines.WriteString(verb + " " + email + "\n")
	}
	return lines.String() + "seeded demo data, an account created above signs in with " + seed.AdminPassword +
		", a kept account keeps its own password\n"
}

// wantDemoPosts checks that store holds as many demo posts in each status as one seed stores.
func wantDemoPosts(t *testing.T, store content.Store) {
	t.Helper()
	for status, want := range seed.DemoStatuses() {
		_, total, err := store.List(t.Context(), content.Filter{
			Type: content.TypePost, Statuses: []content.Status{status}, Page: 1, PerPage: 1,
		})
		if err != nil || total != want {
			t.Errorf("the file holds %d %s posts (%v), want the %d one seed stores", total, status, err, want)
		}
	}
}

// wantDemoPages checks that store serves each published demo page at its nested address.
func wantDemoPages(t *testing.T, store content.Store) {
	t.Helper()
	for path, title := range map[string]string{
		"pages/about": "About", "pages/about/team": "Team", "pages/about/team/volunteers": "Volunteers",
	} {
		page, err := store.PublishedByPath(t.Context(), path)
		if err != nil || page.Title != title {
			t.Errorf("%s serves %q (%v), want the demo page %q", path, page.Title, err, title)
		}
	}
}

// wantDemoTypes checks that types holds each demo type under its description and each demo group once.
func wantDemoTypes(t *testing.T, types content.TypeStore) {
	t.Helper()
	listed, err := types.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v, want the demo types", err)
	}
	if described, want := descriptionsOf(listed), demoDescriptions(); !maps.Equal(described, want) {
		t.Errorf("descriptions = %v, want %v", described, want)
	}
	groups, err := types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want the demo groups", err)
	}
	if got, want := groupShapesOf(groups), demoGroupShapes(); !reflect.DeepEqual(got, want) {
		t.Errorf("stored groups = %+v, want %+v", got, want)
	}
}
