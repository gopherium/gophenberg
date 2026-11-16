// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"errors"

	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/content/typestore"
)

// pathTaken is the text SQLite gives a write onto an address another item holds.
const pathTaken = "UNIQUE constraint failed: core_content.path"

// contentRulesBroken lists the stored rule each SQLite failure of a content write stands for, in match order.
var contentRulesBroken = []typestore.Rule{
	{Context: pathTaken, Broken: typestore.ContentPathUnique},
}

// readError returns the text of a SQLite unique failure as the context the content rules read.
func readError(err error) (typestore.Failure, bool) {
	if !errors.Is(dbsqlite.Classify(err), dbkit.ErrUnique) {
		return typestore.Failure{}, false
	}
	return typestore.Failure{Context: err.Error()}, true
}
