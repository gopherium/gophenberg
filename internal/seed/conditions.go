// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// OnSaleFieldKey is the key the seeded switch a rule reads stores its value under.
const OnSaleFieldKey = "on-sale"

// SaleNoteFieldKey is the key the seeded field shown by a rule stores its value under.
const SaleNoteFieldKey = "sale-note"

// OnSaleField returns the switch the seeded rule reads.
func OnSaleField() content.Field {
	now := time.Now().UTC()
	return content.Field{
		TypeKey:   content.TypePost,
		Key:       OnSaleFieldKey,
		Label:     "On sale",
		Kind:      content.FieldKindBoolean,
		Settings:  map[string]any{"instructions": "Turn this on to say more about the offer."},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// SaleNoteField returns the field the seeding shows only while the switch is on.
func SaleNoteField() content.Field {
	now := time.Now().UTC()
	return content.Field{
		TypeKey: content.TypePost,
		Key:     SaleNoteFieldKey,
		Label:   "Sale note",
		Kind:    content.FieldKindText,
		Settings: map[string]any{
			"conditions": content.ConditionsSetting(content.Rules{{{
				Source: OnSaleFieldKey, Operator: content.OperatorIs, Value: "true",
			}}}),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Conditions declares the seeded switch and the field its rule shows, the source first.
func Conditions(ctx context.Context, types *content.Registry) error {
	postType, err := types.ByKey(ctx, content.TypePost)
	if err != nil {
		return fmt.Errorf("seed post type lookup: %w", err)
	}
	for _, held := range postType.Fields {
		if held.Key == OnSaleFieldKey {
			return nil
		}
	}
	for _, wanted := range []content.Field{OnSaleField(), SaleNoteField()} {
		if _, err := types.CreateField(ctx, wanted); err != nil {
			return fmt.Errorf("seed %s field: %w", wanted.Key, err)
		}
	}
	return nil
}
