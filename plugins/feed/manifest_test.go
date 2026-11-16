// SPDX-License-Identifier: Apache-2.0

package feed_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gopherium/framework/pluginkit/wire"

	"github.com/gopherium/gophenberg/internal/pluginsettings"
)

// resolvedOver returns the settings the site works out for the feed over env, from those its manifest declares.
func resolvedOver(t *testing.T, env map[string]string) *pluginsettings.Set {
	t.Helper()
	raw, err := os.ReadFile("plugin.json")
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	var manifest wire.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decoding the manifest: %v", err)
	}
	declared := map[string][]wire.Setting{manifest.ID: manifest.Settings}
	return pluginsettings.New(declared, func(key string) string { return env[key] }, nil)
}

func TestTheManifestDefaultsTheTitleAndThePostCountTheTestsLend(t *testing.T) {
	t.Parallel()

	set := resolvedOver(t, nil)
	lent, want := set.For("feed"), defaults()

	if err := set.Rejection("feed"); err != nil {
		t.Fatalf("Rejection() = %v, want the manifest's settings accepted", err)
	}
	if got := lent.Text("title"); got != want.Text("title") {
		t.Errorf("title = %q, want the default %q", got, want.Text("title"))
	}
	if got := lent.Count("items"); got != want.Count("items") {
		t.Errorf("items = %d, want the default %d", got, want.Count("items"))
	}
}

func TestTheSiteReadsTheFeedSettingsUnderThePluginNamesTrimmed(t *testing.T) {
	t.Parallel()

	lent := resolvedOver(t, map[string]string{
		"GOPHENBERG_PLUGIN_FEED__TITLE": "  Field Notes  ",
		"GOPHENBERG_PLUGIN_FEED__ITEMS": " 5 ",
	}).For("feed")

	if got := lent.Text("title"); got != "Field Notes" {
		t.Errorf("title = %q, want %q", got, "Field Notes")
	}
	if got := lent.Count("items"); got != 5 {
		t.Errorf("items = %d, want 5", got)
	}
}

func TestTheManifestRejectsAPostCountThatIsNotAWholeNumberAboveZero(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"banana", "0", "-3", "2.5"} {
		err := resolvedOver(t, map[string]string{"GOPHENBERG_PLUGIN_FEED__ITEMS": raw}).Rejection("feed")

		if err == nil || !strings.Contains(err.Error(), "GOPHENBERG_PLUGIN_FEED__ITEMS") {
			t.Errorf("Rejection() with the post count %q = %v, want it rejected naming the variable", raw, err)
		}
	}
}
