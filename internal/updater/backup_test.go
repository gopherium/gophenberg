// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"

	"github.com/gopherium/gophenberg/internal/installer"
	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/internal/updater"
)

// dumpBody is what every fake pg_dump writes as its dump.
const dumpBody = "a dump of the site database"

// fakeDump is a pg_dump stand-in: the version line it prints, the exit code of a dump and the log of its last dump.
type fakeDump struct {
	bin  string
	log  string
	line string
	exit int
}

// newFakeDump writes a pg_dump stand-in that prints line for --version and exits with exit after a dump.
func newFakeDump(t *testing.T, line string, exit int) fakeDump {
	t.Helper()
	return scriptedDump(t, line, exit, "")
}

// scriptedDump writes a pg_dump stand-in like newFakeDump that runs the shell line first before each dump.
func scriptedDump(t *testing.T, line string, exit int, first string) fakeDump {
	t.Helper()
	dir := t.TempDir()
	dump := fakeDump{bin: filepath.Join(dir, "pg_dump"), log: filepath.Join(dir, "dump.log"), line: line, exit: exit}
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "--version" ]; then
  echo %q
  exit 0
fi
%sfor arg in "$@"
do
  printf '%%s\n' "$arg"
done > %q
printf 'PGPASSWORD=%%s\n' "$PGPASSWORD" >> %q
if [ %d -ne 0 ]; then
  echo "pg_dump: error: connection to server failed" >&2
  printf 'half a dump'
  exit %d
fi
printf %q
`, line, first, dump.log, dump.log, exit, exit, dumpBody)
	if err := os.WriteFile(dump.bin, []byte(script), 0o755); err != nil {
		t.Fatalf("writing the fake pg_dump: %v", err)
	}
	return dump
}

// pgDumpOf returns a pg_dump stand-in of the PostgreSQL major that dumps without a failure.
func pgDumpOf(t *testing.T, major int) fakeDump {
	t.Helper()
	return newFakeDump(t, fmt.Sprintf("pg_dump (PostgreSQL) %d.2 (Ubuntu %d.2-1.pgdg24.04+1)", major, major), 0)
}

// stateCopy returns the shell line that copies the state file of layout over the state file of copies.
func stateCopy(t *testing.T, layout, copies updater.Layout) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(copies.StateFile()), 0o755); err != nil {
		t.Fatalf("making the state folder of %s: %v", copies.Root, err)
	}
	return fmt.Sprintf("cp %q %q\n", layout.StateFile(), copies.StateFile())
}

// logged returns the lines the fake pg_dump logged of its last dump, failing t when it did not dump.
func (d fakeDump) logged(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile(d.log)
	if err != nil {
		t.Fatalf("reading the log of the fake pg_dump: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
}

// backupInto returns the backup of the database at address into a new folder, with the fake dump.
func backupInto(t *testing.T, dump fakeDump, address string) updater.Backup {
	t.Helper()
	return updater.Backup{
		PgDump: dump.bin, DatabaseURL: address, Folder: filepath.Join(t.TempDir(), "backups"),
		Name: "0.20.0-20261020T120000Z.dump", Keep: 3,
	}
}

// folderNames returns the names in the folder at path, sorted.
func folderNames(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("listing %s: %v", path, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestDumpMajorReadsTheMajorPgDumpPrints(t *testing.T) {
	t.Parallel()

	major, err := updater.DumpMajor(t.Context(), pgDumpOf(t, 17).bin)

	if err != nil || major != 17 {
		t.Errorf("DumpMajor() = %d, %v, want 17 and nil", major, err)
	}
}

func TestDumpMajorFindsPgDumpOnThePath(t *testing.T) {
	dump := pgDumpOf(t, 18)
	t.Setenv("PATH", filepath.Dir(dump.bin))

	major, err := updater.DumpMajor(t.Context(), "pg_dump")

	if err != nil || major != 18 {
		t.Errorf("DumpMajor(pg_dump) = %d, %v, want the 18 of the pg_dump on the path", major, err)
	}
}

func TestDumpMajorRefusesAPgDumpThatCannotRun(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "pg_dump")

	_, err := updater.DumpMajor(t.Context(), missing)

	wantRefused(t, err, "updater: run "+missing+" --version")
}

func TestDumpMajorRefusesAPgDumpThatNamesNoVersion(t *testing.T) {
	t.Parallel()

	dump := newFakeDump(t, "pg_dump, a dumper", 0)

	_, err := updater.DumpMajor(t.Context(), dump.bin)

	wantRefused(t, err, `updater: `+dump.bin+` --version printed "pg_dump, a dumper", which names no PostgreSQL version`)
}

func TestTakeWritesTheDumpAndPassesThePasswordOutsideTheCommandLine(t *testing.T) {
	t.Parallel()

	dump := pgDumpOf(t, 18)
	backup := backupInto(t, dump, "postgres://maria:s3cret@db.example.com:5432/site?sslmode=require")

	if err := backup.Take(t.Context()); err != nil {
		t.Fatalf("Take() = %v, want nil", err)
	}

	kept := filepath.Join(backup.Folder, backup.Name)
	wantFile(t, kept, dumpBody)
	if info, err := os.Stat(kept); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the backup has mode %v, %v, want 0600, since it holds every row of the site", info.Mode(), err)
	}
	if names := folderNames(t, backup.Folder); !slices.Equal(names, []string{backup.Name}) {
		t.Errorf("the backups folder holds %v, want only the backup", names)
	}
	want := []string{
		"--format=custom", "--no-password", "--dbname=postgres://maria@db.example.com:5432/site?sslmode=require",
		"PGPASSWORD=s3cret",
	}
	if logged := dump.logged(t); !slices.Equal(logged, want) {
		t.Errorf("pg_dump ran with %q, want %q", logged, want)
	}
}

func TestTakeMovesThePasswordOfTheQueryOutOfTheCommandLine(t *testing.T) {
	t.Parallel()

	dump := pgDumpOf(t, 18)
	backup := backupInto(t, dump, "postgres://maria@db.example.com/site?password=s3cret&sslmode=require")

	if err := backup.Take(t.Context()); err != nil {
		t.Fatalf("Take() = %v, want nil", err)
	}

	want := []string{
		"--format=custom", "--no-password", "--dbname=postgres://maria@db.example.com/site?sslmode=require",
		"PGPASSWORD=s3cret",
	}
	if logged := dump.logged(t); !slices.Equal(logged, want) {
		t.Errorf("pg_dump ran with %q, want %q", logged, want)
	}
}

func TestTakeLeavesThePasswordToTheEnvironmentWhenTheAddressNamesNone(t *testing.T) {
	t.Setenv("PGPASSWORD", "from the environment")
	dump := pgDumpOf(t, 18)
	backup := backupInto(t, dump, "postgresql://db.example.com/site")

	if err := backup.Take(t.Context()); err != nil {
		t.Fatalf("Take() = %v, want nil", err)
	}

	want := []string{
		"--format=custom", "--no-password", "--dbname=postgresql://db.example.com/site",
		"PGPASSWORD=from the environment",
	}
	if logged := dump.logged(t); !slices.Equal(logged, want) {
		t.Errorf("pg_dump ran with %q, want %q", logged, want)
	}
}

func TestTakeHandsPgDumpTheQueryAsWrittenWhenItHoldsNoPassword(t *testing.T) {
	t.Parallel()

	dump := pgDumpOf(t, 18)
	query := "?options=-c%20statement_timeout%3D0&application_name=Maria+Site"
	backup := backupInto(t, dump, "postgres://maria:s3cret@db.example.com/site"+query)

	if err := backup.Take(t.Context()); err != nil {
		t.Fatalf("Take() = %v, want nil", err)
	}

	want := []string{
		"--format=custom", "--no-password", "--dbname=postgres://maria@db.example.com/site" + query, "PGPASSWORD=s3cret",
	}
	if logged := dump.logged(t); !slices.Equal(logged, want) {
		t.Errorf("pg_dump ran with %q, want %q", logged, want)
	}
}

func TestTakeHandsPgDumpThePeerAddressOfAServerInstallAsWritten(t *testing.T) {
	t.Setenv("PGPASSWORD", "")
	address := installer.DatabaseAddress
	dump := pgDumpOf(t, 16)

	if err := dbkitpg.CheckAddress(address); err != nil {
		t.Errorf("CheckAddress(%q) = %v, want the address taken", address, err)
	}
	config, err := pgx.ParseConfig(address)
	if err != nil {
		t.Fatalf("pgx.ParseConfig(%q) = %v, want the address read", address, err)
	}
	read := [3]string{config.Host, config.User, config.Database}
	if read != [3]string{"/var/run/postgresql", "gophenberg", "gophenberg"} {
		t.Errorf("pgx reads %q as the host, user and database %q, want the gophenberg role and database over the "+
			"local socket", address, read)
	}
	if err := backupInto(t, dump, address).Take(t.Context()); err != nil {
		t.Fatalf("Take() = %v, want nil", err)
	}

	want := []string{"--format=custom", "--no-password", "--dbname=" + address, "PGPASSWORD="}
	if logged := dump.logged(t); !slices.Equal(logged, want) {
		t.Errorf("pg_dump ran with %q, want %q", logged, want)
	}
}

func TestTakeWritesASpaceOfTheQueryAsLibpqReadsItWhenItMovesThePassword(t *testing.T) {
	t.Parallel()

	dump := pgDumpOf(t, 18)
	options := "options=-c%20statement_timeout%3D0"
	backup := backupInto(t, dump, "postgres://maria@db.example.com/site?password=s3cret&"+options)

	if err := backup.Take(t.Context()); err != nil {
		t.Fatalf("Take() = %v, want nil", err)
	}

	want := []string{
		"--format=custom", "--no-password", "--dbname=postgres://maria@db.example.com/site?" + options,
		"PGPASSWORD=s3cret",
	}
	if logged := dump.logged(t); !slices.Equal(logged, want) {
		t.Errorf("pg_dump ran with %q, want %q", logged, want)
	}
}

func TestTakeBacksUpTheTestDatabaseWithARealPgDump(t *testing.T) {
	t.Parallel()

	address := emptyDatabase(t)
	pgDump, pgRestore := realTool(t, "pg_dump"), realTool(t, "pg_restore")
	major, err := updater.DumpMajor(t.Context(), pgDump)
	if server := serverMajor(t, testdb.Open(t, address)); err != nil || major < server {
		t.Skipf("pg_dump on the path is %d, %v, older than PostgreSQL %d of the test server", major, err, server)
	}
	backup := updater.Backup{
		PgDump: pgDump, DatabaseURL: address + "&options=-c%20statement_timeout%3D0",
		Folder: filepath.Join(t.TempDir(), "backups"), Name: "0.20.0-20261020T120000Z.dump", Keep: 3,
	}

	if err := backup.Take(t.Context()); err != nil {
		t.Fatalf("Take() with the real pg_dump = %v, want nil", err)
	}

	listed, err := exec.CommandContext(t.Context(), pgRestore, "--list", filepath.Join(backup.Folder, backup.Name)).
		CombinedOutput()
	if err != nil {
		t.Errorf("pg_restore --list of the backup = %v: %s, want the backup read", err, listed)
	}
}

// realTool returns the PostgreSQL client tool called name on the path, skipping t when the path holds none.
func realTool(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("no %s on the path: %v", name, err)
	}
	return path
}

func TestTakeRefusesAnAddressThatIsNotAPostgresURL(t *testing.T) {
	t.Parallel()

	for _, address := range []string{"host=db.example.com password=s3cret", "postgres://db.example.com/%zz"} {
		dump := pgDumpOf(t, 18)

		err := backupInto(t, dump, address).Take(t.Context())

		wantRefused(t, err, "updater: the backup reads the database address only as a postgres:// URL")
		wantMissing(t, dump.log)
	}
}

func TestTakeKeepsOnlyTheNewestBackups(t *testing.T) {
	t.Parallel()

	backup := backupInto(t, pgDumpOf(t, 18), "postgres://db.example.com/site")
	held := []string{
		"0.19.0-20261001T000000Z.dump", "0.19.5-rc.1-20261010T000000Z.dump", "0.20.0-20261015T000000Z.dump",
		"0.19.0-later.dump", "notes.txt",
	}
	if err := os.MkdirAll(backup.Folder, 0o755); err != nil {
		t.Fatalf("making the backups folder: %v", err)
	}
	for _, name := range held {
		if err := os.WriteFile(filepath.Join(backup.Folder, name), []byte("an older dump"), 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	if err := backup.Take(t.Context()); err != nil {
		t.Fatalf("Take() = %v, want nil", err)
	}

	want := []string{
		"0.19.0-later.dump", "0.19.5-rc.1-20261010T000000Z.dump", "0.20.0-20261015T000000Z.dump", backup.Name,
		"notes.txt",
	}
	if names := folderNames(t, backup.Folder); !slices.Equal(names, want) {
		t.Errorf("the backups folder holds %v, want the newest 3 backups and every other file %v", names, want)
	}
}

// placeBackups writes an older dump under each of names into the backups folder of backup, failing t when it cannot.
func placeBackups(t *testing.T, backup updater.Backup, names ...string) {
	t.Helper()
	if err := os.MkdirAll(backup.Folder, 0o755); err != nil {
		t.Fatalf("making the backups folder: %v", err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(backup.Folder, name), []byte("an older dump"), 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
}

func TestTakeKeepsTheBackupItTookWhenTheClockWentBack(t *testing.T) {
	t.Parallel()

	backup := backupInto(t, pgDumpOf(t, 18), "postgres://db.example.com/site")
	backup.Keep = 2
	placeBackups(t, backup,
		"0.19.0-20261024T000000Z.dump", "0.20.0-20261025T000000Z.dump", "0.20.0-20261030T000000Z.dump")

	if err := backup.Take(t.Context()); err != nil {
		t.Fatalf("Take() = %v, want nil", err)
	}

	want := []string{backup.Name, "0.20.0-20261030T000000Z.dump"}
	if names := folderNames(t, backup.Folder); !slices.Equal(names, want) {
		t.Errorf("the backups folder holds %v, want the backup just taken and the newest other %v", names, want)
	}
}

func TestTakeKeepsTheBackupTheStateNamesPastTheNumberItKeeps(t *testing.T) {
	t.Parallel()

	backup := backupInto(t, pgDumpOf(t, 18), "postgres://db.example.com/site")
	backup.Keep, backup.Held = 1, "0.19.0-20261001T000000Z.dump"
	placeBackups(t, backup, backup.Held, "0.19.5-20261010T000000Z.dump")

	if err := backup.Take(t.Context()); err != nil {
		t.Fatalf("Take() = %v, want nil", err)
	}

	want := []string{backup.Held, backup.Name}
	if names := folderNames(t, backup.Folder); !slices.Equal(names, want) {
		t.Errorf("the backups folder holds %v, want the backup the state names and the one just taken %v", names, want)
	}
}

func TestTakeLeavesTheSQLiteBackupsWhenItPrunes(t *testing.T) {
	t.Parallel()

	backup := backupInto(t, pgDumpOf(t, 18), "postgres://db.example.com/site")
	backup.Keep = 1
	copies := []string{
		"0.19.0-20261002T000000Z-update.sqlite", "0.19.0-20261003T000000Z-scheduled.sqlite",
		"0.19.0-20261004T000000Z-manual.sqlite",
	}
	placeBackups(t, backup, append([]string{"0.19.0-20261001T000000Z.dump"}, copies...)...)

	if err := backup.Take(t.Context()); err != nil {
		t.Fatalf("Take() = %v, want nil", err)
	}

	want := append(slices.Clone(copies), backup.Name)
	if names := folderNames(t, backup.Folder); !slices.Equal(names, want) {
		t.Errorf("the backups folder holds %v, want every SQLite backup beside the dump just taken %v", names, want)
	}
}

func TestTakeRefusesAFailedDumpAndKeepsNoPartOfIt(t *testing.T) {
	t.Parallel()

	backup := backupInto(t, newFakeDump(t, "pg_dump (PostgreSQL) 18.0", 2), "postgres://db.example.com/site")

	err := backup.Take(t.Context())

	wantRefused(t, err, "updater: back up the database with "+backup.PgDump+": exit status 2: "+
		"pg_dump: error: connection to server failed")
	if names := folderNames(t, backup.Folder); len(names) != 0 {
		t.Errorf("the backups folder holds %v after a failed dump, want nothing", names)
	}
}

func TestTakeRefusesABackupsFolderItCannotWrite(t *testing.T) {
	t.Parallel()

	backup := backupInto(t, pgDumpOf(t, 18), "postgres://db.example.com/site")
	if err := os.MkdirAll(backup.Folder, 0o755); err != nil {
		t.Fatalf("making the backups folder: %v", err)
	}
	denied(t, backup.Folder, 0o500)

	err := backup.Take(t.Context())

	wantRefused(t, err, "updater: write the backup")
}

func TestTakeRefusesABackupsFolderItCannotSaveToDisk(t *testing.T) {
	t.Parallel()

	backup := backupInto(t, pgDumpOf(t, 18), "postgres://db.example.com/site")
	if err := os.MkdirAll(backup.Folder, 0o755); err != nil {
		t.Fatalf("making the backups folder: %v", err)
	}
	denied(t, backup.Folder, 0o300)

	err := backup.Take(t.Context())

	wantRefused(t, err, "updater: save the backup to disk: open "+backup.Folder+": permission denied")
}

func TestTakeRefusesABackupItCannotKeepUnderItsName(t *testing.T) {
	t.Parallel()

	backup := backupInto(t, pgDumpOf(t, 18), "postgres://db.example.com/site")
	if err := os.MkdirAll(filepath.Join(backup.Folder, backup.Name, "kept"), 0o755); err != nil {
		t.Fatalf("making a folder under the name of the backup: %v", err)
	}

	err := backup.Take(t.Context())

	wantRefused(t, err, "updater: keep the backup")
	wantMissing(t, filepath.Join(backup.Folder, backup.Name+".partial"))
}
