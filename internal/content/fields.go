// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"strings"
	"time"
)

// ErrFieldNotFound reports that the type declares no field under the key.
var ErrFieldNotFound = errors.New("content: field not found")

// ErrFieldTaken reports that the type already declares a field under the key.
var ErrFieldTaken = errors.New("content: field key taken")

// ErrInvalidFieldKey reports that a field key is not a lowercase word.
var ErrInvalidFieldKey = errors.New("content: invalid field key")

// ErrInvalidFieldLabel reports that a field carries no label.
var ErrInvalidFieldLabel = errors.New("content: invalid field label")

// ErrInvalidFieldKind reports that a field kind is not one the CMS holds.
var ErrInvalidFieldKind = errors.New("content: invalid field kind")

// ErrRelationNeedsTarget reports that a relation field names no target type.
var ErrRelationNeedsTarget = errors.New("content: a relation field needs a target type")

// ErrFieldNotRelational reports that only a relation field takes a target or many.
var ErrFieldNotRelational = errors.New("content: only a relation field takes a target or many")

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
)

// Field describes one typed field a content type declares.
type Field struct {
	ID        int
	TypeKey   string
	Key       string
	Label     string
	Kind      FieldKind
	RelatesTo string
	Many      bool
	Required  bool
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
	return f.validateRelation()
}

// validFieldKind reports whether the CMS holds values of the kind.
func validFieldKind(kind FieldKind) bool {
	switch kind {
	case FieldKindText, FieldKindNumber, FieldKindBoolean, FieldKindDate, FieldKindMedia, FieldKindRelation:
		return true
	default:
		return false
	}
}

// validateRelation reports whether the relation shape matches the kind.
func (f Field) validateRelation() error {
	if f.Kind != FieldKindRelation {
		if f.RelatesTo != "" || f.Many {
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
