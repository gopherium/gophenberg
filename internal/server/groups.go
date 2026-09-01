// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// groupResponse is a field group as the admin API answers it.
type groupResponse struct {
	ID        int             `json:"id"`
	Title     string          `json:"title"`
	Location  content.Rules   `json:"location"`
	Position  int             `json:"position"`
	Active    bool            `json:"active"`
	Fields    []fieldResponse `json:"fields"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// groupListResponse is the stored field groups as the admin API answers them.
type groupListResponse struct {
	Items []groupResponse `json:"items"`
}

// paramResponse is one rule source as the admin API answers it.
type paramResponse struct {
	Source    string           `json:"source"`
	Operators []string         `json:"operators"`
	Values    []choiceResponse `json:"values"`
}

// choiceResponse is one value a rule source offers.
type choiceResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// paramListResponse is the rule sources a location may read.
type paramListResponse struct {
	Items []paramResponse `json:"items"`
}

// newGroupResponse builds a groupResponse, normalizing timestamps to UTC.
func newGroupResponse(g content.Group) groupResponse {
	fields := make([]fieldResponse, len(g.Fields))
	for i, f := range g.Fields {
		fields[i] = newFieldResponse(f)
	}
	location := g.Location
	if location == nil {
		location = content.Rules{}
	}
	return groupResponse{
		ID:        g.ID,
		Title:     g.Title,
		Location:  location,
		Position:  g.Position,
		Active:    g.Active,
		Fields:    fields,
		CreatedAt: g.CreatedAt.UTC(),
		UpdatedAt: g.UpdatedAt.UTC(),
	}
}

// newGroupListResponse builds the listing of every stored group.
func newGroupListResponse(groups []content.Group) groupListResponse {
	items := make([]groupResponse, len(groups))
	for i, g := range groups {
		items[i] = newGroupResponse(g)
	}
	return groupListResponse{Items: items}
}

// handleGroupList returns an http.HandlerFunc listing the stored field groups.
func (s *server) handleGroupList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groups, err := s.types.Groups(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, newGroupListResponse(groups))
	}
}

// handleGroupParams returns an http.HandlerFunc listing the sources a location rule may read.
func (s *server) handleGroupParams() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := s.types.Params(r.Context()).All()
		items := make([]paramResponse, len(params))
		for i, p := range params {
			choices, err := p.Values(r.Context())
			if err != nil {
				respondDomainError(w, err)
				return
			}
			items[i] = paramResponse{
				Source: p.Name(), Operators: p.Operators(), Values: offeredChoices(choices),
			}
		}
		authkit.Respond(w, http.StatusOK, paramListResponse{Items: items})
	}
}

// offeredChoices returns the values a rule source offers as the admin reads them.
func offeredChoices(choices []content.Choice) []choiceResponse {
	offered := make([]choiceResponse, len(choices))
	for i, c := range choices {
		offered[i] = choiceResponse{Value: c.Value, Label: c.Label}
	}
	return offered
}

// handleGroupCreate returns an http.HandlerFunc storing a new field group.
func (s *server) handleGroupCreate() http.HandlerFunc {
	type request struct {
		Title    string        `json:"title"`
		Location content.Rules `json:"location"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := decodeKnown[request](w, r)
		if err != nil {
			respondBodyError(w, err)
			return
		}
		created, err := s.types.CreateGroup(r.Context(), content.Group{
			Title: req.Title, Location: req.Location,
		})
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusCreated, newGroupResponse(created))
	}
}

// handleGroupPatch returns an http.HandlerFunc carrying a group's title, location and resting flag.
func (s *server) handleGroupPatch() http.HandlerFunc {
	type request struct {
		Title    *string        `json:"title"`
		Location *content.Rules `json:"location"`
		Active   *bool          `json:"active"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		stored, err := s.storedGroup(r)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		req, err := decodeKnown[request](w, r)
		if err != nil {
			respondBodyError(w, err)
			return
		}
		if req.Title != nil {
			stored.Title = *req.Title
		}
		if req.Location != nil {
			stored.Location = *req.Location
		}
		if req.Active != nil {
			stored.Active = *req.Active
		}
		updated, err := s.types.UpdateGroup(r.Context(), stored)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, newGroupResponse(updated))
	}
}

// handleGroupDelete returns an http.HandlerFunc removing a group with its fields and their values.
func (s *server) handleGroupDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := groupIDOf(r)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		if err := s.types.DeleteGroup(r.Context(), id); err != nil {
			respondDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleGroupOrder returns an http.HandlerFunc storing the order the groups are read in.
func (s *server) handleGroupOrder() http.HandlerFunc {
	type request struct {
		Order []int `json:"order"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := decodeKnown[request](w, r)
		if err != nil {
			respondBodyError(w, err)
			return
		}
		reordered, err := s.types.ReorderGroups(r.Context(), req.Order)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, newGroupListResponse(reordered))
	}
}

// fieldCreateRequest is a field declaration as the admin API reads it.
type fieldCreateRequest struct {
	Key       string         `json:"key"`
	Label     string         `json:"label"`
	Kind      string         `json:"kind"`
	RelatesTo string         `json:"relates_to"`
	Many      bool           `json:"many"`
	Required  bool           `json:"required"`
	Settings  map[string]any `json:"settings"`
}

// handleGroupFieldCreate returns an http.HandlerFunc declaring a field inside a group.
func (s *server) handleGroupFieldCreate() http.HandlerFunc {
	type request = fieldCreateRequest
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := groupIDOf(r)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		req, err := decodeKnown[request](w, r)
		if err != nil {
			respondBodyError(w, err)
			return
		}
		asked, err := content.NewField(content.Field{
			Key:       req.Key,
			Label:     req.Label,
			Kind:      content.FieldKind(req.Kind),
			RelatesTo: req.RelatesTo,
			Many:      req.Many,
			Required:  req.Required,
			Settings:  req.Settings,
		})
		if err != nil {
			respondDomainError(w, err)
			return
		}
		created, err := s.types.CreateFieldInGroup(r.Context(), id, asked)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusCreated, newFieldResponse(created))
	}
}

// handleSubFieldCreate returns an http.HandlerFunc declaring a field inside a container.
func (s *server) handleSubFieldCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := groupIDOf(r)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		req, err := decodeKnown[fieldCreateRequest](w, r)
		if err != nil {
			respondBodyError(w, err)
			return
		}
		parent, err := s.fieldAtPath(r, id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		asked, err := content.NewSubField(content.Field{
			Key:       req.Key,
			Label:     req.Label,
			Kind:      content.FieldKind(req.Kind),
			RelatesTo: req.RelatesTo,
			Many:      req.Many,
			Required:  req.Required,
			Settings:  req.Settings,
		}, parent.Kind)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		created, err := s.types.CreateSubField(r.Context(), parent.ID, asked)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusCreated, newFieldResponse(created))
	}
}

// handleSubFieldDelete returns an http.HandlerFunc removing a field inside a container.
func (s *server) handleSubFieldDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := groupIDOf(r)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		held, err := s.fieldAtPath(r, id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		if err := s.types.DeleteSubField(r.Context(), held.ID); err != nil {
			respondDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// fieldAtPath returns the field the request path addresses inside its group, however deep it stands.
func (s *server) fieldAtPath(r *http.Request, groupID int) (content.Field, error) {
	groups, err := s.types.Groups(r.Context())
	if err != nil {
		return content.Field{}, err
	}
	for _, g := range groups {
		if g.ID != groupID {
			continue
		}
		return fieldDown(g.Fields, strings.Split(chi.URLParam(r, "fieldPath"), "."))
	}
	return content.Field{}, content.ErrGroupNotFound
}

// fieldDown returns the field the keys address among the declared ones.
func fieldDown(declared []content.Field, keys []string) (content.Field, error) {
	for _, f := range declared {
		if f.Key != keys[0] {
			continue
		}
		if len(keys) == 1 {
			return f, nil
		}
		return fieldDown(f.Fields, keys[1:])
	}
	return content.Field{}, content.ErrFieldNotFound
}

// fieldPatchRequest is a field edit as the admin API reads it.
type fieldPatchRequest struct {
	Label     *string         `json:"label"`
	Required  *bool           `json:"required"`
	Settings  *map[string]any `json:"settings"`
	UpdatedAt *time.Time      `json:"updated_at"`
}

// handleGroupFieldPatch returns an http.HandlerFunc carrying a field's label, required flag and settings.
func (s *server) handleGroupFieldPatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := groupIDOf(r)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		req, err := decodeKnown[fieldPatchRequest](w, r)
		if err != nil {
			respondBodyError(w, err)
			return
		}
		if req.UpdatedAt == nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "missing updated_at", Code: "body_field_required", Meta: map[string]any{"field": "updated_at"},
			})
			return
		}
		updated, err := s.patchGroupField(r, id, req)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, newFieldResponse(updated))
	}
}

// patchGroupField applies the edit to the stored field once the expectation holds.
func (s *server) patchGroupField(r *http.Request, id int, req fieldPatchRequest) (content.Field, error) {
	keys := strings.Split(chi.URLParam(r, "fieldKey"), ".")
	stored, err := s.groupFieldDown(r, id, keys)
	if err != nil {
		return content.Field{}, err
	}
	if !req.UpdatedAt.Equal(stored.UpdatedAt) {
		return content.Field{}, content.ErrConflict
	}
	if req.Label != nil {
		stored.Label = *req.Label
	}
	if req.Required != nil {
		stored.Required = *req.Required
	}
	if req.Settings != nil {
		stored.Settings = *req.Settings
	}
	if len(keys) == 1 {
		return s.types.UpdateFieldInGroup(r.Context(), id, stored, *req.UpdatedAt)
	}
	return s.types.UpdateSubField(r.Context(), stored.ID, stored, *req.UpdatedAt)
}

// groupFieldDown returns the field the keys address inside the group, however deep it stands.
func (s *server) groupFieldDown(r *http.Request, groupID int, keys []string) (content.Field, error) {
	groups, err := s.types.Groups(r.Context())
	if err != nil {
		return content.Field{}, err
	}
	for _, g := range groups {
		if g.ID != groupID {
			continue
		}
		return fieldDown(g.Fields, keys)
	}
	return content.Field{}, content.ErrGroupNotFound
}

// handleSubFieldOrder returns an http.HandlerFunc standing the fields inside a container as asked.
func (s *server) handleSubFieldOrder() http.HandlerFunc {
	type request struct {
		Order []string `json:"order"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := groupIDOf(r)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		req, err := decodeKnown[request](w, r)
		if err != nil {
			respondBodyError(w, err)
			return
		}
		held, err := s.fieldAtPath(r, id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		reordered, err := s.types.ReorderSubFields(r.Context(), held.ID, req.Order)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]fieldResponse, len(reordered))
		for i, f := range reordered {
			items[i] = newFieldResponse(f)
		}
		authkit.Respond(w, http.StatusOK, fieldListResponse{Items: items})
	}
}

// handleGroupFieldDelete returns an http.HandlerFunc removing a field and the values it held.
func (s *server) handleGroupFieldDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := groupIDOf(r)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		if err := s.types.DeleteFieldInGroup(r.Context(), id, chi.URLParam(r, "fieldKey")); err != nil {
			respondDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleGroupFieldOrder returns an http.HandlerFunc storing a group's field declaration order.
func (s *server) handleGroupFieldOrder() http.HandlerFunc {
	type request struct {
		Order []string `json:"order"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := groupIDOf(r)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		req, err := decodeKnown[request](w, r)
		if err != nil {
			respondBodyError(w, err)
			return
		}
		reordered, err := s.types.ReorderFieldsInGroup(r.Context(), id, req.Order)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]fieldResponse, len(reordered))
		for i, f := range reordered {
			items[i] = newFieldResponse(f)
		}
		authkit.Respond(w, http.StatusOK, fieldListResponse{Items: items})
	}
}

// handleGroupFieldMove returns an http.HandlerFunc carrying a field into another group.
func (s *server) handleGroupFieldMove() http.HandlerFunc {
	type request struct {
		ToGroup int `json:"to_group"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := groupIDOf(r)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		req, err := decodeKnown[request](w, r)
		if err != nil {
			respondBodyError(w, err)
			return
		}
		moved, err := s.types.MoveField(r.Context(), id, chi.URLParam(r, "fieldKey"), req.ToGroup)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, newFieldResponse(moved))
	}
}

// storedGroup returns the group the request names, or the reason it holds none.
func (s *server) storedGroup(r *http.Request) (content.Group, error) {
	id, err := groupIDOf(r)
	if err != nil {
		return content.Group{}, err
	}
	groups, err := s.types.Groups(r.Context())
	if err != nil {
		return content.Group{}, err
	}
	for _, g := range groups {
		if g.ID == id {
			return g, nil
		}
	}
	return content.Group{}, content.ErrGroupNotFound
}

// groupIDOf returns the group identifier the path names, or the reason it names none.
func groupIDOf(r *http.Request) (int, error) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		return 0, content.Refuse(content.ErrGroupNotFound, "group_id_malformed",
			"content: the group identifier is not a number", nil)
	}
	return id, nil
}
