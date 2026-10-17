// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/themehost"
	"github.com/gopherium/gophenberg/internal/version"
)

func TestAboutPrintsTheBuildFactsAsOneDocument(t *testing.T) {
	t.Parallel()

	got := testkit.Run(t, Program(testkit.Getenv(map[string]string{}), noPlugins), "", "about", "-json")

	if got.Code != 0 {
		t.Fatalf("about -json exited with %d and stderr %q, want 0", got.Code, got.Stderr)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(got.Stdout), &fields); err != nil {
		t.Fatalf("about -json printed %q, want one JSON document: %v", got.Stdout, err)
	}
	var facts buildFacts
	if err := json.Unmarshal([]byte(got.Stdout), &facts); err != nil {
		t.Fatalf("reading the build facts: %v", err)
	}
	want := buildFacts{Schema: 1, Name: "gophenberg", Version: version.Version(), Kits: themehost.ServedKits()}
	if len(fields) != 6 || !facts.equal(want) {
		t.Errorf("about -json = %s, want exactly schema, name, version, commit, stamp and kits as %+v", got.Stdout, want)
	}
}

func TestAboutPrintsOneLineForPeople(t *testing.T) {
	t.Parallel()

	got := testkit.Run(t, Program(testkit.Getenv(map[string]string{}), noPlugins), "", "about")

	kits := themehost.ServedKits()
	want := "Gophenberg " + version.Version() + ", custom build, theme kits " + kits[0] + " to " + kits[len(kits)-1] + "\n"
	if got.Code != 0 || got.Stdout != want {
		t.Errorf("about = %d, %q, %q, want 0 and %q", got.Code, got.Stdout, got.Stderr, want)
	}
}

func TestAboutSummaryNamesTheStampAndTheShortCommit(t *testing.T) {
	t.Parallel()

	facts := buildFacts{
		Name: "gophenberg", Version: "0.20.0", Stamp: "official",
		Commit: "4c3ba6000f6e3b3a1a9d2c8f7e5b4a3c2d1e0f9a", Kits: []string{"0.9.0", "0.15.1"},
	}

	want := "Gophenberg 0.20.0, official build, commit 4c3ba60, theme kits 0.9.0 to 0.15.1"
	if got := facts.summary(); got != want {
		t.Errorf("summary() = %q, want %q", got, want)
	}
}

func TestAboutNeedsNoDatabase(t *testing.T) {
	t.Parallel()

	for name, address := range map[string]string{
		"no database":         "",
		"a PostgreSQL server": unreachableDatabaseURL,
		"a SQLite file":       "sqlite:/srv/gophenberg/missing.db",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			env := map[string]string{"GOPHENBERG_DATABASE_URL": address}
			got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "about", "-json")

			if got.Code != 0 {
				t.Errorf("about -json beside %s exited with %d and stderr %q, want 0", name, got.Code, got.Stderr)
			}
		})
	}
}

// equal reports whether two sets of build facts hold the same values.
func (f buildFacts) equal(other buildFacts) bool {
	return f.Schema == other.Schema && f.Name == other.Name && f.Version == other.Version &&
		f.Commit == other.Commit && f.Stamp == other.Stamp && slices.Equal(f.Kits, other.Kits)
}
