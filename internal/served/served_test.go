// SPDX-License-Identifier: Apache-2.0

package served_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/served"
)

// errStoreDown stands for a reader that will not answer.
var errStoreDown = errors.New("the store is down")

// fakeLinks answers with the targets and pointers it was built with.
type fakeLinks struct {
	targets     content.Targets
	pointers    []content.Pointer
	total       int
	targetsErr  error
	pointingErr error
}

// TargetsOf returns the targets the stored relations name.
func (s fakeLinks) TargetsOf(context.Context, uuid.UUID) (content.Targets, error) {
	return s.targets, s.targetsErr
}

// PointingAt returns the items pointing at the target through the field.
func (s fakeLinks) PointingAt(
	context.Context, uuid.UUID, int, int, int,
) ([]content.Pointer, int, error) {
	if s.pointingErr != nil {
		return nil, 0, s.pointingErr
	}
	return s.pointers, s.total, nil
}

// fakeGroups answers with the groups it was built with.
type fakeGroups struct {
	held []content.Group
	err  error
}

// Groups returns the stored field groups.
func (s fakeGroups) Groups(context.Context) ([]content.Group, error) { return s.held, s.err }

// Params returns the registry a location reads its sources through.
func (fakeGroups) Params(context.Context) *content.ParamRegistry {
	return content.DefaultParamRegistry(nil)
}

// fakeLibrary answers with the files it was built with.
type fakeLibrary struct {
	held []media.Media
	err  error
}

// ByIDs returns the stored files, or the failure it was built with.
func (s fakeLibrary) ByIDs(context.Context, []int64) ([]media.Media, error) {
	return s.held, s.err
}

// postType returns a type declaring the given fields.
func postType(fields ...content.Field) content.Type {
	return content.Type{Key: content.TypePost, Active: true, Fields: fields}
}

// anItem returns a stored item holding the given values.
func anItem(values content.Values) content.Content {
	return content.Content{ID: uuid.Must(uuid.NewV7()), Type: content.TypePost, Fields: values}
}

func TestNamedTargetsAddressesEveryTarget(t *testing.T) {
	t.Parallel()

	id := uuid.Must(uuid.NewV7())

	held := served.NamedTargets([]content.Target{{ID: id, Title: "News", Path: "categories/news"}})

	if len(held) != 1 || held[0].ID != id.String() || held[0].Title != "News" {
		t.Errorf("NamedTargets() = %+v, want the target named and addressed", held)
	}
	if held[0].Path != "categories/news" {
		t.Errorf("Path = %q, want the address the theme links to", held[0].Path)
	}
}

func TestNamedPointersCarriesTheTypeBesideTheTarget(t *testing.T) {
	t.Parallel()

	id := uuid.Must(uuid.NewV7())

	held := served.NamedPointers([]content.Pointer{
		{ID: id, Type: "category", Title: "News", Path: "categories/news"},
	})

	if len(held) != 1 || held[0].Type != "category" || held[0].ID != id.String() {
		t.Errorf("NamedPointers() = %+v, want the pointer typed and addressed", held)
	}
}

func TestFileOfAddressesTheStoredFileAndItsSizes(t *testing.T) {
	t.Parallel()

	held := served.FileOf(media.Media{
		ID: 12, File: "2026/08/sunrise.jpg", Title: "Sunrise", MimeType: "image/jpeg",
		Sizes: map[string]media.Rendition{
			"large": {File: "2026/08/sunrise-1024.jpg", Width: 1024, Height: 576, MimeType: "image/jpeg"},
		},
	})

	if held.Src != "/media/2026/08/sunrise.jpg" || held.Title != "Sunrise" {
		t.Errorf("FileOf() = %+v, want the stored file addressed", held)
	}
	if held.Sizes["large"].Src != "/media/2026/08/sunrise-1024.jpg" {
		t.Errorf("sizes = %+v, want each rendition addressed", held.Sizes)
	}
}

func TestValuesNamesTargetsAndServesFiles(t *testing.T) {
	t.Parallel()

	target := uuid.Must(uuid.NewV7())
	held := postType(
		content.Field{Key: "venue", Kind: content.FieldKindText},
		content.Field{Key: "maker", Kind: content.FieldKindRelation, RelatesTo: "car", Many: true},
		content.Field{Key: "cover", Kind: content.FieldKindMedia},
	)
	stores := served.Stores{
		Links:   fakeLinks{targets: content.Targets{"maker": {{ID: target, Title: "News"}}}},
		Library: fakeLibrary{held: []media.Media{{ID: 12, File: "a.jpg"}}},
	}

	values, totals, err := served.Values(
		t.Context(), stores, held, anItem(content.Values{"venue": "Hall", "cover": float64(12)}))

	if err != nil {
		t.Fatalf("Values() error = %v, want nil", err)
	}
	if values["venue"] != "Hall" {
		t.Errorf("venue = %v, want the stored value carried", values["venue"])
	}
	if named, ok := values["maker"].([]served.Target); !ok || len(named) != 1 {
		t.Errorf("maker = %#v, want the target named", values["maker"])
	}
	if file, ok := values["cover"].(served.File); !ok || file.Src != "/media/a.jpg" {
		t.Errorf("cover = %#v, want the file served", values["cover"])
	}
	if len(totals) != 0 {
		t.Errorf("totals = %v, want none where the type declares no backlinks", totals)
	}
}

func TestValuesLeavesTheStoredItemAsItWasRead(t *testing.T) {
	t.Parallel()

	held := postType(
		content.Field{Key: "cover", Kind: content.FieldKindMedia},
		content.Field{
			Key: "author", Kind: content.FieldKindSection,
			Fields: []content.Field{{Key: "portrait", Kind: content.FieldKindMedia}},
		},
		content.Field{
			Key: "team", Kind: content.FieldKindRepeater,
			Fields: []content.Field{{Key: "shot", Kind: content.FieldKindMedia}},
		},
	)
	stored := anItem(content.Values{
		"cover":  float64(12),
		"author": map[string]any{"portrait": float64(12)},
		"team":   []any{map[string]any{"shot": float64(12)}},
	})
	stores := served.Stores{
		Links:   fakeLinks{},
		Library: fakeLibrary{held: []media.Media{{ID: 12, File: "a.jpg"}}},
	}

	if _, _, err := served.Values(t.Context(), stores, held, stored); err != nil {
		t.Fatalf("Values() error = %v, want nil", err)
	}

	if stored.Fields["cover"] != float64(12) {
		t.Errorf("cover = %#v, want the identity the item was read with", stored.Fields["cover"])
	}
	inside, _ := stored.Fields["author"].(map[string]any)
	if inside["portrait"] != float64(12) {
		t.Errorf("the section holds %#v, want the identity it was read with", inside["portrait"])
	}
	rows, _ := stored.Fields["team"].([]any)
	row, _ := rows[0].(map[string]any)
	if row["shot"] != float64(12) {
		t.Errorf("the row holds %#v, want the identity it was read with", row["shot"])
	}
}

func TestValuesKeepsBackWhatTheRulesHide(t *testing.T) {
	t.Parallel()

	held := postType(
		content.Field{Key: "on-sale", Kind: content.FieldKindBoolean},
		content.Field{Key: "sale-note", Kind: content.FieldKindText, Settings: map[string]any{
			"conditions": []any{[]any{map[string]any{
				"source": "on-sale", "operator": "==", "value": "true",
			}}},
		}},
	)

	values, _, err := served.Values(t.Context(), served.Stores{Links: fakeLinks{}}, held,
		anItem(content.Values{"sale-note": "Half price"}))

	if err != nil {
		t.Fatalf("Values() error = %v, want nil", err)
	}
	if _, carried := values["sale-note"]; carried {
		t.Errorf("values = %v, want the hidden note kept back", values)
	}
}

func TestValuesReportsWhatItCannotRead(t *testing.T) {
	t.Parallel()

	for name, stores := range map[string]served.Stores{
		"the targets it cannot read": {Links: fakeLinks{targetsErr: errStoreDown}},
		"the files it cannot read": {
			Links: fakeLinks{}, Library: fakeLibrary{err: errStoreDown},
		},
		"the groups it cannot read": {
			Links: fakeLinks{}, Groups: fakeGroups{err: errStoreDown},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := postType(content.Field{Key: "cover", Kind: content.FieldKindMedia})

			_, _, err := served.Values(t.Context(), stores, held,
				anItem(content.Values{"cover": float64(12)}))

			if !errors.Is(err, errStoreDown) {
				t.Errorf("Values() error = %v, want %v", err, errStoreDown)
			}
		})
	}
}

// readingGroups returns the group holding a relation beside the group reading it backwards.
func readingGroups() []content.Group {
	return []content.Group{
		{
			ID: 1, Key: "cars", Title: "Cars", Active: true,
			Location: content.Rules{{{
				Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "car",
			}}},
			Fields: []content.Field{
				{ID: 7, Key: "maker", Kind: content.FieldKindRelation, RelatesTo: content.TypePost},
			},
		},
		{
			ID: 2, Key: "makers", Title: "Makers", Active: true,
			Location: content.Rules{{{
				Source: content.ScreenContentType, Operator: content.OperatorIs, Value: content.TypePost,
			}}},
			Fields: []content.Field{
				{Key: "note", Kind: content.FieldKindText},
				{Key: "linked-from", Kind: content.FieldKindBacklinks, Settings: map[string]any{
					content.SettingSourceGroup: "cars",
					content.SettingSourceField: []any{"maker"},
				}},
			},
		},
	}
}

func TestValuesListsWhatPointsAtTheItem(t *testing.T) {
	t.Parallel()

	pointer := uuid.Must(uuid.NewV7())
	stores := served.Stores{
		Links: fakeLinks{
			pointers: []content.Pointer{{ID: pointer, Type: "car", Title: "A car"}},
			total:    3,
		},
		Groups: fakeGroups{held: readingGroups()},
	}
	held := postType(content.Field{Key: "linked-from", Kind: content.FieldKindBacklinks})

	values, totals, err := served.Values(t.Context(), stores, held, anItem(nil))

	if err != nil {
		t.Fatalf("Values() error = %v, want nil", err)
	}
	if named, ok := values["linked-from"].([]served.Pointer); !ok || len(named) != 1 {
		t.Fatalf("linked-from = %#v, want the pointer named", values["linked-from"])
	}
	if totals["linked-from"] != 3 {
		t.Errorf("totals = %v, want every pointer counted behind the page", totals)
	}
}

func TestPointingReportsWhatItCannotRead(t *testing.T) {
	t.Parallel()

	stores := served.Stores{
		Links:  fakeLinks{pointingErr: errStoreDown},
		Groups: fakeGroups{held: readingGroups()},
	}
	held := postType(content.Field{Key: "linked-from", Kind: content.FieldKindBacklinks})

	_, err := served.Pointing(t.Context(), stores, held, anItem(nil), content.Values{}, nil)

	if !errors.Is(err, errStoreDown) {
		t.Errorf("Pointing() error = %v, want %v", err, errStoreDown)
	}
}

func TestPointingReadsNothingWhereNoGroupIsNamed(t *testing.T) {
	t.Parallel()

	held := postType(content.Field{Key: "linked-from", Kind: content.FieldKindBacklinks})

	totals, err := served.Pointing(
		t.Context(), served.Stores{Links: fakeLinks{}}, held, anItem(nil), content.Values{}, nil)

	if err != nil {
		t.Fatalf("Pointing() error = %v, want nil", err)
	}
	if len(totals) != 0 {
		t.Errorf("totals = %v, want none where no group reads a relation", totals)
	}
}

func TestPointingSkipsAKeyTheRulesHide(t *testing.T) {
	t.Parallel()

	stores := served.Stores{
		Links:  fakeLinks{pointingErr: errStoreDown},
		Groups: fakeGroups{held: readingGroups()},
	}
	held := postType(content.Field{Key: "linked-from", Kind: content.FieldKindBacklinks})

	totals, err := served.Pointing(t.Context(), stores, held, anItem(nil),
		content.Values{}, map[string]bool{"linked-from": true})

	if err != nil {
		t.Fatalf("Pointing() error = %v, want the hidden key never read", err)
	}
	if len(totals) != 0 {
		t.Errorf("totals = %v, want no count for a field nobody may see", totals)
	}
}

func TestPointingLeavesOutABacklinksWhoseSourceIsGone(t *testing.T) {
	t.Parallel()

	groups := readingGroups()
	groups[0].Fields = nil
	stores := served.Stores{Links: fakeLinks{}, Groups: fakeGroups{held: groups}}
	held := postType(content.Field{Key: "linked-from", Kind: content.FieldKindBacklinks})

	totals, err := served.Pointing(t.Context(), stores, held, anItem(nil), content.Values{}, nil)

	if err != nil {
		t.Fatalf("Pointing() error = %v, want nil", err)
	}
	if len(totals) != 0 {
		t.Errorf("totals = %v, want none where the source relation is gone", totals)
	}
}

func TestPointingSkipsAGroupPlacedElsewhere(t *testing.T) {
	t.Parallel()

	groups := readingGroups()
	groups[1].Active = false
	stores := served.Stores{Links: fakeLinks{}, Groups: fakeGroups{held: groups}}
	held := postType(content.Field{Key: "linked-from", Kind: content.FieldKindBacklinks})

	totals, err := served.Pointing(t.Context(), stores, held, anItem(nil), content.Values{}, nil)

	if err != nil {
		t.Fatalf("Pointing() error = %v, want nil", err)
	}
	if len(totals) != 0 {
		t.Errorf("totals = %v, want none where the group reading them is switched off", totals)
	}
}
