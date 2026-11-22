// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// typeRootWordCases are the cases over the route word a type takes when it hands the root over.
var typeRootWordCases = []Case{
	{"UpdateHandsTheRootToATypeWithAnUnusableLabel", updateHandsTheRootToATypeWithAnUnusableLabel},
	{"UpdateHandsTheRootToATypeWithAnUnderscoredLabel", updateHandsTheRootToATypeWithAnUnderscoredLabel},
	{"UpdateRefusesToHandTheRootOntoATakenKey", updateRefusesToHandTheRootOntoATakenKey},
	{"UpdateKeepsAPageKeyedRootOffThePageWord", updateKeepsAPageKeyedRootOffThePageWord},
	{"UpdateRefusesAHandOverWhoseOldRootKeyIsReserved", updateRefusesAHandOverWhoseOldRootKeyIsReserved},
}

// relabelPost stores the built-in post type under new labels.
func relabelPost(t *testing.T, types content.TypeStore, singular, plural string) {
	t.Helper()
	relabelled := PostType()
	relabelled.SingularLabel, relabelled.PluralLabel = singular, plural
	relabelled.UpdatedAt = time.Now().UTC()
	if _, err := types.Update(t.Context(), relabelled); err != nil {
		t.Fatalf("relabelling the default type: %v", err)
	}
}

// rootPage asks the store to hand the root to the page type and returns what it reports.
func rootPage(t *testing.T, types content.TypeStore) (content.Type, error) {
	t.Helper()
	promoted := PageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()
	return types.Update(t.Context(), promoted)
}

// updateHandsTheRootToATypeWithAnUnusableLabel gives the demoted type its key as a fallback route word.
func updateHandsTheRootToATypeWithAnUnusableLabel(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	relabelPost(t, s.Types, "3D Model", "3D Models")
	item := StoreItem(t, s.Content, PostType(), nil, "Hello World", author)

	if _, err := rootPage(t, s.Types); err != nil {
		t.Fatalf("handing the root to the page type: %v", err)
	}

	post, err := s.Types.ByKey(t.Context(), content.TypePost)
	if err != nil {
		t.Fatalf("ByKey(post) error = %v, want nil", err)
	}
	if post.Default || post.RouteWord != content.TypePost {
		t.Errorf("post = %+v, want demoted type using its key as route word", post)
	}
	page, err := s.Types.ByKey(t.Context(), "page")
	if err != nil {
		t.Fatalf("ByKey(page) error = %v, want nil", err)
	}
	if !page.Default {
		t.Errorf("page = %+v, want page type holding the root", page)
	}
	if got := AddressOf(t, s.Content, item.ID); got != "post/hello-world" {
		t.Errorf("post address = %q, want it carried under the type key", got)
	}
}

// updateHandsTheRootToATypeWithAnUnderscoredLabel moves the default under its label with the underscores broken.
func updateHandsTheRootToATypeWithAnUnderscoredLabel(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	relabelPost(t, s.Types, "FAQ_Item", "FAQ_Items")
	post := StoreItem(t, s.Content, PostType(), nil, "Hello World", author)

	if _, err := rootPage(t, s.Types); err != nil {
		t.Fatalf("handing the root to the page type: %v", err)
	}

	if held, err := s.Types.ByKey(t.Context(), content.TypePost); err != nil || held.RouteWord != "faq-items" {
		t.Errorf("post = %+v, %v, want it under faq-items", held, err)
	}
	if got := AddressOf(t, s.Content, post.ID); got != "faq-items/hello-world" {
		t.Errorf("post address = %q, want it carried under faq-items", got)
	}
}

// updateRefusesToHandTheRootOntoATakenKey refuses a hand over whose fallback key another type holds as its route word.
func updateRefusesToHandTheRootOntoATakenKey(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	van, err := content.NewType("van", "Van", "Vans", content.TypePost)
	if err != nil {
		t.Fatalf("NewType(van) error = %v, want nil", err)
	}
	if _, err := s.Types.Create(t.Context(), van); err != nil {
		t.Fatalf("Create(van) error = %v, want nil", err)
	}
	RegisterPageType(t, s.Types)
	relabelPost(t, s.Types, "3D Model", "3D Models")
	about := MustNest(t, s.Content, nil, "About", author)

	_, err = rootPage(t, s.Types)

	if !errors.Is(err, content.ErrRouteWordTaken) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrRouteWordTaken)
	}
	derivedWordTaken(t, err, content.TypePost, "3D Models", content.TypePost)
	if held, err := s.Types.ByKey(t.Context(), content.TypePost); err != nil || !held.Default || held.RouteWord != "" {
		t.Errorf("post = %+v, %v, want it still holding the root", held, err)
	}
	if held, err := s.Types.ByKey(t.Context(), "van"); err != nil || held.RouteWord != content.TypePost {
		t.Errorf("van = %+v, %v, want it still under post", held, err)
	}
	if got := AddressOf(t, s.Content, about.ID); got != "pages/about" {
		t.Errorf("page address = %q, want the refused hand over to have moved nothing", got)
	}
}

// updateKeepsAPageKeyedRootOffThePageWord refuses to move a page-keyed default under page, the paging word.
func updateKeepsAPageKeyedRootOffThePageWord(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	rooted, err := rootPage(t, s.Types)
	if err != nil {
		t.Fatalf("handing the root to the page type: %v", err)
	}
	rooted.SingularLabel, rooted.PluralLabel = "3D Page", "3D Pages"
	rooted.UpdatedAt = time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), rooted); err != nil {
		t.Fatalf("relabelling the page type: %v", err)
	}
	promoted := PostType()
	promoted.UpdatedAt = time.Now().UTC()

	_, err = s.Types.Update(t.Context(), promoted)

	if !errors.Is(err, content.ErrInvalidRouteWord) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrInvalidRouteWord)
	}
	derivedWordMalformed(t, err, "page", "3D Pages", "3d-pages")
	if held, err := s.Types.ByKey(t.Context(), "page"); err != nil || !held.Default || held.RouteWord != "" {
		t.Errorf("page = %+v, %v, want it still holding the root", held, err)
	}
	if held, err := s.Types.ByKey(t.Context(), content.TypePost); err != nil || held.Default || held.RouteWord != "posts" {
		t.Errorf("post = %+v, %v, want it still under posts", held, err)
	}
	if got := AddressOf(t, s.Content, about.ID); got != "about" {
		t.Errorf("page address = %q, want the refused hand over to have moved nothing", got)
	}
}

// updateRefusesAHandOverWhoseOldRootKeyIsReserved refuses a hand over whose fallback key is a word the CMS keeps.
func updateRefusesAHandOverWhoseOldRootKeyIsReserved(t *testing.T, s Stores) {
	files, err := content.NewType("media", "3D File", "3D Files", "files")
	if err != nil {
		t.Fatalf("NewType(media) error = %v, want nil", err)
	}
	stored, err := s.Types.Create(t.Context(), files)
	if err != nil {
		t.Fatalf("Create(media) error = %v, want nil", err)
	}
	stored.Default, stored.UpdatedAt = true, time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), stored); err != nil {
		t.Fatalf("handing the root to the media type: %v", err)
	}
	RegisterPageType(t, s.Types)

	_, err = rootPage(t, s.Types)

	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrRouteWordReserved)
	}
	derivedWordReserved(t, err, "media", "3D Files", "media")
	if held, err := s.Types.ByKey(t.Context(), "media"); err != nil || !held.Default || held.RouteWord != "" {
		t.Errorf("media = %+v, %v, want it still holding the root", held, err)
	}
	if held, err := s.Types.ByKey(t.Context(), "page"); err != nil || held.Default || held.RouteWord != "pages" {
		t.Errorf("page = %+v, %v, want it still under pages", held, err)
	}
}
