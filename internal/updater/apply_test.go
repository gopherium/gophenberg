// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"database/sql"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/updater"
)

const (
	// running is the version the binary of every job test runs.
	running = "0.20.0"
	// offered is the version the update site of every job test offers.
	offered = "0.21.0"
)

// jobFixture is one job test: an install root running 0.20.0, an update site offering 0.21.0, a database and a pg_dump.
type jobFixture struct {
	t        *testing.T
	layout   updater.Layout
	site     *updateSite
	artifact update.Artifact
	dump     fakeDump
	env      map[string]string
	address  string
	db       *sql.DB
	store    *memoryStore
	now      time.Time
}

// newJobFixture returns a job test whose update site offers the official 0.21.0 under the minimum version 0.20.0.
func newJobFixture(t *testing.T) *jobFixture {
	t.Helper()
	address := emptyDatabase(t)
	f := &jobFixture{
		t: t, layout: updater.Layout{Root: t.TempDir()}, site: newUpdateSite(t), dump: pgDumpOf(t, 99),
		address: address, db: testdb.Open(t, address), store: &memoryStore{values: map[string]string{}},
		now: time.Now().UTC().Truncate(time.Second),
	}
	f.env = map[string]string{
		"GOPHENBERG_INSTALL_ROOT": f.layout.Root, "GOPHENBERG_UPDATE_DOWNLOAD_CAP_MB": "1",
		"GOPHENBERG_UPDATE_DISK_MARGIN_MB": "0", "GOPHENBERG_PG_DUMP_BIN": f.dump.bin,
	}
	f.placeBuild(running)
	f.point(updater.Current, running)
	f.artifact = f.site.archive(releaseArchive(t, aboutScript(offered, "official", 0)), f.site.release)
	f.offer(running, keepOffer)
	return f
}

// keepOffer leaves an offer as it is.
func keepOffer(*update.Offer) {}

// offer publishes the index offering 0.21.0 under minimum with the archive of the fixture, changed by edit.
func (f *jobFixture) offer(minimum string, edit func(*update.Offer)) {
	f.t.Helper()
	offer := offerOf(f.t, offered, minimum, f.artifact)
	edit(&offer)
	f.site.publish(f.site.release, offer)
}

// placeBuild writes a stand-in gophenberg of the build called name into the versions folder.
func (f *jobFixture) placeBuild(name string) {
	f.t.Helper()
	if err := os.MkdirAll(f.layout.Build(name), 0o755); err != nil {
		f.t.Fatalf("making the build folder of %s: %v", name, err)
	}
	if err := os.WriteFile(f.layout.Binary(name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		f.t.Fatalf("writing the build of %s: %v", name, err)
	}
}

// point points the link called link at the build called name, failing the test when it cannot.
func (f *jobFixture) point(link, name string) {
	f.t.Helper()
	if err := f.layout.Point(link, name); err != nil {
		f.t.Fatalf("pointing %s at %s: %v", link, name, err)
	}
}

// job returns the job of the fixture as it stands: an official build taking the newest offer.
func (f *jobFixture) job() updater.Job {
	f.t.Helper()
	return updater.Job{
		Settings: settingsOf(f.t, f.env), Check: f.site.config(running, f.now), Store: f.store, DB: f.db,
		DatabaseURL: f.address, Stamp: "official",
	}
}

// backupName returns the name of the backup a job of the fixture takes, relative to the install root.
func (f *jobFixture) backupName() string {
	return filepath.Join("backups", running+"-"+f.now.Format("20060102T150405Z")+".dump")
}

// tree returns every entry under root: the target of each link, the bytes of each file and a mark for each folder.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	held := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case entry.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			held[path] = "link to " + target
			return err
		case entry.IsDir():
			held[path] = "folder"
			return nil
		}
		body, err := os.ReadFile(path)
		held[path] = string(body)
		return err
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return held
}

// wantPutBack fails t unless f runs 0.20.0 again with no trace of 0.21.0 and the state held, its failure aside.
func (f *jobFixture) wantPutBack(held updater.State) {
	f.t.Helper()
	wantLink(f.t, filepath.Join(f.layout.Root, "current"), filepath.Join("versions", running))
	wantMissing(f.t, f.layout.Build(offered))
	wantMissing(f.t, f.layout.Download(offered))
	if partials, _ := filepath.Glob(filepath.Join(f.layout.Backups(), "*.partial")); len(partials) > 0 {
		f.t.Errorf("the backups folder keeps %v, want no partial backup", partials)
	}
	state := readState(f.t, f.layout)
	state.Failure = nil
	if !reflect.DeepEqual(state, held) {
		f.t.Errorf("the state after a failed job is %+v, want the state before it %+v", state, held)
	}
}

// wantFailure fails t unless the state file of f records the job from 0.20.0 to 0.21.0 failing now at step with text.
func (f *jobFixture) wantFailure(step updater.Step, text string) {
	f.t.Helper()
	failure := readState(f.t, f.layout).Failure
	if failure == nil || !failure.At.Equal(f.now) {
		f.t.Errorf("the state after a failed job records the failure %+v, want one at %v", failure, f.now)
		return
	}
	named := *failure
	named.At, named.Error = time.Time{}, ""
	want := updater.JobFailure{From: running, To: offered, Step: step}
	if named != want || !strings.Contains(failure.Error, text) {
		f.t.Errorf("the state after a failed job records the failure %+v, want the job from %s to %s at the step %s "+
			"saying %q", failure, running, offered, step, text)
	}
}

func TestApplySwitchesToTheNewBuildAndKeepsTheOldOne(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)

	state, err := f.job().Apply(t.Context())

	if err != nil {
		t.Fatalf("Apply() = %v, want the update switched", err)
	}
	want := updater.State{Step: updater.StepSwitched, From: running, To: offered, Backup: f.backupName()}
	if !reflect.DeepEqual(state, want) {
		t.Errorf("Apply() = %+v, want %+v", state, want)
	}
	if held := readState(t, f.layout); !reflect.DeepEqual(held, want) {
		t.Errorf("the state file holds %+v, want %+v", held, want)
	}
	wantLink(t, filepath.Join(f.layout.Root, "current"), filepath.Join("versions", offered))
	wantLink(t, filepath.Join(f.layout.Root, "previous"), filepath.Join("versions", running))
	wantFile(t, f.layout.Binary(offered), aboutScript(offered, "official", 0))
	wantFile(t, f.layout.Binary(running), "#!/bin/sh\n")
	wantFile(t, filepath.Join(f.layout.Root, f.backupName()), dumpBody)
	wantMissing(t, f.layout.Download(offered))
}

func TestApplyHandsPgDumpThePasswordOutsideTheCommandLine(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)

	if _, err := f.job().Apply(t.Context()); err != nil {
		t.Fatalf("Apply() = %v, want the update switched", err)
	}

	address, err := url.Parse(f.address)
	if err != nil {
		t.Fatalf("reading the address of the test database: %v", err)
	}
	password, _ := address.User.Password()
	address.User = url.User(address.User.Username())
	logged := f.dump.logged(t)
	if !slices.Contains(logged, "--dbname="+address.String()) || !slices.Contains(logged, "PGPASSWORD="+password) {
		t.Errorf("pg_dump ran with %q, want the address without its password and PGPASSWORD set", logged)
	}
}

func TestApplyStopsWhileAnotherUpdateRuns(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	release := locked(t, f.db)
	defer release()
	before := tree(t, f.layout.Root)

	_, err := f.job().Apply(t.Context())

	if !errors.Is(err, updater.ErrUpdateRuns) {
		t.Errorf("Apply() while another job holds the lock = %v, want %v", err, updater.ErrUpdateRuns)
	}
	if after := tree(t, f.layout.Root); !reflect.DeepEqual(after, before) {
		t.Errorf("the install root changed to %v, want it left as %v", after, before)
	}
}

func TestApplyRefusesADatabaseItCannotRead(t *testing.T) {
	t.Parallel()

	f := newJobFixture(t)
	_ = f.db.Close()

	_, err := f.job().Apply(t.Context())

	wantRefused(t, err, "updater: read the database server")
}

func TestApplyLeavesEveryPartialFileButADumpsInTheBackupsFolder(t *testing.T) {
	t.Parallel()

	for name, stop := range map[string]func(*jobFixture){
		"a job that fails": func(f *jobFixture) { f.site.replace(f.artifact, []byte("another archive")) },
		"a job that clears a stopped one": func(f *jobFixture) {
			writeState(f.t, f.layout, updater.State{Step: updater.StepBackup, From: running, To: "0.20.5"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newJobFixture(t)
			stop(f)
			dump := filepath.Join(f.layout.Backups(), "0.20.0-20261019T120000Z.dump.partial")
			other := filepath.Join(f.layout.Backups(), "0.20.0-20261019T120000Z-update.sqlite.dbkit-snapshot.partial")
			if err := os.MkdirAll(f.layout.Backups(), 0o700); err != nil {
				t.Fatalf("making the backups folder: %v", err)
			}
			for _, path := range []string{dump, other} {
				if err := os.WriteFile(path, []byte("being written"), 0o600); err != nil {
					t.Fatalf("writing %s: %v", path, err)
				}
			}

			_, _ = f.job().Apply(t.Context())

			wantMissing(t, dump)
			wantFile(t, other, "being written")
		})
	}
}

// planCase is an update the plan refuses: the change to the fixture or the job that makes it, and what the error says.
type planCase struct {
	change func(*jobFixture, *updater.Job)
	texts  []string
}

// planCases are the updates the plan refuses before anything in the install root changes, by name.
var planCases = map[string]planCase{
	"a custom build": {
		change: func(_ *jobFixture, job *updater.Job) { job.Stamp = "" },
		texts:  []string{"updater: this is a custom build of Gophenberg 0.20.0", "rebuild it on Gophenberg 0.21.0"},
	},
	"no install root": {
		change: func(f *jobFixture, job *updater.Job) {
			f.env["GOPHENBERG_INSTALL_ROOT"] = ""
			job.Settings = settingsOf(f.t, f.env)
		},
		texts: []string{"updater: GOPHENBERG_INSTALL_ROOT is empty", "update by hand to Gophenberg 0.21.0"},
	},
	"no archive for this platform": {
		change: func(f *jobFixture, _ *updater.Job) {
			f.artifact.ID = "plan9-amd64"
			f.offer(running, keepOffer)
		},
		texts: []string{"publishes no archive for " + update.Platform(), "build Gophenberg 0.21.0 from source"},
	},
	"a newer PostgreSQL": {
		change: func(f *jobFixture, _ *updater.Job) {
			f.offer(running, func(offer *update.Offer) { offer.PostgreSQL = 99 })
		},
		texts: []string{"Gophenberg 0.21.0 needs PostgreSQL 99 or newer and this database runs PostgreSQL"},
	},
	"a theme kit it does not serve": {
		change: func(_ *jobFixture, job *updater.Job) { job.Kit = "0.8.0" },
		texts: []string{
			"the active theme is built on theme kit 0.8.0, which Gophenberg 0.21.0 does not serve",
			"activate a theme built on one of the kits 0.9.0 first",
		},
	},
	"a theme that does not load": {
		change: func(_ *jobFixture, job *updater.Job) {
			job.ThemeError = errors.New("themehost: theme not installed: vanished declares no theme.json")
		},
		texts: []string{
			"updater: the active theme does not load, so the start check of Gophenberg 0.21.0 could not tell it from " +
				"a theme the update broke, fix the theme or deactivate it first",
			"themehost: theme not installed: vanished declares no theme.json",
		},
	},
	"a binary that is not the current build": {
		change: func(f *jobFixture, _ *updater.Job) { f.point(updater.Current, "0.19.0") },
		texts: []string{
			"does not name the build of Gophenberg 0.20.0 that runs this command",
			"so run update:apply as /", filepath.Join("current", "gophenberg"),
		},
	},
	"an install root with no current link": {
		change: func(f *jobFixture, _ *updater.Job) {
			if err := os.Remove(filepath.Join(f.layout.Root, "current")); err != nil {
				f.t.Fatalf("deleting the current link: %v", err)
			}
		},
		texts: []string{
			"holds no current link that names a build, so point GOPHENBERG_INSTALL_ROOT at the install root",
			"or update by hand to Gophenberg 0.21.0 as https://docs.gophenberg.org/self-hosting/updates-and-backups/",
			"updater: read the current link",
		},
	},
	"an update that waits for its first start": {
		change: func(f *jobFixture, _ *updater.Job) {
			writeState(f.t, f.layout, updater.State{Step: updater.StepSwitched, From: "0.19.0", To: running})
		},
		texts: []string{"the update to Gophenberg 0.20.0 waits for its first start"},
	},
	"too little free disk": {
		change: func(f *jobFixture, job *updater.Job) {
			f.env["GOPHENBERG_UPDATE_DISK_MARGIN_MB"] = "1073741824"
			job.Settings = settingsOf(f.t, f.env)
		},
		texts: []string{"MB free and the update needs", "GOPHENBERG_UPDATE_DISK_MARGIN_MB", "free some disk space"},
	},
	"no pg_dump": {
		change: func(f *jobFixture, job *updater.Job) {
			job.Settings.PgDump = filepath.Join(f.t.TempDir(), "pg_dump")
		},
		texts: []string{
			"the backup needs pg_dump", "point GOPHENBERG_PG_DUMP_BIN at it",
			"or run update:apply -skip-backup to update without one",
		},
	},
	"a pg_dump older than the server": {
		change: func(f *jobFixture, job *updater.Job) { job.Settings.PgDump = pgDumpOf(f.t, 9).bin },
		texts:  []string{"is pg_dump 9, so install a newer one or point GOPHENBERG_PG_DUMP_BIN at it"},
	},
	"no pg_dump for a release that raised the minimum version": {
		change: func(f *jobFixture, job *updater.Job) {
			f.offer(offered, keepOffer)
			job.SkipBackup = true
			job.Settings.PgDump = filepath.Join(f.t.TempDir(), "pg_dump")
		},
		texts: []string{
			"the backup needs pg_dump",
			"since Gophenberg 0.21.0 raised the minimum version past 0.20.0 and only the backup can take the site back",
		},
	},
	"a database address pg_dump cannot take": {
		change: func(_ *jobFixture, job *updater.Job) { job.DatabaseURL = "host=localhost dbname=site" },
		texts:  []string{"the backup needs GOPHENBERG_DATABASE_URL written as a postgres:// URL"},
	},
	"an index no listed key signed": {
		change: func(f *jobFixture, _ *updater.Job) {
			f.site.publish(f.site.stranger, offerOf(f.t, offered, running, f.artifact))
		},
		texts: []string{"refused the index, no release or standby key the list names signed it"},
	},
	"a state file it cannot read": {
		change: func(f *jobFixture, _ *updater.Job) {
			writeState(f.t, f.layout, updater.State{})
			if err := os.WriteFile(f.layout.StateFile(), []byte("{"), 0o644); err != nil {
				f.t.Fatalf("writing the state file: %v", err)
			}
		},
		texts: []string{"updater: read the update state"},
	},
}

func TestApplyRefusesWhatTheInstallCannotTakeBeforeItChangesAnything(t *testing.T) {
	t.Parallel()

	for name, refused := range planCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newJobFixture(t)
			job := f.job()
			refused.change(f, &job)
			before := tree(t, f.layout.Root)

			_, err := job.Apply(t.Context())

			for _, text := range refused.texts {
				wantRefused(t, err, text)
			}
			if after := tree(t, f.layout.Root); !reflect.DeepEqual(after, before) {
				t.Errorf("the install root changed to %v, want it left as %v", after, before)
			}
		})
	}
}
