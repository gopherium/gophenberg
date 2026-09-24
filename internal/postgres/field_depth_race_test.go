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

// queuedInTurn queues the writes on the field groups lock one after the other, lets them go, and returns each answer.
func queuedInTurn(t *testing.T, pool *pgxpool.Pool, writes ...func() error) []error {
	t.Helper()
	release := sessionLock(t, pool)
	answered := make([]error, len(writes))
	var done sync.WaitGroup
	for i, write := range writes {
		done.Go(func() { answered[i] = write() })
		writersQueued(t, pool, i+1)
	}
	release()
	done.Wait()
	return answered
}

// secondRefusedWithin asserts the first write landed, the second was refused as too deep and no field passes the limit.
func secondRefusedWithin(t *testing.T, pool *pgxpool.Pool, answered []error, limit int) {
	t.Helper()
	if answered[0] != nil {
		t.Errorf("the first write error = %v, want nil", answered[0])
	}
	if !errors.Is(answered[1], content.ErrFieldTooDeep) {
		t.Errorf("the second write error = %v, want %v", answered[1], content.ErrFieldTooDeep)
	}
	var deepest int
	if err := pool.QueryRow(t.Context(), `SELECT max(depth) FROM core.content_fields`).Scan(&deepest); err != nil {
		t.Fatalf("reading the deepest field: %v, want nil", err)
	}
	if deepest > limit {
		t.Errorf("a field stands at depth %d, want %d at most", deepest, limit)
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

	answered := queuedInTurn(t, pool,
		func() error {
			_, err := registry.MoveField(t.Context(), author.ID, author.GroupID, address.ID)
			return err
		},
		func() error {
			_, err := registry.MoveField(t.Context(), street.ID, author.GroupID, author.ID)
			return err
		},
	)

	secondRefusedWithin(t, pool, answered, 1)
}

func TestAMoveAndANewSubFieldAtTheSameMomentLeaveNoFieldPastTheLimit(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	registry := content.NewRegistry(store).WithFieldDepth(1)
	author := declareSection(t, store, "author")
	address := declareSection(t, store, "address")
	name := fieldOn(t, "", "name", content.FieldKindText, "")

	answered := queuedInTurn(t, pool,
		func() error {
			_, err := registry.MoveField(t.Context(), author.ID, author.GroupID, address.ID)
			return err
		},
		func() error {
			_, err := registry.CreateSubField(t.Context(), author.ID, name)
			return err
		},
	)

	secondRefusedWithin(t, pool, answered, 1)
}
