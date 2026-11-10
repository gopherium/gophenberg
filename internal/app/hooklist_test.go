// SPDX-License-Identifier: Apache-2.0

package app

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/version"
)

func TestListedHooksOfReadsTheHooksEachPluginListsAndDefines(t *testing.T) {
	t.Parallel()

	listed, err := listedHooksOf([]byte(`[
		{"id": "forum", "name": "Forum", "hooks": ["public.head-tags"], "defines": ["forum.badges", {"hook": "forum.check"}]},
		{"id": "solved", "name": "Solved", "hooks": ["forum.badges"]},
		{"id": "quiet", "name": "Quiet"}
	]`))

	if err != nil {
		t.Fatalf("listedHooksOf() = %v, want the index read", err)
	}
	want := map[string][]string{"forum": {"public.head-tags"}, "solved": {"forum.badges"}, "quiet": nil}
	if !maps.EqualFunc(listed.byPlugin, want, slices.Equal) {
		t.Errorf("listedHooksOf() lists %v, want %v", listed.byPlugin, want)
	}
	if !slices.Equal(listed.defined, []string{"forum.badges", "forum.check"}) {
		t.Errorf("listedHooksOf() defines %v, want forum.badges and forum.check", listed.defined)
	}
}

func TestListedHooksOfListsNoHookWithoutAnIndex(t *testing.T) {
	t.Parallel()

	listed, err := listedHooksOf(nil)

	if err != nil || len(listed.byPlugin) != 0 || listed.defined != nil {
		t.Errorf("listedHooksOf(nil) = %+v (%v), want no hook and no error", listed, err)
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
