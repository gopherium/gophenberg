// SPDX-License-Identifier: Apache-2.0

// Package contentchanges tells plugins when a saved write changes what visitors see.
package contentchanges

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/sdk"
)

// unshapedLine is the warning the site logs for each change it could not read for plugins.
const unshapedLine = "plugins not told of a content change"

// unlistedLine is the warning the site logs when it could not read the children a write moved.
const unlistedLine = "plugins not told of moved children"

// Listeners hears the content changes of a running site.
type Listeners interface {
	Listens() bool
	ContentChanged(ctx context.Context, change sdk.ContentChange)
}

// Shaper shapes a stored item into the item plugins read.
type Shaper interface {
	Item(ctx context.Context, c content.Content) (sdk.Item, error)
}

// Types answers a content type the site serves, or an error for one it does not.
type Types interface {
	Active(ctx context.Context, key string) (content.Type, error)
}

// Items reads the stored items a write moved beside the one it wrote.
type Items interface {
	Children(ctx context.Context, id uuid.UUID) (int, error)
	List(ctx context.Context, f content.Filter) ([]content.ListedItem, int, error)
	ByID(ctx context.Context, id uuid.UUID) (content.Content, error)
}

// Settings are what a Notifier tells, reads through and logs to.
type Settings struct {
	Listeners Listeners
	Shaper    Shaper
	Types     Types
	Items     Items
	// Budget is the time each read for plugins may take.
	Budget time.Duration
	Logger *slog.Logger
}

// Notifier tells the listeners what each saved write changed of what visitors see, one write of an item at a time.
type Notifier struct {
	settings Settings
	mu       sync.Mutex
	held     map[uuid.UUID]*heldItem
}

// heldItem is the lock of one item, a slot one write fills at a time, and how many writes hold it or wait for it.
type heldItem struct {
	lock  chan struct{}
	users int
}

// errHeldPastBudget is the error the site logs for a moved item another write held past the budget.
var errHeldPastBudget = errors.New("contentchanges: another write held the moved item past the budget")

// New returns a [Notifier] over settings.
func New(settings Settings) *Notifier {
	return &Notifier{settings: settings, held: map[uuid.UUID]*heldItem{}}
}

// Hold locks the item id against the other writes plugins hear until release, so their changes keep their order.
func (n *Notifier) Hold(id uuid.UUID) (release func()) {
	if !n.settings.Listeners.Listens() {
		return func() {}
	}
	release, _ = n.holdWithin(context.Background(), id)
	return release
}

// holdWithin locks the item id until release, or reports false when ctx ends before the item is free.
func (n *Notifier) holdWithin(ctx context.Context, id uuid.UUID) (release func(), held bool) {
	item := n.join(id)
	select {
	case item.lock <- struct{}{}:
		return func() {
			<-item.lock
			n.leave(id, item)
		}, true
	case <-ctx.Done():
		n.leave(id, item)
		return nil, false
	}
}

// join returns the lock of the item id, counting one more write that holds it or waits for it.
func (n *Notifier) join(id uuid.UUID) *heldItem {
	n.mu.Lock()
	defer n.mu.Unlock()
	item, ok := n.held[id]
	if !ok {
		item = &heldItem{lock: make(chan struct{}, 1)}
		n.held[id] = item
	}
	item.users++
	return item
}

// leave counts one write less on the item id, dropping its lock once no write holds it or waits for it.
func (n *Notifier) leave(id uuid.UUID, item *heldItem) {
	n.mu.Lock()
	defer n.mu.Unlock()
	item.users--
	if item.users == 0 {
		delete(n.held, id)
	}
}

// Before readies the write from before, shaping it now when visitors see it, and returns what tells its change.
func (n *Notifier) Before(ctx context.Context, before content.Content) func(context.Context, *content.Content) {
	if !n.settings.Listeners.Listens() {
		return func(context.Context, *content.Content) {}
	}
	w := &write{n: n, before: before}
	w.seen, w.seenErr = n.shown(ctx, before)
	if w.seen {
		w.shaped, w.shapeErr = n.shape(ctx, before)
	}
	return w.after
}

// write is one saved write of an item: the item before it, whether visitors saw it and how they saw it.
type write struct {
	n        *Notifier
	before   content.Content
	seen     bool
	seenErr  error
	shaped   sdk.Item
	shapeErr error
}

// after tells the listeners what the write changed, after nil for a delete, then each published child it moved.
func (w *write) after(ctx context.Context, after *content.Content) {
	w.tellOwn(ctx, after)
	if after != nil {
		w.n.tellMoved(ctx, w.before, *after)
	}
}

// tellOwn tells the listeners the change the write made to its own item, or logs the read that kept it from them.
func (w *write) tellOwn(ctx context.Context, after *content.Content) {
	shown, err := w.n.shownAfter(ctx, after)
	if err == nil {
		err = w.seenErr
	}
	if err != nil {
		w.n.warn(statusChange(after), err)
		return
	}
	if shown {
		w.n.tellPublished(ctx, *after, w.previousPath(*after))
		return
	}
	if w.seen {
		w.n.tell(ctx, sdk.ContentChange{Kind: gone(after), Item: w.shaped}, w.shapeErr)
	}
}

// previousPath returns the path visitors reached the item at before the write moved it, empty when it did not.
func (w *write) previousPath(after content.Content) string {
	if !w.seen || w.before.Path == after.Path {
		return ""
	}
	return w.before.Path
}

// statusChange returns the change the status after the write points to, after nil for a delete.
func statusChange(after *content.Content) sdk.ChangeKind {
	if after != nil && after.Status == content.StatusPublished {
		return sdk.Published
	}
	return gone(after)
}

// gone returns the change of an item visitors saw and no longer see, after nil for a delete.
func gone(after *content.Content) sdk.ChangeKind {
	if after == nil {
		return sdk.Deleted
	}
	if after.Status == content.StatusTrash {
		return sdk.Trashed
	}
	return sdk.Unpublished
}

// tellMoved tells the listeners of each published child the write moved with its parent, at the path it left.
func (n *Notifier) tellMoved(ctx context.Context, before, after content.Content) {
	if before.Path == after.Path {
		return
	}
	children, err := n.movedChildren(ctx, after)
	if err != nil {
		n.settings.Logger.Warn(unlistedLine, "parent", after.Path, "error", err)
		return
	}
	for _, child := range children {
		n.tellChild(ctx, child.ID, before, after)
	}
}

// movedChildren returns the published items the move of parent carried and no write changed since, maybe none.
func (n *Notifier) movedChildren(ctx context.Context, parent content.Content) ([]content.Content, error) {
	readCtx, cancel := context.WithTimeout(ctx, n.settings.Budget)
	defer cancel()
	count, err := n.settings.Items.Children(readCtx, parent.ID)
	if err != nil || count == 0 {
		return nil, err
	}
	published, err := n.published(readCtx, parent.Type)
	var under []content.Content
	for _, item := range published {
		if strings.HasPrefix(item.Path, parent.Path+"/") && item.UpdatedAt.Equal(parent.UpdatedAt) {
			under = append(under, item)
		}
	}
	return under, err
}

// published returns every published item of the type key, a page at a time.
func (n *Notifier) published(ctx context.Context, key string) ([]content.Content, error) {
	var all []content.Content
	for page := 1; ; page++ {
		listed, total, err := n.settings.Items.List(ctx, content.Filter{
			Type: key, Statuses: []content.Status{content.StatusPublished}, Page: page, PerPage: content.MaxPerPage,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range listed {
			all = append(all, item.Content)
		}
		if len(listed) == 0 || len(all) >= total {
			return all, nil
		}
	}
}

// tellChild tells the listeners the child id is published where the move from before to after carried it.
func (n *Notifier) tellChild(ctx context.Context, id uuid.UUID, before, after content.Content) {
	holdCtx, cancel := context.WithTimeout(ctx, n.settings.Budget)
	release, held := n.holdWithin(holdCtx, id)
	cancel()
	if !held {
		n.warn(sdk.Published, errHeldPastBudget)
		return
	}
	defer release()
	child, shown, err := n.carried(ctx, id, after.UpdatedAt)
	if err != nil {
		n.warn(sdk.Published, err)
		return
	}
	if shown {
		n.tellPublished(ctx, child, before.Path+strings.TrimPrefix(child.Path, after.Path))
	}
}

// carried returns the child id as stored, and whether visitors see it where the move stamped moved left it.
func (n *Notifier) carried(ctx context.Context, id uuid.UUID, moved time.Time) (content.Content, bool, error) {
	readCtx, cancel := context.WithTimeout(ctx, n.settings.Budget)
	child, err := n.settings.Items.ByID(readCtx, id)
	cancel()
	if errors.Is(err, content.ErrNotFound) {
		return content.Content{}, false, nil
	}
	if err != nil || !child.UpdatedAt.Equal(moved) {
		return content.Content{}, false, err
	}
	shown, err := n.shown(ctx, child)
	return child, shown, err
}

// tellPublished shapes c and tells the listeners it is published, after the path it left when it moved.
func (n *Notifier) tellPublished(ctx context.Context, c content.Content, previous string) {
	item, err := n.shape(ctx, c)
	n.tell(ctx, sdk.ContentChange{Kind: sdk.Published, Item: item, PreviousPath: previous}, err)
}

// tell hands the listeners the change, or logs the error that kept its item from them.
func (n *Notifier) tell(ctx context.Context, change sdk.ContentChange, err error) {
	if err != nil {
		n.warn(change.Kind, err)
		return
	}
	n.settings.Listeners.ContentChanged(ctx, change)
}

// warn logs the change of kind the listeners were not told of, and the error that kept it from them.
func (n *Notifier) warn(kind sdk.ChangeKind, err error) {
	n.settings.Logger.Warn(unshapedLine, "change", kind, "error", err)
}

// shownAfter reports whether visitors see the item after the write, after nil for a delete.
func (n *Notifier) shownAfter(ctx context.Context, after *content.Content) (bool, error) {
	if after == nil {
		return false, nil
	}
	return n.shown(ctx, *after)
}

// shown reports whether visitors see c, published under a type the site serves, or the error that left it unknown.
func (n *Notifier) shown(ctx context.Context, c content.Content) (bool, error) {
	if c.Status != content.StatusPublished {
		return false, nil
	}
	readCtx, cancel := context.WithTimeout(ctx, n.settings.Budget)
	defer cancel()
	_, err := n.settings.Types.Active(readCtx, c.Type)
	if errors.Is(err, content.ErrTypeInactive) || errors.Is(err, content.ErrTypeNotFound) {
		return false, nil
	}
	return err == nil, err
}

// shape returns the item plugins read of c, read within the budget.
func (n *Notifier) shape(ctx context.Context, c content.Content) (sdk.Item, error) {
	readCtx, cancel := context.WithTimeout(ctx, n.settings.Budget)
	defer cancel()
	return n.settings.Shaper.Item(readCtx, c)
}
