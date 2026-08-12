// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"
	"sync"
)

// Registry answers which content types the CMS holds, caching what it reads.
type Registry struct {
	mu     sync.RWMutex
	store  TypeStore
	byKey  map[string]Type
	order  []Type
	loaded bool
}

// NewRegistry returns a [Registry] reading through store.
func NewRegistry(store TypeStore) *Registry {
	return &Registry{store: store}
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
	if err := r.settled(ctx, t); err != nil {
		return Type{}, err
	}
	updated, err := r.store.Update(ctx, t)
	if err != nil {
		return Type{}, err
	}
	r.invalidate()
	return updated, nil
}

// settled reports whether the edit leaves the registry with one active default and one type per route word.
func (r *Registry) settled(ctx context.Context, t Type) error {
	types, err := r.All(ctx)
	if err != nil {
		return err
	}
	for _, stored := range types {
		if stored.Key == t.Key {
			if stored.Default && (!t.Default || !t.Active) {
				return ErrDefaultRequired
			}
			continue
		}
		if stored.RouteWord == t.RouteWord {
			return wordConflict(t.RouteWord)
		}
		if stored.Default && t.Default {
			return ErrRootTaken
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
	if t.Default {
		return ErrDefaultRequired
	}
	if err := r.store.Delete(ctx, key); err != nil {
		return err
	}
	r.invalidate()
	return nil
}

// load fills the cache when it is cold.
func (r *Registry) load(ctx context.Context) error {
	r.mu.RLock()
	loaded := r.loaded
	r.mu.RUnlock()
	if loaded {
		return nil
	}
	types, err := r.store.List(ctx)
	if err != nil {
		return err
	}
	byKey := make(map[string]Type, len(types))
	for _, t := range types {
		byKey[t.Key] = t
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byKey, r.order, r.loaded = byKey, types, true
	return nil
}

// invalidate drops the cache so the next read sees the registry as stored.
func (r *Registry) invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byKey, r.order, r.loaded = nil, nil, false
}
