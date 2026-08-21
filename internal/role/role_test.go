// SPDX-License-Identifier: Apache-2.0

package role_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/role"
)

func TestPrivilegedNamesAdminAlone(t *testing.T) {
	t.Parallel()

	held := role.Privileged()

	if !held.Holds(role.Admin) {
		t.Errorf("Privileged() = %v, want it to hold %q", held, role.Admin)
	}
	for _, unprivileged := range []string{role.Editor, role.Author, ""} {
		if held.Holds(unprivileged) {
			t.Errorf("Privileged() holds %q, want the administration surface closed to it", unprivileged)
		}
	}
}

func TestPrivilegedCannotBeChangedByItsCaller(t *testing.T) {
	t.Parallel()

	held := role.Privileged()
	held[0] = role.Author

	if again := role.Privileged(); !again.Holds(role.Admin) {
		t.Errorf("Privileged() = %v after a caller wrote to it, want %q untouched", again, role.Admin)
	}
}

func TestMayChange(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	stranger := uuid.New()
	tests := map[string]struct {
		rank  string
		actor uuid.UUID
		want  bool
	}{
		"an admin changes another account's work":  {role.Admin, stranger, true},
		"an editor changes another account's work": {role.Editor, stranger, true},
		"an author changes its own work":           {role.Author, owner, true},
		"an author is refused another account's":   {role.Author, stranger, false},
		"an unreadable rank is refused another's":  {"archivist", stranger, false},
		"an unreadable rank still changes its own": {"archivist", owner, true},
		"an unranked account is refused another's": {"", stranger, false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := role.MayChange(test.rank, test.actor, owner); got != test.want {
				t.Errorf("MayChange(%q, actor, owner) = %v, want %v", test.rank, got, test.want)
			}
		})
	}
}
