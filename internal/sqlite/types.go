// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"

	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// TypeStore meets the type registry contract.
var _ content.TypeStore = (*TypeStore)(nil)

// TypeStore persists content types, field groups and their fields in the core tables of the SQLite file.
type TypeStore struct {
	*typestore.Store
}

// NewTypeStore returns a [TypeStore] on the site's one SQLite handle.
func NewTypeStore(handle *sql.DB) *TypeStore {
	queries := db.New(handle)
	return &TypeStore{Store: typestore.New(typestore.Backend{
		Engine:    dbkit.SQLite,
		Pool:      typeQueries{queries: queries},
		InTx:      typeInTx(handle, queries),
		Rules:     typeRulesBroken,
		ReadError: readError,
	})}
}

// typeInTx returns the transactor running each type write in one transaction on handle, marking its failure.
func typeInTx(handle *sql.DB, queries *db.Queries) typestore.Transactor {
	return func(ctx context.Context, write func(typestore.Queries) error) error {
		return dbsqlite.Classify(dbkit.InTx(ctx, handle, func(tx *sql.Tx) error {
			return write(typeQueries{queries: queries.WithTx(tx)})
		}))
	}
}
