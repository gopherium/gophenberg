// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"
)

// memoryStore holds one scenario's accounts and sessions in memory.
type memoryStore struct {
	mu       sync.Mutex
	users    map[string]gouncer.User
	sessions map[string]gouncer.Session
}

// newMemoryStore returns an empty account store.
func newMemoryStore() *memoryStore {
	return &memoryStore{
		users:    make(map[string]gouncer.User),
		sessions: make(map[string]gouncer.Session),
	}
}

// CreateUser stores the account, refusing an email already taken.
func (m *memoryStore) CreateUser(_ context.Context, user gouncer.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, taken := m.users[user.Email]; taken {
		return gouncer.ErrEmailTaken
	}
	m.users[user.Email] = user
	return nil
}

// UserByEmail returns the account with the email.
func (m *memoryStore) UserByEmail(_ context.Context, email string) (gouncer.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, found := m.users[email]
	if !found {
		return gouncer.User{}, gouncer.ErrUserNotFound
	}
	return user, nil
}

// CreateSession stores the session.
func (m *memoryStore) CreateSession(_ context.Context, session gouncer.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[hex.EncodeToString(session.TokenHash)] = session
	return nil
}

// UserBySession returns the account owning an unexpired session.
func (m *memoryStore) UserBySession(_ context.Context, tokenHash []byte, now time.Time) (gouncer.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, found := m.sessions[hex.EncodeToString(tokenHash)]
	if !found || !session.ExpiresAt.After(now) {
		return gouncer.User{}, gouncer.ErrSessionNotFound
	}
	for _, user := range m.users {
		if user.ID == session.UserID {
			return user, nil
		}
	}
	return gouncer.User{}, gouncer.ErrSessionNotFound
}

// DeleteSession removes the session.
func (m *memoryStore) DeleteSession(_ context.Context, tokenHash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, hex.EncodeToString(tokenHash))
	return nil
}

// ListUsers returns every account in email order.
func (m *memoryStore) ListUsers(_ context.Context) ([]gouncer.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	users := make([]gouncer.User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Email < users[j].Email })
	return users, nil
}

// SetUserDisabled updates whether the account may log in.
func (m *memoryStore) SetUserDisabled(_ context.Context, id uuid.UUID, disabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for email, user := range m.users {
		if user.ID == id {
			user.Disabled = disabled
			m.users[email] = user
			return nil
		}
	}
	return gouncer.ErrUserNotFound
}

// SetUserDisabledUnderCover disables an account, refusing to leave no enabled account privileged.
func (m *memoryStore) SetUserDisabledUnderCover(
	_ context.Context, id uuid.UUID, disabled bool, privileged gouncer.Roles,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.refuseUncoveredLocked(id, privileged, disabled); err != nil {
		return err
	}
	return m.updateLocked(id, func(user *gouncer.User) { user.Disabled = disabled })
}

// SetUserRole writes an account's role, refusing to leave no enabled account privileged.
func (m *memoryStore) SetUserRole(
	_ context.Context, id uuid.UUID, role string, privileged gouncer.Roles,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.refuseUncoveredLocked(id, privileged, !privileged.Holds(role)); err != nil {
		return err
	}
	return m.updateLocked(id, func(user *gouncer.User) { user.Role = role })
}

// updateLocked changes one stored account, the caller holding the lock.
func (m *memoryStore) updateLocked(id uuid.UUID, change func(*gouncer.User)) error {
	for email, user := range m.users {
		if user.ID == id {
			change(&user)
			m.users[email] = user
			return nil
		}
	}
	return gouncer.ErrUserNotFound
}

// refuseUncoveredLocked refuses a write leaving no enabled account privileged, the caller holding the lock.
func (m *memoryStore) refuseUncoveredLocked(id uuid.UUID, privileged gouncer.Roles, removesCover bool) error {
	if len(privileged) == 0 || !removesCover {
		return nil
	}
	held, cover := false, 0
	for _, user := range m.users {
		if user.Disabled || !privileged.Holds(user.Role) {
			continue
		}
		if user.ID == id {
			held = true
			continue
		}
		cover++
	}
	if held && cover == 0 {
		return gouncer.ErrLastPrivileged
	}
	return nil
}
