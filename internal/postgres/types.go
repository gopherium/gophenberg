// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"errors"

	"github.com/gopherium/framework/dbkit"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

var _ content.TypeStore = (*TypeStore)(nil)

// routeWordConstraint names the unique index over the route words in use.
const routeWordConstraint = "content_types_route_word_idx"

// typeKeyConstraint names the primary key over the type keys in use.
const typeKeyConstraint = "content_types_pkey"

// uniqueViolationCode is what Postgres reports when a row breaks a unique index.
const uniqueViolationCode = "23505"

// restrictViolationCode is what Postgres reports when a restricted row is still referenced.
const restrictViolationCode = "23001"

// foreignKeyViolationCode is what Postgres reports when a reference points at no row.
const foreignKeyViolationCode = "23503"

// deadlockCode is what Postgres reports when it breaks a deadlock.
const deadlockCode = "40P01"

// fieldKeyConstraint names the unique index over one group's field keys.
const fieldKeyConstraint = "content_fields_scope_key_unique"

// containerKeyConstraint names the index holding each key once inside a container.
const containerKeyConstraint = "content_fields_container_key_unique"

// groupKeyConstraint names the unique constraint over field group keys.
const groupKeyConstraint = "field_groups_key_unique"

// fieldGroupConstraint names the reference a field keeps to its group.
const fieldGroupConstraint = "content_fields_group_fkey"

// fieldTargetConstraint names the reference a relation field keeps to its target type.
const fieldTargetConstraint = "content_fields_relates_to_fkey"

// TypeStore persists the content type registry in the core schema.
type TypeStore struct {
	*typestore.Store
}

// NewTypeStore returns a [TypeStore] backed by pool.
func NewTypeStore(pool *pgxpool.Pool) *TypeStore {
	queries := db.New(pool)
	return &TypeStore{Store: typestore.New(typestore.Backend{
		Engine:    dbkit.Postgres,
		Pool:      typeQueries{conn: pool, queries: queries},
		InTx:      inTx(pool, queries),
		Rules:     rulesBroken,
		ReadError: readError,
	})}
}

// rulesBroken lists the stored rule each Postgres error code and constraint name stands for, in match order.
var rulesBroken = []typestore.Rule{
	{Code: restrictViolationCode, Broken: typestore.TypeHeldByContent},
	{Name: slugConstraint, Broken: typestore.ContentPathUnique},
	{Code: uniqueViolationCode, Name: groupKeyConstraint, Broken: typestore.GroupKeyUnique},
	{Code: uniqueViolationCode, Name: fieldKeyConstraint, Broken: typestore.FieldKeyUnique},
	{Code: uniqueViolationCode, Name: containerKeyConstraint, Broken: typestore.FieldKeyUnique},
	{Code: uniqueViolationCode, Name: typeKeyConstraint, Broken: typestore.TypeKeyUnique},
	{Code: uniqueViolationCode, Name: routeWordConstraint, Broken: typestore.RouteWordUnique},
	{Code: foreignKeyViolationCode, Name: fieldTargetConstraint, Broken: typestore.FieldTargetExists},
	{Code: foreignKeyViolationCode, Name: fieldGroupConstraint, Broken: typestore.FieldGroupExists},
}

// readError returns the code and the constraint name a Postgres error carries, and whether err holds one.
func readError(err error) (code, name string, found bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return "", "", false
	}
	return pgErr.Code, pgErr.ConstraintName, true
}

// originOf returns the origin a nullable column holds, empty for a definition the site made.
func originOf(origin *string) string {
	if origin == nil {
		return ""
	}
	return *origin
}

// originColumn returns the origin as the nullable column holds it.
func originColumn(origin string) *string {
	if origin == "" {
		return nil
	}
	return &origin
}
