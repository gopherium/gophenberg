// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// routeWordSaveCases are the content cases that file each top-level item under the word its type holds at the save.
var routeWordSaveCases = []Case{
	{"CreateFilesATopItemUnderTheWordItsTypeHoldsAtTheSave", createFilesATopItemUnderTheWordItsTypeHoldsAtTheSave},
	{
		"CreateHoldingValuesFilesATopItemUnderTheWordItsTypeHoldsAtTheSave",
		createHoldingValuesFilesATopItemUnderTheWordItsTypeHoldsAtTheSave,
	},
	{
		"UpdateLiftingAnItemFilesItUnderTheWordItsTypeHoldsAtTheSave",
		updateLiftingAnItemFilesItUnderTheWordItsTypeHoldsAtTheSave,
	},
	{"UpdateFilesATopItemOutsideItsRouteWordBackUnderIt", updateFilesATopItemOutsideItsRouteWordBackUnderIt},
	{"CreateOfATypeDeletedSinceItWasReadAnswersAnUnknownType", createOfATypeDeletedSinceItWasReadAnswersAnUnknownType},
	{"CreateKeepsAWordTheCMSKeepsOffTheRootOnceItsTypeTookIt", createKeepsAWordTheCMSKeepsOffTheRootOnceItsTypeTookIt},
	{"UpdateKeepsAStrayOfTheRootTypeOffAWordTheCMSKeeps", updateKeepsAStrayOfTheRootTypeOffAWordTheCMSKeeps},
}

// routeWordStopCases are the type cases that stop a route word change while an address sits outside the old word.
var routeWordStopCases = []Case{
	{"UpdateStopsWhileItemsSitOutsideTheOldRouteWord", updateStopsWhileItemsSitOutsideTheOldRouteWord},
	{
		"UpdateStopsAHandOverOfTheRootWhileItemsSitOutsideTheOldRouteWord",
		updateStopsAHandOverOfTheRootWhileItemsSitOutsideTheOldRouteWord,
	},
	{"UpdateMovesTheRouteWordOnceTheItemOutsideItIsSavedAgain", updateMovesTheRouteWordOnceTheItemOutsideItIsSavedAgain},
}

// movedTo stores the type under the route word and returns it as stored.
func movedTo(t *testing.T, types content.TypeStore, kind content.Type, word string) content.Type {
	t.Helper()
	moved := kind
	moved.RouteWord, moved.UpdatedAt = word, time.Now().UTC()
	stored, err := types.Update(t.Context(), moved)
	if err != nil {
		t.Fatalf("moving %s to %s: %v", kind.Key, word, err)
	}
	return stored
}

// plantedAt sets the item's address through the stores' hook and returns the item as stored.
func plantedAt(t *testing.T, s Stores, item content.Content, path string) content.Content {
	t.Helper()
	if s.PlantPath == nil {
		t.Fatalf("the stores lend no PlantPath, want one to set %q on %q", path, item.Title)
	}
	s.PlantPath(t, item.ID, path)
	planted, err := s.Content.ByID(t.Context(), item.ID)
	if err != nil {
		t.Fatalf("ByID(%q) error = %v, want nil", item.Title, err)
	}
	if planted.Path != path {
		t.Fatalf("%q answers at %q after the plant, want %q", item.Title, planted.Path, path)
	}
	return planted
}

// outsideNamed fails the case unless the error carries address_outside_route_word, the type, word, count and first.
func outsideNamed(t *testing.T, err error, key, word string, items int, path string) {
	t.Helper()
	if code, _ := content.CodeOf(err); code != "address_outside_route_word" {
		t.Errorf("code = %q, want address_outside_route_word", code)
	}
	held, _ := content.DetailsOf(err)
	if held["type"] != key || held["word"] != word || held["items"] != items || held["path"] != path {
		t.Errorf("details = %v, want the type %s under %s with %d outside, first %s", held, key, word, items, path)
	}
}

// nestingCar stores the car type told to nest and returns it.
func nestingCar(t *testing.T, types content.TypeStore) content.Type {
	t.Helper()
	car := CarType(t)
	car.Hierarchical = true
	stored, err := types.Create(t.Context(), car)
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	return stored
}

// createFilesATopItemUnderTheWordItsTypeHoldsAtTheSave files an item built before its type moved under the new word.
func createFilesATopItemUnderTheWordItsTypeHoldsAtTheSave(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	car, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	built, err := content.New(car, nil, "Roadster", author)
	if err != nil {
		t.Fatalf("New(Roadster) error = %v, want nil", err)
	}
	movedTo(t, s.Types, car, "vehicles")

	created, err := s.Content.Create(t.Context(), built)

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Path != "vehicles/roadster" {
		t.Errorf("Create() path = %q, want vehicles/roadster, the word car holds at the save", created.Path)
	}
}

// createHoldingValuesFilesATopItemUnderTheWordItsTypeHoldsAtTheSave files an item holding values under the new word.
func createHoldingValuesFilesATopItemUnderTheWordItsTypeHoldsAtTheSave(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	car, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	DeclareTypedField(t, s.Types, car.Key, "subtitle")
	built, err := content.New(car, nil, "Roadster", author)
	if err != nil {
		t.Fatalf("New(Roadster) error = %v, want nil", err)
	}
	built.Fields = content.Values{"subtitle": "Two seats"}
	movedTo(t, s.Types, car, "vehicles")

	created, err := s.Content.Create(t.Context(), built)

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Path != "vehicles/roadster" {
		t.Errorf("Create() path = %q, want vehicles/roadster, the word car holds at the save", created.Path)
	}
	if created.Fields["subtitle"] != "Two seats" {
		t.Errorf("Create() values = %v, want the subtitle Two seats kept", created.Fields)
	}
}

// updateLiftingAnItemFilesItUnderTheWordItsTypeHoldsAtTheSave lifts an item out of its parent under the new word.
func updateLiftingAnItemFilesItUnderTheWordItsTypeHoldsAtTheSave(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	stale := nestingCar(t, s.Types)
	fleet := StoreItem(t, s.Content, stale, nil, "Fleet", author)
	roadster := StoreItem(t, s.Content, stale, &fleet, "Roadster", author)
	movedTo(t, s.Types, stale, "vehicles")
	fresh, err := s.Content.ByID(t.Context(), roadster.ID)
	if err != nil {
		t.Fatalf("ByID(Roadster) error = %v, want nil", err)
	}
	lifted, err := content.Reparent(stale, fresh, nil, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	lifted.UpdatedAt = time.Now().UTC()

	updated, err := s.Content.Update(t.Context(), lifted, fresh.UpdatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Path != "vehicles/roadster" {
		t.Errorf("Update() path = %q, want vehicles/roadster, the word car holds at the save", updated.Path)
	}
}

// updateFilesATopItemOutsideItsRouteWordBackUnderIt saves an item set outside its type's word back under the word.
func updateFilesATopItemOutsideItsRouteWordBackUnderIt(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	car, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	moved := movedTo(t, s.Types, car, "vehicles")
	stray := plantedAt(t, s, StoreItem(t, s.Content, moved, nil, "Roadster", author), "cars/roadster")

	updated, err := s.Content.Update(t.Context(), EditTitle(stray, "Roadster Coupe"), stray.UpdatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Path != "vehicles/roadster" {
		t.Errorf("Update() path = %q, want vehicles/roadster, the word car holds at the save", updated.Path)
	}
}

// createOfATypeDeletedSinceItWasReadAnswersAnUnknownType names the type a create could not find and stores nothing.
func createOfATypeDeletedSinceItWasReadAnswersAnUnknownType(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	car, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	built, err := content.New(car, nil, "Roadster", author)
	if err != nil {
		t.Fatalf("New(Roadster) error = %v, want nil", err)
	}
	if err := s.Types.Delete(t.Context(), car.Key); err != nil {
		t.Fatalf("Delete(car) error = %v, want nil", err)
	}

	_, err = s.Content.Create(t.Context(), built)

	if !errors.Is(err, content.ErrInvalidType) {
		t.Errorf("Create() error = %v, want %v", err, content.ErrInvalidType)
	}
	if code, _ := content.CodeOf(err); code != "type_unknown" {
		t.Errorf("code = %q, want type_unknown", code)
	}
	if held, _ := content.DetailsOf(err); held["value"] != car.Key {
		t.Errorf("details = %v, want the type %s named", held, car.Key)
	}
	if StillStored(t, s.Content, built) {
		t.Errorf("item stored, want the create of a deleted type to store nothing")
	}
}

// createKeepsAWordTheCMSKeepsOffTheRootOnceItsTypeTookIt rejects an item filed at a kept word of the root.
func createKeepsAWordTheCMSKeepsOffTheRootOnceItsTypeTookIt(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	car, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	built, err := content.New(car, nil, "Admin", author)
	if err != nil {
		t.Fatalf("New(Admin) error = %v, want nil", err)
	}
	promoted := car
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), promoted); err != nil {
		t.Fatalf("handing the root to the car type: %v", err)
	}

	created, err := s.Content.Create(t.Context(), built)

	if !errors.Is(err, content.ErrReservedAddress) {
		t.Errorf("Create() error = %v, path %q, want %v", err, created.Path, content.ErrReservedAddress)
	}
	carriedOnto(t, err, "admin")
	if StillStored(t, s.Content, built) {
		t.Errorf("item stored, want the rejected create to store nothing")
	}
}

// updateKeepsAStrayOfTheRootTypeOffAWordTheCMSKeeps rejects a rename the store would file at a kept word of the root.
func updateKeepsAStrayOfTheRootTypeOffAWordTheCMSKeeps(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	stray := plantedAt(t, s, StoreItem(t, s.Content, PostType(), nil, "Hello", author), "posts/hello")
	renamed, err := stray.Rename("admin")
	if err != nil {
		t.Fatalf("Rename(admin) error = %v, want nil", err)
	}
	renamed.UpdatedAt = time.Now().UTC()

	updated, err := s.Content.Update(t.Context(), renamed, stray.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrReservedAddress) {
		t.Fatalf("Update() error = %v, path %q, want %v", err, updated.Path, content.ErrReservedAddress)
	}
	carriedOnto(t, err, "admin")
	unmoved(t, s.Content, stray)
}

// updateStopsWhileItemsSitOutsideTheOldRouteWord keeps the type and every address while items sit outside its word.
func updateStopsWhileItemsSitOutsideTheOldRouteWord(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	car, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	moved := movedTo(t, s.Types, car, "vehicles")
	held := []content.Content{StoreItem(t, s.Content, moved, nil, "Sedan", author)}
	for _, stray := range []struct{ title, path string }{
		{"Roadster", "cars/roadster"}, {"Coupe", "cars-sport/coupe"}, {"Van", "vehicles-old/van"}, {"Hatch", "vehicles"},
	} {
		held = append(held, plantedAt(t, s, StoreItem(t, s.Content, moved, nil, stray.title, author), stray.path))
	}
	toGuides := moved
	toGuides.RouteWord, toGuides.UpdatedAt = "guides", time.Now().UTC()

	_, err = s.Types.Update(t.Context(), toGuides)

	if !errors.Is(err, content.ErrAddressOutsideRouteWord) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrAddressOutsideRouteWord)
	}
	outsideNamed(t, err, "car", "vehicles", 4, "cars-sport/coupe")
	if stored, err := s.Types.ByKey(t.Context(), "car"); err != nil || stored.RouteWord != "vehicles" {
		t.Errorf("car under %q, %v, want it still under vehicles", stored.RouteWord, err)
	}
	unmoved(t, s.Content, held...)
}

// updateStopsAHandOverOfTheRootWhileItemsSitOutsideTheOldRouteWord keeps both types and their items where they stand.
func updateStopsAHandOverOfTheRootWhileItemsSitOutsideTheOldRouteWord(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	car, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	moved := movedTo(t, s.Types, car, "vehicles")
	stray := plantedAt(t, s, StoreItem(t, s.Content, moved, nil, "Roadster", author), "cars/roadster")
	hello := StoreItem(t, s.Content, PostType(), nil, "Hello", author)
	promoted := moved
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()

	_, err = s.Types.Update(t.Context(), promoted)

	if !errors.Is(err, content.ErrAddressOutsideRouteWord) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrAddressOutsideRouteWord)
	}
	outsideNamed(t, err, "car", "vehicles", 1, "cars/roadster")
	if post, err := s.Types.ByKey(t.Context(), content.TypePost); err != nil || !post.Default || post.RouteWord != "" {
		t.Errorf("post default %t under %q, %v, want it still the default at the root", post.Default, post.RouteWord, err)
	}
	if stored, err := s.Types.ByKey(t.Context(), "car"); err != nil || stored.Default || stored.RouteWord != "vehicles" {
		t.Errorf("car default %t under %q, %v, want it still under vehicles", stored.Default, stored.RouteWord, err)
	}
	unmoved(t, s.Content, hello, stray)
}

// updateMovesTheRouteWordOnceTheItemOutsideItIsSavedAgain moves the type once a save filed its stray under the word.
func updateMovesTheRouteWordOnceTheItemOutsideItIsSavedAgain(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	car, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	moved := movedTo(t, s.Types, car, "vehicles")
	stray := plantedAt(t, s, StoreItem(t, s.Content, moved, nil, "Roadster", author), "cars/roadster")
	toGuides := moved
	toGuides.RouteWord, toGuides.UpdatedAt = "guides", time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), toGuides); !errors.Is(err, content.ErrAddressOutsideRouteWord) {
		t.Fatalf("Update() error = %v, want the change stopped while Roadster sits outside vehicles", err)
	}
	if _, err := s.Content.Update(t.Context(), EditTitle(stray, "Roadster Coupe"), stray.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("saving Roadster again: %v", err)
	}

	movedTo(t, s.Types, moved, "guides")

	if got := AddressOf(t, s.Content, stray.ID); got != "guides/roadster" {
		t.Errorf("Roadster answers at %q, want guides/roadster once saved and carried", got)
	}
}
