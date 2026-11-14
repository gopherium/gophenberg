// SPDX-License-Identifier: Apache-2.0

package contentchanges_test

import (
	"bytes"
	"errors"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/contentchanges"
	"github.com/gopherium/gophenberg/sdk"
)

// roomy is a read budget no test read runs out of.
const roomy = time.Minute

// storedAs returns a stored post titled title in status at the path hello.
func storedAs(title string, status content.Status) content.Content {
	return content.Content{
		ID: uuid.Must(uuid.NewV7()), Type: content.TypePost, Title: title, Path: "hello", Status: status,
	}
}

// after returns a pointer to the item titled After in status.
func after(status content.Status) *content.Content {
	written := storedAs("After", status)
	return &written
}

// changeOf returns the change of kind carrying the item titled title at the path hello.
func changeOf(kind sdk.ChangeKind, title string) sdk.ContentChange {
	return sdk.ContentChange{Kind: kind, Item: sdk.Item{Title: title, Path: "hello"}}
}

func TestAPublishedPostMovedTellsThePathVisitorsLeft(t *testing.T) {
	t.Parallel()

	heard := &listeners{}
	moved := storedAs("After", content.StatusPublished)
	moved.Path = "hello-again"

	write(t, notifier(heard, &shaper{}), storedAs("Before", content.StatusPublished), &moved)

	want := []sdk.ContentChange{{
		Kind: sdk.Published, Item: sdk.Item{Title: "After", Path: "hello-again"}, PreviousPath: "hello",
	}}
	if !reflect.DeepEqual(heard.heard, want) {
		t.Errorf("the listeners heard %+v, want %+v", heard.heard, want)
	}
}

// notifier returns a notifier over heard and shaped, every type active, no children and a roomy budget.
func notifier(heard *listeners, shaped *shaper) *contentchanges.Notifier {
	return contentchanges.New(contentchanges.Settings{
		Listeners: heard, Shaper: shaped, Types: &types{}, Items: &items{}, Budget: roomy,
		Logger: slog.New(slog.DiscardHandler),
	})
}

// write runs one write from before to written through n, holding the item while it runs.
func write(t *testing.T, n *contentchanges.Notifier, before content.Content, written *content.Content) {
	t.Helper()
	release := n.Hold(before.ID)
	defer release()
	n.Before(t.Context(), before)(t.Context(), written)
}

func TestAWriteTellsTheListenersEachChangeVisitorsSee(t *testing.T) {
	t.Parallel()

	published, draft, private := content.StatusPublished, content.StatusDraft, content.StatusPrivate
	for name, tc := range map[string]struct {
		before content.Status
		after  *content.Content
		want   sdk.ContentChange
	}{
		"a draft published":             {draft, after(published), changeOf(sdk.Published, "After")},
		"a published post edited":       {published, after(published), changeOf(sdk.Published, "After")},
		"a private post published":      {private, after(published), changeOf(sdk.Published, "After")},
		"a published post set to draft": {published, after(draft), changeOf(sdk.Unpublished, "Before")},
		"a published post set pending":  {published, after(content.StatusPending), changeOf(sdk.Unpublished, "Before")},
		"a published post made private": {published, after(private), changeOf(sdk.Unpublished, "Before")},
		"a published post trashed":      {published, after(content.StatusTrash), changeOf(sdk.Trashed, "Before")},
		"a published post deleted":      {published, nil, changeOf(sdk.Deleted, "Before")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			heard := &listeners{}

			write(t, notifier(heard, &shaper{}), storedAs("Before", tc.before), tc.after)

			if !reflect.DeepEqual(heard.heard, []sdk.ContentChange{tc.want}) {
				t.Errorf("the listeners heard %+v, want %+v", heard.heard, tc.want)
			}
		})
	}
}

func TestAWriteVisitorsNeverSeeTellsNothing(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		before content.Status
		after  *content.Content
	}{
		"a draft edited":               {content.StatusDraft, after(content.StatusDraft)},
		"a draft trashed":              {content.StatusDraft, after(content.StatusTrash)},
		"a draft deleted":              {content.StatusDraft, nil},
		"a trashed post restored":      {content.StatusTrash, after(content.StatusDraft)},
		"a trashed post deleted":       {content.StatusTrash, nil},
		"a private post trashed":       {content.StatusPrivate, after(content.StatusTrash)},
		"a pending post sent to draft": {content.StatusPending, after(content.StatusDraft)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			heard, shaped := &listeners{}, &shaper{}

			write(t, notifier(heard, shaped), storedAs("Before", tc.before), tc.after)

			if len(heard.heard) != 0 || len(shaped.shaped) != 0 {
				t.Errorf("the listeners heard %+v after shaping %v, want nothing heard or shaped", heard.heard, shaped.shaped)
			}
		})
	}
}

func TestAnItemOfATypeTheSiteDoesNotServeCountsAsUnseen(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		types *types
		after *content.Content
	}{
		"an inactive type deleted":          {&types{inactive: content.TypePost}, nil},
		"an inactive type edited":           {&types{inactive: content.TypePost}, after(content.StatusPublished)},
		"a type the registry no longer has": {&types{err: content.ErrTypeNotFound}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			heard, shaped := &listeners{}, &shaper{}
			logged := &bytes.Buffer{}
			n := contentchanges.New(contentchanges.Settings{
				Listeners: heard, Shaper: shaped, Types: tc.types, Items: &items{}, Budget: roomy,
				Logger: slog.New(slog.NewTextHandler(logged, nil)),
			})

			write(t, n, storedAs("Before", content.StatusPublished), tc.after)

			if len(heard.heard) != 0 || len(shaped.shaped) != 0 || logged.Len() != 0 {
				t.Errorf("the listeners heard %+v after shaping %v and the log reads %q, want nothing for an unseen type",
					heard.heard, shaped.shaped, logged)
			}
		})
	}
}

func TestAWriteLogsATypeItCannotReadAndTellsNothing(t *testing.T) {
	t.Parallel()

	published, draft := content.StatusPublished, content.StatusDraft
	for name, tc := range map[string]struct {
		before   content.Status
		after    *content.Content
		failFrom int
		kind     string
	}{
		"a published post deleted, read before":      {published, nil, 1, "deleted"},
		"a published post set to draft, read before": {published, after(draft), 1, "unpublished"},
		"a published post edited, read after":        {published, after(published), 2, "published"},
		"a draft published, read after":              {draft, after(published), 1, "published"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			heard := &listeners{}
			logged := &bytes.Buffer{}
			n := contentchanges.New(contentchanges.Settings{
				Listeners: heard, Shaper: &shaper{},
				Types: &types{err: errors.New("the registry is unreachable"), failFrom: tc.failFrom},
				Items: &items{}, Budget: roomy, Logger: slog.New(slog.NewTextHandler(logged, nil)),
			})

			write(t, n, storedAs("Before", tc.before), tc.after)

			want := `level=WARN msg="plugins not told of a content change" change=` + tc.kind +
				` error="the registry is unreachable"`
			if len(heard.heard) != 0 || !strings.Contains(logged.String(), want) {
				t.Errorf("the listeners heard %+v and the log reads %q, want nothing heard and %q", heard.heard, logged, want)
			}
		})
	}
}

func TestAWriteShapesTheItemVisitorsSawBeforeTheWriteRuns(t *testing.T) {
	t.Parallel()

	heard, shaped := &listeners{}, &shaper{}
	n := notifier(heard, shaped)

	tell := n.Before(t.Context(), storedAs("Before", content.StatusPublished))
	shapedBeforeTheWrite := slices.Clone(shaped.shaped)
	tell(t.Context(), nil)

	if !slices.Equal(shapedBeforeTheWrite, []string{"Before"}) || !slices.Equal(shaped.shaped, []string{"Before"}) {
		t.Errorf("shaped %v before the write and %v in all, want the item shaped once before the write",
			shapedBeforeTheWrite, shaped.shaped)
	}
	if want := []sdk.ContentChange{changeOf(sdk.Deleted, "Before")}; !reflect.DeepEqual(heard.heard, want) {
		t.Errorf("the listeners heard %+v, want %+v", heard.heard, want)
	}
}

func TestEveryReadForPluginsRunsUnderTheBudget(t *testing.T) {
	t.Parallel()

	shaped := &shaper{}

	write(t, notifier(&listeners{}, shaped), storedAs("Before", content.StatusDraft), after(content.StatusPublished))

	if !shaped.deadline {
		t.Error("the shaper read with no deadline, want the budget as its deadline")
	}
}

func TestAWriteShapesNothingWhenNoPluginListens(t *testing.T) {
	t.Parallel()

	heard, shaped := &listeners{deaf: true}, &shaper{}

	write(t, notifier(heard, shaped), storedAs("Before", content.StatusPublished), nil)

	if len(heard.heard) != 0 || len(shaped.shaped) != 0 {
		t.Errorf("the listeners heard %+v after shaping %v, want nothing shaped for a deaf site", heard.heard, shaped.shaped)
	}
}

func TestAWriteLogsAnItemItCannotShapeAndTellsNothing(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		after *content.Content
		kind  string
	}{
		"before a delete": {nil, "deleted"},
		"after a publish": {after(content.StatusPublished), "published"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			heard := &listeners{}
			logged := &bytes.Buffer{}
			before := storedAs("Before", content.StatusPublished)
			n := contentchanges.New(contentchanges.Settings{
				Listeners: heard, Shaper: &shaper{err: errors.New("the media library is unreachable")}, Types: &types{},
				Items: &items{}, Budget: roomy, Logger: slog.New(slog.NewTextHandler(logged, nil)),
			})

			write(t, n, before, tc.after)

			want := `level=WARN msg="plugins not told of a content change" change=` + tc.kind +
				` error="the media library is unreachable"`
			if len(heard.heard) != 0 || !strings.Contains(logged.String(), want) {
				t.Errorf("the listeners heard %+v and the log reads %q, want nothing heard and %q", heard.heard, logged, want)
			}
		})
	}
}
