// SPDX-License-Identifier: Apache-2.0

package post_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/post"
)

// newDraft returns a draft post for transition tests.
func newDraft(t *testing.T) post.Post {
	t.Helper()
	p, err := post.New("post", "Hello World", uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	return p
}

func TestParseStatusAcceptsEveryStoredStatus(t *testing.T) {
	t.Parallel()

	for _, want := range []post.Status{
		post.StatusDraft,
		post.StatusPending,
		post.StatusPrivate,
		post.StatusScheduled,
		post.StatusPublished,
		post.StatusTrash,
	} {
		got, err := post.ParseStatus(string(want))

		if err != nil {
			t.Errorf("ParseStatus(%q) error = %v, want nil", want, err)
		}
		if got != want {
			t.Errorf("ParseStatus(%q) = %q, want %q", want, got, want)
		}
	}
}

func TestParseStatusRejectsUnknownValues(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "publsh", "DRAFT", "deleted"} {
		if _, err := post.ParseStatus(in); !errors.Is(err, post.ErrInvalidStatus) {
			t.Errorf("ParseStatus(%q) error = %v, want %v", in, err, post.ErrInvalidStatus)
		}
	}
}

func TestTransitionMatrix(t *testing.T) {
	t.Parallel()

	editable := []post.Status{post.StatusDraft, post.StatusPending, post.StatusPrivate, post.StatusPublished}
	all := append(append([]post.Status{}, editable...), post.StatusScheduled, post.StatusTrash)

	for _, from := range all {
		for _, to := range all {
			from, to := from, to
			t.Run(string(from)+" to "+string(to), func(t *testing.T) {
				t.Parallel()

				p := newDraft(t)
				p.Status = from

				err := p.Transition(to)

				if want := wantsTransition(from, to); want && err != nil {
					t.Errorf("Transition(%q from %q) error = %v, want nil", to, from, err)
				} else if !want && !errors.Is(err, post.ErrInvalidTransition) {
					t.Errorf("Transition(%q from %q) error = %v, want %v", to, from, err, post.ErrInvalidTransition)
				}
			})
		}
	}
}

// wantsTransition reports whether the cycle allows moving from one status to another.
func wantsTransition(from, to post.Status) bool {
	if from == to {
		return true
	}
	if to == post.StatusScheduled || from == post.StatusTrash {
		return false
	}
	return true
}

func TestTransitionStampsPublishedAtOnce(t *testing.T) {
	t.Parallel()

	p := newDraft(t)

	if err := p.Transition(post.StatusPublished); err != nil {
		t.Fatalf("publishing: %v", err)
	}
	if p.PublishedAt == nil {
		t.Fatal("PublishedAt is nil after publishing, want a stamped instant")
	}
	first := *p.PublishedAt

	if err := p.Transition(post.StatusDraft); err != nil {
		t.Fatalf("unpublishing: %v", err)
	}
	if p.PublishedAt == nil || !p.PublishedAt.Equal(first) {
		t.Errorf("PublishedAt = %v after unpublishing, want it preserved as %v", p.PublishedAt, first)
	}

	if err := p.Transition(post.StatusPublished); err != nil {
		t.Fatalf("republishing: %v", err)
	}
	if !p.PublishedAt.Equal(first) {
		t.Errorf("PublishedAt = %v after republishing, want the original %v", p.PublishedAt, first)
	}
}

func TestTransitionBumpsUpdatedAt(t *testing.T) {
	t.Parallel()

	p := newDraft(t)
	p.UpdatedAt = p.UpdatedAt.Add(-time.Hour)
	before := p.UpdatedAt

	if err := p.Transition(post.StatusPending); err != nil {
		t.Fatalf("Transition() error = %v, want nil", err)
	}

	if !p.UpdatedAt.After(before) {
		t.Errorf("UpdatedAt = %v, want an instant after %v", p.UpdatedAt, before)
	}
	if !p.CreatedAt.Equal(p.CreatedAt.UTC()) {
		t.Error("CreatedAt changed on transition, want it untouched")
	}
}

func TestTransitionToAnUnknownStatusFails(t *testing.T) {
	t.Parallel()

	p := newDraft(t)

	if err := p.Transition(post.Status("bogus")); !errors.Is(err, post.ErrInvalidStatus) {
		t.Errorf("Transition() error = %v, want %v", err, post.ErrInvalidStatus)
	}
}

func TestRestoreReturnsATrashedPostToDraft(t *testing.T) {
	t.Parallel()

	p := newDraft(t)
	if err := p.Transition(post.StatusPublished); err != nil {
		t.Fatalf("publishing: %v", err)
	}
	published := *p.PublishedAt
	if err := p.Transition(post.StatusTrash); err != nil {
		t.Fatalf("trashing: %v", err)
	}
	p.UpdatedAt = p.UpdatedAt.Add(-time.Hour)

	if err := p.Restore(); err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}

	if p.Status != post.StatusDraft {
		t.Errorf("Status = %q, want %q", p.Status, post.StatusDraft)
	}
	if p.PublishedAt == nil || !p.PublishedAt.Equal(published) {
		t.Errorf("PublishedAt = %v, want it preserved as %v", p.PublishedAt, published)
	}
	if !p.UpdatedAt.After(p.CreatedAt) {
		t.Error("UpdatedAt was not bumped by Restore()")
	}
}

func TestRestoreRejectsPostsThatAreNotTrashed(t *testing.T) {
	t.Parallel()

	p := newDraft(t)

	if err := p.Restore(); !errors.Is(err, post.ErrInvalidTransition) {
		t.Errorf("Restore() error = %v, want %v", err, post.ErrInvalidTransition)
	}
}
