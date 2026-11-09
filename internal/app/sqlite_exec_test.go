// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
	"github.com/gopherium/gophenberg/internal/version"
)

const (
	// stampedCommit is a synthetic commit a test stamps into its own build of gophenberg.
	stampedCommit = "0123456789abcdef0123456789abcdef01234567"
	// binaryRunLimit is how long one run of the built binary may take before the test stops it.
	binaryRunLimit = 30 * time.Second
	// newFileHold is how long another connection keeps the write lock of a new file on a system without /proc.
	newFileHold = 500 * time.Millisecond
	// openPoll is how often the test looks again whether each run has the database file open.
	openPoll = time.Millisecond
	// endGrace is how long a run whose open files cannot be read has to show it ended before the test stops.
	endGrace = time.Second
)

// binaryRun is one run of the built binary that started and that the test has not waited for yet.
type binaryRun struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	waited chan error
	ended  chan struct{}
}

// startBinary starts the built binary with args and stdin under env in a folder of its own, stopped at binaryRunLimit.
func startBinary(t *testing.T, binary string, env []string, stdin io.Reader, args ...string) binaryRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), binaryRunLimit)
	run := binaryRun{cmd: exec.CommandContext(ctx, binary, args...), cancel: cancel, stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{}, waited: make(chan error, 1), ended: make(chan struct{})}
	run.cmd.Dir = t.TempDir()
	run.cmd.Env = env
	run.cmd.Stdin = stdin
	run.cmd.Stdout = run.stdout
	run.cmd.Stderr = run.stderr
	if err := run.cmd.Start(); err != nil {
		cancel()
		t.Fatalf("gophenberg %q: %v", args, err)
	}
	go func() {
		run.waited <- run.cmd.Wait()
		close(run.ended)
	}()
	return run
}

// hasEnded reports whether the run has ended, without waiting for it.
func (r binaryRun) hasEnded() bool {
	select {
	case <-r.ended:
		return true
	default:
		return false
	}
}

// wait waits for the run to end and returns its answer.
func (r binaryRun) wait(t *testing.T) testkit.Result {
	t.Helper()
	defer r.cancel()
	var exitErr *exec.ExitError
	if err := <-r.waited; err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("gophenberg %q: %v", r.cmd.Args[1:], err)
	}
	return testkit.Result{Code: r.cmd.ProcessState.ExitCode(), Stdout: r.stdout.String(), Stderr: r.stderr.String()}
}

// runBinary runs the built binary with args and stdin under env from a folder of its own and returns its answer.
func runBinary(t *testing.T, binary string, env []string, stdin io.Reader, args ...string) testkit.Result {
	t.Helper()
	return startBinary(t, binary, env, stdin, args...).wait(t)
}

// holdWriteLock holds the write lock of the SQLite file at path from a connection outside dbkit, until let go.
func holdWriteLock(t *testing.T, path string) (letGo func()) {
	t.Helper()
	conn, err := openSQLiteFile(t, path).Driver().Open(path)
	if err != nil {
		t.Fatalf("opening a plain driver connection to the SQLite file: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	plain, ok := conn.(driver.ExecerContext)
	if !ok {
		t.Fatalf("the SQLite driver connection %T runs no statement", conn)
	}
	if _, err := plain.ExecContext(t.Context(), "BEGIN IMMEDIATE", nil); err != nil {
		t.Fatalf("BEGIN IMMEDIATE on the plain driver connection: %v", err)
	}
	return func() {
		if _, err := plain.ExecContext(t.Context(), "ROLLBACK", nil); err != nil {
			t.Fatalf("ROLLBACK on the plain driver connection: %v", err)
		}
	}
}

// waitUntilOpen waits until each run has the file at path open or has ended, or for newFileHold without /proc.
func waitUntilOpen(t *testing.T, path string, runs []binaryRun) {
	t.Helper()
	if runtime.GOOS != "linux" {
		time.Sleep(newFileHold)
		return
	}
	file, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolving the SQLite file %s: %v", path, err)
	}
	waiting := make(map[int]binaryRun, len(runs))
	for i, run := range runs {
		waiting[i+1] = run
	}
	deadline := time.Now().Add(binaryRunLimit)
	for {
		maps.DeleteFunc(waiting, func(_ int, run binaryRun) bool {
			return openOrEnded(t, run, file)
		})
		if len(waiting) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("gophenberg runs %v of %d neither opened %s nor ended within %v",
				slices.Sorted(maps.Keys(waiting)), len(runs), path, binaryRunLimit)
		}
		time.Sleep(openPoll)
	}
}

// openOrEnded reports whether run has the file open or has ended, failing t when its open files cannot be read.
func openOrEnded(t *testing.T, run binaryRun, file string) bool {
	t.Helper()
	if run.hasEnded() {
		return true
	}
	open, err := hasOpen(run.cmd.Process.Pid, file)
	if err == nil {
		return open
	}
	select {
	case <-run.ended:
		return true
	case <-time.After(endGrace):
	}
	t.Fatalf("reading the open files of gophenberg process %d: %v", run.cmd.Process.Pid, err)
	return false
}

// hasOpen reports whether the process pid has the file at path open, as its /proc folder lists it.
func hasOpen(pid int, path string) (bool, error) {
	fds := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	entries, err := os.ReadDir(fds)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(entries, func(entry os.DirEntry) bool {
		target, err := os.Readlink(filepath.Join(fds, entry.Name()))
		return err == nil && target == path
	}), nil
}

// cronJobCount returns how many cron jobs the SQLite file behind db holds.
func cronJobCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	lookup := "SELECT count(*) FROM core_cron_jobs"
	if err := db.QueryRowContext(t.Context(), lookup).Scan(&count); err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	return count
}

func TestSQLiteBinaryMigratesChecksAndNamesWhatItCannotRun(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	path := emptySQLiteFile(t)
	env = append(env, "GOPHENBERG_DATABASE_URL="+sqlitePrefix+path, "GOPHENBERG_ADDR=localhost:0")

	migrated := runBinary(t, binary, env, nil, "migrate")
	checked := runBinary(t, binary, env, nil, "check")
	served := runBinary(t, binary, env, nil, "serve")
	listed := runBinary(t, binary, env, nil, "account:list")

	if migrated.Code != gonsole.ExitDone || migrated.Stdout != migratedLines {
		t.Errorf("gophenberg migrate = %d, stdout %q, stderr %q, want 0 and %q",
			migrated.Code, migrated.Stdout, migrated.Stderr, migratedLines)
	}
	db := openSQLiteFile(t, path)
	for _, table := range []string{
		"auth_goose_db_version", "gonsole_goose_db_version", "core_goose_db_version", "core_cron_jobs",
	} {
		if !hasTable(t, db, table) {
			t.Errorf("the file holds no %s table after gophenberg migrate, want every owner's schema", table)
		}
	}
	if want := "settings, plugins and command names are valid\n"; checked.Code != gonsole.ExitDone ||
		checked.Stdout != want {
		t.Errorf("gophenberg check = %d, stdout %q, stderr %q, want 0 and %q",
			checked.Code, checked.Stdout, checked.Stderr, want)
	}
	if want := "serve: " + errPostgresOnly.Error(); served.Code != gonsole.ExitFailed ||
		!strings.Contains(served.Stderr, want) {
		t.Errorf("gophenberg serve = %d with stderr %q, want 1 and %q", served.Code, served.Stderr, want)
	}
	if jobs := cronJobCount(t, db); jobs != 0 {
		t.Errorf("core_cron_jobs holds %d rows after gophenberg serve, want none since no cron job ran", jobs)
	}
	if listed.Code != gonsole.ExitDone || listed.Stdout != "" {
		t.Errorf("gophenberg account:list = %d, stdout %q, stderr %q, want 0 and no account listed",
			listed.Code, listed.Stdout, listed.Stderr)
	}
}

func TestSQLiteBinaryMigratesANewFileFromTwoRunsBehindAHeldWriteLock(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	path := emptySQLiteFile(t)
	env = append(env, "GOPHENBERG_DATABASE_URL="+sqlitePrefix+path)
	letGo := holdWriteLock(t, path)

	runs := []binaryRun{startBinary(t, binary, env, nil, "migrate"), startBinary(t, binary, env, nil, "migrate")}
	waitUntilOpen(t, path, runs)
	letGo()

	for i, run := range runs {
		if got := run.wait(t); got.Code != gonsole.ExitDone || got.Stdout != migratedLines {
			t.Errorf("gophenberg migrate run %d of 2 = %d, stdout %q, stderr %q, want 0 and %q",
				i+1, got.Code, got.Stdout, got.Stderr, migratedLines)
		}
	}
	lone := emptySQLiteFile(t)
	if got := testkit.Run(t, Program(migratingSQLite(lone), noPlugins), "", "migrate"); got.Code != gonsole.ExitDone {
		t.Fatalf("migrate of a second new file = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	want := appliedVersions(t, openSQLiteFile(t, lone))
	if got := appliedVersions(t, openSQLiteFile(t, path)); !maps.EqualFunc(got, want, slices.Equal) {
		t.Errorf("the versions after two runs at once = %v, want the %v one run leaves on a new file", got, want)
	}
}

// stampedBinary builds gophenberg with commit stamped in, into a folder of t, and returns its path.
func stampedBinary(t *testing.T, commit string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "gophenberg")
	build := exec.CommandContext(t.Context(), "go", "build", "-ldflags",
		"-X github.com/gopherium/gophenberg/internal/version.commit="+commit, "-o", binary, "../../cmd/gophenberg")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building gophenberg stamped with the commit %s: %v\n%s", commit, err, out)
	}
	return binary
}

// unsetEnv returns the environment of the test process without its GOPHENBERG_ settings, then settings.
func unsetEnv(settings ...string) []string {
	kept := slices.DeleteFunc(os.Environ(), func(entry string) bool { return strings.HasPrefix(entry, "GOPHENBERG_") })
	return append(kept, settings...)
}

func TestSQLiteBinaryBacksUpAFileAnOlderBuildMigrated(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	path := olderMigratedSQLiteFile(t)

	got := runBinary(t, binary, append(env, "GOPHENBERG_DATABASE_URL="+sqlitePrefix+path), nil, "migrate")

	if got.Code != gonsole.ExitDone || got.Stdout != migratedLines {
		t.Fatalf("gophenberg migrate = %d, stdout %q, stderr %q, want 0 and %q",
			got.Code, got.Stdout, got.Stderr, migratedLines)
	}
	backup := oneUpdateBackupIn(t, backupsBeside(path))
	if held := lastMigratedBy(t, copiedBackup(t, backup)); held.Version != olderVersion {
		t.Errorf("the update backup records %+v as its last migration, want the file as %s left it", held, olderVersion)
	}
}

func TestSQLiteBinariesAtOnceOnAnOlderFileLeaveOneBackupOfItAsItWas(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	path := olderMigratedSQLiteFile(t)
	env = append(env, "GOPHENBERG_DATABASE_URL="+sqlitePrefix+path)

	runs := []binaryRun{startBinary(t, binary, env, nil, "migrate"), startBinary(t, binary, env, nil, "migrate")}

	for i, run := range runs {
		got := run.wait(t)
		migrated := got.Code == gonsole.ExitDone && got.Stdout == migratedLines
		waited := got.Code == gonsole.ExitFailed && strings.Contains(got.Stderr, "GOPHENBERG_SQLITE_MIGRATE_LOCK_WAIT") &&
			strings.Contains(got.Stderr, "GOPHENBERG_SQLITE_MIGRATE_LOCK_POLL")
		if !migrated && !waited {
			t.Errorf("gophenberg migrate run %d of 2 = %d, stdout %q, stderr %q, want it migrated or both lock settings named",
				i+1, got.Code, got.Stdout, got.Stderr)
		}
	}
	copied := copiedBackup(t, oneUpdateBackupIn(t, backupsBeside(path)))
	if held := lastMigratedBy(t, copied); held.Version != olderVersion {
		t.Errorf("the update backup records %+v as its last migration, want the file as %s left it", held, olderVersion)
	}
	if needed, err := sqlite.NeedsBackup(t.Context(), copied, thisBuild()); err != nil || !needed {
		t.Errorf("NeedsBackup() of the update backup = %t, %v, want true, the file as it was before both runs",
			needed, err)
	}
}

func TestSQLiteBinaryOfAnotherCommitBacksUpAndRecordsItsCommit(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	binary := stampedBinary(t, stampedCommit)
	env := unsetEnv("GOPHENBERG_DATABASE_URL="+sqlitePrefix+path, "GOPHENBERG_UPDATE_CHECKS=false")

	got := runBinary(t, binary, env, nil, "migrate")

	if got.Code != gonsole.ExitDone || got.Stdout != migratedLines {
		t.Fatalf("the stamped gophenberg migrate = %d, stdout %q, stderr %q, want 0 and %q",
			got.Code, got.Stdout, got.Stderr, migratedLines)
	}
	backup := oneUpdateBackupIn(t, backupsBeside(path))
	if named, _ := backupfiles.Read(filepath.Base(backup)); named.Version != version.Version() {
		t.Errorf("the update backup is named %s, want the %s the in-process migrate recorded", backup, version.Version())
	}
	want := sqlite.Build{Version: version.Version(), Commit: stampedCommit}
	if held := lastMigratedBy(t, openSQLiteFile(t, path)); held != want {
		t.Errorf("the file records %+v as its last migration, want %+v", held, want)
	}
}

func TestSQLiteBinaryRunsTheAccountCommandsAndKeepsTheirRecords(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	path := emptySQLiteFile(t)
	env = append(env, "GOPHENBERG_DATABASE_URL="+sqlitePrefix+path)
	succeeds(t, runBinary(t, binary, env, nil, "migrate"), "migrated accounts")
	for _, role := range []string{"admin", "author"} {
		succeeds(t, runBinary(t, binary, env, strings.NewReader(typedPassword+"\n"),
			"account:create-admin", "-email", role+"@example.com", "-name", "Holder", "-role", role),
			"created user "+role+"@example.com")
	}

	changed := runBinary(t, binary, env, nil,
		"account:role", "author@example.com", "editor", "-yes", "-as", "admin@example.com")
	execSQLiteFile(t, path, "UPDATE auth_users SET role = '' WHERE email = 'author@example.com'")
	granted := runBinary(t, binary, env, nil,
		"account:grant-role", "-role", "author", "-yes", "-as", "admin@example.com")
	listed := runBinary(t, binary, env, nil, "account:list", "-json")
	recorded := runBinary(t, binary, env, nil, "account:records")

	succeeds(t, changed, "set author@example.com to editor")
	succeeds(t, granted, "granted author to 1 account")
	want := map[string]string{"admin@example.com": "admin", "author@example.com": "author"}
	if held := listedRoles(t, listed); !maps.Equal(held, want) {
		t.Errorf("gophenberg account:list -json holds the roles %v, want %v", held, want)
	}
	lines := strings.FieldsFunc(succeeds(t, recorded, ""), func(r rune) bool { return r == '\n' })
	for _, applied := range []string{"admin@example.com  account:role", "admin@example.com  account:grant-role"} {
		if !slices.ContainsFunc(lines, func(line string) bool { return strings.Contains(line, applied) }) {
			t.Errorf("gophenberg account:records lists %q, want a record of %s", lines, applied)
		}
	}
	if len(lines) != 2 {
		t.Errorf("gophenberg account:records lists %d records, want the 2 changes applied", len(lines))
	}
}

// succeeds fails t unless got exited with 0 and its output carries text, and returns that output.
func succeeds(t *testing.T, got testkit.Result, text string) string {
	t.Helper()
	if got.Code != gonsole.ExitDone || !strings.Contains(got.Stdout, text) {
		t.Fatalf("gophenberg = %d, stdout %q, stderr %q, want 0 and %q", got.Code, got.Stdout, got.Stderr, text)
	}
	return got.Stdout
}

// listedRoles returns the role of each account the document account:list -json answered holds, keyed by address.
func listedRoles(t *testing.T, listed testkit.Result) map[string]string {
	t.Helper()
	var listing struct {
		Accounts []struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(succeeds(t, listed, "")), &listing); err != nil {
		t.Fatalf("reading the account:list -json document %q: %v", listed.Stdout, err)
	}
	roles := make(map[string]string, len(listing.Accounts))
	for _, account := range listing.Accounts {
		roles[account.Email] = account.Role
	}
	return roles
}

// execSQLiteFile runs statement on the SQLite file at path through a handle it closes before it returns.
func execSQLiteFile(t *testing.T, path, statement string) {
	t.Helper()
	db, err := openDefaultSQLite(sqlitePrefix + path)
	if err != nil {
		t.Fatalf("opening the SQLite file: %v", err)
	}
	_, err = db.ExecContext(t.Context(), statement)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatalf("%s: %v", statement, err)
	}
}
