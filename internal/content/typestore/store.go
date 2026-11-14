// SPDX-License-Identifier: Apache-2.0

// Package typestore runs the type store's steps over the statements any database engine lends it.
package typestore

import (
	"context"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
)

var _ content.TypeStore = (*Store)(nil)

// Backend holds what one database engine lends the type store.
type Backend struct {
	Engine    dbkit.Engine
	Pool      Queries
	InTx      Transactor
	Rules     []Rule
	ReadError func(err error) (Failure, bool)
	Classify  func(err error) error
}

// Transactor runs write on statements bound to one transaction, committing when write returns nil.
type Transactor func(ctx context.Context, write func(Queries) error) error

// Store keeps the content type registry and its field groups through one backend.
type Store struct {
	backend Backend
}

// New returns a [Store] that runs its steps on the backend.
func New(backend Backend) *Store {
	return &Store{backend: backend}
}

// classify runs the backend's error classifier when it has one, leaving err unchanged otherwise.
func (s *Store) classify(err error) error {
	if s.backend.Classify != nil {
		return s.backend.Classify(err)
	}
	return err
}
