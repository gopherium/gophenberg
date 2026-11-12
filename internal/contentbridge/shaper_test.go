// SPDX-License-Identifier: Apache-2.0

package contentbridge_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/contentbridge"
	"github.com/gopherium/gophenberg/sdk"
)

func TestShaperShapesAStoredItemAsPluginsReadIt(t *testing.T) {
	t.Parallel()

	stored := publishedPost("Spring workshop", `<p onclick="alert(1)">Body</p>`)
	stored.Fields = content.Values{"venue": "Hall", "seats": float64(40)}
	stored.Status = content.StatusTrash

	got, err := contentbridge.NewShaper(&recordingPostStore{}, typedFields{}, nil, nil).Item(t.Context(), stored)

	want := sdk.Item{
		ID: sdk.ID(stored.ID), Type: stored.Type, Path: stored.Path, Slug: stored.Slug, Title: "Spring workshop",
		Excerpt: stored.Excerpt, Content: "<p>Body</p>", Fields: map[string]any{"venue": "Hall", "seats": float64(40)},
		PublishedAt: *stored.PublishedAt, UpdatedAt: stored.UpdatedAt,
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Item() = %+v, %v, want %+v", got, err, want)
	}
}

func TestShaperReportsATypeItCannotRead(t *testing.T) {
	t.Parallel()

	stored := publishedPost("Spring workshop", "<p>Body</p>")
	gone := errors.New("the registry is unreachable")

	_, err := contentbridge.NewShaper(&recordingPostStore{}, typedFields{err: gone}, nil, nil).Item(t.Context(), stored)

	if !errors.Is(err, gone) || err.Error() != "contentbridge: read the type post: the registry is unreachable" {
		t.Errorf("Item() = %v, want the type named and the failure wrapped", err)
	}
}
