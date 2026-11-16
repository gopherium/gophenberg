// SPDX-License-Identifier: Apache-2.0

package pluginsettings_test

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit/settings"
	"github.com/gopherium/framework/pluginkit/wire"

	"github.com/gopherium/gophenberg/internal/pluginsettings"
	"github.com/gopherium/gophenberg/sdk"
)

// readings returns what each reader answers for the notes setting of its type.
func readings(notes sdk.Settings) []string {
	return []string{
		notes.Text("title"),
		strconv.Itoa(notes.Count("per_page")),
		strconv.FormatBool(notes.Flag("open")),
		notes.Duration("keep_for").String(),
		notes.Text("order"),
		fmt.Sprintf("%q", notes.Lines("tags")),
		notes.Secret("api_key"),
	}
}

func TestEachReaderAnswersTheDefaultWhenTheEnvironmentSetsNone(t *testing.T) {
	t.Parallel()

	set, logs := notesSet(nil)

	got := readings(set.For("notes"))

	want := []string{"Notes", "10", "false", "720h0m0s", "newest", `["news"]`, ""}
	if !slices.Equal(got, want) {
		t.Errorf("notes reads %q, want the defaults %q", got, want)
	}
	if logs.Len() != 0 {
		t.Errorf("the log holds %q, want no line for reads that match the manifest", logs)
	}
}

func TestAValueInTheEnvironmentWinsOverTheDefault(t *testing.T) {
	t.Parallel()

	set, _ := notesSet(map[string]string{
		"GOPHENBERG_PLUGIN_NOTES__TITLE":    "  Field Notes  ",
		"GOPHENBERG_PLUGIN_NOTES__PER_PAGE": "25",
		"GOPHENBERG_PLUGIN_NOTES__OPEN":     "true",
		"GOPHENBERG_PLUGIN_NOTES__KEEP_FOR": "48h",
		"GOPHENBERG_PLUGIN_NOTES__ORDER":    "oldest",
		"GOPHENBERG_PLUGIN_NOTES__TAGS":     "news,, events",
		"GOPHENBERG_PLUGIN_NOTES__API_KEY":  " " + knownSecret + " ",
	})

	got := readings(set.For("notes"))

	want := []string{"Field Notes", "25", "true", "48h0m0s", "oldest", `["news" "events"]`, " " + knownSecret + " "}
	if !slices.Equal(got, want) {
		t.Errorf("notes reads %q, want the environment's values %q", got, want)
	}
}

func TestAPluginWhoseIDHoldsAHyphenReadsUnderUnderscores(t *testing.T) {
	t.Parallel()

	set, _ := setOver(map[string][]wire.Setting{
		"notes-pro": {{Key: "per_page", Type: settings.TypeCount, Default: "10"}},
	}, map[string]string{"GOPHENBERG_PLUGIN_NOTES_PRO__PER_PAGE": "30"})

	if got := set.For("notes-pro").Count("per_page"); got != 30 {
		t.Errorf("notes-pro reads %d for per_page, want 30 from GOPHENBERG_PLUGIN_NOTES_PRO__PER_PAGE", got)
	}
}

func TestABlankVariableLeavesTheDefaultInPlace(t *testing.T) {
	t.Parallel()

	for _, blank := range []string{"", "   "} {
		set, _ := notesSet(map[string]string{"GOPHENBERG_PLUGIN_NOTES__TITLE": blank})

		if got := set.For("notes").Text("title"); got != "Notes" {
			t.Errorf("notes reads %q for title under %q, want the default Notes", got, blank)
		}
	}
}

func TestLinesAnswersANewSliceOnEveryCall(t *testing.T) {
	t.Parallel()

	set, _ := notesSet(map[string]string{"GOPHENBERG_PLUGIN_NOTES__TAGS": "news, events"})
	notes := set.For("notes")

	first := notes.Lines("tags")
	if len(first) == 0 {
		t.Fatal("tags reads no items, want news and events")
	}
	first[0] = "changed"

	if second := notes.Lines("tags"); !slices.Equal(second, []string{"news", "events"}) {
		t.Errorf("the second read is %q after the first was changed, want [news events]", second)
	}
}

func TestAnUndeclaredKeyReadsTheZeroValueAndLogsOnce(t *testing.T) {
	t.Parallel()

	set, logs := notesSet(nil)

	for range 2 {
		notes := set.For("notes")
		got := fmt.Sprint([]any{notes.Text("per_pages"), notes.Count("per_pages"), notes.Flag("per_pages"),
			notes.Duration("per_pages"), notes.Lines("per_pages"), notes.Secret("per_pages")})
		if want := fmt.Sprint([]any{"", 0, false, time.Duration(0), []string(nil), ""}); got != want {
			t.Errorf("an undeclared key reads %q, want the zero values %q", got, want)
		}
	}
	line := `level=ERROR msg="plugin read an undeclared setting" plugin=notes key=per_pages`
	if lines := linesHolding(logs, line); lines != 1 {
		t.Errorf("the log holds %d lines %q, want 1:\n%s", lines, line, logs)
	}
}

func TestAReadWithAnotherTypesReaderReadsTheZeroValueAndLogsOnce(t *testing.T) {
	t.Parallel()

	cases := []struct {
		key  string
		read func(sdk.Settings, string) any
		want any
		line string
	}{
		{"title", func(s sdk.Settings, key string) any { return s.Count(key) }, 0, "key=title type=text read=count"},
		{"per_page", func(s sdk.Settings, key string) any { return s.Flag(key) }, false,
			"key=per_page type=count read=flag"},
		{"open", func(s sdk.Settings, key string) any { return s.Duration(key) }, "0s",
			"key=open type=flag read=duration"},
		{"keep_for", func(s sdk.Settings, key string) any { return s.Lines(key) }, "[]",
			"key=keep_for type=duration read=lines"},
		{"order", func(s sdk.Settings, key string) any { return s.Secret(key) }, "",
			"key=order type=choice read=secret"},
		{"tags", func(s sdk.Settings, key string) any { return s.Text(key) }, "", "key=tags type=lines read=text"},
		{"title", func(s sdk.Settings, key string) any { return s.Secret(key) }, "", "key=title type=text read=secret"},
		{"api_key", func(s sdk.Settings, key string) any { return s.Count(key) }, 0,
			"key=api_key type=secret read=count"},
	}
	for _, c := range cases {
		t.Run(c.line, func(t *testing.T) {
			t.Parallel()

			set, logs := notesSet(map[string]string{"GOPHENBERG_PLUGIN_NOTES__API_KEY": knownSecret})

			for range 2 {
				if got := fmt.Sprint(c.read(set.For("notes"), c.key)); got != fmt.Sprint(c.want) {
					t.Errorf("the read answers %q, want the zero value %q", got, fmt.Sprint(c.want))
				}
			}
			line := `level=ERROR msg="plugin read a setting as another type" plugin=notes ` + c.line
			if lines := linesHolding(logs, line); lines != 1 {
				t.Errorf("the log holds %d lines %q, want 1:\n%s", lines, line, logs)
			}
		})
	}
}

func TestTextOnASecretAnswersEmptyAndTheLogHoldsNoValue(t *testing.T) {
	t.Parallel()

	set, logs := notesSet(map[string]string{"GOPHENBERG_PLUGIN_NOTES__API_KEY": knownSecret})

	if got := set.For("notes").Text("api_key"); got != "" {
		t.Errorf("Text on a secret answers %q, want nothing", got)
	}
	line := `msg="plugin read a setting as another type" plugin=notes key=api_key type=secret read=text`
	if lines := linesHolding(logs, line); lines != 1 {
		t.Errorf("the log holds %d lines %q, want 1:\n%s", lines, line, logs)
	}
	if strings.Contains(logs.String(), knownSecret) {
		t.Errorf("the log holds the secret:\n%s", logs)
	}
}

func TestEachPluginLogsItsOwnMisreads(t *testing.T) {
	t.Parallel()

	set, logs := notesSet(nil)

	set.For("notes").Count("per_pages")
	set.For("gallery").Count("per_pages")

	for _, id := range []string{"notes", "gallery"} {
		line := `msg="plugin read an undeclared setting" plugin=` + id + " key=per_pages"
		if lines := linesHolding(logs, line); lines != 1 {
			t.Errorf("the log holds %d lines %q, want 1:\n%s", lines, line, logs)
		}
	}
}

func TestMisreadsFromManyGoroutinesLogOnce(t *testing.T) {
	t.Parallel()

	set, logs := notesSet(nil)

	var reads sync.WaitGroup
	for range 8 {
		reads.Go(func() { set.For("notes").Count("per_pages") })
	}
	reads.Wait()

	if lines := linesHolding(logs, `msg="plugin read an undeclared setting"`); lines != 1 {
		t.Errorf("the log holds %d undeclared reads from 8 goroutines, want 1:\n%s", lines, logs)
	}
}

func TestCoreAnswersTheTrustedProxiesReadOnceAndTrimmed(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_TRUSTED_PROXIES": " 192.0.2.0/24 "}
	set := pluginsettings.New(map[string][]wire.Setting{"notes": notesDeclared()}, testkit.Getenv(env), nil)
	env["GOPHENBERG_TRUSTED_PROXIES"] = "198.51.100.0/24"

	for _, id := range []string{"notes", "gallery"} {
		if got := set.For(id).Core("TRUSTED_PROXIES"); got != "192.0.2.0/24" {
			t.Errorf("%s reads %q for TRUSTED_PROXIES, want 192.0.2.0/24 as the site read it at start", id, got)
		}
	}
}

func TestACoreSettingOffTheListAnswersEmptyAndLogsOnce(t *testing.T) {
	t.Parallel()

	set, logs := notesSet(map[string]string{"GOPHENBERG_DATABASE_URL": "postgres://example.com/site"})

	for range 2 {
		if got := set.For("notes").Core("DATABASE_URL"); got != "" {
			t.Errorf("notes reads %q for DATABASE_URL, want nothing for a core setting off the list", got)
		}
	}
	line := `level=ERROR msg="plugin read a core setting off the list" plugin=notes name=DATABASE_URL`
	if lines := linesHolding(logs, line); lines != 1 {
		t.Errorf("the log holds %d lines %q, want 1:\n%s", lines, line, logs)
	}
	if strings.Contains(logs.String(), "example.com") {
		t.Errorf("the log holds the value of a core setting off the list:\n%s", logs)
	}
}

func TestMisreadsUnderANilLoggerAnswerTheZeroValues(t *testing.T) {
	t.Parallel()

	set := pluginsettings.New(map[string][]wire.Setting{"notes": notesDeclared()}, testkit.Getenv(nil), nil)
	notes := set.For("notes")

	got := fmt.Sprint([]any{notes.Count("per_pages"), notes.Count("title"), notes.Core("DATABASE_URL")})

	if got != "[0 0 ]" {
		t.Errorf("misreads under a nil logger answer %q, want the zero values", got)
	}
}
