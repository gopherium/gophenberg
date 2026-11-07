// SPDX-License-Identifier: Apache-2.0

package typestore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// errConnectionGone is the failure a statement or a transaction answers with when the database drops.
var errConnectionGone = errors.New("the connection is gone")

// errCheckRaised is what the caller's check raises under the lock.
var errCheckRaised = errors.New("the check refused the edit")

// everyEngine lists the engines each text is checked under.
var everyEngine = []dbkit.Engine{dbkit.Postgres, dbkit.SQLite}

// brokeRule is a statement failure carrying the code and the name of the stored rule it broke.
type brokeRule struct{ code, name string }

// Error returns the code and the name of the broken rule.
func (b brokeRule) Error() string { return b.code + " on " + b.name }

// struckIn is a statement failure carrying a code and the context the engine struck it in.
type struckIn struct{ code, context string }

// Error returns the code and the context of the failure.
func (s struckIn) Error() string { return s.code + " " + s.context }

// readBrokenRule returns the code, the name and the context a test failure carries, and whether err is one.
func readBrokenRule(err error) (Failure, bool) {
	var broke brokeRule
	if errors.As(err, &broke) {
		return Failure{Code: broke.code, Name: broke.name}, true
	}
	var struck struckIn
	if errors.As(err, &struck) {
		return Failure{Code: struck.code, Context: struck.context}, true
	}
	return Failure{}, false
}

// storeReading returns a store matching the rules against what readBrokenRule reads.
func storeReading(rules []Rule) *Store {
	return New(Backend{Engine: dbkit.Postgres, Rules: rules, ReadError: readBrokenRule})
}

// orderedRules lends a code on any name first, then a name on any code, then code and name pairs.
var orderedRules = []Rule{
	{Code: "restrict", Broken: TypeHeldByContent},
	{Name: "content path", Broken: ContentPathUnique},
	{Code: "unique", Name: "type key", Broken: TypeKeyUnique},
	{Code: "unique", Name: "route word", Broken: RouteWordUnique},
	{Code: "unique", Name: "route word", Broken: GroupKeyUnique},
	{Code: "reference", Name: "field target", Broken: FieldTargetExists},
}

// catchAll lends one rule naming neither a code nor a name.
var catchAll = []Rule{{Broken: FieldGroupExists}}

func TestBrokenTakesTheFirstRuleMatchingTheCodeAndTheName(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		err  error
		want Constraint
	}{
		"a code that comes first over a later name": {brokeRule{"restrict", "content path"}, TypeHeldByContent},
		"a code on a name no rule holds":            {brokeRule{"restrict", "anything"}, TypeHeldByContent},
		"a name under a code no rule holds":         {brokeRule{"reference", "content path"}, ContentPathUnique},
		"a name under a code a later pair holds":    {brokeRule{"unique", "content path"}, ContentPathUnique},
		"a pair":                                    {brokeRule{"unique", "type key"}, TypeKeyUnique},
		"a pair two rules hold":                     {brokeRule{"unique", "route word"}, RouteWordUnique},
		"another pair":                              {brokeRule{"reference", "field target"}, FieldTargetExists},
		"a pair carried inside a wrap": {
			fmt.Errorf("create: %w", brokeRule{"unique", "type key"}), TypeKeyUnique,
		},
		"a known name under another code": {brokeRule{"reference", "type key"}, NoConstraint},
		"a known code on an unknown name": {brokeRule{"unique", "slug"}, NoConstraint},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := storeReading(orderedRules).broken(test.err); got != test.want {
				t.Errorf("broken(%v) = %d, want %d", test.err, got, test.want)
			}
		})
	}
}

func TestBrokenMatchesAnyFoundFailureUnderARuleNamingNeitherCodeNorName(t *testing.T) {
	t.Parallel()

	for name, err := range map[string]error{
		"a code and a name":         brokeRule{"unique", "type key"},
		"neither a code nor a name": brokeRule{},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := storeReading(catchAll).broken(err); got != FieldGroupExists {
				t.Errorf("broken(%v) = %d, want %d", err, got, FieldGroupExists)
			}
		})
	}
}

func TestBrokenNamesNoRuleWhenTheReaderFindsNone(t *testing.T) {
	t.Parallel()

	if got := storeReading(catchAll).broken(errConnectionGone); got != NoConstraint {
		t.Errorf("broken(%v) = %d, want %d", errConnectionGone, got, NoConstraint)
	}
}

func TestBrokenNamesNoRuleForNoError(t *testing.T) {
	t.Parallel()

	if got := storeReading(catchAll).broken(nil); got != NoConstraint {
		t.Errorf("broken(nil) = %d, want %d", got, NoConstraint)
	}
}

func TestBrokenNamesNoRuleForANameReadWithoutACode(t *testing.T) {
	t.Parallel()

	pair := []Rule{{Code: "unique", Name: "type key", Broken: TypeKeyUnique}}
	uncoded := brokeRule{"", "type key"}

	if got := storeReading(pair).broken(uncoded); got != NoConstraint {
		t.Errorf("broken(%v) = %d, want %d", uncoded, got, NoConstraint)
	}
}

func TestBrokenNamesNoRuleWhenTheEngineLendsNone(t *testing.T) {
	t.Parallel()

	if got := storeReading(nil).broken(brokeRule{"unique", "type key"}); got != NoConstraint {
		t.Errorf("broken() = %d, want %d", got, NoConstraint)
	}
}

// readAsTypeKey finds a taken type key in every error, as an engine naming one rule for each failure would.
func readAsTypeKey(error) (Failure, bool) {
	return Failure{Code: "unique", Name: "type key"}, true
}

// contextRules lends a code with a text its context must hold first, then the same code in any context.
var contextRules = []Rule{
	{Code: "deadlock", Context: "content path", Broken: AddressCheckDeadlocked},
	{Code: "deadlock", Broken: TypeHeldByContent},
}

func TestBrokenMatchesWhatTheReaderItIsHandedFindsAgainstTheRulesItIsHanded(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		rules []Rule
		read  func(err error) (Failure, bool)
		err   error
		want  Constraint
	}{
		"a context holding the rule's text": {
			contextRules, readBrokenRule, struckIn{"deadlock", "while checking the content path index"},
			AddressCheckDeadlocked,
		},
		"a context that is the rule's text": {
			contextRules, readBrokenRule, struckIn{"deadlock", "content path"}, AddressCheckDeadlocked,
		},
		"a context missing the rule's text": {
			contextRules, readBrokenRule, struckIn{"deadlock", "while locking a parent row"}, TypeHeldByContent,
		},
		"no context at all": {contextRules, readBrokenRule, struckIn{"deadlock", ""}, TypeHeldByContent},
		"a context holding the text under another code": {
			contextRules, readBrokenRule, struckIn{"unique", "while checking the content path index"}, NoConstraint,
		},
		"the first of two rules matching": {
			orderedRules, readBrokenRule, brokeRule{"restrict", "content path"}, TypeHeldByContent,
		},
		"a name under any code": {orderedRules, readBrokenRule, brokeRule{"reference", "content path"}, ContentPathUnique},
		"a code and a name":     {orderedRules, readBrokenRule, brokeRule{"unique", "type key"}, TypeKeyUnique},
		"a rule naming neither": {catchAll, readBrokenRule, brokeRule{}, FieldGroupExists},
		"a failure carried inside a wrap": {
			orderedRules, readBrokenRule, fmt.Errorf("create: %w", brokeRule{"unique", "route word"}), RouteWordUnique,
		},
		"a failure no rule names":              {orderedRules, readBrokenRule, brokeRule{"unique", "slug"}, NoConstraint},
		"an error the reader finds nothing in": {catchAll, readBrokenRule, errConnectionGone, NoConstraint},
		"no error":                             {catchAll, readBrokenRule, nil, NoConstraint},
		"no rules":                             {nil, readBrokenRule, brokeRule{"unique", "type key"}, NoConstraint},
		"a reader finding a rule in any error": {orderedRules, readAsTypeKey, errConnectionGone, TypeKeyUnique},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := Broken(test.rules, test.read, test.err); got != test.want {
				t.Errorf("Broken(%v) = %d, want %d", test.err, got, test.want)
			}
		})
	}
}

// tailRules lends one rule per constraint, each under its own code and name.
var tailRules = []Rule{
	{Code: "unique", Name: "group key", Broken: GroupKeyUnique},
	{Code: "unique", Name: "field key", Broken: FieldKeyUnique},
	{Code: "reference", Name: "field target", Broken: FieldTargetExists},
	{Code: "reference", Name: "field group", Broken: FieldGroupExists},
	{Code: "unique", Name: "type key", Broken: TypeKeyUnique},
	{Code: "unique", Name: "route word", Broken: RouteWordUnique},
	{Code: "restrict", Name: "held type", Broken: TypeHeldByContent},
	{Code: "unique", Name: "content path", Broken: ContentPathUnique},
}

// transactionStep names the transaction itself as the step a stub fails.
const transactionStep = "transaction"

// stubQueries answers the statements it serves from the rows it holds, failing the one it names with its failure.
type stubQueries struct {
	Queries
	failing string
	failure error
	groups  []GroupRow
	fields  []FieldRow
}

// stubbedStore returns a store on the engine sending every statement to the stub, in a transaction or not.
func stubbedStore(engine dbkit.Engine, stub stubQueries) *Store {
	return New(Backend{
		Engine:    engine,
		Pool:      stub,
		InTx:      stub.inTx,
		Rules:     tailRules,
		ReadError: readBrokenRule,
	})
}

// failedTransaction returns a store on the engine whose every transaction answers with the given error.
func failedTransaction(engine dbkit.Engine, given error) *Store {
	return stubbedStore(engine, stubQueries{failing: transactionStep, failure: given})
}

// fails returns the stub's failure when the step is the one it names.
func (q stubQueries) fails(step string) error {
	if step == q.failing {
		return q.failure
	}
	return nil
}

// inTx runs the write on the stub, or answers with the failure when the transaction is the step it names.
func (q stubQueries) inTx(_ context.Context, write func(Queries) error) error {
	if err := q.fails(transactionStep); err != nil {
		return err
	}
	return write(q)
}

// ListContentTypes answers with no types.
func (q stubQueries) ListContentTypes(context.Context) ([]TypeRow, error) {
	return nil, q.fails("ListContentTypes")
}

// GetContentType answers with an empty type.
func (q stubQueries) GetContentType(context.Context, string) (TypeRow, error) {
	return TypeRow{}, q.fails("GetContentType")
}

// CreateContentType answers with the type it was given.
func (q stubQueries) CreateContentType(_ context.Context, t TypeRow) (TypeRow, error) {
	return t, q.fails("CreateContentType")
}

// LockContentType answers with an empty type.
func (q stubQueries) LockContentType(context.Context, string) (TypeRow, error) {
	return TypeRow{}, q.fails("LockContentType")
}

// CountNestedContent answers with no nested items.
func (q stubQueries) CountNestedContent(context.Context, string) (int64, error) {
	return 0, q.fails("CountNestedContent")
}

// DeleteContentType answers with one type gone.
func (q stubQueries) DeleteContentType(context.Context, string) (int64, error) {
	return 1, q.fails("DeleteContentType")
}

// AdoptContentType answers with one type adopted.
func (q stubQueries) AdoptContentType(context.Context, string, time.Time) (int64, error) {
	return 1, q.fails("AdoptContentType")
}

// TypeKeys answers with no keys.
func (q stubQueries) TypeKeys(context.Context) ([]string, error) {
	return nil, q.fails("TypeKeys")
}

// LockFieldGroups answers with the lock taken.
func (q stubQueries) LockFieldGroups(context.Context) error {
	return q.fails("LockFieldGroups")
}

// ListFieldGroups answers with the groups the stub holds.
func (q stubQueries) ListFieldGroups(context.Context) ([]GroupRow, error) {
	return q.groups, q.fails("ListFieldGroups")
}

// FieldGroupKeys answers with no keys.
func (q stubQueries) FieldGroupKeys(context.Context) ([]string, error) {
	return nil, q.fails("FieldGroupKeys")
}

// CreateFieldGroup answers with the group it was given.
func (q stubQueries) CreateFieldGroup(_ context.Context, g GroupRow) (GroupRow, error) {
	return g, q.fails("CreateFieldGroup")
}

// UpdateFieldGroup answers with the group it was given.
func (q stubQueries) UpdateFieldGroup(_ context.Context, g GroupRow) (GroupRow, error) {
	return g, q.fails("UpdateFieldGroup")
}

// ReorderFieldGroups answers with the order stored.
func (q stubQueries) ReorderFieldGroups(context.Context, []int) error {
	return q.fails("ReorderFieldGroups")
}

// CarryStrayFieldsOfGroup answers with the strays carried.
func (q stubQueries) CarryStrayFieldsOfGroup(context.Context, int) error {
	return q.fails("CarryStrayFieldsOfGroup")
}

// AdoptFieldGroup answers with one group adopted.
func (q stubQueries) AdoptFieldGroup(context.Context, string, time.Time) (int64, error) {
	return 1, q.fails("AdoptFieldGroup")
}

// AdoptFieldsInGroup answers with the fields adopted.
func (q stubQueries) AdoptFieldsInGroup(context.Context, string, time.Time) error {
	return q.fails("AdoptFieldsInGroup")
}

// ListContentFields answers with the fields the stub holds.
func (q stubQueries) ListContentFields(context.Context) ([]FieldRow, error) {
	return q.fields, q.fails("ListContentFields")
}

// UpdateContentField answers with the field it was given.
func (q stubQueries) UpdateContentField(_ context.Context, f FieldRow, _ time.Time) (FieldRow, error) {
	return f, q.fails("UpdateContentField")
}

// UpdateSubContentField answers with the field it was given.
func (q stubQueries) UpdateSubContentField(_ context.Context, f FieldRow, _ time.Time) (FieldRow, error) {
	return f, q.fails("UpdateSubContentField")
}

// ReorderContentFields answers with one field moved.
func (q stubQueries) ReorderContentFields(context.Context, int, []string) (int64, error) {
	return 1, q.fails("ReorderContentFields")
}

// ReorderSubContentFields answers with one field moved.
func (q stubQueries) ReorderSubContentFields(context.Context, int, []string) (int64, error) {
	return 1, q.fails("ReorderSubContentFields")
}

// DeleteFieldByID answers with one field gone.
func (q stubQueries) DeleteFieldByID(context.Context, int) (int64, error) {
	return 1, q.fails("DeleteFieldByID")
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

// fieldWriteMeanings holds the error each rule a field write breaks stands for.
var fieldWriteMeanings = map[Constraint]error{
	FieldKeyUnique:    content.ErrFieldTaken,
	FieldTargetExists: content.ErrTargetUnknown,
	FieldGroupExists:  content.ErrGroupNotFound,
}

// typeUpdateMeanings holds the error each rule a type update breaks stands for.
var typeUpdateMeanings = map[Constraint]error{
	TypeKeyUnique:     content.ErrTypeTaken,
	RouteWordUnique:   content.ErrRouteWordTaken,
	ContentPathUnique: content.ErrSlugTaken,
}

// typeUpdatePasses lists what a type update passes unchanged.
var typeUpdatePasses = []error{
	content.ErrTypeNotFound, content.ErrRouteWordReserved, content.ErrInvalidRouteWord, content.ErrNestingInUse,
}

// movePasses lists what a field move passes unchanged.
var movePasses = []error{
	content.ErrFieldNotFound, content.ErrGroupNotFound, content.ErrFieldTaken,
	content.ErrFieldInsideItself, content.ErrFieldTooDeep,
}

// fieldDeletePasses lists what a field delete passes unchanged.
var fieldDeletePasses = []error{content.ErrGroupNotFound, content.ErrFieldNotFound}

// everySentinel lists each sentinel some tail passes unchanged today.
var everySentinel = []error{
	content.ErrGroupNotFound, content.ErrFieldNotFound, content.ErrFieldTaken, content.ErrFieldInsideItself,
	content.ErrFieldTooDeep, content.ErrFieldShape, content.ErrTypeNotFound, content.ErrRouteWordReserved,
	content.ErrInvalidRouteWord, content.ErrNestingInUse,
}

// tailCase is one step's last word on an error: the rules it maps, what it passes and the text it wraps the rest in.
type tailCase struct {
	run    func(ctx context.Context, engine dbkit.Engine, given error) error
	table  map[Constraint]error
	passes []error
	check  bool
	wrap   string
}

// tails holds every error tail of the type store, as each step answers today.
var tails = map[string]tailCase{
	"groupWriteFailure": {
		run: func(_ context.Context, engine dbkit.Engine, given error) error {
			return stubbedStore(engine, stubQueries{}).groupWriteFailure(given)
		},
		table: map[Constraint]error{GroupKeyUnique: content.ErrGroupKeyTaken},
		wrap:  "postgres: create field group: ",
	},
	"updateGroupFailure": {
		run: func(_ context.Context, engine dbkit.Engine, given error) error {
			return stubbedStore(engine, stubQueries{}).updateGroupFailure(given)
		},
		passes: []error{content.ErrGroupNotFound, content.ErrFieldTaken},
		check:  true,
		wrap:   "postgres: update field group: ",
	},
	"fieldWriteFailure": {
		run: func(_ context.Context, engine dbkit.Engine, given error) error {
			return stubbedStore(engine, stubQueries{}).fieldWriteFailure(given)
		},
		table: fieldWriteMeanings,
		wrap:  "postgres: create content field: ",
	},
	"updateTypeFailure": {
		run: func(_ context.Context, engine dbkit.Engine, given error) error {
			return stubbedStore(engine, stubQueries{}).updateTypeFailure(given)
		},
		table:  typeUpdateMeanings,
		passes: typeUpdatePasses,
		wrap:   "postgres: update content type: ",
	},
	"moveFailure": {
		run: func(_ context.Context, engine dbkit.Engine, given error) error {
			return stubbedStore(engine, stubQueries{}).moveFailure(given)
		},
		table:  fieldWriteMeanings,
		passes: movePasses,
		check:  true,
		wrap:   "postgres: create content field: ",
	},
	"Create": {
		run: func(ctx context.Context, engine dbkit.Engine, given error) error {
			stub := stubQueries{failing: "CreateContentType", failure: given}
			return errOf(stubbedStore(engine, stub).Create(ctx, content.Type{Key: "book"}))
		},
		table: map[Constraint]error{TypeKeyUnique: content.ErrTypeTaken, RouteWordUnique: content.ErrRouteWordTaken},
		wrap:  "postgres: create content type: ",
	},
	"Delete": {
		run: func(ctx context.Context, engine dbkit.Engine, given error) error {
			stub := stubQueries{failing: "DeleteContentType", failure: given}
			return stubbedStore(engine, stub).Delete(ctx, "book")
		},
		table: map[Constraint]error{TypeHeldByContent: content.ErrTypeInUse, FieldTargetExists: content.ErrTypeTargeted},
		wrap:  "postgres: delete content type: ",
	},
	"Update": {
		run: func(ctx context.Context, engine dbkit.Engine, given error) error {
			return errOf(failedTransaction(engine, given).Update(ctx, content.Type{Key: "book"}))
		},
		table:  typeUpdateMeanings,
		passes: typeUpdatePasses,
		wrap:   "postgres: update content type: ",
	},
	"UpdateGroup": {
		run: func(ctx context.Context, engine dbkit.Engine, given error) error {
			return errOf(failedTransaction(engine, given).UpdateGroup(ctx, content.Group{ID: 1}, nil, nil))
		},
		passes: []error{content.ErrGroupNotFound, content.ErrFieldTaken},
		check:  true,
		wrap:   "postgres: update field group: ",
	},
	"MoveField": {
		run: func(ctx context.Context, engine dbkit.Engine, given error) error {
			return errOf(failedTransaction(engine, given).MoveField(ctx, 5, 1, 0, 3, nil))
		},
		table:  fieldWriteMeanings,
		passes: movePasses,
		check:  true,
		wrap:   "postgres: create content field: ",
	},
	"DeleteGroup": {
		run: func(ctx context.Context, engine dbkit.Engine, given error) error {
			return failedTransaction(engine, given).DeleteGroup(ctx, 1, nil)
		},
		passes: []error{content.ErrGroupNotFound},
		check:  true,
		wrap:   "postgres: delete field group: ",
	},
	"DeleteFieldsOfGroup": {
		run: func(ctx context.Context, engine dbkit.Engine, given error) error {
			return failedTransaction(engine, given).DeleteFieldsOfGroup(ctx, 1, []string{"rating"}, nil)
		},
		passes: fieldDeletePasses,
		check:  true,
		wrap:   "postgres: delete fields of group: ",
	},
	"DeleteFieldInGroup": {
		run: func(ctx context.Context, engine dbkit.Engine, given error) error {
			return failedTransaction(engine, given).DeleteFieldInGroup(ctx, 1, "rating", nil)
		},
		passes: fieldDeletePasses,
		check:  true,
		wrap:   "postgres: delete content field: ",
	},
	"DeleteSubField": {
		run: func(ctx context.Context, engine dbkit.Engine, given error) error {
			return failedTransaction(engine, given).DeleteSubField(ctx, 5, nil)
		},
		passes: []error{content.ErrFieldNotFound},
		check:  true,
		wrap:   "postgres: delete sub field: ",
	},
	"CreateFieldInGroup": {
		run: func(ctx context.Context, engine dbkit.Engine, given error) error {
			return errOf(failedTransaction(engine, given).CreateFieldInGroup(ctx, 1, content.Field{Key: "rating"}, nil))
		},
		table:  fieldWriteMeanings,
		passes: []error{content.ErrGroupNotFound, content.ErrFieldTaken},
		check:  true,
		wrap:   "postgres: create content field: ",
	},
	"CreateSubField": {
		run: func(ctx context.Context, engine dbkit.Engine, given error) error {
			return errOf(failedTransaction(engine, given).CreateSubField(ctx, 4, content.Field{Key: "street"}, 3))
		},
		table:  fieldWriteMeanings,
		passes: []error{content.ErrFieldNotFound, content.ErrFieldShape, content.ErrFieldTooDeep},
		wrap:   "postgres: create content field: ",
	},
}

func TestEachTailGivesTheErrorItsTableHoldsAndWrapsEveryOtherBrokenRule(t *testing.T) {
	t.Parallel()

	for name, test := range tails {
		for _, engine := range everyEngine {
			t.Run(name+" under "+engine.String(), func(t *testing.T) {
				t.Parallel()

				for _, rule := range tailRules {
					given := brokeRule{rule.Code, rule.Name}
					got := test.run(t.Context(), engine, given)
					want, mapped := test.table[rule.Broken]
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

				given := brokeRule{"unique", "slug"}

				checkWrapped(t, engine, test.wrap, given, test.run(t.Context(), engine, given))
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

				for _, sentinel := range everySentinel {
					got := test.run(t.Context(), engine, sentinel)
					if !slices.Contains(test.passes, sentinel) {
						checkWrapped(t, engine, test.wrap, sentinel, got)
						continue
					}
					if got != sentinel {
						t.Errorf("tail(%v) = %v, want it passed unchanged", sentinel, got)
					}
				}
			})
		}
	}
}

func TestEachTailPassesACheckErrorOnlyWhereItDoesToday(t *testing.T) {
	t.Parallel()

	for name, test := range tails {
		for _, engine := range everyEngine {
			t.Run(name+" under "+engine.String(), func(t *testing.T) {
				t.Parallel()

				given := recheckError{errCheckRaised}
				got := test.run(t.Context(), engine, given)

				if !test.check {
					checkWrapped(t, engine, test.wrap, given, got)
					return
				}
				if got != given {
					t.Errorf("tail(%v) = %v, want the check error passed unchanged", given, got)
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

				got := test.run(t.Context(), engine, errConnectionGone)
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

func TestEachStepWrapsAFailedStatementInTodaysText(t *testing.T) {
	t.Parallel()

	heldGroup := GroupRow{ID: 1, Key: "details", Title: "Details", Location: []byte("[]"), Active: true}
	heldField := FieldRow{ID: 5, GroupID: 1, Key: "rating", Kind: "number", Settings: []byte("{}")}
	passing := func([]content.Group, []content.Type) error { return nil }
	gone := ": " + errConnectionGone.Error()

	for name, test := range map[string]struct {
		stub stubQueries
		run  func(ctx context.Context, s *Store) error
		want string
	}{
		"List when the types do not read": {
			stubQueries{failing: "ListContentTypes"},
			func(ctx context.Context, s *Store) error { return errOf(s.List(ctx)) },
			"postgres: list content types" + gone,
		},
		"ByKey when the type does not read": {
			stubQueries{failing: "GetContentType"},
			func(ctx context.Context, s *Store) error { return errOf(s.ByKey(ctx, "book")) },
			"postgres: get content type" + gone,
		},
		"Create when the type is not stored": {
			stubQueries{failing: "CreateContentType"},
			func(ctx context.Context, s *Store) error { return errOf(s.Create(ctx, content.Type{Key: "book"})) },
			"postgres: create content type" + gone,
		},
		"Delete when the type is not removed": {
			stubQueries{failing: "DeleteContentType"},
			func(ctx context.Context, s *Store) error { return s.Delete(ctx, "book") },
			"postgres: delete content type" + gone,
		},
		"Nested when the count does not read": {
			stubQueries{failing: "CountNestedContent"},
			func(ctx context.Context, s *Store) error { return errOf(s.Nested(ctx, "book")) },
			"postgres: count nested content" + gone,
		},
		"Update when the type does not lock": {
			stubQueries{failing: "LockContentType"},
			func(ctx context.Context, s *Store) error { return errOf(s.Update(ctx, content.Type{Key: "book"})) },
			"postgres: update content type" + gone,
		},
		"AdoptType when the type is not adopted": {
			stubQueries{failing: "AdoptContentType"},
			func(ctx context.Context, s *Store) error { return s.AdoptType(ctx, "book") },
			"postgres: adopt content type" + gone,
		},
		"AdoptGroup when the group is not adopted": {
			stubQueries{failing: "AdoptFieldGroup"},
			func(ctx context.Context, s *Store) error { return s.AdoptGroup(ctx, "details") },
			"postgres: adopt field group" + gone,
		},
		"AdoptGroup when its fields are not adopted": {
			stubQueries{failing: "AdoptFieldsInGroup"},
			func(ctx context.Context, s *Store) error { return s.AdoptGroup(ctx, "details") },
			"postgres: adopt fields in group" + gone,
		},
		"ListGroups when the groups do not read": {
			stubQueries{failing: "ListFieldGroups"},
			func(ctx context.Context, s *Store) error { return errOf(s.ListGroups(ctx)) },
			"postgres: list field groups" + gone,
		},
		"ListGroups when the fields do not read": {
			stubQueries{failing: "ListContentFields"},
			func(ctx context.Context, s *Store) error { return errOf(s.ListGroups(ctx)) },
			"postgres: list content fields" + gone,
		},
		"ReorderGroups when the order is not stored": {
			stubQueries{failing: "ReorderFieldGroups"},
			func(ctx context.Context, s *Store) error { return s.ReorderGroups(ctx, []int{2, 1}) },
			"postgres: reorder field groups" + gone,
		},
		"CreateGroup when the lock is not taken": {
			stubQueries{failing: "LockFieldGroups"},
			func(ctx context.Context, s *Store) error {
				return errOf(s.CreateGroup(ctx, content.Group{Key: "details", Title: "Details"}))
			},
			"postgres: lock field groups" + gone,
		},
		"CreateGroup when the keys held do not read": {
			stubQueries{failing: "FieldGroupKeys"},
			func(ctx context.Context, s *Store) error {
				return errOf(s.CreateGroup(ctx, content.Group{Title: "Details"}))
			},
			"postgres: list field group keys" + gone,
		},
		"CreateGroup when the group is not stored": {
			stubQueries{failing: "CreateFieldGroup"},
			func(ctx context.Context, s *Store) error {
				return errOf(s.CreateGroup(ctx, content.Group{Key: "details", Title: "Details"}))
			},
			"postgres: create field group" + gone,
		},
		"UpdateGroup when the lock is not taken": {
			stubQueries{failing: "LockFieldGroups"},
			func(ctx context.Context, s *Store) error {
				return errOf(s.UpdateGroup(ctx, content.Group{ID: 1, Title: "Details"}, nil, nil))
			},
			"postgres: update field group: postgres: lock field groups" + gone,
		},
		"UpdateGroup when the group is not stored": {
			stubQueries{failing: "UpdateFieldGroup"},
			func(ctx context.Context, s *Store) error {
				return errOf(s.UpdateGroup(ctx, content.Group{ID: 1, Title: "Details"}, nil, nil))
			},
			"postgres: update field group" + gone,
		},
		"UpdateGroup when a field pointed anew is not stored": {
			stubQueries{failing: "UpdateContentField"},
			func(ctx context.Context, s *Store) error {
				repointed := []content.Field{{Key: "rating"}}
				return errOf(s.UpdateGroup(ctx, content.Group{ID: 1, Title: "Details"}, repointed, nil))
			},
			"postgres: update field group" + gone,
		},
		"DeleteGroup when the lock is not taken": {
			stubQueries{failing: "LockFieldGroups"},
			func(ctx context.Context, s *Store) error { return s.DeleteGroup(ctx, 1, nil) },
			"postgres: delete field group" + gone,
		},
		"DeleteGroup when its strays are not carried": {
			stubQueries{failing: "CarryStrayFieldsOfGroup", groups: []GroupRow{heldGroup}},
			func(ctx context.Context, s *Store) error { return s.DeleteGroup(ctx, 1, nil) },
			"postgres: delete field group" + gone,
		},
		"DeleteFieldsOfGroup when the lock is not taken": {
			stubQueries{failing: "LockFieldGroups"},
			func(ctx context.Context, s *Store) error {
				return s.DeleteFieldsOfGroup(ctx, 1, []string{"rating"}, nil)
			},
			"postgres: delete fields of group" + gone,
		},
		"DeleteFieldInGroup when the lock is not taken": {
			stubQueries{failing: "LockFieldGroups"},
			func(ctx context.Context, s *Store) error { return s.DeleteFieldInGroup(ctx, 1, "rating", nil) },
			"postgres: delete content field" + gone,
		},
		"DeleteSubField when the lock is not taken": {
			stubQueries{failing: "LockFieldGroups"},
			func(ctx context.Context, s *Store) error { return s.DeleteSubField(ctx, 5, nil) },
			"postgres: delete sub field" + gone,
		},
		"DeleteSubField when the field is not removed": {
			stubQueries{failing: "DeleteFieldByID", groups: []GroupRow{heldGroup}, fields: []FieldRow{heldField}},
			func(ctx context.Context, s *Store) error { return s.DeleteSubField(ctx, 5, nil) },
			"postgres: delete sub field" + gone,
		},
		"UpdateFieldInGroup when the lock is not taken": {
			stubQueries{failing: "LockFieldGroups"},
			func(ctx context.Context, s *Store) error {
				return errOf(s.UpdateFieldInGroup(ctx, 1, content.Field{Key: "rating"}, time.Time{}, nil))
			},
			"postgres: lock field groups" + gone,
		},
		"UpdateFieldInGroup when the type keys for the check do not read": {
			stubQueries{failing: "TypeKeys"},
			func(ctx context.Context, s *Store) error {
				return errOf(s.UpdateFieldInGroup(ctx, 1, content.Field{Key: "rating"}, time.Time{}, passing))
			},
			"postgres: list type keys" + gone,
		},
		"UpdateFieldInGroup when the field is not stored": {
			stubQueries{failing: "UpdateContentField"},
			func(ctx context.Context, s *Store) error {
				return errOf(s.UpdateFieldInGroup(ctx, 1, content.Field{Key: "rating"}, time.Time{}, nil))
			},
			"postgres: update content field" + gone,
		},
		"UpdateSubField when the field is not stored": {
			stubQueries{failing: "UpdateSubContentField"},
			func(ctx context.Context, s *Store) error {
				return errOf(s.UpdateSubField(ctx, 5, content.Field{Key: "rating"}, time.Time{}))
			},
			"postgres: update sub field" + gone,
		},
		"ReorderSubFields when the order is not stored": {
			stubQueries{failing: "ReorderSubContentFields"},
			func(ctx context.Context, s *Store) error { return s.ReorderSubFields(ctx, 5, []string{"rating"}) },
			"postgres: reorder sub fields" + gone,
		},
		"ReorderFieldsInGroup when the order is not stored": {
			stubQueries{failing: "ReorderContentFields"},
			func(ctx context.Context, s *Store) error { return s.ReorderFieldsInGroup(ctx, 1, []string{"rating"}) },
			"postgres: reorder content fields" + gone,
		},
		"MoveField when the lock is not taken": {
			stubQueries{failing: "LockFieldGroups"},
			func(ctx context.Context, s *Store) error { return errOf(s.MoveField(ctx, 5, 1, 0, 3, nil)) },
			"postgres: create content field: postgres: lock field groups" + gone,
		},
		"CreateSubField when the lock is not taken": {
			stubQueries{failing: "LockFieldGroups"},
			func(ctx context.Context, s *Store) error {
				return errOf(s.CreateSubField(ctx, 4, content.Field{Key: "street"}, 3))
			},
			"postgres: create content field: postgres: lock field groups" + gone,
		},
		"CreateFieldInGroup when the lock is not taken": {
			stubQueries{failing: "LockFieldGroups"},
			func(ctx context.Context, s *Store) error {
				return errOf(s.CreateFieldInGroup(ctx, 1, content.Field{Key: "rating"}, nil))
			},
			"postgres: create content field: postgres: lock field groups" + gone,
		},
	} {
		for _, engine := range everyEngine {
			t.Run(name+" under "+engine.String(), func(t *testing.T) {
				t.Parallel()

				stub := test.stub
				stub.failure = errConnectionGone

				err := test.run(t.Context(), stubbedStore(engine, stub))

				if want := underEngine(engine, test.want); err == nil || err.Error() != want {
					t.Errorf("error = %v, want %q", err, want)
				}
				if !errors.Is(err, errConnectionGone) {
					t.Errorf("errors.Is(%v, %v) = false, want the failure carried", err, errConnectionGone)
				}
			})
		}
	}
}
