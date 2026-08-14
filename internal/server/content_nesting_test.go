// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// pageType returns a hierarchical page type answering under pages.
func pageType() content.Type {
	t := postType()
	t.Key, t.SingularLabel, t.PluralLabel = "page", "Page", "Pages"
	t.RouteWord, t.Hierarchical, t.Default = "pages", true, false
	return t
}

// authedNestingServer returns a signed-in handler over a registry holding a hierarchical type.
func authedNestingServer(t *testing.T) http.Handler {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	types := newFakeTypeStore()
	types.register(pageType())
	posts := newFakePostStore()
	return authedServerWithStores(t, server.Config{Users: users, Content: posts, Types: types})
}

// createPage posts a page, optionally under a parent, and returns what the API answered.
func createPage(t *testing.T, handler http.Handler, title, parent string) *typeRecorder {
	t.Helper()
	body := `{"type":"page","title":"` + title + `"}`
	if parent != "" {
		body = `{"type":"page","title":"` + title + `","parent_id":"` + parent + `"}`
	}
	recorder := doRequest(t, handler, http.MethodPost, "/api/content", body)
	return &typeRecorder{code: recorder.Code, body: decodeBody[postBody](t, recorder)}
}

// typeRecorder carries the status and decoded body of a content answer.
type typeRecorder struct {
	code int
	body postBody
}

func TestContentCreateAddressesRootContent(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)

	about := createPage(t, handler, "About", "")

	if about.code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", about.code, http.StatusCreated)
	}
	if about.body.Path != "pages/about" {
		t.Errorf("path = %q, want the type's route word", about.body.Path)
	}
	if about.body.ParentID != nil {
		t.Errorf("parent = %v, want none", about.body.ParentID)
	}
}

func TestContentCreateNestsUnderAParent(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)
	about := createPage(t, handler, "About", "")

	team := createPage(t, handler, "Team", about.body.ID.String())

	if team.code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", team.code, http.StatusCreated)
	}
	if team.body.Path != "pages/about/team" {
		t.Errorf("path = %q, want the ancestor chain", team.body.Path)
	}
	if team.body.ParentID == nil || *team.body.ParentID != about.body.ID {
		t.Errorf("parent = %v, want %v", team.body.ParentID, about.body.ID)
	}
}

func TestContentCreateRefusesAParentAFlatTypeCannotHold(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)
	hello := doRequest(t, handler, http.MethodPost, "/api/content", `{"title":"Hello World"}`)
	stored := decodeBody[postBody](t, hello)

	recorder := doRequest(t, handler, http.MethodPost, "/api/content",
		`{"title":"Nested","parent_id":"`+stored.ID.String()+`"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
}

func TestContentCreateRefusesAMissingParent(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)

	recorder := doRequest(t, handler, http.MethodPost, "/api/content",
		`{"type":"page","title":"Orphan","parent_id":"00000000-0000-0000-0000-000000000001"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
}

func TestContentCreateRefusesAnAddressAnotherTypeAnswersUnder(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)

	recorder := doRequest(t, handler, http.MethodPost, "/api/content", `{"title":"Pages"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
	if body := recorder.Body.String(); !strings.Contains(body, "Pages") {
		t.Errorf("body = %q, want the owner of the address named", body)
	}
}

func TestContentPatchRenamesTheAddress(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)
	about := createPage(t, handler, "About", "")

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+about.body.ID.String(),
		versionedBody(t, about.body.UpdatedAt, map[string]any{"slug": "company"}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if body := decodeBody[postBody](t, recorder); body.Path != "pages/company" {
		t.Errorf("path = %q, want the rename carried into the address", body.Path)
	}
}

func TestContentPatchMovesAnItemUnderAnotherParent(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)
	about := createPage(t, handler, "About", "")
	company := createPage(t, handler, "Company", "")

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+about.body.ID.String(),
		versionedBody(t, about.body.UpdatedAt, map[string]any{"parent_id": company.body.ID.String()}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[postBody](t, recorder)
	if body.Path != "pages/company/about" {
		t.Errorf("path = %q, want it under its new parent", body.Path)
	}
	if body.ParentID == nil || *body.ParentID != company.body.ID {
		t.Errorf("parent = %v, want %v", body.ParentID, company.body.ID)
	}
}

func TestContentPatchReturnsAnItemToTheRoot(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)
	about := createPage(t, handler, "About", "")
	team := createPage(t, handler, "Team", about.body.ID.String())

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+team.body.ID.String(),
		versionedBody(t, team.body.UpdatedAt, map[string]any{"parent_id": nil}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[postBody](t, recorder)
	if body.Path != "pages/team" || body.ParentID != nil {
		t.Errorf("body = %q under %v, want it back at the type's root", body.Path, body.ParentID)
	}
}

func TestContentPatchRefusesNestingAnItemInsideItself(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)
	about := createPage(t, handler, "About", "")
	team := createPage(t, handler, "Team", about.body.ID.String())

	itself := doRequest(t, handler, http.MethodPatch, "/api/content/"+about.body.ID.String(),
		versionedBody(t, about.body.UpdatedAt, map[string]any{"parent_id": about.body.ID.String()}))
	descendant := doRequest(t, handler, http.MethodPatch, "/api/content/"+about.body.ID.String(),
		versionedBody(t, about.body.UpdatedAt, map[string]any{"parent_id": team.body.ID.String()}))

	for name, recorder := range map[string]int{"itself": itself.Code, "its child": descendant.Code} {
		if recorder != http.StatusUnprocessableEntity {
			t.Errorf("nesting under %s = %d, want %d", name, recorder, http.StatusUnprocessableEntity)
		}
	}
}

func TestContentPatchRefusesAMalformedParent(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)
	about := createPage(t, handler, "About", "")

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+about.body.ID.String(),
		versionedBody(t, about.body.UpdatedAt, map[string]any{"parent_id": "not-a-uuid"}))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
}

func TestContentPatchRefusesRenamingIntoAReservedAddress(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)
	created := doRequest(t, handler, http.MethodPost, "/api/content", `{"type":"post","title":"Innocent"}`)
	post := decodeBody[postBody](t, created)

	body := `{"updated_at":"` + post.UpdatedAt.Format(time.RFC3339Nano) + `","slug":"admin"}`
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+post.ID.String(), body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
}

func TestContentPatchRefusesAParentOnItsWayOut(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)
	about := createPage(t, handler, "About", "")
	team := createPage(t, handler, "Team", "")

	doRequest(t, handler, http.MethodDelete, "/api/content/"+about.body.ID.String(), "")

	body := `{"updated_at":"` + team.body.UpdatedAt.Format(time.RFC3339Nano) +
		`","parent_id":"` + about.body.ID.String() + `"}`
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+team.body.ID.String(), body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
}

func TestContentPatchRefusesCarryingAChainPastTheLimit(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)
	deep := createPage(t, handler, "Level 1", "")
	for level := 2; level <= 9; level++ {
		deep = createPage(t, handler, "Level", deep.body.ID.String())
	}
	branch := createPage(t, handler, "Branch", "")
	leaf := createPage(t, handler, "Leaf", branch.body.ID.String())
	createPage(t, handler, "Deeper", leaf.body.ID.String())

	body := `{"updated_at":"` + branch.body.UpdatedAt.Format(time.RFC3339Nano) +
		`","parent_id":"` + deep.body.ID.String() + `"}`
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+branch.body.ID.String(), body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, the branch is three levels tall", recorder.Code, http.StatusUnprocessableEntity)
	}
}

func TestContentPatchCannotReachTheTrash(t *testing.T) {
	t.Parallel()

	handler := authedNestingServer(t)
	about := createPage(t, handler, "About", "")
	createPage(t, handler, "Team", about.body.ID.String())

	body := `{"updated_at":"` + about.body.UpdatedAt.Format(time.RFC3339Nano) + `","status":"trash"}`
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+about.body.ID.String(), body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, the trash is reached by deleting", recorder.Code, http.StatusUnprocessableEntity)
	}
}
