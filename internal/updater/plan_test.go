// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/updater"
)

// planAnswer returns what Plan answers for job, applying a plan that passed so its database connection goes back.
func planAnswer(t *testing.T, job updater.Job) error {
	t.Helper()
	planned, err := job.Plan(t.Context())
	if err == nil {
		_, _ = planned.Apply(t.Context())
	}
	return err
}

func TestPlanHoldsTheUpdateLockAndChangesNothingUntilTheJobApplies(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	before := tree(t, f.layout.Root)

	planned, err := f.job().Plan(t.Context())

	if err != nil {
		t.Fatalf("Plan() = %v, want every check passed", err)
	}
	if after := tree(t, f.layout.Root); !reflect.DeepEqual(after, before) {
		t.Errorf("the install root changed to %v before the job applied, want it left as %v", after, before)
	}
	if _, err := updater.Lock(t.Context(), f.db); !errors.Is(err, updater.ErrUpdateRuns) {
		t.Errorf("Lock() while the job is planned = %v, want %v", err, updater.ErrUpdateRuns)
	}
	state, err := planned.Apply(t.Context())
	if err != nil || state.Step != updater.StepSwitched || state.To != offered {
		t.Fatalf("Apply() = %+v, %v, want the update to %s switched", state, err, offered)
	}
	locked(t, f.db)()
}

func TestPlanLetsGoOfTheUpdateLockWhenACheckRefuses(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	job := f.job()
	job.Stamp = ""

	err := planAnswer(t, job)

	wantRefused(t, err, "updater: this is a custom build of Gophenberg 0.20.0")
	locked(t, f.db)()
}

// markCase is a plan that stops before the download: the change to the fixture or the job, and whether it refuses.
type markCase struct {
	change  func(*jobFixture, *updater.Job)
	refused bool
}

// errUnread is what a setting store that cannot be read answers.
var errUnread = errors.New("the setting store cannot be read")

// unreadStore is a setting store whose every read fails.
type unreadStore struct {
	*memoryStore
}

// Lookup fails with errUnread.
func (unreadStore) Lookup(context.Context, string) (string, bool, error) {
	return "", false, errUnread
}

func TestPlanMarksWhatTheChecksRefuseAndLeavesWhatFailedUnmarked(t *testing.T) {
	t.Parallel()

	cases := map[string]markCase{}
	for name, refused := range planCases {
		cases[name] = markCase{change: refused.change, refused: true}
	}
	cases["a state file it cannot read"] = markCase{change: planCases["a state file it cannot read"].change}
	cases["a database it cannot read"] = markCase{change: func(f *jobFixture, _ *updater.Job) { _ = f.db.Close() }}
	cases["a setting store it cannot read"] = markCase{
		change: func(f *jobFixture, job *updater.Job) { job.Store = unreadStore{f.store} },
	}
	cases["a version the update site does not offer"] = markCase{
		change: func(_ *jobFixture, job *updater.Job) { job.To = "9.9.9" }, refused: true,
	}
	cases["a version that failed its start check"] = markCase{
		change: func(f *jobFixture, job *updater.Job) {
			writeState(f.t, f.layout, updater.State{Refused: []string{offered}})
			job.To = offered
		},
		refused: true,
	}
	for name, marked := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newJobFixture(t)
			job := f.job()
			marked.change(f, &job)

			err := planAnswer(t, job)

			if err == nil || errors.Is(err, updater.ErrRefused) != marked.refused {
				t.Errorf("Plan() = %v, want it marked as refused %t", err, marked.refused)
			}
		})
	}
}

func TestReasonOfNamesWhyThePlanRefusedAnInstallThatCannotUpdateItself(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]updater.Reason{
		"a custom build":       updater.CustomBuild,
		"no install root":      updater.NoInstallRoot,
		"a newer PostgreSQL":   "",
		"too little free disk": "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newJobFixture(t)
			job := f.job()
			planCases[name].change(f, &job)

			err := planAnswer(t, job)

			if got := updater.ReasonOf(err); err == nil || got != want {
				t.Errorf("ReasonOf(%v) = %q, want %q", err, got, want)
			}
		})
	}
}

func TestReasonOfNamesNoReasonForNoError(t *testing.T) {
	t.Parallel()

	if got := updater.ReasonOf(nil); got != "" {
		t.Errorf("ReasonOf(nil) = %q, want none", got)
	}
}
