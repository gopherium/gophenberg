// SPDX-License-Identifier: Apache-2.0

package role_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/gopherium/framework/goncierge"

	"github.com/gopherium/gophenberg/internal/role"
)

func TestCoreGrantsEachCoreRoleItsCapabilities(t *testing.T) {
	t.Parallel()

	registry := goncierge.New()
	err := registry.Replace(role.Source, role.Core())

	if err != nil {
		t.Fatalf("Replace(%q, Core()) = %v, want the core's names accepted", role.Source, err)
	}
	tests := map[string][]string{
		role.Admin: {
			string(role.AccessAdmin), string(role.ActivatePlugins), string(role.ChangeOthersWork),
			string(role.ManageSettings), string(role.ManageThemes), string(role.ManageTypes), string(role.ManageUsers),
			string(role.UpdateCore),
		},
		role.Editor: {string(role.AccessAdmin), string(role.ChangeOthersWork)},
		role.Author: {string(role.AccessAdmin)},
		role.Member: nil,
	}
	for named, want := range tests {
		if held := registry.CapabilitiesOf(named); !slices.Equal(held, want) {
			t.Errorf("CapabilitiesOf(%q) = %v, want %v", named, held, want)
		}
	}
	if roles := registry.Roles(); !slices.Equal(roles, []string{role.Admin, role.Author, role.Editor, role.Member}) {
		t.Errorf("Roles() = %v, want the four core roles", roles)
	}
	if held := registry.HoldersOf(string(role.ManageUsers)); !slices.Equal(held, []string{role.Admin}) {
		t.Errorf("HoldersOf(ManageUsers) = %v, want admin alone", held)
	}
	if held := registry.HoldersOf(string(role.UpdateCore)); !slices.Equal(held, []string{role.Admin}) {
		t.Errorf("HoldersOf(UpdateCore) = %v, want admin alone", held)
	}
	if held := registry.HoldersOf(string(role.ActivatePlugins)); !slices.Equal(held, []string{role.Admin}) {
		t.Errorf("HoldersOf(ActivatePlugins) = %v, want admin alone", held)
	}
}

func TestEveryStaffRoleAndNoMemberAccessesTheAdmin(t *testing.T) {
	t.Parallel()

	registry := role.Registry()

	staff := []string{role.Admin, role.Author, role.Editor}
	if held := registry.HoldersOf(string(role.AccessAdmin)); !slices.Equal(held, staff) {
		t.Errorf("HoldersOf(AccessAdmin) = %v, want the staff roles %v", held, staff)
	}
	if held := registry.CapabilitiesOf(role.Member); len(held) != 0 {
		t.Errorf("CapabilitiesOf(member) = %v, want none", held)
	}
}

func TestRegistryHoldsTheCoreRolesAlone(t *testing.T) {
	t.Parallel()

	registry := role.Registry()

	if roles := registry.Roles(); !slices.Equal(roles, []string{role.Admin, role.Author, role.Editor, role.Member}) {
		t.Errorf("Roles() = %v, want the four core roles alone", roles)
	}
	if held := registry.HoldersOf(string(role.ManageUsers)); !slices.Equal(held, []string{role.Admin}) {
		t.Errorf("HoldersOf(ManageUsers) = %v, want admin alone", held)
	}
	editorChanges := registry.Can(role.Editor, string(role.ChangeOthersWork))
	if !editorChanges || registry.Can(role.Author, string(role.ChangeOthersWork)) {
		t.Error("the registry does not carry the core's table, want editor to change others' work and author not")
	}
}

func TestCoreCannotBeChangedByItsCaller(t *testing.T) {
	t.Parallel()

	core := role.Core()
	core[0].Capabilities[0] = "manage_everything"
	core[1].Name = "owner"

	again := role.Core()

	if again[0].Capabilities[0] == "manage_everything" || again[1].Name == "owner" {
		t.Errorf("Core() = %v after a caller wrote to it, want the table untouched", again)
	}
}

func TestMayChange(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	stranger := uuid.New()
	changes := []string{string(role.ManageUsers), string(role.ChangeOthersWork)}
	tests := map[string]struct {
		capabilities []string
		actor        uuid.UUID
		want         bool
	}{
		"an account changing others' work changes another's":     {changes, stranger, true},
		"an account carrying nothing changes its own work":       {nil, owner, true},
		"an account carrying nothing is refused another's":       {nil, stranger, false},
		"an account carrying other capabilities is refused":      {[]string{string(role.ManageThemes)}, stranger, false},
		"an account carrying a plugin's capability is refused":   {[]string{"notes.manage"}, stranger, false},
		"an account changing others' work still changes its own": {changes, owner, true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := role.MayChange(test.capabilities, test.actor, owner); got != test.want {
				t.Errorf("MayChange(%v, actor, owner) = %v, want %v", test.capabilities, got, test.want)
			}
		})
	}
}
