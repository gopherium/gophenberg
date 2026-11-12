// SPDX-License-Identifier: Apache-2.0

// Package contentchanges tells plugins when a saved write changes what visitors see.
package contentchanges

import (
	"context"
	"log/slog"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/sdk"
)

// unshapedLine is the warning the site logs for each change it could not shape for plugins.
const unshapedLine = "plugins not told of a content change"

// Listeners hears the content changes of a running site.
type Listeners interface {
	Listens() bool
	ContentChanged(ctx context.Context, change sdk.ContentChange)
}

// Shaper shapes a stored item into the item plugins read.
type Shaper interface {
	Item(ctx context.Context, c content.Content) (sdk.Item, error)
}

// Notifier tells the listeners what each saved write changed of what visitors see.
type Notifier struct {
	listeners Listeners
	shaper    Shaper
	logger    *slog.Logger
}

// New returns a [Notifier] telling listeners, shaping items through shaper and logging to logger.
func New(listeners Listeners, shaper Shaper, logger *slog.Logger) *Notifier {
	return &Notifier{listeners: listeners, shaper: shaper, logger: logger}
}

// Saved tells the listeners what the write from before to after changed for visitors, after nil for a delete.
func (n *Notifier) Saved(ctx context.Context, before content.Content, after *content.Content) {
	if !n.listeners.Listens() {
		return
	}
	kind, shown, changed := kindOf(before, after)
	if !changed {
		return
	}
	item, err := n.shaper.Item(ctx, shown)
	if err != nil {
		n.logger.Warn(unshapedLine, "id", shown.ID, "change", kind, "error", err)
		return
	}
	n.listeners.ContentChanged(ctx, sdk.ContentChange{Kind: kind, Item: item})
}

// kindOf returns how the write from before to after changed what visitors see, the item to tell, and whether it did.
func kindOf(before content.Content, after *content.Content) (sdk.ChangeKind, content.Content, bool) {
	seen := before.Status == content.StatusPublished
	if after == nil {
		return sdk.Deleted, before, seen
	}
	if after.Status == content.StatusPublished {
		return sdk.Published, *after, true
	}
	if after.Status == content.StatusTrash {
		return sdk.Trashed, before, seen
	}
	return sdk.Unpublished, before, seen
}
