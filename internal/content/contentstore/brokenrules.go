// SPDX-License-Identifier: Apache-2.0

package contentstore

import (
	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/typestore"
)

// writeErrors maps the rule a content write can break to the error it stands for.
var writeErrors = map[typestore.Constraint]error{
	typestore.RelationTargetExists: content.ErrTargetNotFound,
}

// slugRetries holds the rules after which a create tries the next numbered slug.
var slugRetries = map[typestore.Constraint]bool{
	typestore.ContentPathUnique:      true,
	typestore.AddressCheckDeadlocked: true,
}

// broken returns the stored rule err broke, as the engine's rules name it.
func (s *Store) broken(err error) typestore.Constraint {
	return typestore.Broken(s.backend.Rules, s.backend.ReadError, err)
}
