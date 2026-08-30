// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// groupingStore holds field groups beside the types the fake registry serves.
type groupingStore struct {
	*fakeTypeStore
	groups          []content.Group
	nextID          int
	groupsErr       error
	createErr       error
	updateErr       error
	deleteErr       error
	moveErr         error
	orderErr        error
	updateFieldErr  error
	deleteFieldErr  error
	reorderFieldErr error

	failReadAfterOrder bool
}

// newGroupingStore returns a store holding the built-in post type and no groups.
func newGroupingStore() *groupingStore {
	return &groupingStore{fakeTypeStore: newFakeTypeStore(), nextID: 1}
}

// ListGroups returns the held groups in position order.
func (s *groupingStore) ListGroups(context.Context) ([]content.Group, error) {
	if s.groupsErr != nil {
		return nil, s.groupsErr
	}
	held := make([]content.Group, len(s.groups))
	copy(held, s.groups)
	return held, nil
}

// CreateGroup stores a new group at the end of the order.
func (s *groupingStore) CreateGroup(_ context.Context, g content.Group) (content.Group, error) {
	if s.createErr != nil {
		return content.Group{}, s.createErr
	}
	g.ID, g.Active, g.Position = s.nextID, true, len(s.groups)+1
	s.nextID++
	s.groups = append(s.groups, g)
	return g, nil
}

// UpdateGroup stores the group's title, location and active flag.
func (s *groupingStore) UpdateGroup(_ context.Context, g content.Group) (content.Group, error) {
	if s.updateErr != nil {
		return content.Group{}, s.updateErr
	}
	for i, held := range s.groups {
		if held.ID != g.ID {
			continue
		}
		held.Title, held.Location, held.Active = g.Title, g.Location, g.Active
		s.groups[i] = held
		return held, nil
	}
	return content.Group{}, content.ErrGroupNotFound
}

// DeleteGroup removes the group and every field it holds.
func (s *groupingStore) DeleteGroup(_ context.Context, id int) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for i, held := range s.groups {
		if held.ID == id {
			s.groups = append(s.groups[:i], s.groups[i+1:]...)
			return nil
		}
	}
	return content.ErrGroupNotFound
}

// ReorderGroups stores the given order on the groups.
func (s *groupingStore) ReorderGroups(_ context.Context, ids []int) error {
	if s.orderErr != nil {
		return s.orderErr
	}
	ordered := make([]content.Group, 0, len(ids))
	for _, id := range ids {
		for _, held := range s.groups {
			if held.ID == id {
				ordered = append(ordered, held)
			}
		}
	}
	s.groups = ordered
	return nil
}

// CreateFieldInGroup declares the field inside the group.
func (s *groupingStore) CreateFieldInGroup(_ context.Context, groupID int, f content.Field) (content.Field, error) {
	if s.createFieldErr != nil {
		return content.Field{}, s.createFieldErr
	}
	for i, held := range s.groups {
		if held.ID != groupID {
			continue
		}
		f.GroupID = groupID
		s.groups[i].Fields = append(held.Fields, f)
		return f, nil
	}
	return content.Field{}, content.ErrGroupNotFound
}

// UpdateFieldInGroup stores the field's label and required flag inside its group.
func (s *groupingStore) UpdateFieldInGroup(
	_ context.Context, groupID int, f content.Field, _ time.Time,
) (content.Field, error) {
	if s.updateFieldErr != nil {
		return content.Field{}, s.updateFieldErr
	}
	for i, held := range s.groups {
		if held.ID != groupID {
			continue
		}
		for j, stored := range held.Fields {
			if stored.Key == f.Key {
				s.groups[i].Fields[j] = f
				return f, nil
			}
		}
	}
	return content.Field{}, content.ErrFieldNotFound
}

// DeleteFieldInGroup removes the field from its group.
func (s *groupingStore) DeleteFieldInGroup(_ context.Context, groupID int, key string) error {
	if s.deleteFieldErr != nil {
		return s.deleteFieldErr
	}
	for i, held := range s.groups {
		if held.ID != groupID {
			continue
		}
		for j, stored := range held.Fields {
			if stored.Key == key {
				s.groups[i].Fields = append(held.Fields[:j], held.Fields[j+1:]...)
				return nil
			}
		}
	}
	return content.ErrFieldNotFound
}

// ReorderFieldsInGroup stores the given order on the group's fields.
func (s *groupingStore) ReorderFieldsInGroup(_ context.Context, groupID int, keys []string) error {
	if s.reorderFieldErr != nil {
		return s.reorderFieldErr
	}
	if s.failReadAfterOrder {
		s.groupsErr = errStoreDown
	}
	for i, held := range s.groups {
		if held.ID != groupID {
			continue
		}
		ordered := make([]content.Field, 0, len(keys))
		for _, key := range keys {
			for _, stored := range held.Fields {
				if stored.Key == key {
					ordered = append(ordered, stored)
				}
			}
		}
		s.groups[i].Fields = ordered
		return nil
	}
	return content.ErrGroupNotFound
}

// MoveField carries the field into another group.
func (s *groupingStore) MoveField(_ context.Context, groupID int, key string, toGroup int) (content.Field, error) {
	if s.moveErr != nil {
		return content.Field{}, s.moveErr
	}
	var carried content.Field
	for i, held := range s.groups {
		if held.ID != groupID {
			continue
		}
		for at, f := range held.Fields {
			if f.Key != key {
				continue
			}
			carried = f
			s.groups[i].Fields = append(held.Fields[:at], held.Fields[at+1:]...)
		}
	}
	if carried.Key == "" {
		return content.Field{}, content.ErrFieldNotFound
	}
	carried.GroupID = toGroup
	for i, held := range s.groups {
		if held.ID == toGroup {
			s.groups[i].Fields = append(held.Fields, carried)
			return carried, nil
		}
	}
	return content.Field{}, content.ErrGroupNotFound
}

// namingPost returns a location matching the built-in post type.
func namingPost() content.Rules {
	return content.Rules{{{
		Source: content.ScreenContentType, Operator: content.OperatorIs, Value: content.TypePost,
	}}}
}

// groupedTextField returns a text field definition ready to declare inside a group.
func groupedTextField(t *testing.T) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{Key: "subtitle", Label: "A Field", Kind: content.FieldKindText})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	return built
}

// groupNaming returns a stored group holding the given location.
func groupNaming(t *testing.T, registry *content.Registry, title string, location content.Rules) content.Group {
	t.Helper()
	created, err := registry.CreateGroup(t.Context(), content.Group{Title: title, Location: location})
	if err != nil {
		t.Fatalf("CreateGroup(%s) error = %v, want nil", title, err)
	}
	return created
}

func TestRegistryReportsAGroupStoreThatWillNotAnswer(t *testing.T) {
	t.Parallel()

	for name, run := range map[string]func(*content.Registry, *groupingStore) error{
		"Groups": func(r *content.Registry, s *groupingStore) error {
			s.groupsErr = errStoreDown
			_, err := r.Groups(t.Context())
			return err
		},
		"CreateGroup": func(r *content.Registry, s *groupingStore) error {
			s.createErr = errStoreDown
			_, err := r.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: namingPost()})
			return err
		},
		"UpdateGroup": func(r *content.Registry, s *groupingStore) error {
			stored := groupNaming(t, r, "Extras", namingPost())
			s.updateErr = errStoreDown
			_, err := r.UpdateGroup(t.Context(), stored)
			return err
		},
		"DeleteGroup": func(r *content.Registry, s *groupingStore) error {
			s.deleteErr = errStoreDown
			return r.DeleteGroup(t.Context(), 1)
		},
		"ReorderGroups": func(r *content.Registry, s *groupingStore) error {
			stored := groupNaming(t, r, "Extras", namingPost())
			s.orderErr = errStoreDown
			_, err := r.ReorderGroups(t.Context(), []int{stored.ID})
			return err
		},
		"CreateFieldInGroup": func(r *content.Registry, s *groupingStore) error {
			stored := groupNaming(t, r, "Extras", namingPost())
			s.createFieldErr = errStoreDown
			_, err := r.CreateFieldInGroup(t.Context(), stored.ID, groupedTextField(t))
			return err
		},
		"MoveField": func(r *content.Registry, s *groupingStore) error {
			from := groupNaming(t, r, "From", namingPost())
			to := groupNaming(t, r, "To", namingPost())
			s.moveErr = errStoreDown
			_, err := r.MoveField(t.Context(), from.ID, "subtitle", to.ID)
			return err
		},
		"UpdateFieldInGroup": func(r *content.Registry, s *groupingStore) error {
			held := groupNaming(t, r, "Extras", namingPost())
			created, err := r.CreateFieldInGroup(t.Context(), held.ID, groupedTextField(t))
			if err != nil {
				return err
			}
			s.updateFieldErr = errStoreDown
			_, err = r.UpdateFieldInGroup(t.Context(),
				held.ID, content.Field{Key: "subtitle", Label: "Renamed"}, created.UpdatedAt)
			return err
		},
		"DeleteFieldInGroup": func(r *content.Registry, s *groupingStore) error {
			held := groupNaming(t, r, "Extras", namingPost())
			if _, err := r.CreateFieldInGroup(t.Context(), held.ID, groupedTextField(t)); err != nil {
				return err
			}
			s.deleteFieldErr = errStoreDown
			return r.DeleteFieldInGroup(t.Context(), held.ID, "subtitle")
		},
		"ReorderFieldsInGroup": func(r *content.Registry, s *groupingStore) error {
			held := groupNaming(t, r, "Extras", namingPost())
			if _, err := r.CreateFieldInGroup(t.Context(), held.ID, groupedTextField(t)); err != nil {
				return err
			}
			s.reorderFieldErr = errStoreDown
			_, err := r.ReorderFieldsInGroup(t.Context(), held.ID, []string{"subtitle"})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newGroupingStore()

			err := run(content.NewRegistry(store), store)

			if !errors.Is(err, errStoreDown) {
				t.Errorf("%s() error = %v, want %v", name, err, errStoreDown)
			}
		})
	}
}

func TestRegistryRefusesAnEditedGroupItCannotSettle(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	stored := groupNaming(t, registry, "Extras", namingPost())

	stored.Title = "   "
	_, err := registry.UpdateGroup(t.Context(), stored)

	if !errors.Is(err, content.ErrInvalidGroupTitle) {
		t.Errorf("UpdateGroup() error = %v, want %v", err, content.ErrInvalidGroupTitle)
	}
}

func TestRegistryUpdatesAGroupTheStoreNoLongerHolds(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())

	_, err := registry.UpdateGroup(t.Context(), content.Group{
		ID: 4242, Title: "Vanished", Location: namingPost(),
	})

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("UpdateGroup() error = %v, want the store's error carried", err)
	}
}

func TestRegistryReportsTheTypesACollisionCheckCannotRead(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	held := groupNaming(t, registry, "Extras", namingPost())
	store.listErr = errStoreDown

	_, err := registry.CreateFieldInGroup(t.Context(), held.ID, groupedTextField(t))

	if !errors.Is(err, errStoreDown) {
		t.Errorf("CreateFieldInGroup() error = %v, want %v", err, errStoreDown)
	}
}

func TestRegistryReportsAGroupsListItCannotRead(t *testing.T) {
	t.Parallel()

	for name, run := range map[string]func(*content.Registry) error{
		"ReorderGroups": func(r *content.Registry) error {
			_, err := r.ReorderGroups(t.Context(), []int{1})
			return err
		},
		"CreateFieldInGroup": func(r *content.Registry) error {
			_, err := r.CreateFieldInGroup(t.Context(), 1, groupedTextField(t))
			return err
		},
		"MoveField": func(r *content.Registry) error {
			_, err := r.MoveField(t.Context(), 1, "subtitle", 2)
			return err
		},
		"UpdateGroup": func(r *content.Registry) error {
			_, err := r.UpdateGroup(t.Context(), content.Group{ID: 1, Title: "Extras", Location: namingPost()})
			return err
		},
		"UpdateFieldInGroup": func(r *content.Registry) error {
			_, err := r.UpdateFieldInGroup(t.Context(), 1, content.Field{Key: "subtitle", Label: "Renamed"}, time.Time{})
			return err
		},
		"DeleteFieldInGroup": func(r *content.Registry) error {
			return r.DeleteFieldInGroup(t.Context(), 1, "subtitle")
		},
		"ReorderFieldsInGroup": func(r *content.Registry) error {
			_, err := r.ReorderFieldsInGroup(t.Context(), 1, []string{"subtitle"})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newGroupingStore()
			store.groupsErr = errStoreDown

			if err := run(content.NewRegistry(store)); !errors.Is(err, errStoreDown) {
				t.Errorf("%s() error = %v, want %v", name, err, errStoreDown)
			}
		})
	}
}

func TestRegistryRefusesAFieldItsGroupCannotHold(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())

	for name, asked := range map[string]content.Field{
		"a malformed key":        {Key: "Not A Key", Label: "Bad", Kind: content.FieldKindText},
		"an unregistered target": {Key: "cars", Label: "Cars", Kind: content.FieldKindRelation, RelatesTo: "vanished"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := registry.CreateFieldInGroup(t.Context(), 1, asked); err == nil {
				t.Errorf("CreateFieldInGroup() = nil, want %s refused", name)
			}
		})
	}
}

func TestRegistryReportsAFieldDeclaredInAGroupThatIsGone(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())

	_, err := registry.CreateFieldInGroup(t.Context(), 4242, groupedTextField(t))

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("CreateFieldInGroup() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}

func TestRegistryRefusesAnOrderNamingAGroupTwice(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	first := groupNaming(t, registry, "First", namingPost())
	groupNaming(t, registry, "Second", namingPost())

	_, err := registry.ReorderGroups(t.Context(), []int{first.ID, first.ID})

	if !errors.Is(err, content.ErrGroupOrder) {
		t.Errorf("ReorderGroups() error = %v, want %v", err, content.ErrGroupOrder)
	}
}

func TestRegistryRefusesAnOrderNamingAGroupThatIsGone(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	groupNaming(t, registry, "First", namingPost())

	_, err := registry.ReorderGroups(t.Context(), []int{4242})

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("ReorderGroups() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}

func TestRegistryOffersTheContentTypesARuleCanName(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}

	params, found := registry.Params(t.Context()).Param(content.ScreenContentType)

	if !found {
		t.Fatal("Param() found nothing, want the content type source offered")
	}
	choices, err := params.Values(t.Context())
	if err != nil {
		t.Fatalf("Values() error = %v, want nil", err)
	}
	if len(choices) != 2 || choices[1].Value != "car" || choices[1].Label != "Cars" {
		t.Errorf("Values() = %v, want a choice per registered type labelled in the plural", choices)
	}
}

func TestRegistryReportsTheTypesItCannotOffer(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	params, _ := registry.Params(t.Context()).Param(content.ScreenContentType)
	store.listErr = errStoreDown

	if _, err := params.Values(t.Context()); !errors.Is(err, errStoreDown) {
		t.Errorf("Values() error = %v, want %v", err, errStoreDown)
	}
}

func TestRegistryTakesTheParamsItIsGiven(t *testing.T) {
	t.Parallel()

	params := content.NewParamRegistry()
	registry := content.NewRegistry(newGroupingStore()).WithParams(params)

	if registry.Params(t.Context()) != params {
		t.Error("Params() answered another registry, want the one it was given")
	}
}

func TestRegistryCreatesAGroup(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())

	created, err := registry.CreateGroup(t.Context(), content.Group{Title: "Article details", Location: namingPost()})

	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if created.Title != "Article details" || !created.Active {
		t.Errorf("created = %+v, want an active group carrying its title", created)
	}
	listed, err := registry.Groups(t.Context())
	if err != nil || len(listed) != 1 {
		t.Fatalf("Groups() = %v, %v, want the stored group listed", listed, err)
	}
}

func TestRegistryRefusesAGroupWithNoTitle(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())

	_, err := registry.CreateGroup(t.Context(), content.Group{Title: "  ", Location: namingPost()})

	if !errors.Is(err, content.ErrInvalidGroupTitle) {
		t.Errorf("CreateGroup() error = %v, want %v", err, content.ErrInvalidGroupTitle)
	}
}

func TestRegistryRefusesALocationItCannotValidate(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	unknown := content.Rules{{{Source: "vanished", Operator: content.OperatorIs, Value: "post"}}}

	_, err := registry.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: unknown})

	if !errors.Is(err, content.ErrRuleSourceUnknown) {
		t.Errorf("CreateGroup() error = %v, want %v", err, content.ErrRuleSourceUnknown)
	}
}

func TestRegistryUpdatesAGroup(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	stored := groupNaming(t, registry, "Article details", namingPost())

	stored.Title = "Renamed"
	stored.Active = false
	updated, err := registry.UpdateGroup(t.Context(), stored)

	if err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil", err)
	}
	if updated.Title != "Renamed" || updated.Active {
		t.Errorf("updated = %+v, want the edited title and the resting flag", updated)
	}
}

func TestRegistryDeletesAGroup(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	stored := groupNaming(t, registry, "Article details", namingPost())

	if err := registry.DeleteGroup(t.Context(), stored.ID); err != nil {
		t.Fatalf("DeleteGroup() error = %v, want nil", err)
	}

	listed, err := registry.Groups(t.Context())
	if err != nil || len(listed) != 0 {
		t.Errorf("Groups() = %v, %v, want no group left", listed, err)
	}
}

func TestRegistryReordersGroups(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	first := groupNaming(t, registry, "First", namingPost())
	second := groupNaming(t, registry, "Second", namingPost())

	reordered, err := registry.ReorderGroups(t.Context(), []int{second.ID, first.ID})

	if err != nil {
		t.Fatalf("ReorderGroups() error = %v, want nil", err)
	}
	if len(reordered) != 2 || reordered[0].ID != second.ID {
		t.Errorf("ReorderGroups() = %v, want the asked order", reordered)
	}
}

func TestRegistryRefusesAnOrderLeavingAGroupOut(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	first := groupNaming(t, registry, "First", namingPost())
	groupNaming(t, registry, "Second", namingPost())

	_, err := registry.ReorderGroups(t.Context(), []int{first.ID})

	if !errors.Is(err, content.ErrGroupOrder) {
		t.Errorf("ReorderGroups() error = %v, want %v", err, content.ErrGroupOrder)
	}
}

func TestRegistryRefusesAFieldKeyAnotherMatchingGroupHolds(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	if _, err := registry.CreateFieldInGroup(t.Context(), held.ID, groupedTextField(t)); err != nil {
		t.Fatalf("declaring the first field: %v, want nil", err)
	}
	rival := groupNaming(t, registry, "Extras", namingPost())

	_, err := registry.CreateFieldInGroup(t.Context(), rival.ID, groupedTextField(t))

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Fatalf("CreateFieldInGroup() error = %v, want %v", err, content.ErrFieldTaken)
	}
	var refused *content.Error
	if !errors.As(err, &refused) || refused.Held["group"] != "Article details" {
		t.Errorf("details = %v, want the group already holding the key named", err)
	}
}

func TestRegistryLetsTwoGroupsHoldAKeyWhenTheyNeverMeet(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}
	posts := groupNaming(t, registry, "Post details", namingPost())
	cars := groupNaming(t, registry, "Car details", content.Rules{{{
		Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "car",
	}}})
	if _, err := registry.CreateFieldInGroup(t.Context(), posts.ID, groupedTextField(t)); err != nil {
		t.Fatalf("declaring the post field: %v, want nil", err)
	}

	_, err := registry.CreateFieldInGroup(t.Context(), cars.ID, groupedTextField(t))

	if err != nil {
		t.Errorf("CreateFieldInGroup() error = %v, want the same key allowed on another type", err)
	}
}

func TestRegistryRefusesALocationBringingTwoGroupsIntoCollision(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}
	posts := groupNaming(t, registry, "Post details", namingPost())
	cars := groupNaming(t, registry, "Car details", content.Rules{{{
		Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "car",
	}}})
	for _, held := range []content.Group{posts, cars} {
		if _, err := registry.CreateFieldInGroup(t.Context(), held.ID, groupedTextField(t)); err != nil {
			t.Fatalf("declaring the field on %s: %v, want nil", held.Title, err)
		}
	}

	cars.Location = namingPost()
	_, err := registry.UpdateGroup(t.Context(), cars)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("UpdateGroup() error = %v, want the collision the new location would make refused", err)
	}
}

func TestRegistryRefusesWakingAGroupIntoACollision(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	posts := groupNaming(t, registry, "Post details", namingPost())
	resting := groupNaming(t, registry, "Resting", namingPost())
	resting.Active = false
	if _, err := registry.UpdateGroup(t.Context(), resting); err != nil {
		t.Fatalf("resting the group: %v, want nil", err)
	}
	for _, id := range []int{posts.ID, resting.ID} {
		if _, err := registry.CreateFieldInGroup(t.Context(), id, groupedTextField(t)); err != nil {
			t.Fatalf("declaring the field in %d: %v, want nil", id, err)
		}
	}

	resting.Active = true
	_, err := registry.UpdateGroup(t.Context(), resting)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("UpdateGroup() error = %v, want waking into a collision refused", err)
	}
}

func TestRegistryRelabelsAFieldInsideItsGroup(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	created, err := registry.CreateFieldInGroup(t.Context(), held.ID, groupedTextField(t))
	if err != nil {
		t.Fatalf("declaring the field: %v, want nil", err)
	}

	updated, err := registry.UpdateFieldInGroup(t.Context(), held.ID, content.Field{
		Key: "subtitle", Label: "Renamed", Required: true,
	}, created.UpdatedAt)

	if err != nil {
		t.Fatalf("UpdateFieldInGroup() error = %v, want nil", err)
	}
	if updated.Label != "Renamed" || !updated.Required {
		t.Errorf("updated = %+v, want the new label and the required flag", updated)
	}
}

func TestRegistryReportsRelabelingAFieldNoGroupHolds(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())

	_, err := registry.UpdateFieldInGroup(t.Context(),
		held.ID, content.Field{Key: "absent", Label: "Absent"}, time.Time{})

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("UpdateFieldInGroup() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

func TestRegistryDeletesAFieldInsideItsGroup(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	if _, err := registry.CreateFieldInGroup(t.Context(), held.ID, groupedTextField(t)); err != nil {
		t.Fatalf("declaring the field: %v, want nil", err)
	}

	if err := registry.DeleteFieldInGroup(t.Context(), held.ID, "subtitle"); err != nil {
		t.Fatalf("DeleteFieldInGroup() error = %v, want nil", err)
	}

	groups, err := registry.Groups(t.Context())
	if err != nil || len(groups) != 1 || len(groups[0].Fields) != 0 {
		t.Errorf("Groups() = %v, %v, want the field gone from its group", groups, err)
	}
}

func TestRegistryReordersTheFieldsOfAGroup(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	for _, key := range []string{"subtitle", "footnote"} {
		built, err := content.NewField(content.Field{Key: key, Label: "A Field", Kind: content.FieldKindText})
		if err != nil {
			t.Fatalf("NewField(%s) error = %v, want nil", key, err)
		}
		if _, err := registry.CreateFieldInGroup(t.Context(), held.ID, built); err != nil {
			t.Fatalf("declaring %s: %v, want nil", key, err)
		}
	}

	reordered, err := registry.ReorderFieldsInGroup(t.Context(), held.ID, []string{"footnote", "subtitle"})

	if err != nil {
		t.Fatalf("ReorderFieldsInGroup() error = %v, want nil", err)
	}
	if len(reordered) != 2 || reordered[0].Key != "footnote" {
		t.Errorf("ReorderFieldsInGroup() = %v, want the asked order", reordered)
	}
}

func TestRegistryRefusesRelabelingAFieldToNothing(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	created, err := registry.CreateFieldInGroup(t.Context(), held.ID, groupedTextField(t))
	if err != nil {
		t.Fatalf("declaring the field: %v, want nil", err)
	}

	_, err = registry.UpdateFieldInGroup(t.Context(),
		held.ID, content.Field{Key: "subtitle", Label: ""}, created.UpdatedAt)

	if !errors.Is(err, content.ErrInvalidFieldLabel) {
		t.Errorf("UpdateFieldInGroup() error = %v, want %v", err, content.ErrInvalidFieldLabel)
	}
}

func TestRegistryReportsAnOrderedGroupItCannotReadBack(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	held := groupNaming(t, registry, "Article details", namingPost())
	if _, err := registry.CreateFieldInGroup(t.Context(), held.ID, groupedTextField(t)); err != nil {
		t.Fatalf("declaring the field: %v, want nil", err)
	}
	store.failReadAfterOrder = true

	_, err := registry.ReorderFieldsInGroup(t.Context(), held.ID, []string{"subtitle"})

	if !errors.Is(err, errStoreDown) {
		t.Errorf("ReorderFieldsInGroup() error = %v, want the failed read back reported", err)
	}
}

func TestRegistryReordersFieldsOfAGroupThatIsGone(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())

	_, err := registry.ReorderFieldsInGroup(t.Context(), 4242, []string{"subtitle"})

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("ReorderFieldsInGroup() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}

func TestRegistryRefusesAGroupOrderLeavingAFieldOut(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	if _, err := registry.CreateFieldInGroup(t.Context(), held.ID, groupedTextField(t)); err != nil {
		t.Fatalf("declaring the field: %v, want nil", err)
	}

	_, err := registry.ReorderFieldsInGroup(t.Context(), held.ID, []string{})

	if !errors.Is(err, content.ErrFieldOrder) {
		t.Errorf("ReorderFieldsInGroup() error = %v, want %v", err, content.ErrFieldOrder)
	}
}

func TestRegistryMovesAFieldToAnotherGroup(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	from := groupNaming(t, registry, "Article details", namingPost())
	to := groupNaming(t, registry, "Extras", namingPost())
	if _, err := registry.CreateFieldInGroup(t.Context(), from.ID, groupedTextField(t)); err != nil {
		t.Fatalf("declaring the field: %v, want nil", err)
	}

	moved, err := registry.MoveField(t.Context(), from.ID, "subtitle", to.ID)

	if err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}
	if moved.GroupID != to.ID {
		t.Errorf("GroupID = %d, want the field carried into %d", moved.GroupID, to.ID)
	}
}

func TestRegistryRefusesAMoveIntoACollision(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}
	cars := groupNaming(t, registry, "Car details", content.Rules{{{
		Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "car",
	}}})
	posts := groupNaming(t, registry, "Post details", namingPost())
	shared := groupNaming(t, registry, "Everywhere", content.Rules{{{
		Source: content.ScreenContentType, Operator: content.OperatorIs, Value: content.AnyContentType,
	}}})
	if _, err := registry.CreateFieldInGroup(t.Context(), cars.ID, groupedTextField(t)); err != nil {
		t.Fatalf("declaring the car field: %v, want nil", err)
	}
	if _, err := registry.CreateFieldInGroup(t.Context(), posts.ID, groupedTextField(t)); err != nil {
		t.Fatalf("declaring the post field: %v, want nil", err)
	}

	_, err := registry.MoveField(t.Context(), cars.ID, "subtitle", shared.ID)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("MoveField() error = %v, want the move into a shared group refused", err)
	}
}

func TestRegistryReportsAMoveToAGroupThatIsGone(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	from := groupNaming(t, registry, "Article details", namingPost())
	if _, err := registry.CreateFieldInGroup(t.Context(), from.ID, groupedTextField(t)); err != nil {
		t.Fatalf("declaring the field: %v, want nil", err)
	}

	_, err := registry.MoveField(t.Context(), from.ID, "subtitle", 4242)

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}
