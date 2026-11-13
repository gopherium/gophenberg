// SPDX-License-Identifier: Apache-2.0

package typestore

import (
	"strings"

	"github.com/gopherium/gophenberg/internal/content"
)

// Constraint names a stored rule a statement broke, whatever the engine calls that rule.
type Constraint int

// The stored rules the steps tell apart.
const (
	NoConstraint Constraint = iota
	GroupKeyUnique
	FieldKeyUnique
	FieldTargetExists
	FieldGroupExists
	TypeKeyUnique
	RouteWordUnique
	RootTaken
	TypeHeldByContent
	ContentPathUnique
	RelationTargetExists
	AddressCheckDeadlocked
)

// Failure is what an engine reads from an error it raised: its code, the rule it names and the context it struck in.
type Failure struct {
	Code, Name, Context string
}

// Rule names the stored rule an engine's error code, rule name and context text stand for, an empty one matching any.
type Rule struct {
	Code, Name, Context string
	Broken              Constraint
}

// matches reports whether the rule holds the code, the name and the context an engine read from a failure.
func (r Rule) matches(f Failure) bool {
	return (r.Code == "" || r.Code == f.Code) && (r.Name == "" || r.Name == f.Name) &&
		(r.Context == "" || strings.Contains(f.Context, r.Context))
}

// Broken returns the stored rule the first rule matching what read finds in err names, or none.
func Broken(rules []Rule, read func(err error) (Failure, bool), err error) Constraint {
	failure, found := read(err)
	if !found {
		return NoConstraint
	}
	for _, rule := range rules {
		if rule.matches(failure) {
			return rule.Broken
		}
	}
	return NoConstraint
}

// broken returns the stored rule the first rule matching what the engine reads from err names, or none.
func (s *Store) broken(err error) Constraint {
	return Broken(s.backend.Rules, s.backend.ReadError, err)
}

// errorFor returns the error the table holds for the stored rule err broke, or nil.
func (s *Store) errorFor(table map[Constraint]error, err error) error {
	return table[s.broken(err)]
}

// groupWriteErrors maps the rule a group write can break to the error it stands for.
var groupWriteErrors = map[Constraint]error{
	GroupKeyUnique: content.ErrGroupKeyTaken,
}

// fieldWriteErrors maps each rule a field write can break to the error it stands for.
var fieldWriteErrors = map[Constraint]error{
	FieldKeyUnique:    content.ErrFieldTaken,
	FieldTargetExists: content.ErrTargetUnknown,
	FieldGroupExists:  content.ErrGroupNotFound,
}

// typeWriteErrors maps each rule a type write can break to the error it stands for.
var typeWriteErrors = map[Constraint]error{
	TypeKeyUnique:   content.ErrTypeTaken,
	RouteWordUnique: content.ErrRouteWordTaken,
	RootTaken:       content.ErrRootTaken,
}

// typeUpdateErrors maps each rule a type update can break to the error it stands for.
var typeUpdateErrors = map[Constraint]error{
	TypeKeyUnique:     content.ErrTypeTaken,
	RouteWordUnique:   content.ErrRouteWordTaken,
	ContentPathUnique: content.ErrSlugTaken,
	RootTaken:         content.ErrRootTaken,
}

// typeDeleteErrors maps each rule a type delete can break to the error it stands for.
var typeDeleteErrors = map[Constraint]error{
	TypeHeldByContent: content.ErrTypeInUse,
	FieldTargetExists: content.ErrTypeTargeted,
}
