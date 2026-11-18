// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"errors"
	"fmt"

	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/content/typestore"
)

// pathTaken is the text SQLite gives a write onto an address another item holds.
const pathTaken = "UNIQUE constraint failed: core_content.path"

// restrictCode is the code readError gives a change a RESTRICT key stops.
const restrictCode = "restrict"

// foreignKeyCode is the code readError gives any other foreign key failure.
const foreignKeyCode = "foreign key"

// typeKeyCode is the code readError gives a new type whose key another type holds.
const typeKeyCode = "type key"

// errTypeKeyHeld is what storing a new type answers when another type holds its key.
var errTypeKeyHeld = fmt.Errorf("%w: another type holds the key", dbkit.ErrUnique)

// contentRulesBroken lists the stored rule each SQLite failure of a content write stands for, in match order.
var contentRulesBroken = []typestore.Rule{
	{Context: pathTaken, Broken: typestore.ContentPathUnique},
}

// typeRulesBroken lists the stored rule each SQLite failure of a type, group or field write stands for, in match order.
var typeRulesBroken = []typestore.Rule{
	{Code: restrictCode, Broken: typestore.TypeHeldByContent},
	{Context: pathTaken, Broken: typestore.ContentPathUnique},
	{Context: "UNIQUE constraint failed: core_field_groups.key", Broken: typestore.GroupKeyUnique},
	{Context: "UNIQUE constraint failed: index 'core_content_fields_scope_key_unique'", Broken: typestore.FieldKeyUnique},
	{
		Context: "UNIQUE constraint failed: core_content_fields.parent_field_id, core_content_fields.key",
		Broken:  typestore.FieldKeyUnique,
	},
	{Code: typeKeyCode, Broken: typestore.TypeKeyUnique},
	{Context: "UNIQUE constraint failed: core_content_types.route_word", Broken: typestore.RouteWordUnique},
	{Context: "UNIQUE constraint failed: core_content_types.is_default", Broken: typestore.DefaultUnique},
	{Code: foreignKeyCode, Broken: typestore.FieldTargetExists},
}

// readError returns the code and the text of a SQLite unique or foreign key failure as the rules read them.
func readError(err error) (typestore.Failure, bool) {
	classed := dbsqlite.Classify(err)
	switch {
	case errors.Is(err, errTypeKeyHeld):
		return typestore.Failure{Code: typeKeyCode, Context: err.Error()}, true
	case errors.Is(classed, dbsqlite.ErrRestrict):
		return typestore.Failure{Code: restrictCode, Context: err.Error()}, true
	case errors.Is(classed, dbkit.ErrForeignKey):
		return typestore.Failure{Code: foreignKeyCode, Context: err.Error()}, true
	case errors.Is(classed, dbkit.ErrUnique):
		return typestore.Failure{Context: err.Error()}, true
	}
	return typestore.Failure{}, false
}
