// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gopherium/gophenberg/internal/content/typestore"
)

// addressCheckContext is the context PostgreSQL gives an error it raised while writing the address index.
const addressCheckContext = `while inserting index tuple (0,7) in relation "content_path_unique"`

func TestReadErrorReadsTheCodeTheConstraintAndTheContextAPostgreSQLErrorCarries(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		err  error
		want typestore.Failure
	}{
		"a unique violation with its constraint": {
			&pgconn.PgError{Code: uniqueViolationCode, ConstraintName: fieldKeyConstraint},
			typestore.Failure{Code: uniqueViolationCode, Name: fieldKeyConstraint},
		},
		"a foreign key violation": {
			&pgconn.PgError{Code: foreignKeyViolationCode, ConstraintName: fieldTargetConstraint},
			typestore.Failure{Code: foreignKeyViolationCode, Name: fieldTargetConstraint},
		},
		"a restrict violation": {
			&pgconn.PgError{Code: restrictViolationCode, ConstraintName: "content_type_fkey"},
			typestore.Failure{Code: restrictViolationCode, Name: "content_type_fkey"},
		},
		"a deadlock with the context it struck in": {
			&pgconn.PgError{Code: deadlockCode, Where: addressCheckContext},
			typestore.Failure{Code: deadlockCode, Context: addressCheckContext},
		},
		"a PostgreSQL error wrapped on the way out": {
			fmt.Errorf("retype content paths: %w",
				&pgconn.PgError{Code: uniqueViolationCode, ConstraintName: slugConstraint, Where: "while updating"}),
			typestore.Failure{Code: uniqueViolationCode, Name: slugConstraint, Context: "while updating"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, found := readError(test.err)

			if got != test.want || !found {
				t.Errorf("readError() = (%+v, %t), want (%+v, true)", got, found, test.want)
			}
		})
	}
}

func TestReadErrorFindsNothingInAnErrorPostgreSQLDidNotRaise(t *testing.T) {
	t.Parallel()

	for name, err := range map[string]error{
		"a plain error": errors.New("the connection is gone"),
		"no error":      nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, found := readError(err)

			if got != (typestore.Failure{}) || found {
				t.Errorf("readError() = (%+v, %t), want no code, no constraint, no context and nothing found",
					got, found)
			}
		})
	}
}

func TestRulesBrokenListsEachRuleThePostgreSQLErrorsNameInMatchOrder(t *testing.T) {
	t.Parallel()

	want := []struct {
		name string
		rule typestore.Rule
	}{
		{"content holding the type under any constraint", typestore.Rule{
			Code: restrictViolationCode, Broken: typestore.TypeHeldByContent,
		}},
		{"a content path taken under any code", typestore.Rule{
			Name: slugConstraint, Broken: typestore.ContentPathUnique,
		}},
		{"a group key taken", typestore.Rule{
			Code: uniqueViolationCode, Name: groupKeyConstraint, Broken: typestore.GroupKeyUnique,
		}},
		{"a field key taken in its group", typestore.Rule{
			Code: uniqueViolationCode, Name: fieldKeyConstraint, Broken: typestore.FieldKeyUnique,
		}},
		{"a field key taken in its container", typestore.Rule{
			Code: uniqueViolationCode, Name: containerKeyConstraint, Broken: typestore.FieldKeyUnique,
		}},
		{"a type key taken", typestore.Rule{
			Code: uniqueViolationCode, Name: typeKeyConstraint, Broken: typestore.TypeKeyUnique,
		}},
		{"a route word taken", typestore.Rule{
			Code: uniqueViolationCode, Name: routeWordConstraint, Broken: typestore.RouteWordUnique,
		}},
		{"one default type", typestore.Rule{
			Code: uniqueViolationCode, Name: oneDefaultConstraint, Broken: typestore.RootTaken,
		}},
		{"a relation target reference broken", typestore.Rule{
			Code: foreignKeyViolationCode, Name: fieldTargetConstraint, Broken: typestore.FieldTargetExists,
		}},
		{"a field group reference broken", typestore.Rule{
			Code: foreignKeyViolationCode, Name: fieldGroupConstraint, Broken: typestore.FieldGroupExists,
		}},
	}

	if len(rulesBroken) != len(want) {
		t.Errorf("rulesBroken holds %d rules, want %d", len(rulesBroken), len(want))
	}
	for at, test := range want {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if at >= len(rulesBroken) {
				t.Fatalf("rulesBroken holds no rule at %d, want %+v", at, test.rule)
			}
			if rulesBroken[at] != test.rule {
				t.Errorf("rulesBroken[%d] = %+v, want %+v", at, rulesBroken[at], test.rule)
			}
		})
	}
}

func TestContentRulesBrokenListsEachRuleThePostgreSQLErrorsOfAContentWriteNameInMatchOrder(t *testing.T) {
	t.Parallel()

	want := []typestore.Rule{
		{Name: slugConstraint, Broken: typestore.ContentPathUnique},
		{Code: foreignKeyViolationCode, Name: relationFromConstraint, Broken: typestore.RelationTargetExists},
		{Code: foreignKeyViolationCode, Name: relationFieldConstraint, Broken: typestore.RelationTargetExists},
		{Code: foreignKeyViolationCode, Name: relationTargetConstraint, Broken: typestore.RelationTargetExists},
		{Code: deadlockCode, Context: slugConstraint, Broken: typestore.AddressCheckDeadlocked},
	}

	if !slices.Equal(contentRulesBroken, want) {
		t.Errorf("contentRulesBroken = %+v, want %+v", contentRulesBroken, want)
	}
}

func TestContentRulesBrokenNameTheStoredRuleEachPostgreSQLFailureOfAContentWriteBreaks(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		err  error
		want typestore.Constraint
	}{
		"the address taken on a unique check": {
			&pgconn.PgError{Code: uniqueViolationCode, ConstraintName: "content_path_unique"},
			typestore.ContentPathUnique,
		},
		"the address taken at the commit of a deferred check": {
			fmt.Errorf("commit: %w", &pgconn.PgError{Code: "23505", ConstraintName: "content_path_unique"}),
			typestore.ContentPathUnique,
		},
		"the address named under another code": {
			&pgconn.PgError{Code: "40001", ConstraintName: "content_path_unique"}, typestore.ContentPathUnique,
		},
		"the pointing item gone": {
			&pgconn.PgError{Code: "23503", ConstraintName: "content_relations_from_id_fkey"},
			typestore.RelationTargetExists,
		},
		"the relation field gone": {
			&pgconn.PgError{Code: "23503", ConstraintName: "content_relations_field_id_fkey"},
			typestore.RelationTargetExists,
		},
		"the target gone": {
			&pgconn.PgError{Code: "23503", ConstraintName: "content_relations_to_id_fkey"},
			typestore.RelationTargetExists,
		},
		"a relation key under another code": {
			&pgconn.PgError{Code: uniqueViolationCode, ConstraintName: "content_relations_to_id_fkey"},
			typestore.NoConstraint,
		},
		"a key of the index no rule names": {
			&pgconn.PgError{Code: "23503", ConstraintName: "content_relations_pkey"}, typestore.NoConstraint,
		},
		"a field key the type store names": {
			&pgconn.PgError{Code: "23503", ConstraintName: fieldTargetConstraint}, typestore.NoConstraint,
		},
		"a deadlock broken while writing the address": {
			fmt.Errorf("create content: %w", &pgconn.PgError{Code: "40P01", Where: addressCheckContext}),
			typestore.AddressCheckDeadlocked,
		},
		"a deadlock broken anywhere else": {
			&pgconn.PgError{Code: "40P01", Where: "while locking tuple (0,3) in relation \"content\""},
			typestore.NoConstraint,
		},
		"a deadlock with no context": {&pgconn.PgError{Code: "40P01"}, typestore.NoConstraint},
		"another failure while writing the address": {
			&pgconn.PgError{Code: "57014", Where: addressCheckContext}, typestore.NoConstraint,
		},
		"a plain error": {errors.New("the connection is gone"), typestore.NoConstraint},
		"no error":      {nil, typestore.NoConstraint},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := typestore.Broken(contentRulesBroken, readError, test.err); got != test.want {
				t.Errorf("Broken(%v) = %d, want %d", test.err, got, test.want)
			}
		})
	}
}
