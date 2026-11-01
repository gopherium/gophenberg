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

	for _, test := range []struct {
		name  string
		key   string
		label string
	}{
		{name: "as written", key: "chapter", label: "Media"},
		{name: "trimmed label", key: "chapter", label: " Media "},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			declaration := TypeDefinition{
				Key: test.key, SingularLabel: "Chapter", PluralLabel: test.label,
				PageKind: "single", Default: true, Active: true,
			}
			err := validateTypes([]TypeDefinition{declaration}, nil)

			if !errors.Is(err, content.ErrRouteWordReserved) {
				t.Errorf("validateTypes() error = %v, want %v", err, content.ErrRouteWordReserved)
			}
			if code, _ := content.CodeOf(err); code != "derived_route_word_reserved" {
				t.Errorf("code = %q, want derived_route_word_reserved", code)
			}
			held, _ := content.DetailsOf(err)
			if held["type"] != "chapter" || held["name"] != "Media" || held["word"] != "media" {
				t.Errorf("details = %v, want the type chapter named %q and the word %q", held, "Media", "media")
			}
		})
	}
}

func TestImportKeepsTheGenericErrorUnlessItDerivedTheWord(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		declaration TypeDefinition
		stored      []content.Type
		want        error
	}{
		{
			name: "new type stating a reserved word", want: content.ErrRouteWordReserved,
			declaration: TypeDefinition{
				Key: "chapter", SingularLabel: "Chapter", PluralLabel: "Chapters",
				RouteWord: "media", PageKind: "single", Active: true,
			},
		},
		{
			name: "stored root stating a reserved word", want: content.ErrRouteWordReserved,
			declaration: TypeDefinition{
				Key: "chapter", SingularLabel: "Chapter", PluralLabel: "Chapters",
				RouteWord: "media", PageKind: "single", Default: true, Active: true,
			},
			stored: []content.Type{{Key: "chapter", Default: true}},
		},
		{
			name: "new root with an unknown page kind", want: content.ErrInvalidPageKind,
			declaration: TypeDefinition{
				Key: "chapter", SingularLabel: "Chapter", PluralLabel: "Chapters",
				PageKind: "gallery", Default: true, Active: true,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateTypes([]TypeDefinition{test.declaration}, test.stored)
			if !errors.Is(err, test.want) {
				t.Errorf("validateTypes() error = %v, want %v", err, test.want)
			}
			if code, named := content.CodeOf(err); named {
				t.Errorf("code = %q, want none", code)
			}
		})
	}
}
