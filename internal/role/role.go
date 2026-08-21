// SPDX-License-Identifier: Apache-2.0

// Package role names the ranks a Gophenberg account holds and the capabilities they carry.
package role

import (
	"slices"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"
)

// Admin is the rank holding every authority over the site.
const Admin = "admin"

// Editor is the rank working every account's content and media.
const Editor = "editor"

// Author is the rank working only its own content and media.
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

// carried maps each rank onto the capabilities it holds.
var carried = map[string][]Capability{
	Admin:  {ManageUsers, ManageThemes, ManageTypes, ManageSettings, ChangeOthersWork},
	Editor: {ChangeOthersWork},
	Author: {},
}

// Can reports whether a rank holds the capability, an unknown rank holding none.
func Can(rank string, capability Capability) bool {
	return slices.Contains(carried[rank], capability)
}

// Privileged returns the ranks administering accounts, the cover the safety rails keep.
func Privileged() gouncer.Ranks {
	var ranks gouncer.Ranks
	for rank := range carried {
		if Can(rank, ManageUsers) {
			ranks = append(ranks, rank)
		}
	}
	slices.Sort(ranks)
	return ranks
}

// MayChange reports whether an account of the given rank may change work the author owns.
func MayChange(rank string, actor, author uuid.UUID) bool {
	return Can(rank, ChangeOthersWork) || actor == author
}
