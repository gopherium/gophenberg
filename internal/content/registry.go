// SPDX-License-Identifier: Apache-2.0

package content

import (
	"fmt"
	"math"
	"sync"
)

// TypePost is the name of the built-in post type.
const TypePost = "post"

// Type describes a kind of content the CMS stores.
type Type struct {
	Name         string
	Label        string
	Hierarchical bool
	Revisions    bool
	// RevisionCap bounds the revisions kept per content item. Zero keeps every revision.
	RevisionCap int
}

var registry = struct {
	mu    sync.RWMutex
	types map[string]Type
}{types: make(map[string]Type)}

// init registers the built-in post type.
func init() {
	Register(Type{Name: TypePost, Label: "Posts", Revisions: true, RevisionCap: 100})
}

// Register adds a content type to the registry and panics on an invalid one.
func Register(t Type) {
	if t.Name == "" {
		panic("content: register type without a name")
	}
	if t.RevisionCap > math.MaxInt32 {
		panic(fmt.Sprintf("content: type %q revision cap beyond the row limit", t.Name))
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, ok := registry.types[t.Name]; ok {
		panic(fmt.Sprintf("content: duplicate type %q", t.Name))
	}
	registry.types[t.Name] = t
}

// TypeByName returns the registered type of the given name.
func TypeByName(name string) (Type, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	t, ok := registry.types[name]
	return t, ok
}
