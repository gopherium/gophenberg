// SPDX-License-Identifier: Apache-2.0

// Package contenttest runs one set of behaviour cases against any content.Store.
package contenttest

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// Stores is one fresh, empty set of stores a case runs against.
type Stores struct {
	Content   content.Store
	Types     content.TypeStore
	AddAuthor func(t *testing.T, name string) uuid.UUID
}

// Factory returns fresh stores for one case, skipping the case when its engine is out of reach.
type Factory func(t *testing.T) Stores

// Case is one named behaviour every content store shares.
type Case struct {
	Name string
	Run  func(t *testing.T, s Stores)
}

// Cases returns every content store case in a fixed order.
func Cases() []Case {
	return slices.Concat(
		contentCases, slugCases, revisionCases, autosaveCases, trashCases, pathCases, publishedCases, nestingCases,
		relationCases, backlinkCases, nestedRelationCases, listCases, filterCases, valueCases,
	)
}

// Run runs every content store case in parallel, each on stores of its own.
func Run(t *testing.T, fresh Factory) {
	t.Helper()
	for _, c := range Cases() {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			c.Run(t, fresh(t))
		})
	}
}
