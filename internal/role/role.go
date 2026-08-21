// SPDX-License-Identifier: Apache-2.0

// Package role names the ranks a Gophenberg account holds.
package role

import (
	"github.com/google/uuid"

	"github.com/gopherium/gouncer"
)

// Admin is the rank holding every authority over the site.
const Admin = "admin"

// Editor is the rank working every account's content and media.
const Editor = "editor"

// Author is the rank working only its own content and media.
const Author = "author"

// Privileged returns the ranks that may administer the site.
func Privileged() gouncer.Ranks {
	return gouncer.Ranks{Admin}
}

// MayChange reports whether an account of the given rank may change work the author owns.
func MayChange(rank string, actor, author uuid.UUID) bool {
	if rank == Admin || rank == Editor {
		return true
	}
	return actor == author
}
