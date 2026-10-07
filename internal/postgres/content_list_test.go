// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// trash is the shared fixture under the name these tests use.
var trash = contenttest.Trash

func TestContentStoreCountsReportsDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, _, pool := newContentStoreWithPool(t)
	pool.Close()

	_, err := store.Counts(t.Context(), content.TypePost)

	if err == nil {
		t.Error("Counts() on a closed pool error = nil, want a failure")
	}
}

func TestContentStoreListReportsARejectedNestedQuery(t *testing.T) {
	t.Parallel()

	store, _ := newContentStore(t)

	_, _, err := store.List(t.Context(), content.Filter{Type: content.TypePost, Hierarchy: true, Page: 1, PerPage: -1})

	if err == nil {
		t.Error("List() nesting with a negative page size error = nil, want a failure")
	}
}
