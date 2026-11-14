// SPDX-License-Identifier: Apache-2.0

package pluginhooks_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/pluginhooks"
	"github.com/gopherium/gophenberg/sdk"
)

// roomy are hook settings no test plugin runs out of.
var roomy = pluginhooks.Settings{Budget: time.Minute, SaveBudget: time.Minute, TimedFirings: 20}

// headTags is what a test plugin answers when asked for the head tags of a page.
type headTags = func(ctx context.Context, page sdk.PageMeta) ([]sdk.HeadTag, error)

// plain is a plugin that implements no hook.
type plain struct{ id string }

// ID returns the plugin's id.
func (p plain) ID() string { return p.id }

// Start starts nothing.
func (plain) Start(context.Context) error { return nil }

// Stop stops nothing.
func (plain) Stop(context.Context) error { return nil }

// heading is a plugin that adds the head tags its function answers.
type heading struct {
	plain
	tags headTags
}

// HeadTags answers what the plugin's function answers for the page.
func (h heading) HeadTags(ctx context.Context, page sdk.PageMeta) ([]sdk.HeadTag, error) {
	return h.tags(ctx, page)
}

// headingOf returns a plugin under id that adds the head tags tags answers.
func headingOf(id string, tags headTags) heading {
	return heading{plain: plain{id: id}, tags: tags}
}

// marking returns a plugin under id that adds one meta tag carrying its id.
func marking(id string) heading {
	return headingOf(id, func(context.Context, sdk.PageMeta) ([]sdk.HeadTag, error) {
		return []sdk.HeadTag{markOf(id)}, nil
	})
}

// markOf returns the meta tag the plugin id adds when it marks a page.
func markOf(id string) sdk.HeadTag {
	return sdk.HeadTag{Tag: "meta", Attrs: []sdk.Attr{{Key: "name", Value: id + "-mark"}}}
}

// builtUnder returns the hook set over plugins under settings and the log it writes to.
func builtUnder(t *testing.T, settings pluginhooks.Settings, plugins ...sdk.Plugin) (*pluginhooks.Set, *bytes.Buffer) {
	t.Helper()
	logged := &bytes.Buffer{}
	set, err := pluginhooks.New(plugins, settings, slog.New(slog.NewTextHandler(logged, nil)))
	if err != nil {
		t.Fatalf("New() = %v, want the hooks built", err)
	}
	return set, logged
}

// built returns the hook set over plugins with roomy settings and the log it writes to.
func built(t *testing.T, plugins ...sdk.Plugin) (*pluginhooks.Set, *bytes.Buffer) {
	t.Helper()
	return builtUnder(t, roomy, plugins...)
}

// loggedLine fails the test unless the log carries line.
func loggedLine(t *testing.T, logged *bytes.Buffer, line string) {
	t.Helper()
	if !strings.Contains(logged.String(), line) {
		t.Errorf("the log %q, want %q among it", logged, line)
	}
}

func TestCheckPassesAPluginWhoseListedHooksMatchItsMethods(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		plugin  sdk.Plugin
		listed  []string
		defined []string
	}{
		"no hook and no method":        {plugin: plain{id: "notes"}},
		"a listed hook and its method": {plugin: marking("notes"), listed: []string{sdk.HeadTagsHook}},
		"a hook another plugin defines": {
			plugin: plain{id: "badges"}, listed: []string{"forum.badges"}, defined: []string{"forum.badges"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := pluginhooks.Check(tc.plugin, tc.listed, tc.defined); err != nil {
				t.Errorf("Check() = %v, want no error", err)
			}
		})
	}
}

func TestCheckRefusesAPluginWhoseListedHooksDifferFromItsMethods(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		plugin sdk.Plugin
		listed []string
		want   string
	}{
		"a listed hook with no method": {
			plugin: plain{id: "notes"}, listed: []string{sdk.HeadTagsHook},
			want: "lists public.head-tags but has no HeadTags method",
		},
		"a method whose hook is not listed": {
			plugin: marking("notes"),
			want:   "has a HeadTags method but does not list public.head-tags",
		},
		"a hook nobody defines": {
			plugin: plain{id: "notes"}, listed: []string{"public.head-tag"},
			want: "lists public.head-tag, a hook neither the site nor a plugin defines",
		},
		"a hook listed twice": {
			plugin: marking("notes"), listed: []string{sdk.HeadTagsHook, sdk.HeadTagsHook},
			want: "lists public.head-tags twice",
		},
		"every problem at once": {
			plugin: marking("notes"), listed: []string{"public.head-tag", "public.head-tag"},
			want: "lists public.head-tag, a hook neither the site nor a plugin defines\n" +
				"lists public.head-tag twice\nhas a HeadTags method but does not list public.head-tags",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := pluginhooks.Check(tc.plugin, tc.listed, nil); err == nil || err.Error() != tc.want {
				t.Errorf("Check() = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestNewRefusesSettingsAHookCannotRunUnder(t *testing.T) {
	t.Parallel()

	_, err := pluginhooks.New(nil, pluginhooks.Settings{TimedFirings: 20}, slog.New(slog.DiscardHandler))

	if err == nil || !strings.Contains(err.Error(), "the budget must stand above zero") {
		t.Errorf("New() = %v, want the empty budget refused", err)
	}
}

func TestANilSetAddsNoHeadTag(t *testing.T) {
	t.Parallel()

	var set *pluginhooks.Set

	if set.Heads() || set.HeadTags(t.Context(), sdk.PageMeta{}) != nil {
		t.Error("a nil set says it adds head tags, want none")
	}
}

func TestASetOverPluginsThatAddNoHeadTagSaysSo(t *testing.T) {
	t.Parallel()

	set, _ := built(t, plain{id: "events"}, plain{id: "notes"})

	if set.Heads() || set.HeadTags(t.Context(), sdk.PageMeta{}) != nil {
		t.Error("the set says it adds head tags, want none from plugins that add none")
	}
}

func TestHeadTagsAnswersTheTagsOfEachPluginInOrder(t *testing.T) {
	t.Parallel()

	set, logged := built(t, marking("events"), plain{id: "quiet"}, marking("notes"))

	got := set.HeadTags(t.Context(), sdk.PageMeta{Kind: sdk.PageArchive, Path: "/"})

	if want := []sdk.HeadTag{markOf("events"), markOf("notes")}; !slices.EqualFunc(got, want, sameTag) {
		t.Errorf("HeadTags() = %+v, want %+v", got, want)
	}
	if !set.Heads() || logged.Len() != 0 {
		t.Errorf("Heads() = %v with the log %q, want true and nothing logged", set.Heads(), logged)
	}
}

func TestHeadTagsHandsEachPluginThePage(t *testing.T) {
	t.Parallel()

	var seen []sdk.PageMeta
	keep := headingOf("notes", func(_ context.Context, page sdk.PageMeta) ([]sdk.HeadTag, error) {
		seen = append(seen, page)
		return nil, nil
	})
	set, _ := built(t, keep)
	page := sdk.PageMeta{Kind: sdk.PageItem, Type: "post", Path: "/hello", Page: 0, SiteTitle: "A Site"}

	set.HeadTags(t.Context(), page)

	if !slices.Equal(seen, []sdk.PageMeta{page}) {
		t.Errorf("the plugin saw %+v, want the page %+v", seen, page)
	}
}

func TestHeadTagsGivesEachPluginAnItemOfItsOwn(t *testing.T) {
	t.Parallel()

	rewrite := headingOf("events", func(_ context.Context, page sdk.PageMeta) ([]sdk.HeadTag, error) {
		page.Item.Title = "Rewritten"
		return nil, nil
	})
	var title string
	read := headingOf("notes", func(_ context.Context, page sdk.PageMeta) ([]sdk.HeadTag, error) {
		title = page.Item.Title
		return nil, nil
	})
	set, _ := built(t, rewrite, read)
	item := &sdk.Item{Title: "Hello"}

	set.HeadTags(t.Context(), sdk.PageMeta{Kind: sdk.PageItem, Item: item})

	if title != "Hello" || item.Title != "Hello" {
		t.Errorf("the second plugin read %q and the page holds %q, want both Hello", title, item.Title)
	}
}

func TestHeadTagsLogsAPluginThatFailsAndKeepsTheOthers(t *testing.T) {
	t.Parallel()

	failing := headingOf("notes", func(context.Context, sdk.PageMeta) ([]sdk.HeadTag, error) {
		return []sdk.HeadTag{markOf("notes")}, errors.New("no head today")
	})
	set, logged := built(t, marking("events"), failing)

	got := set.HeadTags(t.Context(), sdk.PageMeta{})

	if want := []sdk.HeadTag{markOf("events")}; !slices.EqualFunc(got, want, sameTag) {
		t.Errorf("HeadTags() = %+v, want only the tags of the plugin that did not fail", got)
	}
	loggedLine(t, logged, `level=WARN msg="plugin hook failed" plugin=notes hook=public.head-tags `+
		`error="pluginkit: notes public.head-tags: no head today"`)
}

func TestHeadTagsLogsAPluginThatPanicsWithItsStack(t *testing.T) {
	t.Parallel()

	panicking := headingOf("notes", func(context.Context, sdk.PageMeta) ([]sdk.HeadTag, error) {
		panic("boom")
	})
	set, logged := built(t, panicking, marking("events"))

	got := set.HeadTags(t.Context(), sdk.PageMeta{})

	if want := []sdk.HeadTag{markOf("events")}; !slices.EqualFunc(got, want, sameTag) {
		t.Errorf("HeadTags() = %+v, want the tags of the plugin after the panic", got)
	}
	loggedLine(t, logged, `level=ERROR msg="plugin hook panicked" plugin=notes hook=public.head-tags `+
		`error="pluginkit: notes public.head-tags panicked: boom" stack="goroutine `)
}

func TestHeadTagsLogsAPluginStillWorkingWhenTheTimeRunsOut(t *testing.T) {
	t.Parallel()

	waiting := headingOf("notes", func(ctx context.Context, _ sdk.PageMeta) ([]sdk.HeadTag, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	set, logged := builtUnder(t, pluginhooks.Settings{Budget: time.Nanosecond, SaveBudget: time.Minute, TimedFirings: 20},
		waiting)

	set.HeadTags(t.Context(), sdk.PageMeta{})

	loggedLine(t, logged, `level=INFO msg="plugin hook ran out of time" plugin=notes hook=public.head-tags `+
		`error="pluginkit: notes public.head-tags: context deadline exceeded"`)
}

func TestHeadTagsDropsATagOutsideTheRulesAndKeepsThePluginsOtherTags(t *testing.T) {
	t.Parallel()

	mixed := headingOf("notes", func(context.Context, sdk.PageMeta) ([]sdk.HeadTag, error) {
		return []sdk.HeadTag{{Tag: "script"}, markOf("notes")}, nil
	})
	set, logged := built(t, mixed)

	got := set.HeadTags(t.Context(), sdk.PageMeta{})

	if want := []sdk.HeadTag{markOf("notes")}; !slices.EqualFunc(got, want, sameTag) {
		t.Errorf("HeadTags() = %+v, want the tag inside the rules", got)
	}
	loggedLine(t, logged, fmt.Sprintf("level=WARN msg=%q plugin=notes hook=public.head-tags error=%q",
		"plugin hook failed", "pluginkit: notes public.head-tags: dropped the head tag script: only meta and link are kept"))
}

// heardChange is what a test plugin answers when it hears a content change.
type heardChange = func(ctx context.Context, change sdk.ContentChange) error

// listening is a plugin that hears content changes through its function.
type listening struct {
	plain
	hear heardChange
}

// ContentChanged answers what the plugin's function answers for the change.
func (l listening) ContentChanged(ctx context.Context, change sdk.ContentChange) error {
	return l.hear(ctx, change)
}

// listeningOf returns a plugin under id that hears content changes through hear.
func listeningOf(id string, hear heardChange) listening {
	return listening{plain: plain{id: id}, hear: hear}
}

// recording returns a plugin under id that keeps each change it hears in heard.
func recording(id string, heard *[]string) listening {
	return listeningOf(id, func(_ context.Context, change sdk.ContentChange) error {
		*heard = append(*heard, id+" "+string(change.Kind)+" "+change.Item.Title)
		return nil
	})
}

func TestCheckPassesAPluginThatListsTheContentHookItHears(t *testing.T) {
	t.Parallel()

	if err := pluginhooks.Check(recording("notes", nil), []string{sdk.ContentChangedHook}, nil); err != nil {
		t.Errorf("Check() = %v, want no error", err)
	}
}

func TestCheckRejectsAContentListenerWhoseListDiffersFromItsMethod(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		plugin sdk.Plugin
		listed []string
		want   string
	}{
		"the hook listed with no method": {
			plugin: plain{id: "notes"}, listed: []string{sdk.ContentChangedHook},
			want: "lists content.changed but has no ContentChanged method",
		},
		"the method with the hook not listed": {
			plugin: recording("notes", nil),
			want:   "has a ContentChanged method but does not list content.changed",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := pluginhooks.Check(tc.plugin, tc.listed, nil); err == nil || err.Error() != tc.want {
				t.Errorf("Check() = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestNewRejectsASaveBudgetAHookCannotRunUnder(t *testing.T) {
	t.Parallel()

	_, err := pluginhooks.New(nil, pluginhooks.Settings{Budget: time.Minute, TimedFirings: 20},
		slog.New(slog.DiscardHandler))

	if err == nil || !strings.Contains(err.Error(), "hook content.changed: the budget must stand above zero") {
		t.Errorf("New() = %v, want the empty save budget rejected", err)
	}
}

func TestANilSetHearsNoContentChange(t *testing.T) {
	t.Parallel()

	var set *pluginhooks.Set

	set.ContentChanged(t.Context(), sdk.ContentChange{Kind: sdk.Published})

	if set.Listens() {
		t.Error("a nil set says plugins hear content changes, want none")
	}
}

func TestASetOverPluginsThatHearNoChangeSaysSo(t *testing.T) {
	t.Parallel()

	set, _ := built(t, plain{id: "events"}, marking("notes"))

	if set.Listens() {
		t.Error("the set says plugins hear content changes, want none from plugins that hear none")
	}
}

func TestContentChangedTellsEachListeningPluginInOrder(t *testing.T) {
	t.Parallel()

	var heard []string
	set, logged := built(t, recording("events", &heard), plain{id: "quiet"}, recording("notes", &heard))

	set.ContentChanged(t.Context(), sdk.ContentChange{Kind: sdk.Trashed, Item: sdk.Item{Title: "Spring workshop"}})

	if want := []string{"events trashed Spring workshop", "notes trashed Spring workshop"}; !slices.Equal(heard, want) {
		t.Errorf("the plugins heard %v, want %v", heard, want)
	}
	if !set.Listens() || logged.Len() != 0 {
		t.Errorf("Listens() = %v with the log %q, want true and nothing logged", set.Listens(), logged)
	}
}

func TestContentChangedGivesEachPluginFieldsOfItsOwn(t *testing.T) {
	t.Parallel()

	rewrite := listeningOf("events", func(_ context.Context, change sdk.ContentChange) error {
		change.Item.Fields["seats"] = 0.0
		change.Item.Fields["venue"].(map[string]any)["city"] = "Elsewhere"
		change.Item.Fields["tags"].([]any)[0] = "changed"
		return nil
	})
	var seen map[string]any
	read := listeningOf("notes", func(_ context.Context, change sdk.ContentChange) error {
		seen = change.Item.Fields
		return nil
	})
	set, _ := built(t, rewrite, read)
	held := func() map[string]any {
		return map[string]any{
			"seats": 40.0, "venue": map[string]any{"city": "Lisbon"}, "tags": []any{"workshop"}, "note": nil,
		}
	}
	fields := held()

	set.ContentChanged(t.Context(), sdk.ContentChange{Kind: sdk.Published, Item: sdk.Item{Fields: fields}})

	want := held()
	if !reflect.DeepEqual(seen, want) || !reflect.DeepEqual(fields, want) {
		t.Errorf("the second plugin read %v and the change holds %v, want both %v", seen, fields, want)
	}
}

func TestContentChangedLogsAListenerThatFailsOrPanics(t *testing.T) {
	t.Parallel()

	failing := listeningOf("events", func(context.Context, sdk.ContentChange) error {
		return errors.New("the index is offline")
	})
	panicking := listeningOf("notes", func(context.Context, sdk.ContentChange) error {
		panic("boom")
	})
	set, logged := built(t, failing, panicking)

	set.ContentChanged(t.Context(), sdk.ContentChange{Kind: sdk.Deleted})

	loggedLine(t, logged, `level=WARN msg="plugin hook failed" plugin=events hook=content.changed `+
		`error="pluginkit: events content.changed: the index is offline"`)
	loggedLine(t, logged, `level=ERROR msg="plugin hook panicked" plugin=notes hook=content.changed `+
		`error="pluginkit: notes content.changed panicked: boom" stack="goroutine `)
}

func TestContentChangedRunsUnderTheSaveBudget(t *testing.T) {
	t.Parallel()

	waiting := listeningOf("notes", func(ctx context.Context, _ sdk.ContentChange) error {
		<-ctx.Done()
		return ctx.Err()
	})
	set, logged := builtUnder(t, pluginhooks.Settings{Budget: time.Minute, SaveBudget: time.Nanosecond, TimedFirings: 20},
		waiting)

	set.ContentChanged(t.Context(), sdk.ContentChange{Kind: sdk.Published})

	loggedLine(t, logged, `level=INFO msg="plugin hook ran out of time" plugin=notes hook=content.changed `+
		`error="pluginkit: notes content.changed: context deadline exceeded"`)
}

// sameTag reports whether both tags carry the same name and the same attributes in the same order.
func sameTag(a, b sdk.HeadTag) bool {
	return a.Tag == b.Tag && slices.Equal(a.Attrs, b.Attrs)
}
