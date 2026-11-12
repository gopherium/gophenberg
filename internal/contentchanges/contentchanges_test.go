// SPDX-License-Identifier: Apache-2.0

package contentchanges_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/contentchanges"
	"github.com/gopherium/gophenberg/sdk"
)

// listeners keeps every change it hears, hearing none when deaf.
type listeners struct {
	deaf  bool
	heard []sdk.ContentChange
}

// Listens reports whether the fake hears changes.
func (l *listeners) Listens() bool {
	return !l.deaf
}

// ContentChanged keeps the change.
func (l *listeners) ContentChanged(_ context.Context, change sdk.ContentChange) {
	l.heard = append(l.heard, change)
}

// shaper shapes an item into its title alone, or answers its failure, keeping the titles it shaped.
type shaper struct {
	err    error
	shaped []string
}

// Item returns the item holding the stored item's title, or the shaper's failure.
func (s *shaper) Item(_ context.Context, c content.Content) (sdk.Item, error) {
	s.shaped = append(s.shaped, c.Title)
	return sdk.Item{Title: c.Title}, s.err
}

// storedAs returns a stored item titled title in status.
func storedAs(title string, status content.Status) content.Content {
	return content.Content{ID: uuid.Must(uuid.NewV7()), Type: content.TypePost, Title: title, Status: status}
}

// after returns a pointer to the item titled After in status.
func after(status content.Status) *content.Content {
	written := storedAs("After", status)
	return &written
}

func TestSavedTellsTheListenersEachChangeVisitorsSee(t *testing.T) {
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

			contentchanges.New(heard, &shaper{}, slog.New(slog.DiscardHandler)).
				Saved(t.Context(), storedAs("Before", tc.before), tc.after)

			if !reflect.DeepEqual(heard.heard, []sdk.ContentChange{tc.want}) {
				t.Errorf("the listeners heard %+v, want %+v", heard.heard, tc.want)
			}
		})
	}
}

func TestSavedTellsNothingOfAWriteVisitorsNeverSee(t *testing.T) {
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

			heard := &listeners{}
			shaped := &shaper{}

			contentchanges.New(heard, shaped, slog.New(slog.DiscardHandler)).
				Saved(t.Context(), storedAs("Before", tc.before), tc.after)

			if len(heard.heard) != 0 || len(shaped.shaped) != 0 {
				t.Errorf("the listeners heard %+v after shaping %v, want nothing heard or shaped", heard.heard, shaped.shaped)
			}
		})
	}
}

func TestSavedShapesNothingWhenNoPluginListens(t *testing.T) {
	t.Parallel()

	heard := &listeners{deaf: true}
	shaped := &shaper{}

	contentchanges.New(heard, shaped, slog.New(slog.DiscardHandler)).
		Saved(t.Context(), storedAs("Before", content.StatusPublished), nil)

	if len(heard.heard) != 0 || len(shaped.shaped) != 0 {
		t.Errorf("the listeners heard %+v after shaping %v, want nothing shaped for a deaf site", heard.heard, shaped.shaped)
	}
}

func TestSavedLogsAnItemItCannotShapeAndTellsNothing(t *testing.T) {
	t.Parallel()

	heard := &listeners{}
	logged := &bytes.Buffer{}
	before := storedAs("Before", content.StatusPublished)

	contentchanges.New(heard, &shaper{err: errors.New("the media library is unreachable")},
		slog.New(slog.NewTextHandler(logged, nil))).Saved(t.Context(), before, nil)

	want := `level=WARN msg="plugins not told of a content change" id=` + before.ID.String() +
		` change=deleted error="the media library is unreachable"`
	if len(heard.heard) != 0 || !strings.Contains(logged.String(), want) {
		t.Errorf("the listeners heard %+v and the log reads %q, want nothing heard and %q", heard.heard, logged, want)
	}
}

// changeOf returns the change of kind carrying the item titled title.
func changeOf(kind sdk.ChangeKind, title string) sdk.ContentChange {
	return sdk.ContentChange{Kind: kind, Item: sdk.Item{Title: title}}
}
