// SPDX-License-Identifier: Apache-2.0

// Package contenttest runs one set of behaviour cases against any content, type, site setting or user setting store.
package contenttest

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// Stores is one fresh set of stores holding only the built-in post type, and a hook that sets an item's address.
type Stores struct {
	Content      content.Store
	Types        content.TypeStore
	SiteSettings content.SiteSettingStore
	UserSettings content.UserSettingStore
	AddAuthor    func(t *testing.T, name string) uuid.UUID
	PlantPath    func(t *testing.T, id uuid.UUID, path string)
}

// Factory returns fresh stores for one case, skipping the case when its engine is out of reach.
type Factory func(t *testing.T) Stores

// Case is one named behaviour every store of its kind shares.
type Case struct {
	Name string
	Run  func(t *testing.T, s Stores)
}

// Cases returns every content store case in a fixed order.
func Cases() []Case {
	return slices.Concat(
		contentCases, slugCases, revisionCases, autosaveCases, trashCases, pathCases, publishedCases, nestingCases,
		relationCases, backlinkCases, nestedRelationCases, listCases, filterCases, valueCases, rootPaginationCases,
		routeWordSaveCases,
	)
}

// TypeCases returns every type store case in a fixed order.
func TypeCases() []Case {
	return slices.Concat(
		typeCases, typeRootWordCases, typeNestingCases, groupCases, groupValueCases, groupRecheckCases, groupRepointCases,
		typeRootPaginationCases, originCases, settingCases, kindCases, adoptCases, typeFieldCases, fieldParentCases,
		fieldDepthCases, subFieldWriteCases, fieldMoveCases, fieldMoveValueCases, fieldSweepCases,
		layoutSweepCases, definitionSweepCases, contractKeyCases, contractStampCases, contractOrderCases,
		contractSweepCases, frozenSweepCases, frozenKeepCases, frozenRelationCases, contractWriteCases,
		routeWordStopCases,
	)
}

// SiteSettingCases returns every site setting store case in a fixed order.
func SiteSettingCases() []Case {
	return slices.Clone(siteSettingCases)
}

// UserSettingCases returns every user setting store case in a fixed order.
func UserSettingCases() []Case {
	return slices.Clone(userSettingCases)
}

// Run runs every content store case in parallel, each on stores of its own.
func Run(t *testing.T, fresh Factory) {
	t.Helper()
	runAll(t, Cases(), fresh)
}

// RunTypes runs every type store case in parallel, each on stores of its own.
func RunTypes(t *testing.T, fresh Factory) {
	t.Helper()
	runAll(t, TypeCases(), fresh)
}

// RunSiteSettings runs every site setting store case in parallel, each on stores of its own.
func RunSiteSettings(t *testing.T, fresh Factory) {
	t.Helper()
	runAll(t, SiteSettingCases(), fresh)
}

// RunUserSettings runs every user setting store case in parallel, each on stores of its own.
func RunUserSettings(t *testing.T, fresh Factory) {
	t.Helper()
	runAll(t, UserSettingCases(), fresh)
}

// runAll runs the cases in parallel, each on stores of its own.
func runAll(t *testing.T, cases []Case, fresh Factory) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			c.Run(t, fresh(t))
		})
	}
}
