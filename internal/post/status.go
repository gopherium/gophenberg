// SPDX-License-Identifier: Apache-2.0

package post

import (
	"errors"
	"time"
)

// ErrInvalidStatus reports that a status value is not one the CMS stores.
var ErrInvalidStatus = errors.New("post: invalid status")

// ErrInvalidTransition reports that a status change is not allowed.
var ErrInvalidTransition = errors.New("post: invalid status transition")

// Status is the publication state of a post.
type Status string

// The statuses a post may hold. Scheduled is stored but unreachable until a
// publish worker exists.
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

// Transition moves the post to the given status, stamping PublishedAt on the
// first publication and returning [ErrInvalidTransition] when disallowed.
func (p *Post) Transition(to Status) error {
	if _, err := ParseStatus(string(to)); err != nil {
		return err
	}
	if !p.canTransition(to) {
		return ErrInvalidTransition
	}
	p.Status = to
	if to == StatusPublished && p.PublishedAt == nil {
		published := time.Now().UTC()
		p.PublishedAt = &published
	}
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// canTransition reports whether the post may move to the given status.
func (p *Post) canTransition(to Status) bool {
	if p.Status == to {
		return true
	}
	if to == StatusScheduled || p.Status == StatusTrash {
		return false
	}
	return true
}

// Restore returns a trashed post to draft, or reports [ErrInvalidTransition].
func (p *Post) Restore() error {
	if p.Status != StatusTrash {
		return ErrInvalidTransition
	}
	p.Status = StatusDraft
	p.UpdatedAt = time.Now().UTC()
	return nil
}
