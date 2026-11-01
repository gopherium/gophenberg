// SPDX-License-Identifier: Apache-2.0

package app

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/themehost"
	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/updater"
)

// updatesPage is the page of the manual that shows how a site updates, by hand or on its own.
const updatesPage = "https://docs.gophenberg.org/self-hosting/updates-and-backups/"

// updatePhases say where the last update stands at each step, after the versions it went from and to.
var updatePhases = map[updater.Step]string{
	updater.StepDownload:   "stopped while it downloaded the archive, and the next update:apply clears what it left",
	updater.StepStage:      "stopped while it unpacked the new build, and the next update:apply clears what it left",
	updater.StepBackup:     "stopped while it backed up the database, and the next update:apply clears what it left",
	updater.StepSwitched:   "waits for the new build to start and pass its start check",
	updater.StepCommitted:  "kept, since the new build passed its start check",
	updater.StepFailed:     "the new build raised the minimum version and failed its start check, so the site stays down",
	updater.StepRolledBack: "taken back, so current names the build before it again",
}

// updateCommands returns the commands that check for, apply, take back and show the updates of the core.
func updateCommands(getenv func(string) string, exits *exitCodes, stamp string) []gonsole.Command {
	return []gonsole.Command{
		updateCheckCommand(), updateApplyCommand(getenv, exits, stamp), updateRollbackCommand(exits),
		updateStatusCommand(),
	}
}

// updateApplyCommand returns the command that updates a server install to a newer Gophenberg and asks for a restart.
func updateApplyCommand(getenv func(string) string, exits *exitCodes, stamp string) gonsole.Command {
	return gonsole.Command{
		Name: "update:apply", Summary: "update this server install to a newer Gophenberg, then ask for a restart",
		Flags: applyFlags, JSON: true, Run: postgresOnly(getenv, "update:apply", exits.keep(applyUpdate(stamp))),
	}
}

// applyFlags declares the flags of update:apply.
func applyFlags(fs *flag.FlagSet) {
	fs.String("to", "", "the `version` to update to, the newest one the update site offers this site when empty")
	fs.Bool("retry", false, "take the version in -to again, although it failed its start check after an update")
	fs.Bool("skip-backup", false, "switch with no database backup, for a release that keeps the minimum version")
}

// applyUpdate returns the run of update:apply by a binary of the build stamp given, asking for a restart once it ends.
func applyUpdate(stamp string) func(context.Context, gonsole.Call) error {
	return func(ctx context.Context, call gonsole.Call) error {
		if call.Flags["retry"] == "true" && call.Flags["to"] == "" {
			return gonsole.Misuse(errors.New("update:apply -retry takes the version to try again in -to"))
		}
		job, release, err := applyJob(ctx, call, stamp)
		if err != nil {
			return err
		}
		defer release()
		state, err := job.Apply(ctx)
		if err != nil {
			return err
		}
		summary := switchSummary(job.Settings.Layout().Kept(), state)
		return errors.Join(printState(call, summary, state), restartError{release: state.To})
	}
}

// applyJob returns the job update:apply runs over the settings and the database of call, and what closes its handle.
func applyJob(ctx context.Context, call gonsole.Call, stamp string) (updater.Job, func(), error) {
	install, installErr := updater.SettingsFrom(call.Env)
	updates, updatesErr := updateSettingsFrom(call.Env)
	address, addressErr := call.DatabaseURL()
	if err := errors.Join(installErr, updatesErr, addressErr); err != nil {
		return updater.Job{}, nil, err
	}
	handle, err := openPostgres(call.Env, address)
	if err != nil {
		return updater.Job{}, nil, err
	}
	job, err := withTheme(ctx, call.Env, updater.Job{
		Settings: install, Check: updates.config(0), Store: postgres.NewSettingStore(handle.Pool), DB: handle.DB,
		DatabaseURL: address, Stamp: stamp, To: call.Flags["to"], Retry: call.Flags["retry"] == "true",
		SkipBackup: call.Flags["skip-backup"] == "true",
	})
	if err != nil {
		return updater.Job{}, nil, errors.Join(err, handle.Close())
	}
	return job, func() { _ = handle.Close() }, nil
}

// withTheme returns job with the theme kit the theme the site chose is built on, or with why that theme does not load.
func withTheme(ctx context.Context, env gonsole.Env, job updater.Job) (updater.Job, error) {
	chosen, err := chosenTheme(ctx, env, job.Store)
	if err != nil {
		return updater.Job{}, err
	}
	loaded, err := themehost.Load(env.Value("THEMES_DIR"), chosen)
	job.ThemeError = err
	if loaded != nil {
		job.Kit = loaded.Kit
	}
	return job, nil
}

// chosenTheme returns the theme the site serves through, the one the settings pin ahead of the one the site stored.
func chosenTheme(ctx context.Context, env gonsole.Env, store update.Settings) (string, error) {
	if pinned := env.Value("THEME"); pinned != "" {
		return pinned, nil
	}
	stored, _, err := store.Lookup(ctx, themehost.ActiveKey)
	return stored, err
}

// switchSummary returns the line update:apply prints once it switched builds, naming the backup kept in folder.
func switchSummary(folder string, state updater.State) string {
	line := "switched from Gophenberg " + state.From + " to " + state.To
	if state.Backup == "" {
		return line + ", with no database backup"
	}
	return line + ", and the database backup is " + filepath.Join(folder, state.Backup)
}

// printState prints the state a command ended at, one document under -json or the summary line for people.
func printState(call gonsole.Call, summary string, state updater.State) error {
	if call.JSON {
		return call.Encode(state.WithEmptyLists())
	}
	_, err := fmt.Fprintln(call.Stdout, summary)
	return err
}

// updateRollbackCommand returns the command that takes back the last update, by hand or as the rollback unit runs it.
func updateRollbackCommand(exits *exitCodes) gonsole.Command {
	return gonsole.Command{
		Name: "update:rollback", Summary: "take back the last update, pointing current at the build before it",
		Flags: rollbackFlags, JSON: true, Run: exits.keep(runRollback),
	}
}

// rollbackFlags declares the flags of update:rollback.
func rollbackFlags(fs *flag.FlagSet) {
	fs.Bool("after-failure", false, "act as the rollback unit of the service manager, only after a start that failed")
}

// runRollback takes back the last update, asking for a restart unless the rollback unit of the service manager runs it.
func runRollback(_ context.Context, call gonsole.Call) error {
	settings, err := updater.SettingsFrom(call.Env)
	if err != nil {
		return err
	}
	afterFailure := call.Flags["after-failure"] == "true"
	state, err := rolledBack(settings, call.Env, afterFailure)
	if err != nil {
		return err
	}
	summary := "took back the update from Gophenberg " + state.From + " to " + state.To + ", so current names " +
		state.From + " again"
	printed := printState(call, summary, state)
	if afterFailure {
		return printed
	}
	return errors.Join(printed, restartError{release: state.From})
}

// rolledBack takes back the last update, after a failed start only when the service manager says one failed.
func rolledBack(settings updater.Settings, env gonsole.Env, afterFailure bool) (updater.State, error) {
	if !afterFailure {
		return updater.Rollback(settings)
	}
	return updater.RollbackAfterFailure(settings, updater.Failure{ServiceResult: env.Getenv("MONITOR_SERVICE_RESULT")})
}

// updateStatusCommand returns the command that shows the builds of a server install, its last update and backups.
func updateStatusCommand() gonsole.Command {
	return gonsole.Command{
		Name: "update:status", Summary: "show the builds of this server install, its last update and its backups",
		JSON: true, Run: runStatus,
	}
}

// runStatus prints the status of the install root, one document under -json or one line for each part for people.
func runStatus(_ context.Context, call gonsole.Call) error {
	settings, err := updater.SettingsFrom(call.Env)
	if err != nil {
		return err
	}
	status, err := updater.ReadStatus(settings)
	if err != nil {
		return err
	}
	if call.JSON {
		return call.Encode(status)
	}
	_, err = io.WriteString(call.Stdout, statusText(call.Env, status))
	return err
}

// statusText returns what update:status prints for people, one line for each part of the status.
func statusText(env gonsole.Env, status updater.Status) string {
	if status.Root == "" {
		return env.Key("INSTALL_ROOT") + " is empty, so this install updates by hand, as " + updatesPage + " shows\n"
	}
	backups := make([]string, 0, len(status.Backups))
	for _, backup := range status.Backups {
		backups = append(backups, filepath.Join(status.Data, backup))
	}
	lines := append([]string{
		"install root: " + status.Root,
		"current build: " + cmp.Or(status.Current, "none"),
		"previous build: " + cmp.Or(status.Previous, "none"),
		"last update: " + lastUpdateText(status.Update),
	}, failureLines(status.Update.Failure)...)
	lines = append(lines,
		"refused versions: "+cmp.Or(strings.Join(status.Update.Refused, ", "), "none"),
		"backups kept: "+cmp.Or(strings.Join(backups, ", "), "none"),
	)
	if status.Restore != "" {
		lines = append(lines, "to bring the site back: "+status.Restore)
	}
	return strings.Join(lines, "\n") + "\n"
}

// failureLines returns the line update:status prints of which job failed last, when, where and why, none for none.
func failureLines(failure *updater.JobFailure) []string {
	if failure == nil {
		return nil
	}
	return []string{"last failure: from " + failure.From + " to " + failure.To + ", at the step " +
		string(failure.Step) + " on " + failure.At.UTC().Format(time.RFC3339) + ": " + failure.Error}
}

// lastUpdateText returns what update:status says of the last update the state names.
func lastUpdateText(state updater.State) string {
	versions := "from " + state.From + " to " + state.To + ", "
	switch {
	case state.Step == "":
		return "none yet"
	case state.Step == updater.StepSwitched && slices.Contains(state.Refused, state.To):
		return versions + "the new build failed its start check, so current names " + state.From + " again"
	}
	return versions + cmp.Or(updatePhases[state.Step], "at the step "+string(state.Step))
}
