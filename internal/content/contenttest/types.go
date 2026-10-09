// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// typeCases are the cases over storing, editing and removing content types and handing the root between them.
var typeCases = []Case{
	{"ListsTheBuiltInType", listsTheBuiltInType},
	{"CreatesAndReadsBackAType", createsAndReadsBackAType},
	{"ReportsATakenKey", reportsATakenKey},
	{"ReportsATakenRouteWord", reportsATakenRouteWord},
	{"ReportsAMissingType", reportsAMissingType},
	{"UpdatesTheEditableFields", updatesTheEditableFields},
	{"UpdateReportsAMissingType", updateReportsAMissingType},
	{"DeletesAnEmptyType", deletesAnEmptyType},
	{"UpdateReportsATakenRouteWord", updateReportsATakenRouteWord},
	{"DeleteReportsAMissingType", deleteReportsAMissingType},
	{"KeepsATypeHoldingContent", keepsATypeHoldingContent},
	{"UpdateCarriesContentToTheNewRouteWord", updateCarriesContentToTheNewRouteWord},
	{"UpdateCarriesContentDownFromTheRoot", updateCarriesContentDownFromTheRoot},
	{"UpdateLeavesContentAloneWhenTheRouteWordStays", updateLeavesContentAloneWhenTheRouteWordStays},
	{"UpdateHandsTheRootToAnotherType", updateHandsTheRootToAnotherType},
	{"UpdateRefusesToHandTheRootToAReservedAddress", updateRefusesToHandTheRootToAReservedAddress},
	{"UpdateRefusesToHandTheRootToAnUnusableAddress", updateRefusesToHandTheRootToAnUnusableAddress},
	{"UpdateRefusesToHandTheRootOntoATakenRouteWord", updateRefusesToHandTheRootOntoATakenRouteWord},
	{"UpdateRefusesACarryOntoATakenAddress", updateRefusesACarryOntoATakenAddress},
	{"UpdatePromotesWhenNoDefaultRemains", updatePromotesWhenNoDefaultRemains},
	{"StoredFieldsServeTheGoldenHandshakeShape", storedFieldsServeTheGoldenHandshakeShape},
	{"DeleteRefusesATypeARelationStillTargets", deleteRefusesATypeARelationStillTargets},
}

// listsTheBuiltInType lists the shipped post type as the only one a fresh store holds.
func listsTheBuiltInType(t *testing.T, s Stores) {
	types, err := s.Types.List(t.Context())

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if len(types) != 1 {
		t.Fatalf("List() returned %d types, want the built-in one", len(types))
	}
	post := types[0]
	if post.Key != content.TypePost || post.PluralLabel != "Posts" || !post.Default || !post.Active {
		t.Errorf("List()[0] = %+v, want the active default post type", post)
	}
	if post.RouteWord != "" || post.PageKind != content.PageKindSingle || !post.Revisions {
		t.Errorf("List()[0] = %+v, want the rooted single-page type keeping revisions", post)
	}
	if post.Description != "Manage the posts on this site." {
		t.Errorf("List()[0].Description = %q, want the shipped sentence", post.Description)
	}
}

// createsAndReadsBackAType stores a type and reads it back with its description and UTC stamps.
func createsAndReadsBackAType(t *testing.T, s Stores) {
	car := CarType(t)
	car.Description = "Cars for sale."

	created, err := s.Types.Create(t.Context(), car)

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Key != "car" || created.RouteWord != "cars" || created.SingularLabel != "Car" {
		t.Errorf("Create() = %+v, want the car type", created)
	}
	read, err := s.Types.ByKey(t.Context(), "car")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if read.Key != created.Key || read.CreatedAt.Location() != time.UTC {
		t.Errorf("ByKey() = %+v, want the stored car with UTC stamps", read)
	}
	if created.Description != "Cars for sale." || read.Description != "Cars for sale." {
		t.Errorf("Description = %q created, %q read, want the stored sentence", created.Description, read.Description)
	}
}

// reportsATakenKey refuses a second type under a key already stored.
func reportsATakenKey(t *testing.T, s Stores) {
	if _, err := s.Types.Create(t.Context(), CarType(t)); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	_, err := s.Types.Create(t.Context(), CarType(t))

	if !errors.Is(err, content.ErrTypeTaken) {
		t.Errorf("Create() error = %v, want %v", err, content.ErrTypeTaken)
	}
}

// reportsATakenRouteWord refuses a new type answering under a route word already stored.
func reportsATakenRouteWord(t *testing.T, s Stores) {
	if _, err := s.Types.Create(t.Context(), CarType(t)); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	other, err := content.NewType("van", "Van", "Vans", "cars")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}

	_, err = s.Types.Create(t.Context(), other)

	if !errors.Is(err, content.ErrRouteWordTaken) {
		t.Errorf("Create() error = %v, want %v", err, content.ErrRouteWordTaken)
	}
}

// reportsAMissingType answers type not found for a key nothing holds.
func reportsAMissingType(t *testing.T, s Stores) {
	_, err := s.Types.ByKey(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("ByKey() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

// updatesTheEditableFields stores the new labels, route word, nesting, activity and description.
func updatesTheEditableFields(t *testing.T, s Stores) {
	created, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	edited := created
	edited.SingularLabel, edited.PluralLabel = "Vehicle", "Vehicles"
	edited.RouteWord, edited.Hierarchical, edited.Active = "vehicles", true, false
	edited.Description = "Vehicles for hire."
	edited.UpdatedAt = created.UpdatedAt.Add(time.Second)

	updated, err := s.Types.Update(t.Context(), edited)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.PluralLabel != "Vehicles" || updated.RouteWord != "vehicles" {
		t.Errorf("Update() = %+v, want the relabeled type", updated)
	}
	if updated.Description != "Vehicles for hire." {
		t.Errorf("Update().Description = %q, want the new sentence", updated.Description)
	}
	if !updated.Hierarchical || updated.Active {
		t.Errorf("Update() = %+v, want it nesting and deactivated", updated)
	}
	if updated.Key != "car" || !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("Update() = %+v, want the key and creation stamp untouched", updated)
	}
}

// updateReportsAMissingType answers type not found for an edit to a type nothing holds.
func updateReportsAMissingType(t *testing.T, s Stores) {
	_, err := s.Types.Update(t.Context(), CarType(t))

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

// deletesAnEmptyType removes a type holding no content.
func deletesAnEmptyType(t *testing.T, s Stores) {
	if _, err := s.Types.Create(t.Context(), CarType(t)); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if err := s.Types.Delete(t.Context(), "car"); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	if _, err := s.Types.ByKey(t.Context(), "car"); !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("ByKey() after Delete error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

// updateReportsATakenRouteWord refuses an edit onto a route word another type answers under.
func updateReportsATakenRouteWord(t *testing.T, s Stores) {
	if _, err := s.Types.Create(t.Context(), CarType(t)); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	van, err := content.NewType("van", "Van", "Vans", "vans")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	if _, err := s.Types.Create(t.Context(), van); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	van.RouteWord = "cars"

	_, err = s.Types.Update(t.Context(), van)

	if !errors.Is(err, content.ErrRouteWordTaken) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrRouteWordTaken)
	}
}

// deleteReportsAMissingType answers type not found for a removal of a type nothing holds.
func deleteReportsAMissingType(t *testing.T, s Stores) {
	err := s.Types.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("Delete() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

// keepsATypeHoldingContent refuses to remove a type an item still belongs to.
func keepsATypeHoldingContent(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	car := CarType(t)
	if _, err := s.Types.Create(t.Context(), car); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	StoreItem(t, s.Content, car, nil, "Ford Focus", author)

	err := s.Types.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeInUse) {
		t.Errorf("Delete() error = %v, want %v", err, content.ErrTypeInUse)
	}
}

// deleteRefusesATypeARelationStillTargets refuses to remove a type a relation field points at and keeps it.
func deleteRefusesATypeARelationStillTargets(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	DeclareFields(t, s.Types, content.Field{
		TypeKey: content.TypePost, Key: "garage", Label: "Garage", Kind: content.FieldKindRelation, RelatesTo: "car",
	})

	err := s.Types.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeTargeted) {
		t.Errorf("Delete() error = %v, want %v", err, content.ErrTypeTargeted)
	}
	if _, err := s.Types.ByKey(t.Context(), "car"); err != nil {
		t.Errorf("ByKey(car) error = %v, want the targeted type kept", err)
	}
}

// updateCarriesContentToTheNewRouteWord moves the addresses of the whole tree under the new route word.
func updateCarriesContentToTheNewRouteWord(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, &about, "Team", author)

	moved := PageType()
	moved.RouteWord, moved.UpdatedAt = "sections", time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), moved); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	if got := AddressOf(t, s.Content, about.ID); got != "sections/about" {
		t.Errorf("root path = %q, want it under the new route word", got)
	}
	if got := AddressOf(t, s.Content, team.ID); got != "sections/about/team" {
		t.Errorf("nested path = %q, want the whole tree carried", got)
	}
}

// updateCarriesContentDownFromTheRoot moves the root type's content under the route word it takes.
func updateCarriesContentDownFromTheRoot(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	post := StoreItem(t, s.Content, PostType(), nil, "Hello World", author)

	moved := PostType()
	moved.RouteWord, moved.UpdatedAt = "blog", time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), moved); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	if got := AddressOf(t, s.Content, post.ID); got != "blog/hello-world" {
		t.Errorf("path = %q, want the root content carried under the new word", got)
	}
}

// updateLeavesContentAloneWhenTheRouteWordStays keeps every address when an edit leaves the route word.
func updateLeavesContentAloneWhenTheRouteWordStays(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)

	relabeled := PageType()
	relabeled.PluralLabel, relabeled.UpdatedAt = "Sections", time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), relabeled); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	if got := AddressOf(t, s.Content, about.ID); got != "pages/about" {
		t.Errorf("path = %q, want it left where it answers", got)
	}
}

// updateHandsTheRootToAnotherType lifts the promoted type to the root and moves the old default under its word.
func updateHandsTheRootToAnotherType(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	post := StoreItem(t, s.Content, PostType(), nil, "Hello World", author)
	about := MustNest(t, s.Content, nil, "About", author)

	promoted := PageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), promoted); err != nil {
		t.Fatalf("Update() handing over the root error = %v, want nil", err)
	}

	registered, err := s.Types.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	held := map[string]content.Type{}
	for _, listed := range registered {
		held[listed.Key] = listed
	}
	if !held["page"].Default || held["page"].RouteWord != "" {
		t.Errorf("page = %+v, want it holding the root", held["page"])
	}
	if held[content.TypePost].Default || held[content.TypePost].RouteWord != "posts" {
		t.Errorf("post = %+v, want it moved off the root", held[content.TypePost])
	}
	if held[content.TypePost].Description != "Manage the posts on this site." {
		t.Errorf("post description = %q, want the demoted post to keep it", held[content.TypePost].Description)
	}
	if got := AddressOf(t, s.Content, about.ID); got != "about" {
		t.Errorf("page address = %q, want it lifted to the root", got)
	}
	if got := AddressOf(t, s.Content, post.ID); got != "posts/hello-world" {
		t.Errorf("post address = %q, want it under its own word", got)
	}
}

// updateRefusesToHandTheRootToAReservedAddress refuses a hand over that would move the default under a reserved word.
func updateRefusesToHandTheRootToAReservedAddress(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	relabelled := PostType()
	relabelled.SingularLabel, relabelled.PluralLabel = "Medium", "Media"
	relabelled.UpdatedAt = time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), relabelled); err != nil {
		t.Fatalf("relabelling the default type: %v", err)
	}
	about := MustNest(t, s.Content, nil, "About", author)

	promoted := PageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()

	_, err := s.Types.Update(t.Context(), promoted)

	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrRouteWordReserved)
	}
	if got := AddressOf(t, s.Content, about.ID); got != "pages/about" {
		t.Errorf("page address = %q, want the refused hand over to have moved nothing", got)
	}
}

// updateRefusesToHandTheRootToAnUnusableAddress refuses a hand over that leaves the default no usable word.
func updateRefusesToHandTheRootToAnUnusableAddress(t *testing.T, s Stores) {
	RegisterPageType(t, s.Types)
	relabelled := PostType()
	relabelled.SingularLabel, relabelled.PluralLabel = "3D Model", "3D Models"
	relabelled.UpdatedAt = time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), relabelled); err != nil {
		t.Fatalf("relabelling the default type: %v", err)
	}

	promoted := PageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()

	_, err := s.Types.Update(t.Context(), promoted)

	if !errors.Is(err, content.ErrInvalidRouteWord) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrInvalidRouteWord)
	}
}

// updateRefusesToHandTheRootOntoATakenRouteWord refuses a hand over moving the default onto a word another type holds.
func updateRefusesToHandTheRootOntoATakenRouteWord(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	van, err := content.NewType("van", "Van", "Vans", "posts")
	if err != nil {
		t.Fatalf("NewType(van) error = %v, want nil", err)
	}
	if _, err := s.Types.Create(t.Context(), van); err != nil {
		t.Fatalf("Create(van) error = %v, want nil", err)
	}
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	promoted := PageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()

	_, err = s.Types.Update(t.Context(), promoted)

	if !errors.Is(err, content.ErrRouteWordTaken) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrRouteWordTaken)
	}
	if post, err := s.Types.ByKey(t.Context(), content.TypePost); err != nil || !post.Default {
		t.Errorf("post = %+v, %v, want it still holding the root", post, err)
	}
	if got := AddressOf(t, s.Content, about.ID); got != "pages/about" {
		t.Errorf("page address = %q, want the refused hand over to have moved nothing", got)
	}
}

// updateRefusesACarryOntoATakenAddress refuses a route word that carries an item onto an address another answers at.
func updateRefusesACarryOntoATakenAddress(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	promoted := PageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()
	root, err := s.Types.Update(t.Context(), promoted)
	if err != nil {
		t.Fatalf("handing the root to the page type: %v", err)
	}
	autos := StoreItem(t, s.Content, root, nil, "Autos", author)
	StoreItem(t, s.Content, root, &autos, "Same Slug", author)
	cars, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create(cars) error = %v, want nil", err)
	}
	StoreItem(t, s.Content, cars, nil, "Same Slug", author)

	cars.RouteWord = "autos"
	cars.UpdatedAt = time.Now().UTC()

	_, err = s.Types.Update(t.Context(), cars)

	if !errors.Is(err, content.ErrSlugTaken) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrSlugTaken)
	}
}

// updatePromotesWhenNoDefaultRemains hands the root to a type while no type holds it.
func updatePromotesWhenNoDefaultRemains(t *testing.T, s Stores) {
	stored, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	retired := PostType()
	retired.Default, retired.RouteWord, retired.UpdatedAt = false, "retired-posts", time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), retired); err != nil {
		t.Fatalf("retiring the default type: %v", err)
	}

	stored.Default = true
	stored.UpdatedAt = time.Now().UTC()

	promoted, err := s.Types.Update(t.Context(), stored)

	if err != nil {
		t.Fatalf("Update() error = %v, want the promotion with nothing to demote", err)
	}
	if !promoted.Default {
		t.Error("the promoted type is not the default")
	}
}

// handshakeField mirrors the served field shape a theme reads from the public handshake.
type handshakeField struct {
	Key       string           `json:"key"`
	Label     string           `json:"label"`
	Kind      string           `json:"kind"`
	RelatesTo string           `json:"relates_to,omitempty"`
	Many      bool             `json:"many"`
	Required  bool             `json:"required"`
	Settings  map[string]any   `json:"settings,omitempty"`
	Fields    []handshakeField `json:"fields,omitempty"`
}

// handshakeFields returns the served view of field definitions, however deep they run.
func handshakeFields(held []content.Field) []handshakeField {
	served := make([]handshakeField, len(held))
	for i, f := range held {
		served[i] = handshakeField{
			Key: f.Key, Label: f.Label, Kind: string(f.Kind),
			RelatesTo: f.RelatesTo, Many: f.Many, Required: f.Required,
			Settings: f.Settings, Fields: handshakeFields(f.Fields),
		}
	}
	return served
}

// handshakeFieldsOf returns the served view of a stored type's fields.
func handshakeFieldsOf(t *testing.T, listed []content.Type, typeKey string) string {
	t.Helper()
	for _, held := range listed {
		if held.Key != typeKey {
			continue
		}
		raw, err := json.Marshal(handshakeFields(held.Fields))
		if err != nil {
			t.Fatalf("marshaling the served fields of %s: %v", typeKey, err)
		}
		return string(raw)
	}
	t.Fatalf("type %s is not listed", typeKey)
	return ""
}

// mustField returns the field the domain settles on, failing the test when it refuses one.
func mustField(t *testing.T, seed content.Field) content.Field {
	t.Helper()
	built, err := content.NewField(seed)
	if err != nil {
		t.Fatalf("NewField(%s) error = %v, want nil", seed.Key, err)
	}
	return built
}

// storedFieldsServeTheGoldenHandshakeShape serves stored fields in the exact shape the public handshake promises.
func storedFieldsServeTheGoldenHandshakeShape(t *testing.T, s Stores) {
	ctx := t.Context()
	for _, key := range []string{"book", "car"} {
		built, err := content.NewType(key, "One "+key, "Many "+key, key+"s")
		if err != nil {
			t.Fatalf("NewType(%s) error = %v, want nil", key, err)
		}
		if _, err := s.Types.Create(ctx, built); err != nil {
			t.Fatalf("Create(%s) error = %v, want nil", key, err)
		}
	}
	for _, seed := range []content.Field{
		{TypeKey: "car", Key: "subtitle", Label: "Subtitle", Kind: content.FieldKindText, Required: true},
		{TypeKey: "car", Key: "authors", Label: "Authors", Kind: content.FieldKindRelation, RelatesTo: "book", Many: true},
		{TypeKey: "book", Key: "pages", Label: "Pages", Kind: content.FieldKindNumber},
	} {
		built, err := content.NewField(seed)
		if err != nil {
			t.Fatalf("NewField(%s) error = %v, want nil", seed.Key, err)
		}
		group := FieldsGroupOf(t, s.Types, seed.TypeKey)
		if _, err := s.Types.CreateFieldInGroup(ctx, group.ID, built, nil); err != nil {
			t.Fatalf("CreateFieldInGroup(%s) error = %v, want nil", seed.Key, err)
		}
	}

	crew, err := s.Types.CreateFieldInGroup(ctx, FieldsGroupOf(t, s.Types, "book").ID, mustField(t, content.Field{
		TypeKey: "book", Key: "crew", Label: "Crew", Kind: content.FieldKindRepeater,
	}), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(crew) error = %v, want nil", err)
	}
	if _, err := s.Types.CreateSubField(ctx, crew.ID, mustField(t, content.Field{
		Key: "name", Label: "Name", Kind: content.FieldKindText, Required: true,
	}), content.DefaultFieldDepth); err != nil {
		t.Fatalf("CreateSubField(name) error = %v, want nil", err)
	}

	listed, err := s.Types.List(ctx)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	carGolden := `[{"key":"subtitle","label":"Subtitle","kind":"text","many":false,"required":true},` +
		`{"key":"authors","label":"Authors","kind":"relation","relates_to":"book","many":true,"required":false}]`
	if got := handshakeFieldsOf(t, listed, "car"); got != carGolden {
		t.Errorf("car fields = %s, want the golden %s", got, carGolden)
	}
	bookGolden := `[{"key":"pages","label":"Pages","kind":"number","many":false,"required":false},` +
		`{"key":"crew","label":"Crew","kind":"repeater","many":false,"required":false,` +
		`"fields":[{"key":"name","label":"Name","kind":"text","many":false,"required":true}]}]`
	if got := handshakeFieldsOf(t, listed, "book"); got != bookGolden {
		t.Errorf("book fields = %s, want the golden %s", got, bookGolden)
	}
}
