// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"
	"fmt"
	"sync"
)

// Registry answers which content types the CMS holds, caching what it reads.
type Registry struct {
	mu         sync.RWMutex
	store      TypeStore
	locations  *ParamRegistry
	byKey      map[string]Type
	order      []Type
	loaded     bool
	generation int
}

// NewRegistry returns a [Registry] reading through store.
func NewRegistry(store TypeStore) *Registry {
	return &Registry{store: store}
}

// WithParams returns the registry evaluating locations against the given rule sources.
func (r *Registry) WithParams(params *ParamRegistry) *Registry {
	r.locations = params
	return r
}

// Params returns the rule sources locations evaluate against, the built in ones by default.
func (r *Registry) Params(ctx context.Context) *ParamRegistry {
	r.mu.RLock()
	held := r.locations
	r.mu.RUnlock()
	if held != nil {
		return held
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.locations == nil {
		r.locations = DefaultParamRegistry(func(context.Context) ([]Choice, error) {
			types, err := r.All(ctx)
			if err != nil {
				return nil, err
			}
			choices := make([]Choice, len(types))
			for i, t := range types {
				choices[i] = Choice{Value: t.Key, Label: t.PluralLabel}
			}
			return choices, nil
		})
	}
	return r.locations
}

// All returns every registered type, active or not, in registration order.
func (r *Registry) All(ctx context.Context) ([]Type, error) {
	if err := r.load(ctx); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]Type, len(r.order))
	copy(types, r.order)
	return types, nil
}

// ByKey returns the type carrying the key, or [ErrTypeNotFound].
func (r *Registry) ByKey(ctx context.Context, key string) (Type, error) {
	if err := r.load(ctx); err != nil {
		return Type{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, found := r.byKey[key]
	if !found {
		return Type{}, ErrTypeNotFound
	}
	return t, nil
}

// Active returns the type carrying the key when it is active, or the reason it serves nothing.
func (r *Registry) Active(ctx context.Context, key string) (Type, error) {
	t, err := r.ByKey(ctx, key)
	if err != nil {
		return Type{}, err
	}
	if !t.Active {
		return Type{}, ErrTypeInactive
	}
	return t, nil
}

// Default returns the type living at the root, or [ErrTypeNotFound].
func (r *Registry) Default(ctx context.Context) (Type, error) {
	types, err := r.All(ctx)
	if err != nil {
		return Type{}, err
	}
	for _, t := range types {
		if t.Default {
			return t, nil
		}
	}
	return Type{}, ErrTypeNotFound
}

// ByRouteWord returns the active type answering under the word, or [ErrTypeNotFound].
func (r *Registry) ByRouteWord(ctx context.Context, word string) (Type, error) {
	types, err := r.All(ctx)
	if err != nil {
		return Type{}, err
	}
	for _, t := range types {
		if t.Active && t.RouteWord == word {
			return t, nil
		}
	}
	return Type{}, ErrTypeNotFound
}

// Create registers the type, or reports why it may not join the registry.
func (r *Registry) Create(ctx context.Context, t Type) (Type, error) {
	if err := t.Validate(); err != nil {
		return Type{}, err
	}
	if err := r.free(ctx, t); err != nil {
		return Type{}, err
	}
	created, err := r.store.Create(ctx, t)
	if err != nil {
		return Type{}, err
	}
	r.invalidate()
	return created, nil
}

// free reports whether the key and the route word are open to the type.
func (r *Registry) free(ctx context.Context, t Type) error {
	types, err := r.All(ctx)
	if err != nil {
		return err
	}
	for _, stored := range types {
		if stored.Key == t.Key {
			return ErrTypeTaken
		}
		if stored.RouteWord == t.RouteWord {
			return wordConflict(t.RouteWord)
		}
	}
	return nil
}

// wordConflict returns the reason another type already answers under the route word.
func wordConflict(routeWord string) error {
	if routeWord == "" {
		return ErrRootTaken
	}
	return ErrRouteWordTaken
}

// Update stores the edited type, or reports why the registry keeps it as it stands.
func (r *Registry) Update(ctx context.Context, t Type) (Type, error) {
	if err := t.Validate(); err != nil {
		return Type{}, err
	}
	types, err := r.All(ctx)
	if err != nil {
		return Type{}, err
	}
	stored, found := typeAmong(types, t.Key)
	if !found {
		return Type{}, ErrTypeNotFound
	}
	if err := pluginKeeps(stored, t); err != nil {
		return Type{}, err
	}
	t.Origin = stored.Origin
	if err := settled(types, t); err != nil {
		return Type{}, err
	}
	updated, err := r.store.Update(ctx, t)
	if err != nil {
		return Type{}, err
	}
	r.invalidate()
	return updated, nil
}

// typeAmong returns the type carrying the key among the given ones.
func typeAmong(types []Type, key string) (Type, bool) {
	for _, t := range types {
		if t.Key == key {
			return t, true
		}
	}
	return Type{}, false
}

// settled reports whether the edit leaves the registry with one active default and one type per route word.
func settled(types []Type, t Type) error {
	for _, stored := range types {
		if stored.Key == t.Key {
			if stored.Default && (!t.Default || !t.Active) {
				return ErrDefaultRequired
			}
			continue
		}
		if stored.RouteWord == t.RouteWord && !t.Default {
			return wordConflict(t.RouteWord)
		}
	}
	return nil
}

// Delete removes the type, or reports why the registry keeps it.
func (r *Registry) Delete(ctx context.Context, key string) error {
	t, err := r.ByKey(ctx, key)
	if err != nil {
		return err
	}
	if t.Origin != "" {
		return OwnedBy(t.Origin)
	}
	if t.Default {
		return ErrDefaultRequired
	}
	if err := r.untargeted(ctx, key); err != nil {
		return err
	}
	if err := r.store.Delete(ctx, key); err != nil {
		return err
	}
	r.invalidate()
	return nil
}

// untargeted reports whether a relation field in any group still points at the type.
func (r *Registry) untargeted(ctx context.Context, key string) error {
	groups, err := r.store.ListGroups(ctx)
	if err != nil {
		return err
	}
	for _, g := range groups {
		for _, f := range g.Fields {
			if f.RelatesTo == key {
				return Refuse(ErrTypeTargeted, "type_targeted",
					fmt.Sprintf("%s (%s in %s)", ErrTypeTargeted, f.Key, g.Title),
					Details{"field": f.Key, "group": g.Title})
			}
		}
	}
	return nil
}

// CreateField declares the field on its type, or reports why the registry refuses it.
func (r *Registry) CreateField(ctx context.Context, f Field) (Field, error) {
	if err := f.Validate(); err != nil {
		return Field{}, err
	}
	t, err := r.ByKey(ctx, f.TypeKey)
	if err != nil {
		return Field{}, err
	}
	if f.Kind == FieldKindRelation {
		if _, err := r.ByKey(ctx, f.RelatesTo); err != nil {
			return Field{}, ErrTargetUnknown
		}
	}
	if _, err := fieldOf(t, f.Key); err == nil {
		return Field{}, ErrFieldTaken
	}
	created, err := r.store.CreateField(ctx, f)
	if err != nil {
		return Field{}, err
	}
	r.invalidate()
	return created, nil
}

// orderCovers reports whether keys name every declared field exactly once.
func orderCovers(fields []Field, keys []string) error {
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if _, err := fieldAmong(fields, key); err != nil {
			return err
		}
		if seen[key] {
			return Refuse(ErrFieldOrder, "field_order_incomplete",
				"content: the order names a field twice", Details{"key": key})
		}
		seen[key] = true
	}
	if len(keys) != len(fields) {
		return Refuse(ErrFieldOrder, "field_order_incomplete",
			"content: the order leaves declared fields out", nil)
	}
	return nil
}

// fieldOf returns the declared field carrying the key, or [ErrFieldNotFound].
func fieldOf(t Type, key string) (Field, error) {
	return fieldAmong(t.Fields, key)
}

// fieldAmong returns the field carrying the key, or [ErrFieldNotFound].
func fieldAmong(fields []Field, key string) (Field, error) {
	for _, f := range fields {
		if f.Key == key {
			return f, nil
		}
	}
	return Field{}, ErrFieldNotFound
}

// load fills the cache when it is cold, reading again when a write lands mid read.
func (r *Registry) load(ctx context.Context) error {
	for {
		r.mu.RLock()
		loaded, read := r.loaded, r.generation
		r.mu.RUnlock()
		if loaded {
			return nil
		}
		types, err := r.store.List(ctx)
		if err != nil {
			return err
		}
		if r.publish(types, read) {
			return nil
		}
	}
}

// publish holds the types the read returned, unless a write landed while it read.
func (r *Registry) publish(types []Type, read int) bool {
	byKey := make(map[string]Type, len(types))
	for _, t := range types {
		byKey[t.Key] = t
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.generation != read {
		return false
	}
	r.byKey, r.order, r.loaded = byKey, types, true
	return true
}

// invalidate drops the cache so the next read sees the registry as stored.
func (r *Registry) invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byKey, r.order, r.loaded = nil, nil, false
	r.generation++
}
