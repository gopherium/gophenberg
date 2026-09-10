// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/contentbridge"
	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/served"
	"github.com/gopherium/gophenberg/internal/server"
)

// bridgedFields returns the values a plugin reads for the newest published post.
func bridgedFields(
	t *testing.T, posts content.Store, types content.TypeStore, library media.Store,
) map[string]any {
	t.Helper()
	reader := contentbridge.New(posts, content.NewRegistry(types), library)
	held, err := reader.ListPublished(t.Context(), content.TypePost, 10)
	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}
	if len(held) == 0 {
		t.Fatal("ListPublished() returned nothing, want the published post")
	}
	return held[0].Fields
}

func TestBothPublicSeamsAnswerTheSameFields(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	posts, types := newFakePostStore(), newFakeTypeStore()
	posts.declared = types
	library := newFakeMediaStore()
	stored, err := library.Create(t.Context(),
		media.Media{File: "2026/08/sunrise.jpg", Title: "Sunrise", MimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("storing the file: %v, want nil", err)
	}
	handler := authedServerWithStores(t, server.Config{
		Users: users, Content: posts, Types: types, MediaStore: library,
	})
	declaredRelation(t, handler)
	declaredOn(t, handler, `{"key":"cover","label":"Cover","kind":"media"}`)
	declaredOn(t, handler, `{"key":"venue","label":"Venue","kind":"text"}`)
	news := storedCategoryItem(t, handler)
	held := patchValues(t, handler, draftedPost(t, handler), fmt.Sprintf(
		`{"categories":[%q],"cover":%d,"venue":"Hall"}`, news.ID, stored.ID))
	publishItemAt(t, handler, held)
	publishItemAt(t, handler, news)

	shown := resolvedFields(t, handler, "hello-world")
	bridged := bridgedFields(t, posts, types, library)

	if len(bridged) != len(shown) {
		t.Fatalf("the plugin reads %v, the theme reads %v, want the same keys", bridged, shown)
	}
	for key, answered := range shown {
		sameServedValue(t, key, bridged[key], answered)
	}
}

// sameServedValue reports whether a plugin reads one key exactly as the public answer serves it.
func sameServedValue(t *testing.T, key string, bridged any, answered json.RawMessage) {
	t.Helper()
	held, err := json.Marshal(bridged)
	if err != nil {
		t.Fatalf("reading what the plugin holds under %s: %v", key, err)
	}
	var one, other any
	if err := json.Unmarshal(held, &one); err != nil {
		t.Fatalf("reading what the plugin holds under %s: %v", key, err)
	}
	if err := json.Unmarshal(answered, &other); err != nil {
		t.Fatalf("reading what the theme reads under %s: %v", key, err)
	}
	if fmt.Sprintf("%v", one) != fmt.Sprintf("%v", other) {
		t.Errorf("under %s the plugin reads %s and the theme reads %s", key, held, answered)
	}
}

func TestAPluginReadsNoValueTheRulesHideFromATheme(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	posts, types := newFakePostStore(), newFakeTypeStore()
	posts.declared = types
	handler := authedServerWithStores(t, server.Config{Users: users, Content: posts, Types: types})
	declaredOn(t, handler, `{"key":"on-sale","label":"On sale","kind":"boolean"}`)
	declaredOn(t, handler, `{"key":"sale-note","label":"Sale note","kind":"text","settings":`+
		`{"conditions":[[{"source":"on-sale","operator":"==","value":"true"}]]}}`)
	held := patchValues(t, handler, draftedPost(t, handler),
		`{"on-sale":true,"sale-note":"Half price"}`)
	publishItemAt(t, handler, patchValues(t, handler, held, `{"on-sale":false}`))

	bridged := bridgedFields(t, posts, types, nil)

	if _, carried := bridged["sale-note"]; carried {
		t.Errorf("the plugin reads %v, want the hidden note kept back as a theme has it kept back", bridged)
	}
}

// noteHandler returns a handler and its stores, ready to answer both public seams.
func noteHandler(t *testing.T) (http.Handler, *fakePostStore, *fakeTypeStore) {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	posts, types := newFakePostStore(), newFakeTypeStore()
	posts.declared = types
	return authedServerWithStores(t,
		server.Config{Users: users, Content: posts, Types: types}), posts, types
}

func TestAPluginReadsTheItemsPointingAtOneAsAThemeDoes(t *testing.T) {
	t.Parallel()

	handler, posts, types := noteHandler(t)
	declaredRelation(t, handler)
	if recorder := doRequest(t, handler, http.MethodPatch, "/api/types/category",
		`{"page_kind":"archive"}`); recorder.Code != http.StatusOK {
		t.Fatalf("serving term pages: %d: %s", recorder.Code, recorder.Body.String())
	}
	declaredOnType(t, handler, "category",
		`{"key":"picks","label":"Picks","kind":"relation","relates_to":"post","many":true}`)
	declaredOn(t, handler, fmt.Sprintf(
		`{"key":"linked-from","label":"Linked from","kind":"backlinks","settings":`+
			`{"source_group":%q,"source_field":["picks"]}}`, groupKeyOver(t, handler)))
	post := publishItemAt(t, handler, draftedPost(t, handler))
	publishItemAt(t, handler, patchValues(t, handler, storedCategoryItem(t, handler),
		fmt.Sprintf(`{"picks":[%q]}`, post.ID)))

	bridged := bridgedFields(t, posts, types, nil)

	held, listed := bridged["linked-from"].([]served.Pointer)
	if !listed || len(held) != 1 || held[0].Title != "News" || held[0].Type != "category" {
		t.Errorf("the plugin reads %#v, want the category pointing at the post", bridged["linked-from"])
	}
}
