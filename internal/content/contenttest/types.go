// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
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
	{"UpdateRefusesACarryOntoATakenAddress", updateRefusesACarryOntoATakenAddress},
	{"UpdatePromotesWhenNoDefaultRemains", updatePromotesWhenNoDefaultRemains},
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
