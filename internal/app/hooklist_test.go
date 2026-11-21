// SPDX-License-Identifier: Apache-2.0

package app

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit/settings"
	"github.com/gopherium/framework/pluginkit/wire"

	"github.com/gopherium/gophenberg/internal/version"
)

func TestIndexOfReadsTheHooksAndTheSettingsOfEachPluginFromOneIndex(t *testing.T) {
	t.Parallel()

	read, err := indexOf([]byte(`[
		{"id": "forum", "name": "Forum", "hooks": ["public.head-tags"], "defines": ["forum.badges", {"hook": "forum.check"}],
			"settings": [{"key": "per_page", "type": "count", "default": "10", "min": 1}],
			"description": "Answers questions under each post.", "version": "0.0.1"},
		{"id": "solved", "name": "Solved", "hooks": ["forum.badges"]},
		{"id": "quiet", "name": "Quiet"}
	]`))

	if err != nil {
		t.Fatalf("indexOf() = %v, want the index read", err)
	}
	kept := make([]string, 0, len(read.manifests))
	for _, manifest := range read.manifests {
		kept = append(kept, manifest.ID+": "+manifest.Name+": "+manifest.Description+": "+manifest.Version)
	}
	whole := []string{"forum: Forum: Answers questions under each post.: 0.0.1", "solved: Solved: : ", "quiet: Quiet: : "}
	if !slices.Equal(kept, whole) {
		t.Errorf("indexOf() keeps %q, want every manifest whole in the order of the index", kept)
	}
	listed := read.hooks
	want := map[string][]string{"forum": {"public.head-tags"}, "solved": {"forum.badges"}, "quiet": nil}
	if !maps.EqualFunc(listed.byPlugin, want, slices.Equal) {
		t.Errorf("indexOf() lists %v, want %v", listed.byPlugin, want)
	}
	if !slices.Equal(listed.defined, []string{"forum.badges", "forum.check"}) {
		t.Errorf("indexOf() defines %v, want forum.badges and forum.check", listed.defined)
	}
	least := 1.0
	declared := map[string][]wire.Setting{
		"forum": {{Key: "per_page", Type: settings.TypeCount, Default: "10", Min: &least}}, "solved": nil, "quiet": nil,
	}
	if !reflect.DeepEqual(read.settings, declared) {
		t.Errorf("indexOf() declares %+v, want %+v", read.settings, declared)
	}
}

func TestIndexOfReadsNothingWithoutAnIndex(t *testing.T) {
	t.Parallel()

	read, err := indexOf(nil)

	want := manifestIndex{hooks: listedHooks{byPlugin: map[string][]string{}}, settings: map[string][]wire.Setting{}}
	if err != nil || !reflect.DeepEqual(read, want) {
		t.Errorf("indexOf(nil) = %+v (%v), want no hook, no setting and no error", read, err)
	}
}

func TestServeStopsOnAManifestIndexItCannotRead(t *testing.T) {
	t.Parallel()

	env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL})
	compiled := binary{plugins: noPlugins, index: []byte(`{"id": "feed"`), stamp: version.Stamp()}

	got := testkit.Run(t, commandLine(env, compiled, &exitCodes{}), "", "serve")

	if got.Code != gonsole.ExitFailed || !strings.Contains(got.Stderr, "reading the plugin manifest index") {
		t.Errorf("serve = %d with stderr %q, want 1 and the index named", got.Code, got.Stderr)
	}
}
