// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// pointedAtPostWithStores returns a handler reading pointers at posts, beside the stores behind it.
func pointedAtPostWithStores(t *testing.T) (http.Handler, *fakePostStore, *fakeTypeStore) {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	posts, types := newFakePostStore(), newFakeTypeStore()
	posts.declared = types
	handler := authedServerWithStores(t, server.Config{Users: users, Content: posts, Types: types})
	declaredRelation(t, handler)
	if recorder := doRequest(t, handler, http.MethodPatch, "/api/types/category",
		`{"page_kind":"archive"}`); recorder.Code != http.StatusOK {
		t.Fatalf("serving term pages: %d: %s", recorder.Code, recorder.Body.String())
	}
	declaredOnType(t, handler, "category",
		`{"key":"picks","label":"Picks","kind":"relation","relates_to":"post","many":true}`)
	declaredOn(t, handler, fmt.Sprintf(
		`{"key":"linked-from","label":"Linked from","kind":"backlinks","settings":`+
			`{"source_group":%q,"source_field":["picks"]}}`, groupKeyOver(t, handler, "category")))
	return handler, posts, types
}

// groupKeyOver returns the key of the group placed on the type, raising one when nothing is placed there.
func groupKeyOver(t *testing.T, handler http.Handler, typeKey string) string {
	t.Helper()
	id := groupOver(t, handler, typeKey)
	listed := decodeBody[struct {
		Items []struct {
			ID  int    `json:"id"`
			Key string `json:"key"`
		} `json:"items"`
	}](t, doRequest(t, handler, http.MethodGet, "/api/groups", ""))
	for _, held := range listed.Items {
		if held.ID == id {
			return held.Key
		}
	}
	t.Fatalf("no group carries the identity %d", id)
	return ""
}

// pointedAtPost declares a relation on categories and a backlinks on posts reading it, and returns the handler.
func pointedAtPost(t *testing.T) http.Handler {
	t.Helper()
	handler := termTypeServer(t)
	declaredOnType(t, handler, "category",
		`{"key":"picks","label":"Picks","kind":"relation","relates_to":"post","many":true}`)
	declaredOn(t, handler, fmt.Sprintf(
		`{"key":"linked-from","label":"Linked from","kind":"backlinks","settings":`+
			`{"source_group":%q,"source_field":["picks"]}}`, groupKeyOver(t, handler, "category")))
	return handler
}

func TestResolveListsTheItemsPointingAtOne(t *testing.T) {
	t.Parallel()

	handler := pointedAtPost(t)
	post := publishItemAt(t, handler, draftedPost(t, handler))
	picked := patchValues(t, handler, storedCategoryItem(t, handler), fmt.Sprintf(`{"picks":[%q]}`, post.ID))
	publishItemAt(t, handler, picked)

	served := resolvedFields(t, handler, "hello-world")

	var held []struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Title string `json:"title"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal(served["linked-from"], &held); err != nil {
		t.Fatalf("reading the pointing items: %v: %s", err, served["linked-from"])
	}
	if len(held) != 1 {
		t.Fatalf("linked-from = %s, want the category pointing at the post", served["linked-from"])
	}
	if held[0].Title != "News" || held[0].Type != "category" || held[0].Path != "categories/news" {
		t.Errorf("the pointer reads %+v, want the category named, typed and addressed", held[0])
	}
}

// resolvedTotals returns the counts a resolved item carries beside its fields.
func resolvedTotals(t *testing.T, handler http.Handler, slug string) map[string]int {
	t.Helper()
	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path="+slug, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("resolving %q: %d: %s", slug, recorder.Code, recorder.Body.String())
	}
	answered := decodeBody[struct {
		Item struct {
			FieldTotals map[string]int `json:"field_totals"`
		} `json:"item"`
	}](t, recorder)
	return answered.Item.FieldTotals
}

// pointedAtHiddenPost declares a backlinks field on posts that a switch shows, and points a category at one.
func pointedAtHiddenPost(t *testing.T) (http.Handler, contentValuesBody) {
	t.Helper()
	handler := termTypeServer(t)
	declaredOnType(t, handler, "category",
		`{"key":"picks","label":"Picks","kind":"relation","relates_to":"post","many":true}`)
	declaredOn(t, handler, `{"key":"on-sale","label":"On sale","kind":"boolean"}`)
	declaredOn(t, handler, fmt.Sprintf(
		`{"key":"linked-from","label":"Linked from","kind":"backlinks","settings":`+
			`{"source_group":%q,"source_field":["picks"],`+
			`"conditions":[[{"source":"on-sale","operator":"==","value":"true"}]]}}`,
		groupKeyOver(t, handler, "category")))
	post := publishItemAt(t, handler, draftedPost(t, handler))
	picked := patchValues(t, handler, storedCategoryItem(t, handler), fmt.Sprintf(`{"picks":[%q]}`, post.ID))
	publishItemAt(t, handler, picked)
	return handler, post
}

func TestResolveCountsNoPointersForAFieldTheRulesHide(t *testing.T) {
	t.Parallel()

	handler, _ := pointedAtHiddenPost(t)

	served := resolvedFields(t, handler, "hello-world")
	totals := resolvedTotals(t, handler, "hello-world")

	if _, shown := served["linked-from"]; shown {
		t.Fatalf("fields = %v, want the hidden field left out", served)
	}
	if _, counted := totals["linked-from"]; counted {
		t.Errorf("field_totals = %v, want no count for a field nobody may see", totals)
	}
}

func TestResolveCountsThePointersOfAFieldTheRulesShow(t *testing.T) {
	t.Parallel()

	handler, post := pointedAtHiddenPost(t)
	publishItemAt(t, handler, patchValues(t, handler, post, `{"on-sale":true}`))

	totals := resolvedTotals(t, handler, "hello-world")

	if totals["linked-from"] != 1 {
		t.Errorf("field_totals = %v, want the one pointer counted while the field shows", totals)
	}
}

func TestResolveHidesAPointerNobodyPublished(t *testing.T) {
	t.Parallel()

	handler := pointedAtPost(t)
	post := publishItemAt(t, handler, draftedPost(t, handler))
	patchValues(t, handler, storedCategoryItem(t, handler), fmt.Sprintf(`{"picks":[%q]}`, post.ID))

	served := resolvedFields(t, handler, "hello-world")

	var held []json.RawMessage
	if err := json.Unmarshal(served["linked-from"], &held); err != nil {
		t.Fatalf("reading the pointing items: %v: %s", err, served["linked-from"])
	}
	if len(held) != 0 {
		t.Errorf("linked-from = %s, want a draft pointer kept back", served["linked-from"])
	}
}

func TestContentAnswersTheEditorWhatPointsAtTheItem(t *testing.T) {
	t.Parallel()

	handler := pointedAtPost(t)
	post := publishItemAt(t, handler, draftedPost(t, handler))
	picked := patchValues(t, handler, storedCategoryItem(t, handler), fmt.Sprintf(`{"picks":[%q]}`, post.ID))
	publishItemAt(t, handler, picked)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/"+post.ID, "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	answer := decodeBody[struct {
		Fields map[string][]struct {
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"fields"`
		FieldTotals map[string]int `json:"field_totals"`
	}](t, recorder)
	held := answer.Fields["linked-from"]
	if len(held) != 1 || held[0].Title != "News" || held[0].Type != "category" {
		t.Fatalf("linked-from = %+v, want the category the editor links to", answer.Fields)
	}
	if answer.FieldTotals["linked-from"] != 1 {
		t.Errorf("field_totals = %v, want the one pointer counted", answer.FieldTotals)
	}
}

func TestResolveLeavesOutABacklinksWhoseSourceIsGone(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	posts, types := newFakePostStore(), newFakeTypeStore()
	posts.declared = types
	types.groups = append(types.groups, content.Group{
		ID: 99, Key: "strays", Title: "Strays", Active: true,
		Location: content.Rules{{{
			Source: content.ScreenContentType, Operator: content.OperatorIs, Value: content.TypePost,
		}}},
		Fields: []content.Field{{
			ID: 98, Key: "linked-from", Label: "Linked from", Kind: content.FieldKindBacklinks,
			Settings: map[string]any{
				"source_group": "nobody", "source_field": []any{"picks"},
			},
		}},
	})
	handler := authedServerWithStores(t, server.Config{Users: users, Content: posts, Types: types})
	publishItemAt(t, handler, draftedPost(t, handler))

	served := resolvedFields(t, handler, "hello-world")

	if held, carried := served["linked-from"]; carried {
		t.Errorf("linked-from = %s, want a backlinks reading nothing left out", held)
	}
}

func TestResolveReportsWhatItCannotReadForABacklinks(t *testing.T) {
	t.Parallel()

	for name, breaks := range map[string]func(*fakePostStore, *fakeTypeStore){
		"the pointers it cannot count": func(p *fakePostStore, _ *fakeTypeStore) {
			p.pointingErr = errRegistryDown
		},
		"the groups it cannot read": func(_ *fakePostStore, types *fakeTypeStore) {
			types.listErr = errRegistryDown
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, posts, types := pointedAtPostWithStores(t)
			post := publishItemAt(t, handler, draftedPost(t, handler))
			picked := patchValues(t, handler, storedCategoryItem(t, handler),
				fmt.Sprintf(`{"picks":[%q]}`, post.ID))
			publishItemAt(t, handler, picked)
			breaks(posts, types)

			recorder := doRequest(t, handler, http.MethodGet,
				"/api/content/v1/resolve?path=hello-world", "")

			if recorder.Code == http.StatusOK {
				t.Errorf("status = %d, want the read it cannot run reported", recorder.Code)
			}
		})
	}
}

func TestContentReportsThePointersTheEditorCannotBeShown(t *testing.T) {
	t.Parallel()

	handler, posts, _ := pointedAtPostWithStores(t)
	post := publishItemAt(t, handler, draftedPost(t, handler))
	posts.pointingErr = errRegistryDown

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/"+post.ID, "")

	if recorder.Code == http.StatusOK {
		t.Errorf("status = %d, want the read it cannot run reported", recorder.Code)
	}
}

// namedCategory stores one published category under the title and points it at the item.
func namedCategory(t *testing.T, handler http.Handler, title, at string) {
	t.Helper()
	recorder := doRequest(t, handler, http.MethodPost, "/api/content",
		fmt.Sprintf(`{"type":"category","title":%q}`, title))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("storing %q: %d: %s", title, recorder.Code, recorder.Body.String())
	}
	held := decodeBody[contentValuesBody](t, recorder)
	publishItemAt(t, handler, patchValues(t, handler, held, fmt.Sprintf(`{"picks":[%q]}`, at)))
}

func TestContentAnswersOnePageOfPointersAndCountsThemAll(t *testing.T) {
	t.Parallel()

	handler := pointedAtPost(t)
	post := publishItemAt(t, handler, draftedPost(t, handler))
	for i := range 22 {
		namedCategory(t, handler, fmt.Sprintf("Filed under %d", i), post.ID)
	}

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/"+post.ID, "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	answer := decodeBody[struct {
		Fields      map[string][]struct{} `json:"fields"`
		FieldTotals map[string]int        `json:"field_totals"`
	}](t, recorder)
	if len(answer.Fields["linked-from"]) != 20 {
		t.Errorf("linked-from holds %d, want one page of twenty", len(answer.Fields["linked-from"]))
	}
	if answer.FieldTotals["linked-from"] != 22 {
		t.Errorf("field_totals = %v, want every pointer counted behind the page", answer.FieldTotals)
	}
}

func TestContentKeepsAWriteThatLandedWhenThePointersCannotBeRead(t *testing.T) {
	t.Parallel()

	handler, posts, _ := pointedAtPostWithStores(t)
	held := draftedPost(t, handler)
	posts.pointingErr = errRegistryDown

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID,
		fmt.Sprintf(`{"updated_at":%q,"title":"A second look"}`, held.UpdatedAt))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want the stored write reported as stored: %s",
			recorder.Code, recorder.Body.String())
	}
	answer := decodeBody[struct {
		Title       string         `json:"title"`
		FieldTotals map[string]int `json:"field_totals"`
	}](t, recorder)
	if answer.Title != "A second look" {
		t.Errorf("title = %q, want the write the server kept", answer.Title)
	}
	if len(answer.FieldTotals) != 0 {
		t.Errorf("field_totals = %v, want no counts when the read behind them failed", answer.FieldTotals)
	}
}

func TestContentPatchRefusesASubmittedBacklinksValue(t *testing.T) {
	t.Parallel()

	handler := pointedAtPost(t)
	held := draftedPost(t, handler)

	if code := refusedValues(t, handler, held, `{"linked-from":[]}`); code != "field_shape_value" {
		t.Errorf("code = %q, want field_shape_value", code)
	}
}
