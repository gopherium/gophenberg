// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"slices"

	"github.com/google/uuid"
)

// Session is the account a request is made by, as a plugin reads it.
type Session struct {
	ID    uuid.UUID
	Email string
	Name  string
	Role  string

	capabilities []string
}

// NewSession returns the session a host files for a request.
func NewSession(id uuid.UUID, email, name, role string, capabilities []string) Session {
	return Session{
		ID: id, Email: email, Name: name, Role: role,
		capabilities: slices.Clone(capabilities),
	}
}

// Can reports whether the session holds the named capability.
func (s Session) Can(capability string) bool {
	return slices.Contains(s.capabilities, capability)
}

// sessionKey is the key a session is filed under, owned by this package alone.
type sessionKey struct{}

// WithSession returns a context carrying the session, which only the host fills.
func WithSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, session)
}

// SessionFrom returns the session a request was made by, reporting whether one was filed.
func SessionFrom(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(sessionKey{}).(Session)
	return session, ok
}
