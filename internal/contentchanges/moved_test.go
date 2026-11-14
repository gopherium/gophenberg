// SPDX-License-Identifier: Apache-2.0

package contentchanges_test

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/contentchanges"
	"github.com/gopherium/gophenberg/sdk"
)

// pageAt returns a stored page titled title at path in status, under parent and stamped as it when one is given.
func pageAt(title, path string, status content.Status, parent *content.Content) content.Content {
	page := content.Content{ID: uuid.Must(uuid.NewV7()), Type: "page", Title: title, Path: path, Status: status}
	if parent != nil {
		page.ParentID, page.UpdatedAt = &parent.ID, parent.UpdatedAt
	}
	return page
}

// movedTree returns the parent About before and after it moved to company, and the site's pages after the move.
func movedTree(parentStatus content.Status) (content.Content, content.Content, []content.Content) {
	before := pageAt("About", "about", parentStatus, nil)
	moved := before
	moved.Path, moved.UpdatedAt = "company", time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	team := pageAt("Team", "company/team", content.StatusPublished, &moved)
	lead := pageAt("Lead", "company/team/lead", content.StatusPublished, &team)
	draft := pageAt("Draft", "company/draft", content.StatusDraft, &moved)
	contact := pageAt("Contact", "contact", content.StatusPublished, nil)
	return before, moved, []content.Content{moved, team, draft, lead, contact}
}

// movedChange returns the Published change of the item titled title now at path, which visitors saw at previous.
func movedChange(title, path, previous string) sdk.ContentChange {
	return sdk.ContentChange{Kind: sdk.Published, Item: sdk.Item{Title: title, Path: path}, PreviousPath: previous}
}

func TestAMovedParentTellsTheListenersOfEachPublishedChildItMoved(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		parent content.Status
		want   []sdk.ContentChange
	}{
		"a published parent": {content.StatusPublished, []sdk.ContentChange{
			movedChange("About", "company", "about"), movedChange("Team", "company/team", "about/team"),
			movedChange("Lead", "company/team/lead", "about/team/lead"),
		}},
		"a draft parent": {content.StatusDraft, []sdk.ContentChange{
			movedChange("Team", "company/team", "about/team"), movedChange("Lead", "company/team/lead", "about/team/lead"),
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			heard := &listeners{}
			before, moved, held := movedTree(tc.parent)
			n := contentchanges.New(contentchanges.Settings{
				Listeners: heard, Shaper: &shaper{}, Types: &types{}, Items: &items{held: held}, Budget: roomy,
				Logger: slog.New(slog.DiscardHandler),
			})

			write(t, n, before, &moved)

			if !reflect.DeepEqual(heard.heard, tc.want) {
				t.Errorf("the listeners heard %+v, want %+v", heard.heard, tc.want)
			}
		})
	}
}

func TestAMovedParentLeavesAChildALaterWriteChangedToThatWrite(t *testing.T) {
	t.Parallel()

	for name, since := range map[string]func(team content.Content) *content.Content{
		"a child edited since the move": func(team content.Content) *content.Content {
			team.Title, team.UpdatedAt = "Team edited", team.UpdatedAt.Add(time.Second)
			return &team
		},
		"a child set to draft since the move": func(team content.Content) *content.Content {
			team.Status, team.UpdatedAt = content.StatusDraft, team.UpdatedAt.Add(time.Second)
			return &team
		},
		"a child deleted since the move": func(content.Content) *content.Content { return nil },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			heard := &listeners{}
			logged := &bytes.Buffer{}
			before, moved, held := movedTree(content.StatusPublished)
			stored := &items{held: held}
			stored.rewrite(held[1].ID, since(held[1]))
			n := contentchanges.New(contentchanges.Settings{
				Listeners: heard, Shaper: &shaper{}, Types: &types{}, Items: stored, Budget: roomy,
				Logger: slog.New(slog.NewTextHandler(logged, nil)),
			})

			write(t, n, before, &moved)

			want := []sdk.ContentChange{
				movedChange("About", "company", "about"), movedChange("Lead", "company/team/lead", "about/team/lead"),
			}
			if !reflect.DeepEqual(heard.heard, want) || logged.Len() != 0 {
				t.Errorf("the listeners heard %+v and the log reads %q, want %+v and nothing logged", heard.heard, logged,
					want)
			}
		})
	}
}

func TestAMovedParentReadsEachChildOnlyOnceItHoldsIt(t *testing.T) {
	t.Parallel()

	heard := &listeners{}
	before, moved, held := movedTree(content.StatusPublished)
	stored := &items{held: held}
	n := contentchanges.New(contentchanges.Settings{
		Listeners: heard, Shaper: &shaper{}, Types: &types{}, Items: stored, Budget: roomy,
		Logger: slog.New(slog.DiscardHandler),
	})
	team := held[1]
	release := n.Hold(team.ID)
	done := make(chan struct{})
	go func() {
		defer close(done)
		write(t, n, before, &moved)
	}()
	select {
	case <-done:
		t.Fatal("the move was told in full while another write held a child it carried")
	case <-time.After(20 * time.Millisecond):
	}
	edited := team
	edited.Title, edited.UpdatedAt = "Team edited", team.UpdatedAt.Add(time.Second)
	stored.rewrite(team.ID, &edited)
	release()
	<-done

	want := []sdk.ContentChange{
		movedChange("About", "company", "about"), movedChange("Lead", "company/team/lead", "about/team/lead"),
	}
	if !reflect.DeepEqual(heard.heard, want) {
		t.Errorf("the listeners heard %+v, want %+v with the held child left to its own write", heard.heard, want)
	}
}

func TestAMovedParentFindsItsChildrenAcrossListingPages(t *testing.T) {
	t.Parallel()

	heard := &listeners{}
	before := pageAt("About", "about", content.StatusPublished, nil)
	moved := before
	moved.Path = "company"
	held := []content.Content{moved}
	for i := range content.MaxPerPage + 20 {
		held = append(held, pageAt(fmt.Sprintf("Page %d", i), fmt.Sprintf("company/page-%d", i),
			content.StatusPublished, &moved))
	}
	n := contentchanges.New(contentchanges.Settings{
		Listeners: heard, Shaper: &shaper{}, Types: &types{}, Items: &items{held: held}, Budget: roomy,
		Logger: slog.New(slog.DiscardHandler),
	})

	write(t, n, before, &moved)

	if len(heard.heard) != content.MaxPerPage+21 {
		t.Fatalf("the listeners heard %d changes, want the parent and its %d children", len(heard.heard),
			content.MaxPerPage+20)
	}
	if last := heard.heard[len(heard.heard)-1]; last.PreviousPath != fmt.Sprintf("about/page-%d", content.MaxPerPage+19) {
		t.Errorf("the last change is %+v, want the last child at the path it left", last)
	}
}

func TestAMovedParentWithNoChildrenTellsOnlyItsOwnChange(t *testing.T) {
	t.Parallel()

	heard := &listeners{}
	before := pageAt("About", "about", content.StatusPublished, nil)
	moved := before
	moved.Path = "company"
	n := contentchanges.New(contentchanges.Settings{
		Listeners: heard, Shaper: &shaper{}, Types: &types{}, Items: &items{held: []content.Content{moved}},
		Budget: roomy, Logger: slog.New(slog.DiscardHandler),
	})

	write(t, n, before, &moved)

	if want := []sdk.ContentChange{movedChange("About", "company", "about")}; !reflect.DeepEqual(heard.heard, want) {
		t.Errorf("the listeners heard %+v, want %+v", heard.heard, want)
	}
}

func TestAMovedParentLogsChildrenItCannotRead(t *testing.T) {
	t.Parallel()

	unreachable := errors.New("the database is unreachable")
	for name, tc := range map[string]struct {
		items *items
		types *types
		want  string
	}{
		"a listing that fails": {
			&items{listErr: unreachable}, &types{},
			`level=WARN msg="plugins not told of moved children" parent=company error="the database is unreachable"`,
		},
		"a child that cannot be read": {
			&items{byIDErr: unreachable}, &types{},
			`level=WARN msg="plugins not told of a content change" change=published error="the database is unreachable"`,
		},
		"a child whose type cannot be read": {
			&items{}, &types{err: unreachable, failFrom: 3},
			`level=WARN msg="plugins not told of a content change" change=published error="the database is unreachable"`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			heard := &listeners{}
			logged := &bytes.Buffer{}
			before, moved, held := movedTree(content.StatusPublished)
			tc.items.held = held
			n := contentchanges.New(contentchanges.Settings{
				Listeners: heard, Shaper: &shaper{}, Types: tc.types, Items: tc.items, Budget: roomy,
				Logger: slog.New(slog.NewTextHandler(logged, nil)),
			})

			write(t, n, before, &moved)

			want := []sdk.ContentChange{movedChange("About", "company", "about")}
			if !reflect.DeepEqual(heard.heard, want) || !strings.Contains(logged.String(), tc.want) {
				t.Errorf("the listeners heard %+v and the log reads %q, want %+v and %q", heard.heard, logged, want, tc.want)
			}
		})
	}
}
