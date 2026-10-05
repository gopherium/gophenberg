// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// newDraft returns a draft post for transition tests.
func newDraft(t *testing.T) content.Content {
	t.Helper()
	p, err := content.New(postType(), nil, "Hello World", uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	return p
}

func TestParseStatusAcceptsEveryStoredStatus(t *testing.T) {
	t.Parallel()

	for _, want := range []content.Status{
		content.StatusDraft,
		content.StatusPending,
		content.StatusPrivate,
		content.StatusScheduled,
		content.StatusPublished,
		content.StatusTrash,
	} {
		got, err := content.ParseStatus(string(want))

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
		if _, err := content.ParseStatus(in); !errors.Is(err, content.ErrInvalidStatus) {
			t.Errorf("ParseStatus(%q) error = %v, want %v", in, err, content.ErrInvalidStatus)
		}
	}
}

func TestParseStatusesReadsAListSeparatedByCommas(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string][]content.Status{
		"draft":               {content.StatusDraft},
		"draft,published":     {content.StatusDraft, content.StatusPublished},
		" pending , private ": {content.StatusPending, content.StatusPrivate},
		"draft,pending,private,scheduled,published": {
			content.StatusDraft, content.StatusPending, content.StatusPrivate,
			content.StatusScheduled, content.StatusPublished,
		},
	} {
		got, err := content.ParseStatuses(raw)

		if err != nil || !slices.Equal(got, want) {
			t.Errorf("ParseStatuses(%q) = %v, %v, want %v", raw, got, err, want)
		}
	}
}

func TestParseStatusesRefusesAListNamingAnUnknownOrEmptyStatus(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"draft,lost", "draft,,published", ",", "Draft", "draft,"} {
		if _, err := content.ParseStatuses(raw); !errors.Is(err, content.ErrInvalidStatus) {
			t.Errorf("ParseStatuses(%q) error = %v, want %v", raw, err, content.ErrInvalidStatus)
		}
	}
}

func TestTransitionMatrix(t *testing.T) {
	t.Parallel()

	editable := []content.Status{
		content.StatusDraft, content.StatusPending, content.StatusPrivate, content.StatusPublished,
	}
	all := append(append([]content.Status{}, editable...), content.StatusScheduled, content.StatusTrash)

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
				} else if !want && !errors.Is(err, content.ErrInvalidTransition) {
					t.Errorf("Transition(%q from %q) error = %v, want %v", to, from, err, content.ErrInvalidTransition)
				}
			})
		}
	}
}

// wantsTransition reports whether the cycle allows moving from one status to another.
func wantsTransition(from, to content.Status) bool {
	if from == to {
		return true
	}
	if to == content.StatusScheduled || to == content.StatusTrash || from == content.StatusTrash {
		return false
	}
	return true
}

func TestTransitionStampsPublishedAtOnce(t *testing.T) {
	t.Parallel()

	p := newDraft(t)

	if err := p.Transition(content.StatusPublished); err != nil {
		t.Fatalf("publishing: %v", err)
	}
	if p.PublishedAt == nil {
		t.Fatal("PublishedAt is nil after publishing, want a stamped instant")
	}
	first := *p.PublishedAt

	if err := p.Transition(content.StatusDraft); err != nil {
		t.Fatalf("unpublishing: %v", err)
	}
	if p.PublishedAt == nil || !p.PublishedAt.Equal(first) {
		t.Errorf("PublishedAt = %v after unpublishing, want it preserved as %v", p.PublishedAt, first)
	}

	if err := p.Transition(content.StatusPublished); err != nil {
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

	if err := p.Transition(content.StatusPending); err != nil {
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

	if err := p.Transition(content.Status("bogus")); !errors.Is(err, content.ErrInvalidStatus) {
		t.Errorf("Transition() error = %v, want %v", err, content.ErrInvalidStatus)
	}
}

func TestRestoreReturnsATrashedPostToDraft(t *testing.T) {
	t.Parallel()

	p := newDraft(t)
	if err := p.Transition(content.StatusPublished); err != nil {
		t.Fatalf("publishing: %v", err)
	}
	published := *p.PublishedAt
	p.Status = content.StatusTrash
	p.UpdatedAt = p.UpdatedAt.Add(-time.Hour)

	if err := p.Restore(); err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}

	if p.Status != content.StatusDraft {
		t.Errorf("Status = %q, want %q", p.Status, content.StatusDraft)
	}
	if p.PublishedAt == nil || !p.PublishedAt.Equal(published) {
		t.Errorf("PublishedAt = %v, want it preserved as %v", p.PublishedAt, published)
	}
	if !p.UpdatedAt.After(p.CreatedAt) {
		t.Error("UpdatedAt was not bumped by Restore()")
	}
}

func TestEditableRefusesAnItemInTheTrash(t *testing.T) {
	t.Parallel()

	p := newDraft(t)
	p.Status = content.StatusTrash

	if err := p.Editable(); !errors.Is(err, content.ErrTrashed) {
		t.Errorf("Editable() error = %v, want %v", err, content.ErrTrashed)
	}
}

func TestEditableAllowsAnItemOutOfTheTrash(t *testing.T) {
	t.Parallel()

	statuses := []content.Status{
		content.StatusDraft, content.StatusPending, content.StatusPrivate,
		content.StatusScheduled, content.StatusPublished,
	}
	for _, status := range statuses {
		p := newDraft(t)
		p.Status = status

		if err := p.Editable(); err != nil {
			t.Errorf("Editable() on %s error = %v, want nil", status, err)
		}
	}
}

func TestRestoreRejectsPostsThatAreNotTrashed(t *testing.T) {
	t.Parallel()

	p := newDraft(t)

	if err := p.Restore(); !errors.Is(err, content.ErrInvalidTransition) {
		t.Errorf("Restore() error = %v, want %v", err, content.ErrInvalidTransition)
	}
}
