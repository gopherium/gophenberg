// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// definitionsHeld is the definitions download as the admin API answers it, every member kept as data.
type definitionsHeld struct {
	Format string           `json:"format"`
	Types  []map[string]any `json:"types"`
	Groups []map[string]any `json:"groups"`
}

// keysOf returns the key member of every listed item.
func keysOf(items []map[string]any) []string {
	keys := make([]string, len(items))
	for i, item := range items {
		keys[i], _ = item["key"].(string)
	}
	return keys
}

func TestDefinitionsExportDownloadsWhatTheSiteDefined(t *testing.T) {
	t.Parallel()

	handler, _, types, _ := typedPostServer(t)
	details, err := types.CreateGroup(t.Context(), content.Group{
		Key: "article-details", Title: "Article details",
		Location: content.Rules{{{Source: content.ScreenContentType, Operator: content.OperatorIs, Value: content.TypePost}}},
	})
	if err != nil {
		t.Fatalf("CreateGroup(Article details) error = %v, want nil", err)
	}
	if _, err := types.CreateFieldInGroup(t.Context(), details.ID, content.Field{
		Key: "subtitle", Label: "Subtitle", Kind: content.FieldKindText,
	}); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	if _, err := types.Create(t.Context(), content.Type{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events", Origin: "events",
	}); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if _, err := types.CreateGroup(t.Context(), content.Group{
		Key: "event-details", Title: "Event details", Origin: "events",
	}); err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	recorder := doRequest(t, handler, http.MethodGet, "/api/definitions", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Disposition"); got != `attachment; filename="definitions.json"` {
		t.Errorf("Content-Disposition = %q, want the definitions file offered as an attachment", got)
	}
	var held definitionsHeld
	if err := json.Unmarshal(recorder.Body.Bytes(), &held); err != nil {
		t.Fatalf("decoding the download: %v", err)
	}
	if held.Format != "1.0.0" {
		t.Errorf("format = %q, want 1.0.0", held.Format)
	}
	if keys := keysOf(held.Types); len(keys) != 1 || keys[0] != content.TypePost {
		t.Errorf("type keys = %v, want only the site's post type", keys)
	}
	if keys := keysOf(held.Groups); len(keys) != 1 || keys[0] != "article-details" {
		t.Fatalf("group keys = %v, want only the site's group, body %s", keys, recorder.Body.String())
	}
	for _, noise := range []string{"id", "position", "created_at", "updated_at"} {
		if _, has := held.Groups[0][noise]; has {
			t.Errorf("the group carries %q, want instance noise left out", noise)
		}
	}
	for _, noise := range []string{"fields", "created_at", "updated_at"} {
		if _, has := held.Types[0][noise]; has {
			t.Errorf("the type carries %q, want derived and instance members left out", noise)
		}
	}
	fields, _ := held.Groups[0]["fields"].([]any)
	if len(fields) != 1 {
		t.Fatalf("group fields = %v, want the one stored field", held.Groups[0]["fields"])
	}
	field, _ := fields[0].(map[string]any)
	if field["key"] != "subtitle" || field["kind"] != "text" {
		t.Errorf("field = %v, want the stored subtitle text field", field)
	}
	if _, has := field["created_at"]; has {
		t.Errorf("the field carries created_at, want timestamps left out")
	}
}

func TestDefinitionsExportReportsARegistryItCannotRead(t *testing.T) {
	t.Parallel()

	handler, _, types, _ := typedPostServer(t)
	types.listErr = errors.New("the registry is unreachable")

	recorder := doRequest(t, handler, http.MethodGet, "/api/definitions", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
