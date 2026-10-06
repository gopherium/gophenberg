// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"math"
	"strconv"
	"testing"
	"time"
)

// settingsOf returns the settings under the program prefix over values.
func settingsOf(values map[string]string) Env {
	return Env{Prefix: "GOPHENBERG_", Getenv: func(key string) string { return values[key] }}
}

func TestEnvReadsAPluginSettingUnderTheProgramPrefix(t *testing.T) {
	t.Parallel()

	env := settingsOf(map[string]string{"GOPHENBERG_FEED_ITEMS": " 7 "})

	items, err := env.Within("FEED_").Count("ITEMS", 20)

	if err != nil || items != 7 {
		t.Errorf("Count(ITEMS) = %d, %v, want 7 read trimmed under the program prefix", items, err)
	}
}

func TestEnvNamesASettingUnderItsPrefix(t *testing.T) {
	t.Parallel()

	if got := settingsOf(nil).Within("FEED_").Key("ITEMS"); got != "GOPHENBERG_FEED_ITEMS" {
		t.Errorf("Key(ITEMS) = %q, want the full name under both prefixes", got)
	}
}

func TestEnvValueTrimsTheSetting(t *testing.T) {
	t.Parallel()

	env := settingsOf(map[string]string{"GOPHENBERG_FEED_TITLE": "  Field Notes \t"})

	if got := env.Within("FEED_").Value("TITLE"); got != "Field Notes" {
		t.Errorf("Value(TITLE) = %q, want it trimmed", got)
	}
}

func TestEnvWithNoReaderReadsNothing(t *testing.T) {
	t.Parallel()

	env := Env{Prefix: "GOPHENBERG_"}

	items, err := env.Count("ITEMS", 20)

	if env.Value("TITLE") != "" || err != nil || items != 20 {
		t.Errorf("an Env with no reader reads %q and %d, %v, want nothing and the fallback", env.Value("TITLE"), items, err)
	}
}

func TestEnvCountTakesTheFallbackWhenTheSettingIsEmpty(t *testing.T) {
	t.Parallel()

	items, err := settingsOf(map[string]string{"GOPHENBERG_ITEMS": "   "}).Count("ITEMS", 20)

	if err != nil || items != 20 {
		t.Errorf("Count(ITEMS) = %d, %v, want the fallback 20", items, err)
	}
}

func TestEnvCountRefusesWhatIsNoCount(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		value string
		want  string
	}{
		"a word":           {"banana", `GOPHENBERG_ITEMS: must be a whole number, got "banana"`},
		"a fraction":       {"2.5", `GOPHENBERG_ITEMS: must be a whole number, got "2.5"`},
		"zero":             {"0", `GOPHENBERG_ITEMS: must stand above zero, got "0"`},
		"a negative count": {"-3", `GOPHENBERG_ITEMS: must stand above zero, got "-3"`},
		"past the largest": {
			"99999999999999999999",
			`GOPHENBERG_ITEMS: must stand at or below ` + strconv.Itoa(math.MaxInt) + `, got "99999999999999999999"`,
		},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			items, err := settingsOf(map[string]string{"GOPHENBERG_ITEMS": tc.value}).Count("ITEMS", 20)

			if err == nil || err.Error() != tc.want || items != 0 {
				t.Errorf("Count(ITEMS) = %d, %v, want 0 and %q", items, err, tc.want)
			}
		})
	}
}

func TestEnvRequiredRefusesAnEmptySetting(t *testing.T) {
	t.Parallel()

	env := settingsOf(map[string]string{"GOPHENBERG_FEED_URL": " https://example.com/feed "})

	held, err := env.Within("FEED_").Required("URL")
	_, missing := env.Within("FEED_").Required("TITLE")

	if err != nil || held != "https://example.com/feed" {
		t.Errorf("Required(URL) = %q, %v, want the trimmed value", held, err)
	}
	if missing == nil || missing.Error() != "GOPHENBERG_FEED_TITLE is required" {
		t.Errorf("Required(TITLE) error = %v, want the empty setting named", missing)
	}
}

func TestEnvDurationReadsADurationAboveZero(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		value string
		want  time.Duration
		err   string
	}{
		"empty":        {"", 5 * time.Second, ""},
		"a duration":   {"90s", 90 * time.Second, ""},
		"a word":       {"soon", 0, `GOPHENBERG_GRACE: must be a duration like 30s, got "soon"`},
		"a bare count": {"30", 0, `GOPHENBERG_GRACE: must be a duration like 30s, got "30"`},
		"zero":         {"0s", 0, `GOPHENBERG_GRACE: must stand above zero, got "0s"`},
		"negative":     {"-1m", 0, `GOPHENBERG_GRACE: must stand above zero, got "-1m"`},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			held, err := settingsOf(map[string]string{"GOPHENBERG_GRACE": tc.value}).Duration("GRACE", 5*time.Second)

			if held != tc.want || errorText(err) != tc.err {
				t.Errorf("Duration(GRACE) = %v, %v, want %v and %q", held, err, tc.want, tc.err)
			}
		})
	}
}

func TestEnvFlagReadsTrueOrFalse(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		value string
		want  bool
		err   string
	}{
		"empty": {"", true, ""},
		"false": {"false", false, ""},
		"zero":  {"0", false, ""},
		"true":  {"TRUE", true, ""},
		"maybe": {"maybe", false, `GOPHENBERG_PUBLIC: must be true or false, got "maybe"`},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			held, err := settingsOf(map[string]string{"GOPHENBERG_PUBLIC": tc.value}).Flag("PUBLIC", true)

			if held != tc.want || errorText(err) != tc.err {
				t.Errorf("Flag(PUBLIC) = %t, %v, want %t and %q", held, err, tc.want, tc.err)
			}
		})
	}
}

// errorText returns the text of err, empty when it is nil.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestEnvCountTakesTheLargestCount(t *testing.T) {
	t.Parallel()

	items, err := settingsOf(map[string]string{"GOPHENBERG_ITEMS": strconv.Itoa(math.MaxInt)}).Count("ITEMS", 20)

	if err != nil || items != math.MaxInt {
		t.Errorf("Count(ITEMS) = %d, %v, want the largest whole number taken", items, err)
	}
}
