// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrFieldNotFound reports that the type declares no field under the key.
var ErrFieldNotFound = errors.New("content: field not found")

// ErrFieldTooDeep reports that a field would stand deeper than a container tree runs.
var ErrFieldTooDeep = errors.New("content: field nested too deep")

// MaxFieldDepth is how many containers a field may stand inside.
const MaxFieldDepth = 32

// ErrFieldTaken reports that the type already declares a field under the key.
var ErrFieldTaken = errors.New("content: field key taken")

// ErrFieldOrder reports that an order does not name every declared field exactly once.
var ErrFieldOrder = errors.New("content: incomplete field order")

// ErrInvalidFieldKey reports that a field key is not a lowercase word.
var ErrInvalidFieldKey = errors.New("content: invalid field key")

// ErrInvalidFieldLabel reports that a field carries no label.
var ErrInvalidFieldLabel = errors.New("content: invalid field label")

// ErrInvalidFieldKind reports that a field kind is not one the CMS holds.
var ErrInvalidFieldKind = errors.New("content: invalid field kind")

// ErrRelationNeedsTarget reports that a relation field names no target type.
var ErrRelationNeedsTarget = errors.New("content: a relation field needs a target type")

// ErrFieldNotRelational reports a target or many on a kind that takes neither.
var ErrFieldNotRelational = errors.New("content: the kind takes no target or many")

// ErrTargetUnknown reports that a relation field targets an unregistered type.
var ErrTargetUnknown = errors.New("content: relation target unknown")

// ErrTypeTargeted reports that a relation field of another type still targets the type.
var ErrTypeTargeted = errors.New("content: a field still targets the type")

// FieldKind is the shape of value a field holds.
type FieldKind string

// The field kinds a content type declares.
const (
	FieldKindText     FieldKind = "text"
	FieldKindNumber   FieldKind = "number"
	FieldKindBoolean  FieldKind = "boolean"
	FieldKindDate     FieldKind = "date"
	FieldKindMedia    FieldKind = "media"
	FieldKindRelation FieldKind = "relation"
	FieldKindChoice   FieldKind = "choice"
	FieldKindSection  FieldKind = "section"
	FieldKindRepeater FieldKind = "repeater"
)

// Field describes one typed field a group declares, flattened onto the types its group matches.
type Field struct {
	ID        int
	GroupID   int
	ParentID  int
	TypeKey   string
	Key       string
	Label     string
	Kind      FieldKind
	RelatesTo string
	Many      bool
	Required  bool
	Origin    string
	Settings  map[string]any
	Fields    []Field
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewField returns a field definition ready to store, or the reason it is not one.
func NewField(f Field) (Field, error) {
	f.TypeKey = strings.TrimSpace(f.TypeKey)
	f.Key = strings.TrimSpace(f.Key)
	f.Label = strings.TrimSpace(f.Label)
	f.RelatesTo = strings.TrimSpace(f.RelatesTo)
	now := time.Now().UTC()
	f.CreatedAt, f.UpdatedAt = now, now
	if err := f.Validate(); err != nil {
		return Field{}, err
	}
	return f, nil
}

// Validate reports whether the field definition may be stored.
func (f Field) Validate() error {
	if !typeWord.MatchString(f.Key) {
		return ErrInvalidFieldKey
	}
	if f.Label == "" {
		return ErrInvalidFieldLabel
	}
	if !validFieldKind(f.Kind) {
		return ErrInvalidFieldKind
	}
	if err := ValidateSettings(f.Kind, f.Settings); err != nil {
		return err
	}
	return f.validateRelation()
}

// validFieldKind reports whether the CMS holds values of the kind.
func validFieldKind(kind FieldKind) bool {
	switch kind {
	case FieldKindText, FieldKindNumber, FieldKindBoolean, FieldKindDate,
		FieldKindMedia, FieldKindRelation, FieldKindChoice,
		FieldKindSection, FieldKindRepeater:
		return true
	default:
		return false
	}
}

// Holds reports whether the kind carries sub fields of its own.
func (k FieldKind) Holds() bool {
	return k == FieldKindSection || k == FieldKindRepeater
}

// NewSubField returns a field definition ready to store inside the parent kind, or the reason it is not one.
func NewSubField(f Field, parent FieldKind) (Field, error) {
	if !parent.Holds() {
		return Field{}, Refuse(ErrFieldShape, "field_parent_holds_none",
			fmt.Sprintf("%s: %s holds no sub fields", ErrFieldShape, parent),
			Details{"kind": string(parent)})
	}
	if f.Kind == FieldKindRelation {
		return Field{}, Refuse(ErrFieldShape, "field_relation_inside",
			fmt.Sprintf("%s: a relation stands outside a container", ErrFieldShape),
			Details{"field": f.Key})
	}
	return NewField(f)
}

// holdsMany reports whether the kind takes many values under one key.
func holdsMany(kind FieldKind) bool {
	return kind == FieldKindRelation || kind == FieldKindMedia
}

// validateRelation reports whether the target and many match the kind.
func (f Field) validateRelation() error {
	if f.Many && !holdsMany(f.Kind) {
		return ErrFieldNotRelational
	}
	if f.Kind != FieldKindRelation {
		if f.RelatesTo != "" {
			return ErrFieldNotRelational
		}
		return nil
	}
	if f.RelatesTo == "" {
		return ErrRelationNeedsTarget
	}
	if !typeWord.MatchString(f.RelatesTo) {
		return Refuse(ErrInvalidKey, "relation_target_key_malformed",
			"content: invalid relation target key", nil)
	}
	return nil
}
