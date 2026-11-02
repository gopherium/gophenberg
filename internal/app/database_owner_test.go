// SPDX-License-Identifier: Apache-2.0

package app

import (
	"net/url"
	"strings"
	"testing"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/testdb"
)

// hiddenPassword is the password a test address holds that no answer may repeat.
const hiddenPassword = "hidden-password"

// createOwnerAnswer runs database:create-owner -yes over the two addresses given.
func createOwnerAnswer(t *testing.T, site, superuser string) testkit.Result {
	t.Helper()
	env := map[string]string{"GOPHENBERG_DATABASE_URL": site, "GOPHENBERG_DATABASE_SUPERUSER_URL": superuser}
	return testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "database:create-owner", "-yes")
}

func TestCreateOwnerNamesTheSettingOfAnAddressItCannotRead(t *testing.T) {
	t.Parallel()

	const (
		fine       = "postgres://site:" + hiddenPassword + "@localhost:5432/site"
		bareAt     = "postgres://si@te:" + hiddenPassword + "@localhost:5432/site"
		wrongPort  = "postgres://site:" + hiddenPassword + "@localhost:port/site"
		pooled     = "postgres://site:" + hiddenPassword + "@localhost:5432/site?pool_max_conns=2"
		spaced     = "host=localhost port=notaport user=site password = " + hiddenPassword + " dbname=site"
		split      = "host=localhost port=notaport user=site password=first " + hiddenPassword + " dbname=site"
		unclosed   = "host=localhost user=site password='first " + hiddenPassword + " dbname=site"
		wrongName  = "postgre://site:" + hiddenPassword + ":second@localhost:5432/site"
		repeated   = fine + "?sslmode=disable&sslmode=disable"
		siteKey    = "GOPHENBERG_DATABASE_URL:"
		serverKey  = "GOPHENBERG_DATABASE_SUPERUSER_URL:"
		poolAdvice = "remove pool_max_conns from the address"
		twice      = "the query sets a key twice"
	)
	for name, tc := range map[string]struct {
		site, superuser string
		want            []string
	}{
		"a bare @ in the site's address":                    {bareAt, fine, []string{siteKey}},
		"a bare @ in the superuser's address":               {fine, bareAt, []string{serverKey}},
		"a port pgx cannot read in the site's address":      {wrongPort, fine, []string{siteKey}},
		"a port pgx cannot read in the superuser's address": {fine, wrongPort, []string{serverKey}},
		"a pool setting in the superuser's address":         {fine, pooled, []string{serverKey, poolAdvice}},
		"a spaced password in the site's address":           {spaced, fine, []string{siteKey}},
		"a spaced password in the superuser's address":      {fine, spaced, []string{serverKey}},
		"a split password in the site's address":            {split, fine, []string{siteKey}},
		"a split password in the superuser's address":       {fine, split, []string{serverKey}},
		"an unclosed quote in the site's address":           {unclosed, fine, []string{siteKey}},
		"an unclosed quote in the superuser's address":      {fine, unclosed, []string{serverKey}},
		"a mistyped scheme in the site's address":           {wrongName, fine, []string{siteKey}},
		"a mistyped scheme in the superuser's address":      {fine, wrongName, []string{serverKey}},
		"a key set twice in the site's address":             {repeated, fine, []string{siteKey, twice}},
		"a key set twice in the superuser's address":        {fine, repeated, []string{serverKey, twice}},
	} {
		wantStopped(t, name, createOwnerAnswer(t, tc.site, tc.superuser), tc.want)
	}
}

func TestCreateOwnerStopsOnAddressesThatReachTwoDatabases(t *testing.T) {
	t.Parallel()

	const (
		site  = "postgres://site:" + hiddenPassword + "@localhost:5432"
		super = "postgres://root:" + hiddenPassword + "@localhost:5432"
	)
	for name, tc := range map[string]struct{ site, superuser string }{
		"two database names":              {site + "/one", super + "/two"},
		"no name, so each user's own":     {site, super},
		"a name that is the other user's": {site + "/site", super},
		"two hosts": {
			"postgres://site:" + hiddenPassword + "@one.example.com:5432/site",
			"postgres://root:" + hiddenPassword + "@two.example.com:5432/site",
		},
		"two ports": {
			"postgres://site:" + hiddenPassword + "@localhost:5432/site",
			"postgres://root:" + hiddenPassword + "@localhost:5433/site",
		},
		"two fallback hosts": {
			"postgres://site:" + hiddenPassword + "@127.0.0.1:9,127.0.0.2:9/site",
			"postgres://root:" + hiddenPassword + "@127.0.0.1:9,127.0.0.3:9/site",
		},
		"a fallback host on one side only": {
			"postgres://site:" + hiddenPassword + "@127.0.0.1:9,127.0.0.2:9/site",
			"postgres://root:" + hiddenPassword + "@127.0.0.1:9/site",
		},
	} {
		wantStopped(t, name, createOwnerAnswer(t, tc.site, tc.superuser), []string{
			"GOPHENBERG_DATABASE_URL", "GOPHENBERG_DATABASE_SUPERUSER_URL", "name the same database",
		})
	}
}

func TestCreateOwnerTakesOneHostListWithTwoTLSModes(t *testing.T) {
	t.Parallel()

	const hosts = "@127.0.0.1:9,127.0.0.2:9/site"

	got := createOwnerAnswer(t, "postgres://site:"+hiddenPassword+hosts+"?sslmode=disable",
		"postgres://root:"+hiddenPassword+hosts)

	if got.Code != 1 || !strings.Contains(got.Stderr, "dbkit: begin the hand-over: ") {
		t.Errorf("code = %d, stderr = %q, want both addresses taken and the servers found down", got.Code, got.Stderr)
	}
}

// wantStopped fails t unless got exited with code 1, its error carrying each of want and no password.
func wantStopped(t *testing.T, name string, got testkit.Result, want []string) {
	t.Helper()
	if got.Code != 1 {
		t.Errorf("%s: exit code = %d, want 1, stderr %q", name, got.Code, got.Stderr)
	}
	if strings.Contains(got.Stderr, hiddenPassword) {
		t.Errorf("%s: stderr = %q, want no password in it", name, got.Stderr)
	}
	for _, text := range want {
		if !strings.Contains(got.Stderr, text) {
			t.Errorf("%s: stderr = %q, want it to carry %q", name, got.Stderr, text)
		}
	}
}

func TestCreateOwnerNamesTheMissingSiteAddress(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_DATABASE_SUPERUSER_URL": unreachableDatabaseURL}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "database:create-owner", "-yes")

	if got.Code != 1 || !strings.Contains(got.Stderr, "GOPHENBERG_DATABASE_URL") {
		t.Errorf("code = %d, stderr = %q, want the missing address named", got.Code, got.Stderr)
	}
}

func TestCreateOwnerReportsASuperuserItCannotReach(t *testing.T) {
	t.Parallel()

	got := createOwnerAnswer(t, unreachableDatabaseURL, unreachableDatabaseURL)

	if got.Code != 1 || !strings.Contains(got.Stderr, "dbkit: begin the hand-over: ") {
		t.Errorf("code = %d, stderr = %q, want the failed hand-over reported", got.Code, got.Stderr)
	}
	if strings.Contains(got.Stderr, "GOPHENBERG_DATABASE") {
		t.Errorf("stderr = %q, want no setting blamed for a server that is down", got.Stderr)
	}
}

func TestCreateOwnerNamesTheSiteAddressForARoleNameTheServerCannotKeep(t *testing.T) {
	t.Parallel()

	superuser := testdb.AsServer(t, emptyDatabaseURL(t))
	site, err := url.Parse(superuser)
	if err != nil {
		t.Fatalf("read the test address: %v", err)
	}
	site.User = url.UserPassword(strings.Repeat("r", 64), hiddenPassword)

	got := createOwnerAnswer(t, site.String(), superuser)

	want := "GOPHENBERG_DATABASE_URL: " + dbkitpg.ErrRoleName.Error()
	if got.Code != 1 || !strings.Contains(got.Stderr, want) {
		t.Errorf("code = %d, stderr = %q, want %q", got.Code, got.Stderr, want)
	}
}
