// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestTheCategoryTypeServesTermPages(t *testing.T) {
	t.Parallel()

	held := CategoryType()

	if held.PageKind != content.PageKindArchive {
		t.Errorf("the category type serves %q pages, want %q", held.PageKind, content.PageKindArchive)
	}
	if held.RouteWord != "categories" || !held.Hierarchical || !held.Active {
		t.Errorf("the category type = %+v, want it nesting under categories and serving", held)
	}
	if err := held.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want the seeded type storable", err)
	}
}

func TestTheCategoriesFieldPointsPostsAtCategories(t *testing.T) {
	t.Parallel()

	held := CategoriesField()

	if held.TypeKey != content.TypePost || held.RelatesTo != CategoryTypeKey {
		t.Errorf("the field = %+v, want posts pointing at categories", held)
	}
	if held.Kind != content.FieldKindRelation || !held.Many {
		t.Errorf("the field = %+v, want a relation holding many", held)
	}
	if err := held.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want the seeded field storable", err)
	}
}

func TestTheSeededCategoryCarriesAFixedIdentity(t *testing.T) {
	t.Parallel()

	held := demoCategories()

	if len(held) != 1 {
		t.Fatalf("the seed scripts %d categories, want the one the demo files under", len(held))
	}
	if held[0].title != "News" {
		t.Errorf("the seeded category is %q, want News", held[0].title)
	}
	if held[0].id == "" || held[0].filed == "" {
		t.Errorf("the seeded category = %+v, want a fixed identity and a post to file", held[0])
	}
}

func TestTheSeededCategoryFilesAPublishedPost(t *testing.T) {
	t.Parallel()

	published := make(map[string]bool, len(demoPosts()))
	for _, scripted := range publishedDemoPosts() {
		published[scripted.id] = true
	}

	for _, scripted := range demoCategories() {
		if !published[scripted.filed] {
			t.Errorf("the category %q files %q, want a post the demo publishes", scripted.title, scripted.filed)
		}
	}
}
