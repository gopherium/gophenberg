// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gopherium/gophenberg/internal/themehost"
	"github.com/gopherium/gophenberg/internal/update"
)

const (
	// officialStamp is the build stamp of a binary the release workflow built, the only one that updates itself.
	officialStamp = "official"
	// manualSteps is the page of the manual that shows how to update by hand.
	manualSteps = "https://docs.gophenberg.org/self-hosting/updates-and-backups/"
)

// ErrRefused marks an update a check before its download refused, its message saying why.
var ErrRefused = errors.New("updater: a check refused the update")

// refusedError is what a check before the download says of an update it refuses.
type refusedError struct {
	err error
}

// Error says why the check refused the update.
func (e refusedError) Error() string {
	return e.err.Error()
}

// Unwrap returns what the check refused the update with, beside ErrRefused.
func (e refusedError) Unwrap() []error {
	return []error{e.err, ErrRefused}
}

// plan refuses an update this install cannot take, before anything in the install root changes.
func (r *run) plan(ctx context.Context) error {
	for _, check := range []func() error{
		r.official, r.rooted, r.archived, r.servedPostgres, r.themeLoads, r.servedKit, r.runsCurrent, r.notWaiting,
		r.writable, func() error { return r.dumpReady(ctx) }, r.roomy,
	} {
		if err := check(); err != nil {
			return refusedError{err: err}
		}
	}
	return nil
}

// cannotUpdateError is what a check of the plan says of an install that cannot update itself, and why it cannot.
type cannotUpdateError struct {
	reason  Reason
	message string
}

// Error says why the install cannot update itself and what to do instead.
func (e cannotUpdateError) Error() string {
	return e.message
}

// ReasonOf returns why the install cannot update itself when err says so, empty for any other error and for none.
func ReasonOf(err error) Reason {
	var cannot cannotUpdateError
	if errors.As(err, &cannot) {
		return cannot.reason
	}
	return ""
}

// official refuses a binary the release workflow did not build, since an official archive drops its own plugins.
func (r *run) official() error {
	if r.job.Stamp == officialStamp {
		return nil
	}
	return cannotUpdateError{reason: CustomBuild, message: fmt.Sprintf("updater: this is a %s build of Gophenberg "+
		"%s, which an official archive would replace without the plugins built into it, so rebuild it on Gophenberg %s",
		cmp.Or(r.job.Stamp, "custom"), r.running(), r.to())}
}

// rooted refuses an install with no install root, like the Docker image, which only a person updates.
func (r *run) rooted() error {
	if r.layout.Root != "" {
		return nil
	}
	return cannotUpdateError{reason: NoInstallRoot, message: fmt.Sprintf("updater: %s is empty, so this install "+
		"cannot switch builds on its own, update by hand to Gophenberg %s as %s shows",
		r.job.Settings.names.Key("INSTALL_ROOT"), r.to(), manualSteps)}
}

// archived refuses a release that publishes no archive for this platform, keeping the one it publishes.
func (r *run) archived() error {
	artifact, err := r.chosen.Release.ArchiveFor(update.Platform())
	if err != nil {
		return fmt.Errorf("%w, so build Gophenberg %s from source", err, r.to())
	}
	r.artifact = artifact
	return nil
}

// servedPostgres refuses a release that needs a newer PostgreSQL than the database runs.
func (r *run) servedPostgres() error {
	found := r.chosen.Found
	if !found.NeedsPostgreSQL {
		return nil
	}
	return fmt.Errorf("updater: Gophenberg %s needs PostgreSQL %d or newer and this database runs PostgreSQL %d, "+
		"so upgrade PostgreSQL first", found.Version, found.PostgreSQL, r.server.major)
}

// themeLoads refuses a site whose active theme does not load, which the start check after the switch would refuse.
func (r *run) themeLoads() error {
	if r.job.ThemeError == nil {
		return nil
	}
	return fmt.Errorf("updater: the active theme does not load, so the start check of Gophenberg %s could not tell "+
		"it from a theme the update broke, fix the theme or deactivate it first: %w", r.to(), r.job.ThemeError)
}

// servedKit refuses a release that does not serve the theme kit the active theme is built on.
func (r *run) servedKit() error {
	kits := r.chosen.Found.Kits
	if r.job.Kit == "" || themehost.KitsServe(kits, r.job.Kit) {
		return nil
	}
	return fmt.Errorf("updater: the active theme is built on theme kit %s, which Gophenberg %s does not serve, "+
		"so activate a theme built on one of the kits %s first", r.job.Kit, r.to(), strings.Join(kits, ", "))
}

// runsCurrent refuses an install root with no current link, and a job run by a binary other than the build it names.
func (r *run) runsCurrent() error {
	name, err := r.layout.Named(Current)
	if err != nil {
		return fmt.Errorf("updater: %s holds no current link that names a build, so point %s at the install root "+
			"of this server, or update by hand to Gophenberg %s as %s shows: %w", r.layout.Root,
			r.job.Settings.names.Key("INSTALL_ROOT"), r.to(), manualSteps, err)
	}
	if name == r.running() {
		return nil
	}
	return fmt.Errorf("updater: the current link of %s does not name the build of Gophenberg %s that runs this "+
		"command, so run update:apply as %s", r.layout.Root, r.running(),
		filepath.Join(r.layout.Root, Current, binaryName))
}

// notWaiting refuses a job while the build an earlier update switched to waits for its first start.
func (r *run) notWaiting() error {
	if r.held.Step != StepSwitched || r.held.To != r.running() {
		return nil
	}
	return fmt.Errorf("updater: the update to Gophenberg %s waits for its first start, so restart Gophenberg "+
		"and let that start check it before another update", r.held.To)
}

// writable refuses an install root the job cannot write to, probing the root and each folder of it that is there.
func (r *run) writable() error {
	if err := r.layout.MakeData(); err != nil {
		return err
	}
	for _, dir := range r.layout.folders() {
		err := probe(dir)
		if errors.Is(err, fs.ErrNotExist) && dir != r.layout.Root {
			continue
		}
		if err != nil {
			return fmt.Errorf("updater: cannot write to %s, so give the user Gophenberg runs as read and write "+
				"access to the install root: %w", dir, err)
		}
	}
	return nil
}

// probe writes a file into dir, deletes it and saves dir to disk, as the steps of a job do in each folder they write.
func probe(dir string) error {
	file, err := os.CreateTemp(dir, ".update-check-*")
	if err != nil {
		return err
	}
	_ = file.Close()
	_ = os.Remove(file.Name())
	return syncFolder(dir)
}

// dumpReady refuses an update whose backup cannot be taken: no address pg_dump reads, no pg_dump or an old one.
func (r *run) dumpReady(ctx context.Context) error {
	if !r.backsUp() {
		return nil
	}
	names := r.job.Settings.names
	if _, err := dumpTarget(r.job.DatabaseURL); err != nil {
		return fmt.Errorf("updater: the backup needs %s written as a postgres:// URL, so write it as one%s",
			names.Key("DATABASE_URL"), r.backupHint())
	}
	pgDump := r.job.Settings.PgDump
	major, err := DumpMajor(ctx, pgDump)
	if err != nil {
		return fmt.Errorf("updater: the backup needs pg_dump %d or newer, so install it or point %s at it%s: %w",
			r.server.major, names.Key("PG_DUMP_BIN"), r.backupHint(), err)
	}
	if major < r.server.major {
		return fmt.Errorf("updater: the backup needs pg_dump %d or newer and %s is pg_dump %d, so install a newer "+
			"one or point %s at it%s", r.server.major, pgDump, major, names.Key("PG_DUMP_BIN"), r.backupHint())
	}
	return nil
}

// backupHint says what a person may do without the backup: skip it, or nothing for a release past the minimum version.
func (r *run) backupHint() string {
	if r.chosen.Found.LongJump {
		return fmt.Sprintf(", since Gophenberg %s raised the minimum version past %s and only the backup can take "+
			"the site back", r.to(), r.running())
	}
	return ", or run update:apply -skip-backup to update without one"
}

// roomy refuses an install root or a data folder whose disk cannot hold the archive, the backup and the margin.
func (r *run) roomy() error {
	settings := r.job.Settings
	need := settings.DownloadCap + settings.DiskMargin
	if r.backsUp() {
		need += r.server.size
	}
	for _, dir := range []string{r.layout.Root, r.layout.Kept()} {
		if free := freeBytes(dir); free < need {
			return fmt.Errorf("updater: %s has %d MB free and the update needs %d MB for the archive, the backup and "+
				"%s, so free some disk space first", dir, free>>20, (need+1<<20-1)>>20,
				settings.names.Key("UPDATE_DISK_MARGIN_MB"))
		}
	}
	return nil
}

// freeBytes returns the bytes the file system that holds dir has free for the user Gophenberg runs as, 0 unread.
func freeBytes(dir string) int64 {
	var stats syscall.Statfs_t
	_ = syscall.Statfs(dir, &stats)
	return int64(stats.Bavail * uint64(stats.Bsize))
}
