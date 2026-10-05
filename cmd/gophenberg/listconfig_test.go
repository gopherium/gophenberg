// SPDX-License-Identifier: Apache-2.0

package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/server"
)

// listsRead returns the list settings loadRunConfig reads beside a database address.
func listsRead(t *testing.T, env map[string]string) (server.ListSettings, error) {
	t.Helper()
	env["GOPHENBERG_DATABASE_URL"] = unreachableDatabaseURL
	settings, err := loadRunConfig(testkit.Getenv(env))
	return settings.lists, err
}

func TestListSettingsTakeTheDefaultsWhenTheEnvironmentNamesNone(t *testing.T) {
	t.Parallel()

	held, err := listsRead(t, map[string]string{})

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	want := server.ListSettings{
		PageSizes: []int{10, 20, 50, 100}, PageSize: 20, PageCap: 100,
		ToastDuration: 6 * time.Second, ToastNameLength: 45, FormatLocale: "es-ES",
	}
	if !reflect.DeepEqual(held, want) {
		t.Errorf("lists = %+v, want %+v", held, want)
	}
}

func TestListSettingsReadWhatTheEnvironmentNames(t *testing.T) {
	t.Parallel()

	held, err := listsRead(t, map[string]string{
		"GOPHENBERG_LIST_PAGE_SIZES":     " 5, 15 ,45 ",
		"GOPHENBERG_LIST_PAGE_SIZE":      "15",
		"GOPHENBERG_LIST_PAGE_CAP":       "60",
		"GOPHENBERG_TOAST_DURATION":      "1500ms",
		"GOPHENBERG_TOAST_NAME_LENGTH":   "30",
		"GOPHENBERG_FORMAT_LOCALE":       "en-gb",
		"GOPHENBERG_MEDIA_UPLOAD_CAP_MB": "64",
	})

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	want := server.ListSettings{
		PageSizes: []int{5, 15, 45}, PageSize: 15, PageCap: 60,
		ToastDuration: 1500 * time.Millisecond, ToastNameLength: 30, FormatLocale: "en-GB",
	}
	if !reflect.DeepEqual(held, want) {
		t.Errorf("lists = %+v, want %+v", held, want)
	}
}

func TestListSettingsTakeTheWidestBoundsTheyAllow(t *testing.T) {
	t.Parallel()

	held, err := listsRead(t, map[string]string{
		"GOPHENBERG_LIST_PAGE_SIZES": "1,2,3,4,5,2147483647",
		"GOPHENBERG_LIST_PAGE_SIZE":  "1",
		"GOPHENBERG_LIST_PAGE_CAP":   "2147483647",
		"GOPHENBERG_TOAST_DURATION":  "2147483647ms",
	})

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want the widest bounds taken", err)
	}
	if held.PageCap != 2147483647 || held.ToastDuration != 2147483647*time.Millisecond {
		t.Errorf("lists = %+v, want the largest cap and the longest toast", held)
	}
}

func TestListSettingsTakeTheShortestToast(t *testing.T) {
	t.Parallel()

	held, err := listsRead(t, map[string]string{"GOPHENBERG_TOAST_DURATION": "1ms"})

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want the shortest toast taken", err)
	}
	if held.ToastDuration != time.Millisecond {
		t.Errorf("toast = %v, want 1ms", held.ToastDuration)
	}
}

func TestListSettingsRefuseWhatTheListsCannotUse(t *testing.T) {
	t.Parallel()

	menuHides := "the Items per page menu shows only for 2 to 6 sizes"
	for name, asked := range map[string]struct {
		key   string
		value string
		says  string
	}{
		"one page size":                  {"GOPHENBERG_LIST_PAGE_SIZES", "10", menuHides},
		"seven page sizes":               {"GOPHENBERG_LIST_PAGE_SIZES", "10,20,30,40,50,60,70", menuHides},
		"sizes falling":                  {"GOPHENBERG_LIST_PAGE_SIZES", "20,10", "from the smallest up"},
		"a size named twice":             {"GOPHENBERG_LIST_PAGE_SIZES", "10,10,20", "from the smallest up"},
		"a size of zero":                 {"GOPHENBERG_LIST_PAGE_SIZES", "0,10", "above zero"},
		"a size that is no number":       {"GOPHENBERG_LIST_PAGE_SIZES", "10,many", "whole number"},
		"a size past the cap":            {"GOPHENBERG_LIST_PAGE_SIZES", "10,200", "past GOPHENBERG_LIST_PAGE_CAP 100"},
		"an opening size not offered":    {"GOPHENBERG_LIST_PAGE_SIZE", "30", "one of GOPHENBERG_LIST_PAGE_SIZES"},
		"an opening size that is a word": {"GOPHENBERG_LIST_PAGE_SIZE", "many", "whole number"},
		"a cap under the default sizes": {
			"GOPHENBERG_LIST_PAGE_CAP", "40", "at or above the largest page size 100",
		},
		"a cap of zero":               {"GOPHENBERG_LIST_PAGE_CAP", "0", "above zero"},
		"a cap too large":             {"GOPHENBERG_LIST_PAGE_CAP", "2147483648", "at or below 2147483647"},
		"a toast of nothing":          {"GOPHENBERG_TOAST_DURATION", "0s", "above zero"},
		"a toast under a millisecond": {"GOPHENBERG_TOAST_DURATION", "1500us", "whole number of milliseconds"},
		"a toast no timer holds":      {"GOPHENBERG_TOAST_DURATION", "2147483648ms", "at or below"},
		"a toast that is no duration": {"GOPHENBERG_TOAST_DURATION", "soon", "duration"},
		"a name length of zero":       {"GOPHENBERG_TOAST_NAME_LENGTH", "0", "above zero"},
		"a name length that is a word": {
			"GOPHENBERG_TOAST_NAME_LENGTH", "long", "whole number",
		},
		"a locale that names nothing": {"GOPHENBERG_FORMAT_LOCALE", "und", "BCP 47"},
		"a locale that is no tag":     {"GOPHENBERG_FORMAT_LOCALE", "not a tag", "BCP 47"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := listsRead(t, map[string]string{asked.key: asked.value})

			if err == nil || !strings.Contains(err.Error(), asked.key) || !strings.Contains(err.Error(), asked.says) {
				t.Errorf("loadRunConfig() error = %v, want %s refused saying %q", err, asked.key, asked.says)
			}
		})
	}
}
