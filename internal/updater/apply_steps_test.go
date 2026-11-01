// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/updater"
)

// stateChanged is what update:apply says once another command wrote the update state while it ran.
const stateChanged = "updater: another command wrote the update state while update:apply ran, so update:apply " +
	"stopped and left that state as it is"

// committed is the state an earlier update to 0.20.0 left once its first start passed its check.
var committed = updater.State{Step: "committed", From: "0.19.0", To: running, Refused: []string{"0.19.5"}}

// committedTakenBack is the state update:rollback leaves once it takes back the update committed names.
var committedTakenBack = updater.State{
	Step: updater.StepRolledBack, From: "0.19.0", To: running, Refused: []string{"0.19.5", running},
}

// applied runs the job and fails t unless it switched, returning the state it ended at.
func applied(t *testing.T, job updater.Job) updater.State {
	t.Helper()
	state, err := job.Apply(t.Context())
	if err != nil {
		t.Fatalf("Apply() = %v, want the update switched", err)
	}
	return state
}

// copyFile copies the file at from over the file at to, making its folder, and fails t when it cannot.
func copyFile(t *testing.T, from, to string) {
	t.Helper()
	body, err := os.ReadFile(from)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(to), 0o755)
	}
	if err == nil {
		err = os.WriteFile(to, body, 0o600)
	}
	if err != nil {
		t.Errorf("copying %s to %s: %v", from, to, err)
	}
}

func TestApplyRefusesAnInstallRootItCannotWrite(t *testing.T) {
	t.Parallel()

	for name, denial := range map[string]struct {
		folder string
		mode   os.FileMode
	}{
		"a root it cannot write":          {folder: "", mode: 0o500},
		"a folder it cannot write":        {folder: "versions", mode: 0o500},
		"a root it cannot read":           {folder: "", mode: 0o300},
		"a folder it cannot read":         {folder: "versions", mode: 0o300},
		"a backups folder it cannot read": {folder: "backups", mode: 0o300},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newJobFixture(t)
			if err := os.MkdirAll(filepath.Join(f.layout.Root, denial.folder), 0o755); err != nil {
				t.Fatalf("making the folder %s: %v", denial.folder, err)
			}
			denied(t, filepath.Join(f.layout.Root, denial.folder), denial.mode)

			_, err := f.job().Apply(t.Context())

			wantRefused(t, err, "updater: cannot write to "+filepath.Join(f.layout.Root, denial.folder))
			wantMissing(t, f.layout.Build(offered))
		})
	}
}

func TestApplyTellsWhoToLetWriteToADataFolderItCannotWrite(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	job := f.job()
	job.Settings.Data = t.TempDir()
	denied(t, job.Settings.Data, 0o500)

	_, err := job.Apply(t.Context())

	wantRefused(t, err, "updater: cannot write to "+job.Settings.Data+
		", so give the user Gophenberg runs as read and write access to it")
}

func TestApplyWritesEachStepToTheStateFileBeforeItRunsIt(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	held := updater.State{Step: "committed", From: "0.19.0", To: running, Refused: []string{"0.19.5", offered}}
	writeState(t, f.layout, held)
	copies := map[updater.Step]updater.Layout{}
	for _, step := range []updater.Step{updater.StepDownload, updater.StepStage, updater.StepBackup} {
		copies[step] = updater.Layout{Root: t.TempDir()}
	}
	f.site.onServe(func(path string) {
		if strings.HasSuffix(path, ".tar.gz") {
			copyFile(t, f.layout.StateFile(), copies[updater.StepDownload].StateFile())
		}
	})
	about := strings.Replace(aboutScript(offered, "official", 0), "\n",
		"\n"+stateCopy(t, f.layout, copies[updater.StepStage]), 1)
	f.artifact = f.site.archive(releaseArchive(t, about), f.site.release)
	f.offer(running, keepOffer)
	job := f.job()
	job.To, job.Retry = offered, true
	job.Settings.PgDump = scriptedDump(t, "pg_dump (PostgreSQL) 99.2", 0,
		stateCopy(t, f.layout, copies[updater.StepBackup])).bin

	state := applied(t, job)

	for step, copied := range copies {
		want := updater.State{Step: step, From: running, To: offered, Refused: held.Refused}
		if seen := readState(t, copied); !reflect.DeepEqual(seen, want) {
			t.Errorf("the state file during the %s step held %+v, want %+v", step, seen, want)
		}
	}
	want := updater.State{
		Step: updater.StepSwitched, From: running, To: offered, Backup: f.backupName(), Refused: []string{"0.19.5"},
	}
	if !reflect.DeepEqual(state, want) {
		t.Errorf("Apply() of a retry = %+v, want %+v", state, want)
	}
}

func TestApplyKeepsRefusingAVersionWhenARetryStopsBeforeItsSwitch(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	writeState(t, f.layout, updater.State{Refused: []string{offered}})
	stopped := updater.Layout{Root: t.TempDir()}
	retry := f.job()
	retry.To, retry.Retry = offered, true
	retry.Settings.PgDump = scriptedDump(t, "pg_dump (PostgreSQL) 99.2", 2, stateCopy(t, f.layout, stopped)).bin
	_, err := retry.Apply(t.Context())
	wantRefused(t, err, "updater: back up the database with")
	writeState(t, f.layout, readState(t, stopped))

	_, err = f.job().Apply(t.Context())

	wantRefused(t, err, "run update:apply -to 0.21.0 -retry to try it again")
	wantLink(t, filepath.Join(f.layout.Root, "current"), filepath.Join("versions", running))
}

func TestApplyPointsPreviousBackWhenItCannotPointCurrent(t *testing.T) {
	t.Parallel()

	for name, before := range map[string]string{"no previous link": "", "a previous link": "0.19.0"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newJobFixture(t)
			writeState(t, f.layout, committed)
			if before != "" {
				f.placeBuild(before)
				f.point(updater.Previous, before)
			}
			if err := os.MkdirAll(filepath.Join(f.layout.Root, ".current.partial", "kept"), 0o755); err != nil {
				t.Fatalf("making a folder under the name of the new current link: %v", err)
			}

			_, err := f.job().Apply(t.Context())

			wantRefused(t, err, "updater: point current at 0.21.0")
			f.wantPutBack(committed)
			previous := filepath.Join(f.layout.Root, "previous")
			if before == "" {
				wantMissing(t, previous)
				return
			}
			wantLink(t, previous, filepath.Join("versions", before))
		})
	}
}

func TestApplyKeepsTheBackupTheStateNamesWhenItPutsTheStateBack(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	f.env["GOPHENBERG_UPDATE_BACKUPS_KEEP"] = "1"
	held := updater.State{
		Step: updater.StepFailed, From: "0.19.0", To: running, Attempts: 4, LongJump: true,
		Backup: "backups/0.19.0-20261001T000000Z.dump", Refused: []string{running},
	}
	writeState(t, f.layout, held)
	if err := os.MkdirAll(f.layout.Backups(), 0o755); err != nil {
		t.Fatalf("making the backups folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(f.layout.Root, held.Backup), []byte("an older dump"), 0o600); err != nil {
		t.Fatalf("writing the backup the state names: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(f.layout.Root, "previous", "kept"), 0o755); err != nil {
		t.Fatalf("making a folder called previous: %v", err)
	}

	_, err := f.job().Apply(t.Context())

	wantRefused(t, err, "updater: point previous at 0.20.0")
	f.wantPutBack(held)
	if _, err := os.Stat(filepath.Join(f.layout.Root, held.Backup)); err != nil {
		t.Errorf("the backup the state names is gone (%v), want it kept for the restore steps", err)
	}
}

func TestApplyStopsWhenAnotherCommandWroteTheStateSinceItRead(t *testing.T) {
	t.Parallel()

	for name, meanwhile := range map[string]func(*jobFixture, *updater.Job){
		"before the download": func(f *jobFixture, _ *updater.Job) {
			f.site.onServe(func(string) {
				if err := f.layout.WriteState(committedTakenBack); err != nil {
					f.t.Errorf("writing the state another command writes: %v", err)
				}
			})
		},
		"before the switch": func(f *jobFixture, job *updater.Job) {
			written := updater.Layout{Root: f.t.TempDir()}
			writeState(f.t, written, committedTakenBack)
			job.Settings.PgDump = scriptedDump(f.t, "pg_dump (PostgreSQL) 99.2", 0, stateCopy(f.t, written, f.layout)).bin
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newJobFixture(t)
			writeState(t, f.layout, committed)
			job := f.job()
			meanwhile(f, &job)

			_, err := job.Apply(t.Context())

			wantRefused(t, err, stateChanged)
			if strings.Count(fmt.Sprint(err), "another command wrote") != 1 {
				t.Errorf("Apply() = %v, want the state that changed named once", err)
			}
			f.wantPutBack(committedTakenBack)
			if held := readState(t, f.layout); held.Failure != nil {
				t.Errorf("the state holds the failure %+v, want the state the other command wrote left as it is",
					held.Failure)
			}
		})
	}
}

func TestApplyPointsNoLinkWhenItCannotWriteTheSwitchedState(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	writeState(t, f.layout, committed)
	folder := filepath.Dir(f.layout.StateFile())
	modeRestored(t, folder)
	job := f.job()
	job.Settings.PgDump = scriptedDump(t, "pg_dump (PostgreSQL) 99.2", 0, fmt.Sprintf("chmod 500 %q\n", folder)).bin

	_, err := job.Apply(t.Context())

	wantRefused(t, err, "updater: write the update state")
	wantLink(t, filepath.Join(f.layout.Root, "current"), filepath.Join("versions", running))
	wantMissing(t, filepath.Join(f.layout.Root, "previous"))
	wantMissing(t, f.layout.Build(offered))
	want := updater.State{Step: updater.StepBackup, From: running, To: offered, Refused: committed.Refused}
	if state := readState(t, f.layout); !reflect.DeepEqual(state, want) {
		t.Errorf("the state file holds %+v, want the backup step it wrote last %+v", state, want)
	}
}

func TestApplyStopsWhenItCannotReadTheStateBeforeAStep(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	writeState(t, f.layout, committed)
	job := f.job()
	job.Settings.PgDump = scriptedDump(t, "pg_dump (PostgreSQL) 99.2", 0,
		fmt.Sprintf("printf 'step: switched' > %q\n", f.layout.StateFile())).bin

	_, err := job.Apply(t.Context())

	wantRefused(t, err, "updater: read the update state "+f.layout.StateFile())
	wantLink(t, filepath.Join(f.layout.Root, "current"), filepath.Join("versions", running))
	wantMissing(t, f.layout.Build(offered))
}

func TestApplyCountsTheDatabaseInTheDiskItNeedsOnlyForABackup(t *testing.T) {
	t.Parallel()

	for name, skip := range map[string]bool{"with a backup": false, "without a backup": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newJobFixture(t)
			f.env["GOPHENBERG_UPDATE_DISK_MARGIN_MB"] = "1073741824"
			job := f.job()
			job.SkipBackup = skip
			need := int64(1+1073741824) << 20
			if !skip {
				need += databaseSize(t, f.db)
			}

			_, err := job.Apply(t.Context())

			wantRefused(t, err, fmt.Sprintf("the update needs %d MB for the archive", (need+1<<20-1)>>20))
		})
	}
}

func TestApplyTakesAPgDumpAsNewAsTheServerAndRefusesOneMajorOlder(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	major := serverMajor(t, f.db)
	older := f.job()
	older.Settings.PgDump = pgDumpOf(t, major-1).bin
	same := f.job()
	same.Settings.PgDump = pgDumpOf(t, major).bin

	_, err := older.Apply(t.Context())
	state := applied(t, same)

	wantRefused(t, err, fmt.Sprintf("the backup needs pg_dump %d or newer and %s is pg_dump %d", major,
		older.Settings.PgDump, major-1))
	if state.Step != updater.StepSwitched {
		t.Errorf("Apply() with pg_dump %d = %+v, want the update switched", major, state)
	}
}

func TestApplyWritesBackOnlyTheRefusedVersionsWhenItFailsAfterClearingAStop(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	stopped := updater.State{Step: updater.StepStage, From: running, To: "0.20.5", Refused: []string{"0.19.5"}}
	writeState(t, f.layout, stopped)
	job := f.job()
	job.Settings.PgDump = newFakeDump(t, "pg_dump (PostgreSQL) 99.0", 2).bin

	_, err := job.Apply(t.Context())

	wantRefused(t, err, "updater: back up the database with")
	f.wantPutBack(updater.State{Refused: []string{"0.19.5"}})
}

func TestApplyNamesTheCommandThatRetriesAVersionThatFailed(t *testing.T) {
	t.Parallel()

	for _, named := range []string{"", offered} {
		f := newJobFixture(t)
		writeState(t, f.layout, updater.State{Refused: []string{offered}})
		job := f.job()
		job.To = named

		_, err := job.Apply(t.Context())

		wantRefused(t, err, "updater: Gophenberg 0.21.0 failed its start check after an earlier update")
		wantRefused(t, err, "run update:apply -to 0.21.0 -retry to try it again")
		if !errors.As(err, &update.RefusedError{}) {
			t.Errorf("Apply() = %v, want it to carry the update.RefusedError of the check", err)
		}
		wantMissing(t, f.layout.Build(offered))
	}
}

func TestApplyRetriesAVersionThatFailedWhenAPersonAsks(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	writeState(t, f.layout, updater.State{Refused: []string{"0.20.5", offered}})
	job := f.job()
	job.To, job.Retry = offered, true

	state := applied(t, job)

	if state.Step != updater.StepSwitched || !slices.Equal(state.Refused, []string{"0.20.5"}) {
		t.Errorf("Apply() of a retry = %+v, want it switched and 0.21.0 no longer refused", state)
	}
	wantLink(t, filepath.Join(f.layout.Root, "current"), filepath.Join("versions", offered))
}

func TestApplySkipsTheBackupWhenAskedForAReleaseThatKeepsTheMinimumVersion(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	job := f.job()
	job.SkipBackup = true
	job.Settings.PgDump = filepath.Join(t.TempDir(), "pg_dump")

	state := applied(t, job)

	if state.Backup != "" || state.LongJump {
		t.Errorf("Apply() with the backup skipped = %+v, want no backup named", state)
	}
	wantMissing(t, f.layout.Backups())
}

func TestApplyBacksUpAReleaseThatRaisedTheMinimumVersionEvenWhenAskedToSkip(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	f.offer(offered, keepOffer)
	job := f.job()
	job.SkipBackup = true

	state := applied(t, job)

	if !state.LongJump || state.Backup != f.backupName() {
		t.Errorf("Apply() of a long jump = %+v, want it marked a long jump with the backup %s", state, f.backupName())
	}
	wantFile(t, filepath.Join(f.layout.Root, f.backupName()), dumpBody)
}

// stepCase is a step that fails: the change to the fixture or the job that makes it fail, the step and the error.
type stepCase struct {
	change func(*jobFixture, *updater.Job)
	step   updater.Step
	text   string
}

// stepCases are the steps of a job that fail after the job wrote them, by name.
var stepCases = map[string]stepCase{
	"an archive whose checksum differs": {
		change: func(f *jobFixture, _ *updater.Job) { f.site.replace(f.artifact, []byte("another archive")) },
		step:   updater.StepDownload,
		text:   "refused the archive, its checksum is",
	},
	"an archive no listed key signed": {
		change: func(f *jobFixture, _ *updater.Job) {
			body := releaseArchive(f.t, aboutScript(offered, "official", 0))
			f.artifact = f.site.archive(body, f.site.stranger)
			f.offer(running, keepOffer)
		},
		step: updater.StepDownload,
		text: "refused the archive, no release or standby key the key list names signed it",
	},
	"an archive that holds no binary": {
		change: func(f *jobFixture, _ *updater.Job) {
			f.artifact = f.site.archive(packed(f.t, file("LICENSE", "a license")), f.site.release)
			f.offer(running, keepOffer)
		},
		step: updater.StepStage,
		text: "refused the archive, which holds no gophenberg binary",
	},
	"a build that names another version": {
		change: func(f *jobFixture, _ *updater.Job) {
			body := releaseArchive(f.t, aboutScript("0.21.1", "official", 0))
			f.artifact = f.site.archive(body, f.site.release)
			f.offer(running, keepOffer)
		},
		step: updater.StepStage,
		text: `refused the build of Gophenberg 0.21.0, its about -json names version "0.21.1" and stamp "official"`,
	},
	"a build that is not official": {
		change: func(f *jobFixture, _ *updater.Job) {
			body := releaseArchive(f.t, aboutScript(offered, "", 0))
			f.artifact = f.site.archive(body, f.site.release)
			f.offer(running, keepOffer)
		},
		step: updater.StepStage,
		text: `its about -json names version "0.21.0" and stamp ""`,
	},
	"a build that cannot run": {
		change: func(f *jobFixture, _ *updater.Job) {
			body := releaseArchive(f.t, aboutScript(offered, "official", 3))
			f.artifact = f.site.archive(body, f.site.release)
			f.offer(running, keepOffer)
		},
		step: updater.StepStage,
		text: "refused the build of Gophenberg 0.21.0, its about -json failed: exit status 3",
	},
	"a backup that fails": {
		change: func(f *jobFixture, job *updater.Job) {
			job.Settings.PgDump = newFakeDump(f.t, "pg_dump (PostgreSQL) 99.0", 2).bin
		},
		step: updater.StepBackup,
		text: "updater: back up the database with",
	},
	"a previous link it cannot replace": {
		change: func(f *jobFixture, _ *updater.Job) {
			if err := os.MkdirAll(filepath.Join(f.layout.Root, "previous", "kept"), 0o755); err != nil {
				f.t.Fatalf("making a folder called previous: %v", err)
			}
		},
		step: updater.StepSwitched,
		text: "updater: point previous at 0.20.0",
	},
}

func TestApplyPutsTheInstallRootBackWhenAStepFails(t *testing.T) {
	t.Parallel()

	for name, failing := range stepCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newJobFixture(t)
			writeState(t, f.layout, committed)
			job := f.job()
			failing.change(f, &job)

			_, err := job.Apply(t.Context())

			wantRefused(t, err, failing.text)
			f.wantPutBack(committed)
			f.wantFailure(failing.step, failing.text)
		})
	}
}

func TestApplyRecordsOnlyTheLatestFailure(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	earlier := committed
	earlier.Failure = &updater.JobFailure{At: f.now.Add(-time.Hour), Step: updater.StepStage, Error: "an earlier job"}
	writeState(t, f.layout, earlier)
	f.site.replace(f.artifact, []byte("another archive"))

	_, err := f.job().Apply(t.Context())

	wantRefused(t, err, "refused the archive, its checksum is")
	f.wantPutBack(committed)
	f.wantFailure(updater.StepDownload, "refused the archive, its checksum is")
}

func TestApplyClearsTheFailureAnEarlierJobRecorded(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	earlier := committed
	earlier.Failure = &updater.JobFailure{At: f.now.Add(-time.Hour), Step: updater.StepStage, Error: "an earlier job"}
	writeState(t, f.layout, earlier)

	state := applied(t, f.job())

	if held := readState(t, f.layout); state.Failure != nil || held.Failure != nil {
		t.Errorf("Apply() = %+v and the state file holds %+v, want no failure once a job switched", state, held)
	}
}

func TestApplyRefusesAnArchiveTheTimestampKeySigned(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	writeState(t, f.layout, committed)
	f.artifact = f.site.archive(releaseArchive(t, aboutScript(offered, "official", 0)), f.site.timestamp)
	f.offer(running, keepOffer)
	job := f.job()

	_, err := job.Apply(t.Context())

	wantRefused(t, err, "refused the archive, no release or standby key the key list names signed it")
	f.wantPutBack(committed)
}

func TestApplyClearsAnUpdateThatStoppedBeforeItsSwitchEnded(t *testing.T) {
	t.Parallel()

	for _, step := range []updater.Step{
		updater.StepDownload, updater.StepStage, updater.StepBackup, updater.StepSwitched,
	} {
		t.Run(string(step), func(t *testing.T) {
			t.Parallel()

			f := newJobFixture(t)
			writeState(t, f.layout, updater.State{Step: step, From: running, To: "0.20.5", Refused: []string{"0.19.5"}})
			f.placeBuild("0.20.5")
			left := []string{
				f.layout.Download("0.20.5"), filepath.Join(f.layout.Backups(), "0.20.0-20261019T120000Z.dump.partial"),
			}
			for _, path := range left {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("making the folder of %s: %v", path, err)
				}
				if err := os.WriteFile(path, []byte("left by a stop"), 0o600); err != nil {
					t.Fatalf("writing %s: %v", path, err)
				}
			}

			state := applied(t, f.job())

			if state.To != offered || !slices.Equal(state.Refused, []string{"0.19.5"}) {
				t.Errorf("Apply() after a stop = %+v, want 0.21.0 switched to and 0.19.5 still refused", state)
			}
			for _, path := range append(left, f.layout.Build("0.20.5")) {
				wantMissing(t, path)
			}
		})
	}
}

func TestApplyLeavesAloneTheFilesOfAStateItCannotTrust(t *testing.T) {
	t.Parallel()

	for name, held := range map[string]updater.State{
		"a version that is no version":    {Step: updater.StepStage, From: running, To: "../versions"},
		"the version that runs":           {Step: updater.StepDownload, From: "0.19.0", To: running},
		"a switch from another version":   {Step: updater.StepSwitched, From: "0.19.0", To: "0.20.5"},
		"a step an update never writes":   {Step: "committed", From: running, To: "0.20.5"},
		"a version an update never names": {Step: updater.StepStage, From: running},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newJobFixture(t)
			writeState(t, f.layout, held)
			f.placeBuild("0.20.5")

			applied(t, f.job())

			wantFile(t, f.layout.Binary(running), "#!/bin/sh\n")
			wantFile(t, f.layout.Binary("0.20.5"), "#!/bin/sh\n")
		})
	}
}

func TestApplyKeepsTheStateOfAnUpdateItRefusesToTake(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	writeState(t, f.layout, committed)
	f.site.publish(f.site.release)

	_, err := f.job().Apply(t.Context())

	if !errors.Is(err, update.ErrNoUpdate) {
		t.Errorf("Apply() with nothing newer offered = %v, want %v", err, update.ErrNoUpdate)
	}
	if state := readState(t, f.layout); !reflect.DeepEqual(state, committed) {
		t.Errorf("the state is %+v, want it kept as %+v", state, committed)
	}
}
