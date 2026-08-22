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

func TestCan(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		role       string
		capability role.Capability
		want       bool
	}{
		"an admin administers accounts":           {role.Admin, role.ManageUsers, true},
		"an admin manages themes":                 {role.Admin, role.ManageThemes, true},
		"an admin reshapes the content model":     {role.Admin, role.ManageTypes, true},
		"an admin writes the site settings":       {role.Admin, role.ManageSettings, true},
		"an admin changes work it did not write":  {role.Admin, role.ChangeOthersWork, true},
		"an editor changes work it did not write": {role.Editor, role.ChangeOthersWork, true},
		"an editor cannot administer accounts":    {role.Editor, role.ManageUsers, false},
		"an editor cannot manage themes":          {role.Editor, role.ManageThemes, false},
		"an author holds none of them":            {role.Author, role.ChangeOthersWork, false},
		"an unknown role holds nothing":           {"archivist", role.ManageUsers, false},
		"an account holding no role holds none":   {"", role.ManageSettings, false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := role.Can(test.role, test.capability); got != test.want {
				t.Errorf("Can(%q, %q) = %v, want %v", test.role, test.capability, got, test.want)
			}
		})
	}
}

func TestPrivilegedIsWhoeverAdministersAccounts(t *testing.T) {
	t.Parallel()

	held := role.Privileged()

	for _, named := range []string{role.Admin, role.Editor, role.Author} {
		if held.Holds(named) != role.Can(named, role.ManageUsers) {
			t.Errorf("Privileged().Holds(%q) = %v, want it to follow Can(%q, ManageUsers)",
				named, held.Holds(named), named)
		}
	}
}

func TestMayChange(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	stranger := uuid.New()
	tests := map[string]struct {
		role  string
		actor uuid.UUID
		want  bool
	}{
		"an admin changes another account's work":         {role.Admin, stranger, true},
		"an editor changes another account's work":        {role.Editor, stranger, true},
		"an author changes its own work":                  {role.Author, owner, true},
		"an author is refused another account's":          {role.Author, stranger, false},
		"an unreadable role is refused another's":         {"archivist", stranger, false},
		"an unreadable role still changes its own":        {"archivist", owner, true},
		"an account holding no role is refused another's": {"", stranger, false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := role.MayChange(test.role, test.actor, owner); got != test.want {
				t.Errorf("MayChange(%q, actor, owner) = %v, want %v", test.role, got, test.want)
			}
		})
	}
}
