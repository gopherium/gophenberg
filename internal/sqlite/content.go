// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"database/sql"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contentstore"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// ContentStore meets the content store contract.
var _ content.Store = (*ContentStore)(nil)

// ContentStore persists content items, their revisions and their relations in the core tables of the SQLite file.
type ContentStore struct {
	*contentstore.Store
}

// NewContentStore returns a [ContentStore] on the site's one SQLite handle.
func NewContentStore(handle *sql.DB) *ContentStore {
	queries := db.New(handle)
	return &ContentStore{Store: contentstore.New(contentstore.Backend{
		Engine:    dbkit.SQLite,
		Pool:      contentQueries{typeQueries{queries: queries}},
		InTx:      contentInTx(handle, queries),
		Rules:     contentRulesBroken,
		ReadError: readError,
	})}
}
