// SPDX-License-Identifier: Apache-2.0

// Package role names the roles a Gophenberg account holds and the capabilities they carry.
package role

import (
	"slices"

	"github.com/google/uuid"
	"github.com/gopherium/framework/goncierge"
)

// Source is the source the core grants its roles under in a registry.
const Source = "core"

// Admin is the role holding every authority over the site.
const Admin = "admin"

// Editor is the role working every account's content and media.
const Editor = "editor"

// Author is the role working only its own content and media.
const Author = "author"

// Member is the role of a signed-in account that never enters the admin.
const Member = "member"

// Capability is a named permission a decision point asks for.
type Capability string

// AccessAdmin is the capability entering the admin and working its content and media.
const AccessAdmin Capability = "access_admin"

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

// UpdateCore is the capability reading and running the checks for a newer Gophenberg.
const UpdateCore Capability = "update_core"

// ActivatePlugins is the capability reading which plugins the site holds and whether each one runs.
const ActivatePlugins Capability = "activate_plugins"

// grant is one role and the capabilities it holds.
type grant struct {
	role         string
	capabilities []Capability
}

// carried lists every role in declaration order with the capabilities it holds.
var carried = []grant{
	{Admin, []Capability{
		AccessAdmin, ManageUsers, ManageThemes, ManageTypes, ManageSettings, ChangeOthersWork, UpdateCore,
		ActivatePlugins,
	}},
	{Editor, []Capability{AccessAdmin, ChangeOthersWork}},
	{Author, []Capability{AccessAdmin}},
	{Member, nil},
}

// Core returns the core's roles with the capabilities each carries, for a registry to grant.
func Core() []goncierge.Role {
	roles := make([]goncierge.Role, 0, len(carried))
	for _, entry := range carried {
		held := make([]string, 0, len(entry.capabilities))
		for _, capability := range entry.capabilities {
			held = append(held, string(capability))
		}
		roles = append(roles, goncierge.Role{Name: entry.role, Capabilities: held})
	}
	return roles
}

// Registry returns a registry holding the core's roles alone.
func Registry() *goncierge.Registry {
	registry := goncierge.New()
	_ = registry.Replace(Source, Core())
	return registry
}

// MayChange reports whether an account carrying the capabilities may change work the author owns.
func MayChange(capabilities []string, actor, author uuid.UUID) bool {
	return slices.Contains(capabilities, string(ChangeOthersWork)) || actor == author
}
