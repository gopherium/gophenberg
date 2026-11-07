// SPDX-License-Identifier: Apache-2.0

package contentstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/typestore"
)

// errConnectionGone is the failure a statement or a transaction answers with when the database drops.
var errConnectionGone = errors.New("the connection is gone")

// everyEngine lists the engines each text is checked under.
var everyEngine = []dbkit.Engine{dbkit.Postgres, dbkit.SQLite}

// unnamedRule is a broken rule no rule the engine lends names.
var unnamedRule = brokeRule{"unique", "slug"}

// failingStore returns a store on the engine over the site shelf whose named entry answers every call with given.
func failingStore(t *testing.T, engine dbkit.Engine, entry string, given error) *Store {
	f := newFake(t, site(), noFailure)
	f.breaks[entry] = given
	return f.store(engine)
}

// errOf returns the error a step answered with, dropping its value.
func errOf[T any](_ T, err error) error {
	return err
}

// underEngine returns a text written for postgres as the engine prints it.
func underEngine(engine dbkit.Engine, postgresText string) string {
	return strings.ReplaceAll(postgresText, "postgres:", engine.String()+":")
}

// checkWrapped fails the test unless got carries given behind the tail's text as the engine prints it.
func checkWrapped(t *testing.T, engine dbkit.Engine, prefix string, given, got error) {
	t.Helper()
	want := underEngine(engine, prefix+given.Error())
	if got == nil || got.Error() != want {
		t.Errorf("error = %v, want %q", got, want)
		return
	}
	if !errors.Is(got, given) {
		t.Errorf("errors.Is(%v, %v) = false, want the failure carried", got, given)
	}
}

// writePasses lists the domain errors a content write passes unchanged today.
var writePasses = []error{
	content.ErrNotFound, content.ErrConflict, content.ErrUnknownField, content.ErrNotHierarchical,
	content.ErrTargetNotFound, content.ErrTargetType, content.ErrParentType, content.ErrParentTrashed,
	content.ErrCycle, content.ErrReservedAddress,
}

// passing returns the sentinels each standing for itself.
func passing(sentinels ...error) map[error]error {
	held := make(map[error]error, len(sentinels))
	for _, sentinel := range sentinels {
		held[sentinel] = sentinel
	}
	return held
}

// withMeaning returns the sentinels with one more standing for the error it comes back as.
func withMeaning(held map[error]error, sentinel, meant error) map[error]error {
	held[sentinel] = meant
	return held
}

// tailCase is one step's last word on an error: what each rule and sentinel comes back as, and the text of the rest.
type tailCase struct {
	entry     string
	run       func(ctx context.Context, s *Store) error
	rules     map[typestore.Constraint]error
	sentinels map[error]error
	wrap      string
}

// writeRules holds what a content write gives for each broken rule.
var writeRules = map[typestore.Constraint]error{
	typestore.ContentPathUnique:    content.ErrSlugTaken,
	typestore.RelationTargetExists: content.ErrTargetNotFound,
}

// tails holds every error tail of the content store, as each step answers today.
var tails = map[string]tailCase{
	"Create on the bare path": {
		entry: "CreateContent",
		run: func(ctx context.Context, s *Store) error {
			return errOf(s.Create(ctx, launch(nil)))
		},
		rules: map[typestore.Constraint]error{typestore.ContentPathUnique: content.ErrSlugTaken},
		wrap:  "postgres: create content: ",
	},
	"Create on the held path": {
		entry: beginEvent,
		run: func(ctx context.Context, s *Store) error {
			return errOf(s.Create(ctx, launch(content.Values{"rating": float64(5)})))
		},
		rules:     writeRules,
		sentinels: passing(writePasses...),
		wrap:      "postgres: update content: ",
	},
	"Update": {
		entry: beginEvent,
		run: func(ctx context.Context, s *Store) error {
			return errOf(s.Update(ctx, retitledHello, shelvedAt, nil, 0))
		},
		rules:     writeRules,
		sentinels: passing(writePasses...),
		wrap:      "postgres: update content: ",
	},
	"Update when the sibling slugs do not read": {
		entry: "SiblingSlugTaken",
		run: func(ctx context.Context, s *Store) error {
			return errOf(s.Update(ctx, retitledHello, shelvedAt, nil, 0))
		},
		wrap: "postgres: update content: read sibling slugs: ",
	},
	"Trash": {
		entry: beginEvent,
		run: func(ctx context.Context, s *Store) error {
			return errOf(s.Trash(ctx, helloID, editedAt))
		},
		sentinels: withMeaning(passing(content.ErrNotFound, content.ErrHoldsChildren, content.ErrInvalidTransition),
			sql.ErrNoRows, content.ErrNotFound),
		wrap: "postgres: trash content: ",
	},
	"Restore": {
		entry: beginEvent,
		run: func(ctx context.Context, s *Store) error {
			return errOf(s.Restore(ctx, contactID, editedAt))
		},
		sentinels: withMeaning(passing(content.ErrInvalidTransition), sql.ErrNoRows, content.ErrNotFound),
		wrap:      "postgres: restore content: ",
	},
	"Restore when the original address is not written": {
		entry: "RestoreContent",
		run: func(ctx context.Context, s *Store) error {
			return errOf(s.Restore(ctx, contactID, editedAt))
		},
		rules: map[typestore.Constraint]error{typestore.ContentPathUnique: nil},
		sentinels: withMeaning(withMeaning(passing(content.ErrInvalidTransition),
			sql.ErrNoRows, content.ErrNotFound), content.ErrReservedAddress, nil),
		wrap: "postgres: restore content: ",
	},
	"DeleteRevision": {
		entry: beginEvent,
		run: func(ctx context.Context, s *Store) error {
			return s.DeleteRevision(ctx, helloID, revisionID)
		},
		sentinels: passing(content.ErrNotFound, content.ErrTrashed, content.ErrRevisionNotFound),
		wrap:      "postgres: delete revision: ",
	},
	"SaveAutosave": {
		entry: beginEvent,
		run: func(ctx context.Context, s *Store) error {
			return errOf(s.SaveAutosave(ctx, helloAutosave))
		},
		sentinels: passing(content.ErrNotFound, content.ErrTrashed),
		wrap:      "postgres: save autosave: ",
	},
	"EmptyTrash": {
		entry: beginEvent,
		run: func(ctx context.Context, s *Store) error {
			_, _, err := s.EmptyTrash(ctx, "page", nil)
			return err
		},
		wrap: "postgres: empty the trash: ",
	},
}

// ruleFailures holds the failure that breaks each rule the content store tells apart.
var ruleFailures = map[typestore.Constraint]error{
	typestore.ContentPathUnique:      addressTaken,
	typestore.RelationTargetExists:   targetGone,
	typestore.AddressCheckDeadlocked: addressDeadlocked,
}

func TestEachTailGivesTheErrorItsTableHoldsAndWrapsEveryOtherBrokenRule(t *testing.T) {
	t.Parallel()

	for name, test := range tails {
		for _, engine := range everyEngine {
			t.Run(name+" under "+engine.String(), func(t *testing.T) {
				t.Parallel()

				for rule, given := range ruleFailures {
					got := test.run(t.Context(), failingStore(t, engine, test.entry, given))
					want, mapped := test.rules[rule]
					if !mapped {
						checkWrapped(t, engine, test.wrap, given, got)
						continue
					}
					if got != want {
						t.Errorf("tail(%v) = %v, want %v", given, got, want)
					}
				}
			})
		}
	}
}

func TestEachTailWrapsABrokenRuleNoRuleNames(t *testing.T) {
	t.Parallel()

	for name, test := range tails {
		for _, engine := range everyEngine {
			t.Run(name+" under "+engine.String(), func(t *testing.T) {
				t.Parallel()

				got := test.run(t.Context(), failingStore(t, engine, test.entry, unnamedRule))

				checkWrapped(t, engine, test.wrap, unnamedRule, got)
			})
		}
	}
}

func TestEachTailPassesTheSentinelsItPassesTodayAndWrapsTheOthers(t *testing.T) {
	t.Parallel()

	for name, test := range tails {
		for _, engine := range everyEngine {
			t.Run(name+" under "+engine.String(), func(t *testing.T) {
				t.Parallel()

				for _, sentinel := range contentSentinels {
					got := test.run(t.Context(), failingStore(t, engine, test.entry, sentinel))
					want, meant := test.sentinels[sentinel]
					if !meant {
						checkWrapped(t, engine, test.wrap, sentinel, got)
						continue
					}
					if got != want {
						t.Errorf("tail(%v) = %v, want %v", sentinel, got, want)
					}
				}
			})
		}
	}
}

func TestEachTailWrapsAPlainFailureInTodaysText(t *testing.T) {
	t.Parallel()

	for name, test := range tails {
		for _, engine := range everyEngine {
			t.Run(name+" under "+engine.String(), func(t *testing.T) {
				t.Parallel()

				got := test.run(t.Context(), failingStore(t, engine, test.entry, errConnectionGone))
				if got == nil {
					t.Fatal("tail() = nil, want the failure wrapped")
				}

				checkWrapped(t, engine, test.wrap, errConnectionGone, got)
				if !strings.HasPrefix(got.Error(), engine.String()+": ") {
					t.Errorf("error = %q, want it to start with the engine's name", got)
				}
			})
		}
	}
}

func TestAWriteFailingOnADomainErrorAndABrokenRelationKeyPassesItAsItIs(t *testing.T) {
	t.Parallel()

	given := fmt.Errorf("%w: %w", content.ErrConflict, targetGone)

	got := failingStore(t, dbkit.Postgres, beginEvent, given).writeFailure(given)

	if got != given {
		t.Errorf("writeFailure(%v) = %v, want it passed as it is", given, got)
	}
}

func TestAWriteFailingOnTheAddressAndABrokenRelationKeyPassesItAsItIs(t *testing.T) {
	t.Parallel()

	given := fmt.Errorf("%w: %w", addressTaken, targetGone)

	got := failingStore(t, dbkit.Postgres, beginEvent, given).writeFailure(given)

	if got != given {
		t.Errorf("writeFailure(%v) = %v, want it passed as it is", given, got)
	}
}

func TestCreateTriesEveryNumberedSlugThenTheIdentifiedOneBeforeAnsweringSlugTaken(t *testing.T) {
	t.Parallel()

	f := newFake(t, site(), noFailure)
	f.breaks["CreateContent"] = addressTaken

	_, err := f.store(dbkit.Postgres).Create(t.Context(), launch(nil))

	if err != content.ErrSlugTaken {
		t.Fatalf("Create() error = %v, want %v", err, content.ErrSlugTaken)
	}
	var paths []string
	for _, entry := range f.log {
		paths = append(paths, entry.args.(ContentRow).Path)
	}
	want := []string{"posts/launch"}
	for attempt := 2; attempt <= slugAttempts; attempt++ {
		want = append(want, fmt.Sprintf("posts/launch-%d", attempt))
	}
	want = append(want, "posts/launch-"+strings.ReplaceAll(newID.String(), "-", ""))
	if strings.Join(paths, " ") != strings.Join(want, " ") {
		t.Errorf("addresses tried = %q, want %q", paths, want)
	}
}

func TestUpdateTriesEveryNumberedSiblingSlugBeforeAnsweringSlugTaken(t *testing.T) {
	t.Parallel()

	f := newFake(t, site(), noFailure)
	var slugs []string
	stocked := f.shelf
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		slug := numberedSlug("hello", attempt)
		slugs = append(slugs, slug)
		stocked.items = append(stocked.items, storedRow(ContentRow{ID: newID, Type: "post", Slug: slug}))
	}
	f.shelf = stocked

	_, err := f.store(dbkit.Postgres).Update(t.Context(), retitledHello, shelvedAt, nil, 0)

	if want := "postgres: update content: " + content.ErrSlugTaken.Error(); err == nil || err.Error() != want {
		t.Fatalf("Update() error = %v, want %q", err, want)
	}
	if !errors.Is(err, content.ErrSlugTaken) {
		t.Errorf("errors.Is(%v, %v) = false, want slug taken carried", err, content.ErrSlugTaken)
	}
	if len(f.log) != slugAttempts {
		t.Errorf("log holds %d entries, want one sibling read per attempt", len(f.log))
	}
	for i, entry := range f.log {
		if read := entry.args.(SiblingSlug); read.Slug != slugs[i] {
			t.Errorf("read %d asked for %q, want %q", i, read.Slug, slugs[i])
		}
	}
}

// wrapCase is one statement failing under one method, with the text today's step wraps the failure in.
type wrapCase struct {
	entry string
	run   func(ctx context.Context, s *Store) error
	want  string
}

// wraps holds every statement whose failure a step wraps in a text of its own.
var wraps = map[string]wrapCase{
	"ByID when the item does not read": {
		"GetContent", func(ctx context.Context, s *Store) error { return errOf(s.ByID(ctx, helloID)) },
		"postgres: get content: ",
	},
	"PublishedByPath when the item does not read": {
		"GetPublishedContentByPath",
		func(ctx context.Context, s *Store) error { return errOf(s.PublishedByPath(ctx, "posts/hello")) },
		"postgres: get published content by path: ",
	},
	"Children when the count does not read": {
		"CountChildren", func(ctx context.Context, s *Store) error { return errOf(s.Children(ctx, aboutID)) },
		"postgres: count children: ",
	},
	"Depth when the depth does not read": {
		"ContentDepth", func(ctx context.Context, s *Store) error { return errOf(s.Depth(ctx, aboutID)) },
		"postgres: content depth: ",
	},
	"List when the count does not read": {
		"CountContent", func(ctx context.Context, s *Store) error { return listError(s.List(ctx, postFilter)) },
		"postgres: count content: ",
	},
	"List when the flat page does not read": {
		"ListContent", func(ctx context.Context, s *Store) error { return listError(s.List(ctx, postFilter)) },
		"postgres: list content: ",
	},
	"List when the nested page does not read": {
		"ListNestedContent",
		func(ctx context.Context, s *Store) error { return listError(s.List(ctx, nestedFilter())) },
		"postgres: list content: ",
	},
	"RelatedTo when the count does not read": {
		"CountRelatedContent",
		func(ctx context.Context, s *Store) error { return listError(s.RelatedTo(ctx, adaID, 1, 10)) },
		"postgres: count related content: ",
	},
	"RelatedTo when the page does not read": {
		"ListRelatedContent",
		func(ctx context.Context, s *Store) error { return listError(s.RelatedTo(ctx, adaID, 1, 10)) },
		"postgres: list related content: ",
	},
	"PointingAt when the count does not read": {
		"CountPointingAt",
		func(ctx context.Context, s *Store) error { return listError(s.PointingAt(ctx, adaID, 11, 1, 10)) },
		"postgres: count pointing at: ",
	},
	"PointingAt when the page does not read": {
		"PointingAt",
		func(ctx context.Context, s *Store) error { return listError(s.PointingAt(ctx, adaID, 11, 1, 10)) },
		"postgres: list pointing at: ",
	},
	"TargetsByIDs when the summaries do not read": {
		"SummariesOfTargets",
		func(ctx context.Context, s *Store) error { return errOf(s.TargetsByIDs(ctx, nil)) },
		"postgres: list target summaries: ",
	},
	"CountsByAuthor when the counts do not read": {
		"CountContentByAuthor",
		func(ctx context.Context, s *Store) error { return errOf(s.CountsByAuthor(ctx, "post")) },
		"postgres: count content by author: ",
	},
	"Delete when the item is not removed": {
		"DeleteContent", func(ctx context.Context, s *Store) error { return s.Delete(ctx, helloID) },
		"postgres: delete content: ",
	},
	"Revisions when the revisions do not read": {
		"ListRevisions", func(ctx context.Context, s *Store) error { return errOf(s.Revisions(ctx, helloID)) },
		"postgres: list revisions: ",
	},
	"RevisionByID when the revision does not read": {
		"GetRevision",
		func(ctx context.Context, s *Store) error { return errOf(s.RevisionByID(ctx, helloID, revisionID)) },
		"postgres: get revision: ",
	},
	"Autosave when the autosave does not read": {
		"GetAutosave",
		func(ctx context.Context, s *Store) error { return errOf(s.Autosave(ctx, helloID, editorID)) },
		"postgres: get autosave: ",
	},
	"DeleteAutosave when the autosave is not removed": {
		"DeleteAutosave", func(ctx context.Context, s *Store) error { return s.DeleteAutosave(ctx, helloID, editorID) },
		"postgres: delete autosave: ",
	},
	"Create when the bare item is not stored": {
		"CreateContent", func(ctx context.Context, s *Store) error { return errOf(s.Create(ctx, launch(nil))) },
		"postgres: create content: ",
	},
	"Create when the groups do not read": {
		"ListFieldGroups",
		func(ctx context.Context, s *Store) error { return errOf(s.Create(ctx, history())) },
		"postgres: update content: postgres: list field groups: ",
	},
	"Create when the fields do not read": {
		"ListContentFields",
		func(ctx context.Context, s *Store) error { return errOf(s.Create(ctx, history())) },
		"postgres: update content: postgres: list content fields: ",
	},
	"Update when the sibling slugs do not read": {
		"SiblingSlugTaken", retitleHello, "postgres: update content: read sibling slugs: ",
	},
	"Update when the stored item does not read": {
		"GetContent", retitleHello, "postgres: update content: postgres: get content: ",
	},
	"Update when the item is not written": {
		"UpdateContent", retitleHello, "postgres: update content: ",
	},
	"Update when the snapshot is not stored": {
		"CreateRevision",
		func(ctx context.Context, s *Store) error {
			return errOf(s.Update(ctx, retitledHello, shelvedAt, &helloSnapshot, 10))
		},
		"postgres: update content: postgres: create revision: ",
	},
	"Update when the revisions are not pruned": {
		"PruneRevisions",
		func(ctx context.Context, s *Store) error {
			return errOf(s.Update(ctx, retitledHello, shelvedAt, &helloSnapshot, 10))
		},
		"postgres: update content: postgres: prune revisions: ",
	},
	"Trash when the item is not marked": {
		"TrashContent", func(ctx context.Context, s *Store) error { return errOf(s.Trash(ctx, helloID, editedAt)) },
		"postgres: trash content: ",
	},
	"Restore when the term pages are not refreshed": {
		"RefreshRelationVisibility",
		func(ctx context.Context, s *Store) error { return errOf(s.Restore(ctx, contactID, editedAt)) },
		"postgres: restore content: ",
	},
	"DeleteRevision when the revision is not removed": {
		"DeleteRevision", func(ctx context.Context, s *Store) error { return s.DeleteRevision(ctx, helloID, revisionID) },
		"postgres: delete revision: ",
	},
	"SaveAutosave when the autosave is not stored": {
		"UpsertAutosave", func(ctx context.Context, s *Store) error { return errOf(s.SaveAutosave(ctx, helloAutosave)) },
		"postgres: save autosave: ",
	},
	"EmptyTrash when what is left does not read": {
		"CountTrash",
		func(ctx context.Context, s *Store) error {
			_, _, err := s.EmptyTrash(ctx, "page", nil)
			return err
		},
		"postgres: empty the trash: ",
	},
}

// retitleHello stores the post retitled and rated anew, answering only the error.
func retitleHello(ctx context.Context, s *Store) error {
	return errOf(s.Update(ctx, retitledHello, shelvedAt, nil, 0))
}

// listError returns the error a listing answered with, dropping its page and its count.
func listError[T any](_ T, _ int, err error) error {
	return err
}

func TestEachStepWrapsAFailedStatementInTodaysText(t *testing.T) {
	t.Parallel()

	for name, test := range wraps {
		for _, engine := range everyEngine {
			t.Run(name+" under "+engine.String(), func(t *testing.T) {
				t.Parallel()

				err := test.run(t.Context(), failingStore(t, engine, test.entry, errConnectionGone))

				checkWrapped(t, engine, test.want, errConnectionGone, err)
			})
		}
	}
}
