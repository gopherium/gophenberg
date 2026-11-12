// SPDX-License-Identifier: Apache-2.0

package sdk

import "context"

// ContentChangedHook is the name a plugin lists under hooks in its plugin.json when it implements ContentListener.
const ContentChangedHook = "content.changed"

// ChangeKind names how a saved write changed what visitors see of an item.
type ChangeKind string

// The changes a content listener hears.
const (
	// Published is an item visitors see after the write, new to them or edited.
	Published ChangeKind = "published"
	// Unpublished is an item visitors saw and no longer see, set back to draft, pending review, private or scheduled.
	Unpublished ChangeKind = "unpublished"
	// Trashed is an item visitors saw, now in the trash.
	Trashed ChangeKind = "trashed"
	// Deleted is an item visitors saw, now deleted for good.
	Deleted ChangeKind = "deleted"
)

// ContentChange is one saved write that changed what visitors see.
type ContentChange struct {
	Kind ChangeKind
	// Item is the item as visitors see it after a Published write, or as they saw it before the others.
	Item Item
}

// ContentListener is implemented by plugins that hear each saved write that changes what visitors see.
type ContentListener interface {
	ContentChanged(ctx context.Context, change ContentChange) error
}
