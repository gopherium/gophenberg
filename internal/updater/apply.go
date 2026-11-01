// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/version"
)

const (
	// serverLookup reads the PostgreSQL major of the server and the size in bytes of the site database.
	serverLookup = "SELECT current_setting('server_version_num')::int / 10000, pg_database_size(current_database())"
	// busyTries is how many times a staged binary runs while the system answers that it is still open for writing.
	busyTries = 20
	// busyPause is the wait between two runs of a staged binary still open for writing.
	busyPause = 50 * time.Millisecond
)

// stoppedSteps are the steps a job that stopped before its switch can have written last.
var stoppedSteps = []Step{StepDownload, StepStage, StepBackup}

// Job is one update of a server install as update:apply runs it, with what a person asked of it.
type Job struct {
	Settings    Settings
	Check       update.Config
	Store       update.Settings
	DB          *sql.DB
	DatabaseURL string
	Stamp       string
	Kit         string
	ThemeError  error
	To          string
	Retry       bool
	SkipBackup  bool
}

// server is what a job reads of the database server: its PostgreSQL major and the size of the site database.
type server struct {
	major int
	size  int64
}

// run is one job under way: the job, its install root, the state found there and on disk, the update and what it read.
type run struct {
	job      Job
	layout   Layout
	held     State
	onDisk   State
	previous string
	chosen   update.Chosen
	artifact update.Artifact
	server   server
	archive  *os.File
}

// step is one step of a job that changes the install root, written to the state file before it runs.
type step struct {
	name Step
	run  func(context.Context, *State) error
}

// retryError refuses a version that failed its start check after an earlier update, naming how to retry it.
type retryError struct {
	update.RefusedError
}

// Error says the version failed before and the command that takes it again.
func (e retryError) Error() string {
	return "updater: Gophenberg " + e.Version + " failed its start check after an earlier update, so update:apply " +
		"never takes it again on its own, run update:apply -to " + e.Version + " -retry to try it again"
}

// Unwrap returns the error the check refused the version with.
func (e retryError) Unwrap() error {
	return e.RefusedError
}

// SwitchedError reports a job whose switch stands although a step after it failed, so its build waits for a restart.
type SwitchedError struct {
	Release string
	Err     error
}

// Error says that current names the release and that Gophenberg must restart to start it, with the failure.
func (e SwitchedError) Error() string {
	return fmt.Sprintf("updater: current names Gophenberg %s, so restart Gophenberg to start it: %v", e.Release, e.Err)
}

// Unwrap returns the failure after the switch.
func (e SwitchedError) Unwrap() error {
	return e.Err
}

// Planned is a job whose checks before the download passed, holding the update lock until the rest of it ends.
type Planned struct {
	run     *run
	release func()
}

// Apply locks, checks, plans, downloads, stages, backs up and switches, returning the state the job ends at.
func (j Job) Apply(ctx context.Context) (State, error) {
	planned, err := j.Plan(ctx)
	if err != nil {
		return State{}, err
	}
	return planned.Apply(ctx)
}

// Plan reads the database server, takes the update lock and runs every check that needs no download.
func (j Job) Plan(ctx context.Context) (*Planned, error) {
	facts, err := serverOf(ctx, j.DB)
	if err != nil {
		return nil, err
	}
	release, err := Lock(ctx, j.DB)
	if err != nil {
		return nil, err
	}
	r, err := j.planned(ctx, facts)
	if err != nil {
		release()
		return nil, err
	}
	return &Planned{run: r, release: release}, nil
}

// Apply downloads, stages, backs up and switches, then lets go of the update lock, returning the state the job ends at.
func (p *Planned) Apply(ctx context.Context) (State, error) {
	defer p.release()
	return p.run.apply(ctx)
}

// serverOf reads the PostgreSQL major of the server db connects to and the size of its database.
func serverOf(ctx context.Context, db *sql.DB) (server, error) {
	var facts server
	if err := db.QueryRowContext(ctx, serverLookup).Scan(&facts.major, &facts.size); err != nil {
		return server{}, fmt.Errorf("updater: read the database server: %w", err)
	}
	return facts, nil
}

// planned reads the state, chooses the update and returns the run once no check of the plan refuses it.
func (j Job) planned(ctx context.Context, facts server) (*run, error) {
	r := &run{job: j, layout: j.Settings.Layout(), server: facts}
	if err := r.readState(); err != nil {
		return nil, err
	}
	chosen, err := update.Choose(ctx, r.checkConfig(), j.Store, j.To)
	if err != nil {
		return nil, retryHint(err)
	}
	r.chosen = chosen
	return r, r.plan(ctx)
}

// readState reads the state the install root holds, none for an install with no install root.
func (r *run) readState() error {
	if r.layout.Root == "" {
		return nil
	}
	held, err := r.layout.ReadState()
	r.held, r.onDisk = held, held
	return err
}

// checkConfig returns the check of the update site for this server, refusing the versions the state refuses.
func (r *run) checkConfig() update.Config {
	cfg := r.job.Check
	cfg.Site.ServerMajor = r.server.major
	cfg.Site.Refused = r.held.Refused
	if r.job.Retry {
		cfg.Site = cfg.Site.Lift(r.job.To)
	}
	return cfg
}

// retryHint returns err refused when the update site stopped the choice, naming how to retry a version that failed.
func retryHint(err error) error {
	var refused update.RefusedError
	switch {
	case !errors.Is(err, update.ErrNotTaken):
		return err
	case errors.As(err, &refused):
		return refusedError{err: retryError{RefusedError: refused}}
	}
	return refusedError{err: err}
}

// running returns the version of the binary that runs the job.
func (r *run) running() string {
	return r.job.Check.Site.Running
}

// to returns the version the job updates to.
func (r *run) to() string {
	return r.chosen.Found.Version
}

// backsUp reports whether the job takes a backup: always for a long jump, otherwise unless a person skipped it.
func (r *run) backsUp() bool {
	return r.chosen.Found.LongJump || !r.job.SkipBackup
}

// apply clears what a stopped job left, then runs each step and the switch, putting the install root back on failure.
func (r *run) apply(ctx context.Context) (State, error) {
	r.clearInterrupted()
	r.previous, _ = r.layout.Named(Previous)
	state := State{From: r.running(), To: r.to(), LongJump: r.chosen.Found.LongJump, Refused: r.held.Refused}
	for _, s := range r.steps() {
		state.Step = s.name
		err := r.write(state)
		if err == nil {
			err = s.run(ctx, &state)
		}
		if err != nil {
			return State{}, r.putBack(s.name, err)
		}
	}
	state.Step, state.Refused = StepSwitched, lifting(r.held.Refused, r.to())
	if err := r.switchBuilds(state); err != nil {
		return State{}, r.putBack(StepSwitched, err)
	}
	return state, nil
}

// write writes state over the state file, refusing when another command wrote it after the job last read or wrote it.
func (r *run) write(state State) error {
	return r.whileUnchanged(func() error { return r.save(state) })
}

// save writes state over the state file as the job holds the state lock, keeping it as what the file now holds.
func (r *run) save(state State) error {
	err := r.layout.WriteState(state)
	if err == nil {
		r.onDisk = state
	}
	return err
}

// whileUnchanged runs change under the state lock once the state file still holds what the job last read or wrote.
func (r *run) whileUnchanged(change func() error) error {
	state, unlock, err := r.layout.lockedState()
	if err != nil {
		return err
	}
	defer unlock()
	if !sameState(state, r.onDisk) {
		return errStateChanged
	}
	return change()
}

// steps returns the steps of the job before its switch in order, the backup only when the job takes one.
func (r *run) steps() []step {
	steps := []step{{name: StepDownload, run: r.download}, {name: StepStage, run: r.stage}}
	if r.backsUp() {
		steps = append(steps, step{name: StepBackup, run: r.backUp})
	}
	return steps
}

// download fetches the archive into the downloads folder and checks it, keeping it open for the stage.
func (r *run) download(ctx context.Context, _ *State) error {
	partial := r.layout.Download(r.to())
	_ = os.MkdirAll(filepath.Dir(partial), 0o755)
	archive, err := Download{
		Artifact: r.artifact, Signers: r.chosen.Signers, Partial: partial, Cap: r.job.Settings.DownloadCap,
		Client: NewClient(r.job.Settings.DownloadTimeout),
	}.Fetch(ctx)
	r.archive = archive
	return err
}

// stage unpacks the archive into a new build folder and checks the build, deleting the download either way.
func (r *run) stage(ctx context.Context, _ *State) error {
	defer r.dropArchive()
	build := r.layout.Build(r.to())
	_ = os.MkdirAll(filepath.Dir(build), 0o755)
	_ = os.RemoveAll(build)
	if err := Unpack(r.archive, build); err != nil {
		return err
	}
	return checkBuild(ctx, r.layout.Binary(r.to()), r.to())
}

// dropArchive closes the downloaded archive and deletes it.
func (r *run) dropArchive() {
	_ = r.archive.Close()
	_ = os.Remove(r.archive.Name())
}

// aboutJSON returns what binary answers to about -json.
func aboutJSON(ctx context.Context, binary string) ([]byte, error) {
	for try := 1; ; try++ {
		out, err := exec.CommandContext(ctx, binary, "about", "-json").Output()
		if !errors.Is(err, syscall.ETXTBSY) || try == busyTries {
			return out, err
		}
		time.Sleep(busyPause)
	}
}

// checkBuild runs about -json of the staged binary, refusing one that is not the official build of release.
func checkBuild(ctx context.Context, binary, release string) error {
	out, err := aboutJSON(ctx, binary)
	if err != nil {
		return fmt.Errorf("updater: refused the build of Gophenberg %s, its about -json failed: %w", release, err)
	}
	var about struct {
		Version string `json:"version"`
		Stamp   string `json:"stamp"`
	}
	_ = json.Unmarshal(out, &about)
	if about.Version != release || about.Stamp != officialStamp {
		return fmt.Errorf("updater: refused the build of Gophenberg %s, its about -json names version %q and stamp %q",
			release, about.Version, about.Stamp)
	}
	return nil
}

// backUp takes the backup the switch needs, naming it in state and sparing the backup the state found names.
func (r *run) backUp(ctx context.Context, state *State) error {
	name := backupName(r.running(), r.job.Check.Now())
	state.Backup = filepath.Join(backupsFolder, name)
	return Backup{
		PgDump: r.job.Settings.PgDump, DatabaseURL: r.job.DatabaseURL, Folder: r.layout.Backups(), Name: name,
		Held: filepath.Base(r.held.Backup), Keep: r.job.Settings.BackupsKept,
	}.Take(ctx)
}

// switchBuilds writes state, then points previous at the running build and current at the new one, in one lock hold.
func (r *run) switchBuilds(state State) error {
	return r.whileUnchanged(func() error {
		if err := r.save(state); err != nil {
			return err
		}
		if err := r.layout.Point(Previous, state.From); err != nil {
			return err
		}
		return r.layout.Point(Current, state.To)
	})
}

// putBack deletes what the job wrote, points previous back and writes back the state it found, unless current moved.
func (r *run) putBack(at Step, err error) error {
	if r.switched() {
		return SwitchedError{Release: r.to(), Err: err}
	}
	r.removeFiles(r.to())
	if errors.Is(err, errStateChanged) {
		return errors.Join(err, r.putPreviousBack())
	}
	return errors.Join(err, r.putPreviousBack(), r.write(r.failed(at, err)))
}

// failed returns the state the job found, recording the job's versions, the step at and err as its failure now.
func (r *run) failed(at Step, err error) State {
	state := r.held
	state.Failure = &JobFailure{
		At: r.job.Check.Now().UTC(), From: r.running(), To: r.to(), Step: at, Error: err.Error(),
	}
	return state
}

// switched reports whether the current link already names the build the job switches to, so the switch stands.
func (r *run) switched() bool {
	name, err := r.layout.Named(Current)
	return err == nil && name == r.to()
}

// putPreviousBack points previous back at the build it named before the job, or deletes the link the job made.
func (r *run) putPreviousBack() error {
	named, _ := r.layout.Named(Previous)
	switch {
	case named != r.running():
		return nil
	case r.previous == "":
		return os.Remove(filepath.Join(r.layout.Root, Previous))
	}
	return r.layout.Point(Previous, r.previous)
}

// clearInterrupted deletes the files of an earlier job that stopped before its switch ended, keeping what it refused.
func (r *run) clearInterrupted() {
	if !r.interrupted() {
		return
	}
	r.removeFiles(r.held.To)
	r.held = State{Refused: r.held.Refused}
}

// interrupted reports whether the state names an earlier job of this build that stopped before its switch ended.
func (r *run) interrupted() bool {
	switch {
	case !version.Valid(r.held.To) || r.held.To == r.running():
		return false
	case r.held.Step == StepSwitched:
		return r.held.From == r.running()
	}
	return slices.Contains(stoppedSteps, r.held.Step)
}

// removeFiles deletes the download and the build of release and any partial backup, what a job that fails leaves.
func (r *run) removeFiles(release string) {
	_ = os.Remove(r.layout.Download(release))
	_ = os.RemoveAll(r.layout.Build(release))
	partials, _ := filepath.Glob(filepath.Join(r.layout.Backups(), "*"+partialSuffix))
	for _, partial := range partials {
		_ = os.Remove(partial)
	}
}
