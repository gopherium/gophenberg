// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestFieldWriteFailureNamesWhatTheConstraintRefused(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		code       string
		constraint string
		want       error
	}{
		"a key another field holds": {
			uniqueViolationCode, fieldKeyConstraint, content.ErrFieldTaken,
		},
		"a target no type answers to": {
			foreignKeyViolationCode, fieldTargetConstraint, content.ErrTargetUnknown,
		},
		"a group that is gone": {
			foreignKeyViolationCode, fieldGroupConstraint, content.ErrGroupNotFound,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := fieldWriteFailure(&pgconn.PgError{Code: test.code, ConstraintName: test.constraint})

			if !errors.Is(held, test.want) {
				t.Errorf("fieldWriteFailure() = %v, want %v", held, test.want)
			}
		})
	}
}

func TestFieldWriteFailureWrapsWhatItDoesNotName(t *testing.T) {
	t.Parallel()

	stray := errors.New("the connection is gone")

	held := fieldWriteFailure(stray)

	if !errors.Is(held, stray) {
		t.Errorf("fieldWriteFailure() = %v, want the error it was given carried", held)
	}
}
