// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// spareTypeKey names the type a registry write reshapes while a content edit is in flight.
const spareTypeKey = "note"

// spareType returns an active type the registry can reshape without leaving the site without a default.
func spareType() content.Type {
	t := postType()
	t.Key, t.SingularLabel, t.PluralLabel = spareTypeKey, "Note", "Notes"
	t.RouteWord, t.Default = "notes", false
	return t
}

// newSpareItem returns a draft of the spare type authored by author.
func newSpareItem(t *testing.T, title string, author uuid.UUID) content.Content {
	t.Helper()
	c, err := content.New(spareType(), nil, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	return c
}

// refusalCode returns the code the recorded refusal carries.
func refusalCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var answered struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("reading the refusal: %v", err)
	}
	return answered.Code
}

// hookedPosts is a post store double running a hook when a handler reads through it.
type hookedPosts struct {
	*fakePostStore
	onByID  func(uuid.UUID)
	onDepth func()
}

// newHookedPosts returns an empty post store double whose hooks do nothing.
func newHookedPosts() *hookedPosts {
	return &hookedPosts{fakePostStore: newFakePostStore(), onByID: func(uuid.UUID) {}, onDepth: func() {}}
}

// ByID runs the hook and then answers with the stored item.
func (s *hookedPosts) ByID(ctx context.Context, id uuid.UUID) (content.Content, error) {
	s.onByID(id)
	return s.fakePostStore.ByID(ctx, id)
}

// Depth runs the hook and then answers how many levels nest below the item.
func (s *hookedPosts) Depth(ctx context.Context, id uuid.UUID) (int, error) {
	s.onDepth()
	return s.fakePostStore.Depth(ctx, id)
}

// hookedPostServer returns a signed in handler, the hookable post store, the type store, and the author.
func hookedPostServer(t *testing.T) (http.Handler, *hookedPosts, *fakeTypeStore, gouncer.User) {
	t.Helper()
	users := newFakeUserStore()
	author := addAda(t, users)
	posts := newHookedPosts()
	types := newFakeTypeStore()
	types.register(spareType())
	handler := server.NewServer(server.Config{Users: users, Content: posts, Types: types})
	cookie := loginCookie(t, handler)
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	})
	return authed, posts, types, author
}

// deactivateTheSpareType returns a hook switching the spare type off over the API, which drops the cached registry.
func deactivateTheSpareType(t *testing.T, handler http.Handler) func() {
	return func() {
		recorder := doRequest(t, handler, http.MethodPatch, "/api/types/"+spareTypeKey, `{"active":false}`)
		if recorder.Code != http.StatusOK {
			t.Errorf("switching %q off = %d, want %d: %s", spareTypeKey, recorder.Code,
				http.StatusOK, recorder.Body.String())
		}
	}
}

// breakTheRegistry returns a hook dropping the cached registry and then failing every read that refills it.
func breakTheRegistry(t *testing.T, handler http.Handler, types *fakeTypeStore) func() {
	deactivate := deactivateTheSpareType(t, handler)
	return func() {
		deactivate()
		types.listErr = errRegistryDown
	}
}

func TestPublishedListMasksARegistryItCannotRead(t *testing.T) {
	t.Parallel()

	types := newFakeTypeStore()
	types.listErr = errRegistryDown
	handler := server.NewServer(server.Config{Users: newFakeUserStore(), Content: newFakePostStore(), Types: types})

	recorder := getContent(t, handler, "/api/content/v1/items")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if code := refusalCode(t, recorder); code != "internal" {
		t.Errorf("code = %q, want the registry failure masked", code)
	}
}

func TestPublishedListAnswersWithoutATypeWhenTheRegistryReads(t *testing.T) {
	t.Parallel()

	handler := server.NewServer(server.Config{
		Users: newFakeUserStore(), Content: newFakePostStore(), Types: newFakeTypeStore(),
	})

	recorder := getContent(t, handler, "/api/content/v1/items")

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestAutosaveMasksARegistryItCannotRead(t *testing.T) {
	t.Parallel()

	handler, posts, types, author := typedPostServer(t)
	stored := posts.add(newPost(t, "Original", author.ID))
	types.listErr = errRegistryDown

	recorder := doRequest(t, handler, http.MethodPost, "/api/content/"+stored.ID.String()+"/autosave",
		versionedBody(t, stored.UpdatedAt, map[string]any{"fields": map[string]any{"subtitle": "A subtitle"}}))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if code := refusalCode(t, recorder); code != "internal" {
		t.Errorf("code = %q, want the registry failure masked", code)
	}
}

func TestPatchRefusesAnAddressAnotherTypeAnswersUnder(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	author := addAda(t, users)
	posts := newFakePostStore()
	types := newFakeTypeStore()
	types.register(pageType())
	handler := authedServerWithStores(t, server.Config{Users: users, Content: posts, Types: types})
	stored := posts.add(newPost(t, "Original", author.ID))

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+stored.ID.String(),
		versionedBody(t, stored.UpdatedAt, map[string]any{"slug": "pages"}))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
	if code := refusalCode(t, recorder); code != "address_reserved" {
		t.Errorf("code = %q, want the address kept for the other type", code)
	}
}

func TestNestingReportsATypeSwitchedOffWhileItReadsTheParent(t *testing.T) {
	t.Parallel()

	handler, posts, _, author := hookedPostServer(t)
	parent := posts.add(newSpareItem(t, "Parent", author.ID))
	stored := posts.add(newSpareItem(t, "Original", author.ID))
	deactivate := deactivateTheSpareType(t, handler)
	posts.onByID = func(id uuid.UUID) {
		if id == parent.ID {
			deactivate()
		}
	}

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+stored.ID.String(),
		versionedBody(t, stored.UpdatedAt, map[string]any{"parent_id": parent.ID.String()}))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
	if code := refusalCode(t, recorder); code != "type_inactive" {
		t.Errorf("code = %q, want the type switched off while the nesting was read", code)
	}
}

func TestPatchMasksARegistryDroppedWhileItChecksTheAddress(t *testing.T) {
	t.Parallel()

	handler, posts, types, author := hookedPostServer(t)
	parent := posts.add(newSpareItem(t, "Parent", author.ID))
	nested := newSpareItem(t, "Original", author.ID)
	nested.ParentID = &parent.ID
	stored := posts.add(nested)
	posts.onDepth = breakTheRegistry(t, handler, types)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+stored.ID.String(),
		versionedBody(t, stored.UpdatedAt, map[string]any{"parent_id": nil}))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if code := refusalCode(t, recorder); code != "internal" {
		t.Errorf("code = %q, want the registry failure masked", code)
	}
}

func TestPatchReportsATypeSwitchedOffWhileTheEditLands(t *testing.T) {
	t.Parallel()

	handler, posts, _, author := hookedPostServer(t)
	parent := posts.add(newSpareItem(t, "Parent", author.ID))
	nested := newSpareItem(t, "Original", author.ID)
	nested.ParentID = &parent.ID
	stored := posts.add(nested)
	posts.onDepth = deactivateTheSpareType(t, handler)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+stored.ID.String(),
		versionedBody(t, stored.UpdatedAt, map[string]any{"parent_id": nil}))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
	if code := refusalCode(t, recorder); code != "type_inactive" {
		t.Errorf("code = %q, want the type switched off before the revision was taken", code)
	}
}
