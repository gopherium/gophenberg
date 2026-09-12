// SPDX-License-Identifier: Apache-2.0

package served

import (
	"context"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// Reader reads the items a public answer names beside the item's own values.
type Reader interface {
	TargetsByIDs(ctx context.Context, ids []uuid.UUID) ([]content.Target, error)
	PointingAt(ctx context.Context, target uuid.UUID, field, page, perPage int) ([]content.Pointer, int, error)
}

// Settings reads the values the site chose for itself.
type Settings interface {
	Lookup(ctx context.Context, key string) (string, bool, error)
}

// Stores are the readers a public answer is shaped through.
type Stores struct {
	Links    Reader
	Groups   Groups
	Library  Library
	Settings Settings
}

// PerPage returns how many items a public answer lists at once, as the site chose or by default.
func PerPage(ctx context.Context, settings Settings) int {
	if settings == nil {
		return content.DefaultPerPage
	}
	held, found, err := settings.Lookup(ctx, content.PerPageSettingKey)
	if err != nil {
		return content.DefaultPerPage
	}
	return content.ResolvePerPage(held, found)
}

// Values returns the item's values as every public seam serves them, beside the pointer counts.
func Values(
	ctx context.Context, stores Stores, t content.Type, c content.Content,
) (content.Values, map[string]int, error) {
	values := c.Fields.Merge(nil)
	totals, err := Pointing(ctx, stores, t, c, values, content.Hidden(t.Fields, values))
	if err != nil {
		return nil, nil, err
	}
	shown := content.Shown(t.Fields, values)
	if err := InlineMedia(ctx, stores.Library, t, shown); err != nil {
		return nil, nil, err
	}
	if err := InlineTargets(ctx, stores.Links, t, shown); err != nil {
		return nil, nil, err
	}
	return shown, totals, nil
}

// Pointing writes the items pointing at the content under each backlinks key the hidden set leaves standing.
func Pointing(
	ctx context.Context, stores Stores, t content.Type, c content.Content,
	values content.Values, hidden map[string]bool,
) (map[string]int, error) {
	reading, err := backlinksOn(ctx, stores.Groups, t.Key)
	if err != nil || len(reading) == 0 {
		return nil, err
	}
	totals := make(map[string]int, len(reading))
	held := make(content.Values, len(reading))
	perPage := PerPage(ctx, stores.Settings)
	for key, source := range reading {
		if hidden[key] {
			continue
		}
		found, total, err := stores.Links.PointingAt(ctx, c.ID, source, 1, perPage)
		if err != nil {
			return nil, err
		}
		held[key] = NamedPointers(found)
		totals[key] = total
	}
	for key, found := range held {
		values[key] = found
	}
	return totals, nil
}

// backlinksOn returns the source field each backlinks field of the type reads, keyed by the field's key.
func backlinksOn(ctx context.Context, groups Groups, typeKey string) (map[string]int, error) {
	if groups == nil {
		return nil, nil
	}
	held, err := groups.Groups(ctx)
	if err != nil {
		return nil, err
	}
	params := groups.Params(ctx)
	screen := content.Screen{content.ScreenContentType: typeKey}
	reading := make(map[string]int)
	for _, g := range held {
		if !g.Active || !g.Location.Match(screen, params) {
			continue
		}
		readSources(held, g, reading)
	}
	return reading, nil
}

// readSources writes the source field each backlinks field of the group reads, leaving out what reads none.
func readSources(groups []content.Group, g content.Group, reading map[string]int) {
	for _, f := range g.Fields {
		if f.Kind != content.FieldKindBacklinks {
			continue
		}
		if source, found := content.SourceRelation(groups, f); found {
			reading[f.Key] = source.ID
		}
	}
}
