// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/sqlite"
)

// impatientContentStore returns a content store holding one draft post on a handle that waits briefly for a writer.
func impatientContentStore(t *testing.T) (*sql.DB, *sqlite.ContentStore, content.Content) {
	t.Helper()
	handle := impatientHandle(t)
	store := sqlite.NewContentStore(handle)
	post := contenttest.MustCreate(t, store, "Harbor Notes", addAuthor(t, handle, contenttest.DefaultAuthor))
	return handle, store, post
}

// wantBusy fails t unless the call's err carries the dbkit busy class under the text prefix.
func wantBusy(t *testing.T, call string, err error, prefix string) {
	t.Helper()
	if !errors.Is(err, dbkit.ErrBusy) || !strings.HasPrefix(fmt.Sprint(err), prefix) {
		t.Errorf("%s error = %v, want dbkit.ErrBusy under %q", call, err, prefix)
	}
}

func TestContentDeleteHeldPastTheBusyTimeoutAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	handle, store, post := impatientContentStore(t)
	holdWrites(t, handle)

	err := store.Delete(t.Context(), post.ID)

	wantBusy(t, "Delete() behind another writer", err, "sqlite: delete content: ")
}

func TestContentUpdateHeldPastTheBusyTimeoutAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	handle, store, post := impatientContentStore(t)
	holdWrites(t, handle)

	_, err := store.Update(t.Context(), contenttest.EditTitle(post, "Harbor Notes Revised"), post.UpdatedAt, nil, 0)

	wantBusy(t, "Update() behind another writer", err, "sqlite: update content: ")
}

func TestContentReadsBehindAnExclusiveConnectionAnswerDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	handle, store, post := impatientContentStore(t)
	owner := otherWriter(t, handle)
	for _, statement := range []string{"PRAGMA locking_mode=EXCLUSIVE", "BEGIN IMMEDIATE",
		"UPDATE core_content SET title = 'held'", "COMMIT"} {
		if _, err := owner.ExecContext(t.Context(), statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}

	_, byID := store.ByID(t.Context(), post.ID)
	_, _, listed := store.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: 10})

	wantBusy(t, "ByID() behind an exclusive connection", byID, "sqlite: get content: ")
	wantBusy(t, "List() behind an exclusive connection", listed, "sqlite: count content: ")
}
