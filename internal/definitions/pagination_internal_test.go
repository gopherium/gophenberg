// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestImportKeepsPageReservedUnlessTheTypeAlreadyUsesIt(t *testing.T) {
	t.Parallel()

	declaration := TypeDefinition{
		Key: "chapter", SingularLabel: "Chapter", PluralLabel: "Chapters",
		RouteWord: "page", PageKind: "single", Active: true,
	}
	for _, test := range []struct {
		name   string
		stored []content.Type
		want   error
	}{
		{name: "new type", want: content.ErrRouteWordReserved},
		{
			name: "changed route word", want: content.ErrRouteWordReserved,
			stored: []content.Type{{Key: "chapter", RouteWord: "chapters"}},
		},
		{
			name:   "unchanged legacy route word",
			stored: []content.Type{{Key: "chapter", RouteWord: "page"}},
		},
		{
			name: "new type beside a legacy page type", want: content.ErrRouteWordReserved,
			stored: []content.Type{{Key: "other", RouteWord: "page"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateTypes([]TypeDefinition{declaration}, test.stored)
			if !errors.Is(err, test.want) {
				t.Errorf("validateTypes() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestImportNamesTheTypeWhoseLabelGivesAReservedWord(t *testing.T) {
	t.Parallel()

	declaration := TypeDefinition{
		Key: "chapter", SingularLabel: "Chapter", PluralLabel: "Media",
		PageKind: "single", Default: true, Active: true,
	}
	err := validateTypes([]TypeDefinition{declaration}, nil)

	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Errorf("validateTypes() error = %v, want %v", err, content.ErrRouteWordReserved)
	}
	if code, _ := content.CodeOf(err); code != "derived_route_word_reserved" {
		t.Errorf("code = %q, want derived_route_word_reserved", code)
	}
	if held, _ := content.DetailsOf(err); held["type"] != "chapter" || held["name"] != "Media" || held["word"] != "media" {
		t.Errorf("details = %v, want the type chapter named %q and the word %q", held, "Media", "media")
	}
}
