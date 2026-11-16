// SPDX-License-Identifier: Apache-2.0

package pluginsettings_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit/settings"
	"github.com/gopherium/framework/pluginkit/wire"

	"github.com/gopherium/gophenberg/internal/pluginsettings"
)

// knownSecret is a secret value no log line, rejection or print may hold.
const knownSecret = "key-for-example-184467235"

// bound returns n as a manifest bound.
func bound(n float64) *float64 {
	return &n
}

// notesDeclared returns one setting of each type, as the plugin notes declares them.
func notesDeclared() []wire.Setting {
	return []wire.Setting{
		{Key: "title", Type: settings.TypeText, Default: "Notes"},
		{Key: "per_page", Type: settings.TypeCount, Default: "10", Min: bound(1), Max: bound(50)},
		{Key: "open", Type: settings.TypeFlag, Default: "false"},
		{Key: "keep_for", Type: settings.TypeDuration, Default: "720h"},
		{Key: "order", Type: settings.TypeChoice, Default: "newest", Options: []string{"newest", "oldest"}},
		{Key: "tags", Type: settings.TypeLines, Default: "news"},
		{Key: "api_key", Type: settings.TypeSecret},
	}
}

// setOver returns the set of declared over env, logging into the returned buffer.
func setOver(declared map[string][]wire.Setting, env map[string]string) (*pluginsettings.Set, *bytes.Buffer) {
	logs := &bytes.Buffer{}
	set := pluginsettings.New(declared, testkit.Getenv(env), slog.New(slog.NewTextHandler(logs, nil)))
	return set, logs
}

// notesSet returns the set of the plugin notes and of gallery, which declares none, over env.
func notesSet(env map[string]string) (*pluginsettings.Set, *bytes.Buffer) {
	return setOver(map[string][]wire.Setting{"notes": notesDeclared(), "gallery": nil}, env)
}

// linesHolding returns how many lines of logs hold text.
func linesHolding(logs *bytes.Buffer, text string) int {
	count := 0
	for line := range strings.Lines(logs.String()) {
		if strings.Contains(line, text) {
			count++
		}
	}
	return count
}

// errorText returns the text of err, empty when it is nil.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestAPluginWithNoSettingsGetsAReaderWithNoKeys(t *testing.T) {
	t.Parallel()

	set, logs := notesSet(nil)

	for _, id := range []string{"gallery", "unlisted"} {
		reader := set.For(id)
		if reader == nil {
			t.Fatalf("For(%q) is nil, want a reader with no keys", id)
		}
		if title := reader.Text("title"); title != "" {
			t.Errorf("%s reads %q for title, want nothing from a plugin that declares no settings", id, title)
		}
		if err := set.Rejection(id); err != nil {
			t.Errorf("Rejection(%q) = %v, want nil for a plugin that declares no settings", id, err)
		}
	}
	if lines := linesHolding(logs, `msg="plugin read an undeclared setting" plugin=gallery key=title`); lines != 1 {
		t.Errorf("the log holds %d undeclared reads by gallery, want 1:\n%s", lines, logs)
	}
}

func TestAValueItsTypeRejectsIsThePluginsRejection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		variable string
		value    string
		want     string
	}{
		{"GOPHENBERG_PLUGIN_NOTES__PER_PAGE", "banana",
			`GOPHENBERG_PLUGIN_NOTES__PER_PAGE: must be a whole number, got "banana"`},
		{"GOPHENBERG_PLUGIN_NOTES__PER_PAGE", "60",
			`GOPHENBERG_PLUGIN_NOTES__PER_PAGE: must stand at or below 50, got "60"`},
		{"GOPHENBERG_PLUGIN_NOTES__PER_PAGE", "0",
			`GOPHENBERG_PLUGIN_NOTES__PER_PAGE: must stand at or above 1, got "0"`},
		{"GOPHENBERG_PLUGIN_NOTES__KEEP_FOR", "soon",
			`GOPHENBERG_PLUGIN_NOTES__KEEP_FOR: must be a duration like 30s, got "soon"`},
		{"GOPHENBERG_PLUGIN_NOTES__OPEN", "maybe",
			`GOPHENBERG_PLUGIN_NOTES__OPEN: must be true or false, got "maybe"`},
		{"GOPHENBERG_PLUGIN_NOTES__ORDER", "random",
			`GOPHENBERG_PLUGIN_NOTES__ORDER: must be one of newest, oldest, got "random"`},
	}
	for _, c := range cases {
		t.Run(c.variable+"="+c.value, func(t *testing.T) {
			t.Parallel()

			set, _ := setOver(map[string][]wire.Setting{
				"notes": notesDeclared(),
				"feed":  {{Key: "title", Type: settings.TypeText, Default: "Gophenberg"}},
			}, map[string]string{c.variable: c.value})

			if got := errorText(set.Rejection("notes")); got != c.want {
				t.Errorf("notes is rejected with %q, want %q", got, c.want)
			}
			if err := set.Rejection("feed"); err != nil {
				t.Errorf("feed is rejected with %v, want its settings resolved beside the rejected notes", err)
			}
			if title := set.For("feed").Text("title"); title != "Gophenberg" {
				t.Errorf("feed reads %q for title, want its default beside the rejected notes", title)
			}
		})
	}
}

func TestADeclarationOutsideTheRulesIsThePluginsRejection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		id      string
		setting wire.Setting
		want    string
	}{
		{"an unknown type", "drafts", wire.Setting{Key: "mood", Type: "colour"},
			`settings[0] mood: type "colour" is not one of text, count, flag, duration, choice, secret, lines`},
		{"a key starting with job_", "drafts", wire.Setting{Key: "job_every", Type: settings.TypeText},
			"settings[0] job_every: a key must not start with job_ or db_"},
		{"a key starting with db_", "drafts", wire.Setting{Key: "db_conns", Type: settings.TypeText},
			"settings[0] db_conns: a key must not start with job_ or db_"},
		{"a count with no default", "drafts", wire.Setting{Key: "max_seats", Type: settings.TypeCount},
			"settings[0] max_seats: a count needs a default"},
		{"a secret with a default", "drafts",
			wire.Setting{Key: "api_key", Type: settings.TypeSecret, Default: knownSecret},
			"settings[0] api_key: a secret takes no default, because plugin.json is public"},
		{"an id holding two hyphens in a row", "drafts--old", wire.Setting{Key: "title", Type: settings.TypeText},
			"the id drafts--old must not hold two hyphens in a row or end with one when the plugin has settings"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			set, _ := setOver(map[string][]wire.Setting{c.id: {c.setting}, "notes": notesDeclared()}, nil)

			if got := errorText(set.Rejection(c.id)); got != c.want {
				t.Errorf("%s is rejected with %q, want %q", c.id, got, c.want)
			}
			if err := set.Rejection("notes"); err != nil {
				t.Errorf("notes is rejected with %v, want its settings resolved beside the rejected %s", err, c.id)
			}
		})
	}
}

func TestARejectedPluginGetsAReaderWithNoKeys(t *testing.T) {
	t.Parallel()

	set, _ := notesSet(map[string]string{"GOPHENBERG_PLUGIN_NOTES__PER_PAGE": "banana"})

	reader := set.For("notes")
	if reader == nil {
		t.Fatal("For(notes) is nil, want a reader with no keys for a rejected plugin")
	}
	if title := reader.Text("title"); title != "" {
		t.Errorf("the rejected notes reads %q for title, want nothing", title)
	}
}

func TestAKnownSecretReachesNoLogLineRejectionOrPrint(t *testing.T) {
	t.Parallel()

	set, logs := setOver(map[string][]wire.Setting{
		"notes":  notesDeclared(),
		"drafts": {{Key: "api_key", Type: settings.TypeSecret}},
	}, map[string]string{
		"GOPHENBERG_PLUGIN_NOTES__API_KEY":  knownSecret,
		"GOPHENBERG_PLUGIN_NOTES__PER_PAGE": "banana",
		"GOPHENBERG_PLUGIN_DRAFTS__API_KEY": knownSecret,
	})
	drafts := set.For("drafts")
	misreads := fmt.Sprint(drafts.Text("api_key"), drafts.Count("api_key"), drafts.Lines("api_key"))

	if secret := drafts.Secret("api_key"); secret != knownSecret {
		t.Fatalf("drafts reads %q for api_key, want the secret its variable holds", secret)
	}
	traces := map[string]string{
		"the log":           logs.String(),
		"the rejection":     errorText(set.Rejection("notes")),
		"the misreads":      misreads,
		"%v of the set":     fmt.Sprintf("%v", set),
		"%+v of the set":    fmt.Sprintf("%+v", set),
		"%#v of the set":    fmt.Sprintf("%#v", set),
		"%+v of the reader": fmt.Sprintf("%+v", drafts),
		"%#v of the reader": fmt.Sprintf("%#v", drafts),
	}
	for where, text := range traces {
		if strings.Contains(text, knownSecret) {
			t.Errorf("%s holds the secret: %s", where, text)
		}
	}
}

// countRead reads ITEMS as a count of at most 50, as a manifest count with max 50 reads it.
func countRead(env gonsole.Env) error {
	_, err := env.Count("ITEMS", 20, gonsole.AtMost(50))
	return err
}

// durationRead reads ITEMS as a duration.
func durationRead(env gonsole.Env) error {
	_, err := env.Duration("ITEMS", 0)
	return err
}

// flagRead reads ITEMS as true or false.
func flagRead(env gonsole.Env) error {
	_, err := env.Flag("ITEMS", false)
	return err
}

func TestParseTextsMatchTheCommandLine(t *testing.T) {
	t.Parallel()

	count := wire.Setting{Key: "items", Type: settings.TypeCount, Default: "20", Max: bound(50)}
	duration := wire.Setting{Key: "items", Type: settings.TypeDuration, Default: "1m"}
	flag := wire.Setting{Key: "items", Type: settings.TypeFlag}
	cases := []struct {
		name    string
		setting wire.Setting
		value   string
		read    func(gonsole.Env) error
	}{
		{"a word for a number", count, "banana", countRead},
		{"a fraction", count, "2.5", countRead},
		{"a number past the max", count, "60", countRead},
		{"a number past every whole number", count, "99999999999999999999", countRead},
		{"a bad duration", duration, "soon", durationRead},
		{"a zero duration", duration, "0s", durationRead},
		{"a negative duration", duration, "-1m", durationRead},
		{"a bad flag", flag, "maybe", flagRead},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			getenv := testkit.Getenv(map[string]string{"GOPHENBERG_PLUGIN_FEED__ITEMS": c.value})

			lent := errorText(pluginsettings.New(map[string][]wire.Setting{"feed": {c.setting}}, getenv, nil).
				Rejection("feed"))
			wanted := errorText(c.read(gonsole.Env{Prefix: "GOPHENBERG_PLUGIN_FEED__", Getenv: getenv}))

			if lent == "" {
				t.Fatalf("plugins take %q, want it rejected as the command line rejects it with %q", c.value, wanted)
			}
			if lent != wanted {
				t.Errorf("plugins reject %q with %q, want %q as the command line words it", c.value, lent, wanted)
			}
		})
	}
}
