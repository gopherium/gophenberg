// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"time"
)

// ErrInvalidStatus reports that a status value is not one the CMS stores.
var ErrInvalidStatus = errors.New("content: invalid status")

// ErrInvalidTransition reports that a status change is not allowed.
var ErrInvalidTransition = errors.New("content: invalid status transition")

// Status is the publication state of a content item.
type Status string

// The statuses a content item may hold. Scheduled is stored but unreachable
// until a publish worker exists.
const (
	StatusDraft     Status = "draft"
	StatusPending   Status = "pending"
	StatusPrivate   Status = "private"
	StatusScheduled Status = "scheduled"
	StatusPublished Status = "published"
	StatusTrash     Status = "trash"
)

// ParseStatus returns the [Status] named by value, or [ErrInvalidStatus].
func ParseStatus(value string) (Status, error) {
	switch status := Status(value); status {
	case StatusDraft, StatusPending, StatusPrivate, StatusScheduled, StatusPublished, StatusTrash:
		return status, nil
	default:
		return "", ErrInvalidStatus
	}
}

// Transition moves the content item to the given status, stamping PublishedAt
// on the first publication and returning [ErrInvalidTransition] when disallowed.
func (c *Content) Transition(to Status) error {
	if _, err := ParseStatus(string(to)); err != nil {
		return err
	}
	if !c.canTransition(to) {
		return ErrInvalidTransition
	}
	c.Status = to
	if to == StatusPublished && c.PublishedAt == nil {
		published := time.Now().UTC()
		c.PublishedAt = &published
	}
	c.UpdatedAt = time.Now().UTC()
	return nil
}

// canTransition reports whether the content item may move to the given status.
func (c *Content) canTransition(to Status) bool {
	if c.Status == to {
		return true
	}
	if to == StatusScheduled || c.Status == StatusTrash {
		return false
	}
	return true
}

// Restore returns a trashed content item to draft, or reports [ErrInvalidTransition].
func (c *Content) Restore() error {
	if c.Status != StatusTrash {
		return ErrInvalidTransition
	}
	c.Status = StatusDraft
	c.UpdatedAt = time.Now().UTC()
	return nil
}
