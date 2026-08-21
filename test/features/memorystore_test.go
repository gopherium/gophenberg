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
	ctx context.Context, id uuid.UUID, disabled bool, privileged gouncer.Ranks,
) error {
	if err := m.refuseUncovered(id, privileged, disabled); err != nil {
		return err
	}
	return m.SetUserDisabled(ctx, id, disabled)
}

// SetUserRank writes an account's rank, refusing to leave no enabled account privileged.
func (m *memoryStore) SetUserRank(
	_ context.Context, id uuid.UUID, rank string, privileged gouncer.Ranks,
) error {
	if err := m.refuseUncovered(id, privileged, !privileged.Holds(rank)); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for email, user := range m.users {
		if user.ID == id {
			user.Rank = rank
			m.users[email] = user
			return nil
		}
	}
	return gouncer.ErrUserNotFound
}

// refuseUncovered refuses a write that would leave no enabled account under a privileged rank.
func (m *memoryStore) refuseUncovered(id uuid.UUID, privileged gouncer.Ranks, removesCover bool) error {
	if len(privileged) == 0 || !removesCover {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	held, cover := false, 0
	for _, user := range m.users {
		if user.Disabled || !privileged.Holds(user.Rank) {
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
