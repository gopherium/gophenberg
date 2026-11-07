// SPDX-License-Identifier: Apache-2.0

// Package contentstore runs the content store's steps over the statements any database engine lends it.
package contentstore

import (
	"context"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/typestore"
)

var _ content.Store = (*Store)(nil)

// Backend holds what one database engine lends the content store.
type Backend struct {
	Engine    dbkit.Engine
	Pool      Queries
	InTx      Transactor
	Rules     []typestore.Rule
	ReadError func(err error) (typestore.Failure, bool)
}

// Transactor runs write on one transaction, committing when write returns nil.
type Transactor func(ctx context.Context, write func(Tx) error) error

// Tx sends the statements of one write transaction and runs an attempt it can undo.
type Tx interface {
	Queries
	Savepoint(ctx context.Context, attempt func(Queries) error) error
}

// Store keeps content items, their revisions and their relations through one backend.
type Store struct {
	backend Backend
}

// New returns a [Store] that runs its steps on the backend.
func New(backend Backend) *Store {
	return &Store{backend: backend}
}
