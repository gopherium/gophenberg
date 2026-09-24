// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// deepEnough is a depth limit no field these tests declare comes near.
const deepEnough = content.DefaultFieldDepth

// sectionInside declares a section inside the container and returns it.
func sectionInside(t *testing.T, store *postgres.TypeStore, parent content.Field, key string) content.Field {
	t.Helper()
	built, err := content.NewSubField(content.Field{Key: key, Label: key, Kind: content.FieldKindSection}, parent.Kind)
	if err != nil {
		t.Fatalf("NewSubField(%s) error = %v, want nil", key, err)
	}
	stored, err := store.CreateSubField(t.Context(), parent.ID, built, deepEnough)
	if err != nil {
		t.Fatalf("CreateSubField(%s) error = %v, want nil", key, err)
	}
	return stored
}

// sessionLock holds the field groups lock on a connection of its own and returns what lets it go.
func sessionLock(t *testing.T, pool *pgxpool.Pool) func() {
	t.Helper()
	conn, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatalf("acquiring a connection: %v", err)
	}
	if _, err := conn.Exec(t.Context(), "SELECT pg_advisory_lock(hashtext('core.field_groups'))"); err != nil {
		conn.Release()
		t.Fatalf("holding the lock: %v", err)
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			if _, err := conn.Exec(context.Background(),
				"SELECT pg_advisory_unlock(hashtext('core.field_groups'))"); err != nil {
				t.Errorf("releasing the lock: %v", err)
			}
			conn.Release()
		})
	}
	t.Cleanup(release)
	return release
}

// writersQueued waits until as many connections as asked stand queued on the field groups lock.
func writersQueued(t *testing.T, pool *pgxpool.Pool, count int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var waiting int
		if err := pool.QueryRow(t.Context(),
			`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND wait_event = 'advisory'`,
		).Scan(&waiting); err != nil {
			t.Fatalf("polling pg_stat_activity: %v", err)
		}
		if waiting >= count {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d writers stand queued on the lock, want %d", waiting, count)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestMoveFieldRefusesALandingPastTheLimit(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	outer := declareSection(t, store, "outer")
	holder := declareSection(t, store, "holder")
	sectionInside(t, store, holder, "inner")

	_, err := store.MoveField(t.Context(), holder.ID, outer.GroupID, outer.ID, 1)

	if !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldTooDeep)
	}
	standsUnder(t, pool, holder.ID, holder.GroupID, 0, 0)
}

func TestCreateSubFieldRefusesAFieldPastTheLimit(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	inner := sectionInside(t, store, declareSection(t, store, "outer"), "inner")

	_, err := store.CreateSubField(t.Context(), inner.ID, fieldOn(t, "", "street", content.FieldKindText, ""), 1)

	if !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("CreateSubField() error = %v, want %v", err, content.ErrFieldTooDeep)
	}
	var stored int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM core.content_fields WHERE key = 'street'`).Scan(&stored); err != nil {
		t.Fatalf("counting the street rows: %v, want nil", err)
	}
	if stored != 0 {
		t.Errorf("%d street rows stand, want the refused field left out", stored)
	}
}

func TestTwoMovesAtTheSameMomentLeaveNoFieldPastTheLimit(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	registry := content.NewRegistry(store).WithFieldDepth(1)
	author := declareSection(t, store, "author")
	address := declareSection(t, store, "address")
	street := declareTypedField(t, store, "car", "street")
	release := sessionLock(t, pool)
	moves := []struct{ id, into int }{{author.ID, address.ID}, {street.ID, author.ID}}
	answered := make(chan error, len(moves))
	for _, move := range moves {
		go func() {
			_, err := registry.MoveField(t.Context(), move.id, author.GroupID, move.into)
			answered <- err
		}()
	}
	writersQueued(t, pool, len(moves))

	release()

	refused := 0
	for range moves {
		err := <-answered
		if errors.Is(err, content.ErrFieldTooDeep) {
			refused++
			continue
		}
		if err != nil {
			t.Errorf("MoveField() error = %v, want nil or %v", err, content.ErrFieldTooDeep)
		}
	}
	if refused != 1 {
		t.Errorf("%d moves refused, want the one landing second", refused)
	}
	var deepest int
	if err := pool.QueryRow(t.Context(), `SELECT max(depth) FROM core.content_fields`).Scan(&deepest); err != nil {
		t.Fatalf("reading the deepest field: %v, want nil", err)
	}
	if deepest > 1 {
		t.Errorf("a field stands at depth %d, want 1 at most", deepest)
	}
}
