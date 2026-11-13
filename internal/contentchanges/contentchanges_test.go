// SPDX-License-Identifier: Apache-2.0

package contentchanges_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/contentchanges"
	"github.com/gopherium/gophenberg/sdk"
)

// roomy is a read budget no test read runs out of.
const roomy = time.Minute

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

// shaper shapes an item into its title and path, or answers its failure, keeping the titles it shaped.
type shaper struct {
	err      error
	shaped   []string
	deadline bool
}

// Item returns the item holding the stored item's title and path, or the shaper's failure.
func (s *shaper) Item(ctx context.Context, c content.Content) (sdk.Item, error) {
	s.shaped = append(s.shaped, c.Title)
	_, s.deadline = ctx.Deadline()
	return sdk.Item{Title: c.Title, Path: c.Path}, s.err
}

// types answers every type active but the inactive one, or its failure.
type types struct {
	inactive string
	err      error
}

// Active answers the type, or ErrTypeInactive for the inactive one, or the fake's failure.
func (t types) Active(_ context.Context, key string) (content.Type, error) {
	if t.err != nil {
		return content.Type{}, t.err
	}
	if key == t.inactive {
		return content.Type{}, content.ErrTypeInactive
	}
	return content.Type{Key: key}, nil
}

// items holds stored items by id, listing the published ones of a type.
type items struct {
	held    []content.Content
	listErr error
	byIDErr error
}

// Children counts the items whose parent is id.
func (s *items) Children(_ context.Context, id uuid.UUID) (int, error) {
	count := 0
	for _, held := range s.held {
		if held.ParentID != nil && *held.ParentID == id {
			count++
		}
	}
	return count, nil
}

// List returns one page of the held items of the filter's type in the filter's statuses.
func (s *items) List(_ context.Context, f content.Filter) ([]content.ListedItem, int, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	var matched []content.ListedItem
	for _, held := range s.held {
		if held.Type == f.Type && slices.Contains(f.Statuses, held.Status) {
			matched = append(matched, content.ListedItem{Content: held})
		}
	}
	start := min((f.Page-1)*f.PerPage, len(matched))
	return matched[start:min(start+f.PerPage, len(matched))], len(matched), nil
}

// ByID returns the held item with id.
func (s *items) ByID(_ context.Context, id uuid.UUID) (content.Content, error) {
	if s.byIDErr != nil {
		return content.Content{}, s.byIDErr
	}
	for _, held := range s.held {
		if held.ID == id {
			return held, nil
		}
	}
	return content.Content{}, content.ErrNotFound
}

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
		Listeners: heard, Shaper: shaped, Types: types{}, Items: &items{}, Budget: roomy,
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

func TestAnItemOfAnInactiveTypeCountsAsUnseen(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		types types
		after *content.Content
	}{
		"an inactive type deleted":        {types{inactive: content.TypePost}, nil},
		"an inactive type edited":         {types{inactive: content.TypePost}, after(content.StatusPublished)},
		"a type the registry cannot read": {types{err: errors.New("the registry is unreachable")}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			heard, shaped := &listeners{}, &shaper{}
			n := contentchanges.New(contentchanges.Settings{
				Listeners: heard, Shaper: shaped, Types: tc.types, Items: &items{}, Budget: roomy,
				Logger: slog.New(slog.DiscardHandler),
			})

			write(t, n, storedAs("Before", content.StatusPublished), tc.after)

			if len(heard.heard) != 0 || len(shaped.shaped) != 0 {
				t.Errorf("the listeners heard %+v after shaping %v, want nothing for an unseen type", heard.heard, shaped.shaped)
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
				Listeners: heard, Shaper: &shaper{err: errors.New("the media library is unreachable")}, Types: types{},
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

func TestHoldKeepsTheWritesOfOneItemInOrder(t *testing.T) {
	t.Parallel()

	n := notifier(&listeners{}, &shaper{})
	item := uuid.Must(uuid.NewV7())
	var mu sync.Mutex
	var order []string
	release := n.Hold(item)
	second := make(chan struct{})
	go func() {
		defer close(second)
		release := n.Hold(item)
		defer release()
		mu.Lock()
		order = append(order, "second")
		mu.Unlock()
	}()
	other := n.Hold(uuid.Must(uuid.NewV7()))
	other()
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	order = append(order, "first")
	mu.Unlock()
	release()
	<-second

	if !slices.Equal(order, []string{"first", "second"}) {
		t.Errorf("the writes ran in the order %v, want the second to wait for the first", order)
	}
}

// pageAt returns a stored page titled title at path in status, under parent when one is given.
func pageAt(title, path string, status content.Status, parent *content.Content) content.Content {
	page := content.Content{ID: uuid.Must(uuid.NewV7()), Type: "page", Title: title, Path: path, Status: status}
	if parent != nil {
		page.ParentID = &parent.ID
	}
	return page
}

// movedTree returns the parent About before and after it moved to company, and the site's pages after the move.
func movedTree(parentStatus content.Status) (content.Content, content.Content, []content.Content) {
	before := pageAt("About", "about", parentStatus, nil)
	moved := before
	moved.Path = "company"
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
				Listeners: heard, Shaper: &shaper{}, Types: types{}, Items: &items{held: held}, Budget: roomy,
				Logger: slog.New(slog.DiscardHandler),
			})

			write(t, n, before, &moved)

			if !reflect.DeepEqual(heard.heard, tc.want) {
				t.Errorf("the listeners heard %+v, want %+v", heard.heard, tc.want)
			}
		})
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
		Listeners: heard, Shaper: &shaper{}, Types: types{}, Items: &items{held: held}, Budget: roomy,
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
		Listeners: heard, Shaper: &shaper{}, Types: types{}, Items: &items{held: []content.Content{moved}},
		Budget: roomy, Logger: slog.New(slog.DiscardHandler),
	})

	write(t, n, before, &moved)

	if want := []sdk.ContentChange{movedChange("About", "company", "about")}; !reflect.DeepEqual(heard.heard, want) {
		t.Errorf("the listeners heard %+v, want %+v", heard.heard, want)
	}
}

func TestAMovedParentLogsChildrenItCannotRead(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		items *items
		want  string
	}{
		"a listing that fails": {
			&items{listErr: errors.New("the database is unreachable")},
			`level=WARN msg="plugins not told of moved children" parent=company error="the database is unreachable"`,
		},
		"a child that cannot be read": {
			&items{byIDErr: errors.New("the database is unreachable")},
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
				Listeners: heard, Shaper: &shaper{}, Types: types{}, Items: tc.items, Budget: roomy,
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

func TestHoldLocksNothingWhenNoPluginListens(t *testing.T) {
	t.Parallel()

	n := notifier(&listeners{deaf: true}, &shaper{})
	item := uuid.Must(uuid.NewV7())

	first := n.Hold(item)
	second := n.Hold(item)
	second()
	first()
}
