// SPDX-License-Identifier: Apache-2.0

// Package role names the roles a Gophenberg account holds and the capabilities they carry.
package role

import (
	"slices"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"
)

// Admin is the role holding every authority over the site.
const Admin = "admin"

// Editor is the role working every account's content and media.
const Editor = "editor"

// Author is the role working only its own content and media.
const Author = "author"

// Capability is a named permission a decision point asks for.
type Capability string

// ManageUsers is the capability administering accounts.
const ManageUsers Capability = "manage_users"

// ManageThemes is the capability installing and switching themes.
const ManageThemes Capability = "manage_themes"

// ManageTypes is the capability reshaping the content model.
const ManageTypes Capability = "manage_types"

// ManageSettings is the capability writing the site wide settings.
const ManageSettings Capability = "manage_settings"

// ChangeOthersWork is the capability changing content and media another account wrote.
const ChangeOthersWork Capability = "change_others_work"

// carried maps each role onto the capabilities it holds.
var carried = map[string][]Capability{
	Admin:  {ManageUsers, ManageThemes, ManageTypes, ManageSettings, ChangeOthersWork},
	Editor: {ChangeOthersWork},
	Author: {},
}

// Can reports whether a role holds the capability, an unknown role holding none.
func Can(role string, capability Capability) bool {
	return slices.Contains(carried[role], capability)
}

// CapabilitiesOf returns the capabilities a role carries, named for a caller outside this package.
func CapabilitiesOf(role string) []string {
	held := make([]string, 0, len(carried[role]))
	for _, capability := range carried[role] {
		held = append(held, string(capability))
	}
	return held
}

// Privileged returns the roles administering accounts, the cover the safety rails keep.
func Privileged() gouncer.Roles {
	var roles gouncer.Roles
	for role := range carried {
		if Can(role, ManageUsers) {
			roles = append(roles, role)
		}
	}
	slices.Sort(roles)
	return roles
}

// MayChange reports whether an account of the given role may change work the author owns.
func MayChange(role string, actor, author uuid.UUID) bool {
	return Can(role, ChangeOthersWork) || actor == author
}
