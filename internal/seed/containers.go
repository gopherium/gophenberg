// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// TeamFieldKey is the key the seeded repeater stores its rows under.
const TeamFieldKey = "team"

// TeamField returns the repeater the seeding declares on the post type.
func TeamField() content.Field {
	now := time.Now().UTC()
	return content.Field{
		TypeKey:   content.TypePost,
		Key:       TeamFieldKey,
		Label:     "Team",
		Kind:      content.FieldKindRepeater,
		Settings:  map[string]any{"instructions": "Add a row for everyone who worked on this."},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// TeamRowFields returns the fields the seeding declares inside one row of the repeater.
func TeamRowFields() []content.Field {
	now := time.Now().UTC()
	return []content.Field{
		{Key: "name", Label: "Name", Kind: content.FieldKindText, CreatedAt: now, UpdatedAt: now},
		{Key: "role", Label: "Role", Kind: content.FieldKindText, CreatedAt: now, UpdatedAt: now},
	}
}

// Containers declares the seeded repeater and the fields one of its rows holds.
func Containers(ctx context.Context, types *content.Registry) error {
	postType, err := types.ByKey(ctx, content.TypePost)
	if err != nil {
		return fmt.Errorf("seed post type lookup: %w", err)
	}
	for _, held := range postType.Fields {
		if held.Key == TeamFieldKey {
			return nil
		}
	}
	team, err := types.CreateField(ctx, TeamField())
	if err != nil {
		return fmt.Errorf("seed team field: %w", err)
	}
	for _, row := range TeamRowFields() {
		if _, err := types.CreateSubField(ctx, team.ID, row); err != nil {
			return fmt.Errorf("seed team row field %s: %w", row.Key, err)
		}
	}
	return nil
}
