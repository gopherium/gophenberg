// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"path/filepath"
	"time"
)

// Reason names why an install cannot update itself from the admin.
type Reason string

const (
	// NoInstallRoot is an install with no install root, like the Docker image, which only a person updates.
	NoInstallRoot Reason = "no_install_root"
	// CustomBuild is a binary the release workflow did not build, which an official archive would replace.
	CustomBuild Reason = "custom_build"
)

// Overview is what the admin shows of an install root: the versions it refuses, its last update and its backups.
type Overview struct {
	Refused []string    `json:"refused"`
	Job     *LastUpdate `json:"job"`
	Backups BackupCount `json:"backups"`
}

// LastUpdate is the update the state file names: its step, the versions it went from and to, and the last failure.
type LastUpdate struct {
	Step    Step        `json:"step"`
	From    string      `json:"from"`
	To      string      `json:"to"`
	Failure *JobFailure `json:"failure"`
}

// BackupCount is how many backups the install root keeps, how many it may keep, and when the newest was taken.
type BackupCount struct {
	Kept   int        `json:"kept"`
	Limit  int        `json:"limit"`
	Newest *time.Time `json:"newest"`
}

// ReadOverview returns what the admin shows of the install root the settings name, read as update:status reads it.
func ReadOverview(settings Settings) (Overview, error) {
	status, err := ReadStatus(settings)
	if err != nil {
		return Overview{}, err
	}
	return Overview{
		Refused: status.Update.Refused, Job: status.lastUpdate(),
		Backups: settings.Layout().backupCount(settings.BackupsKept),
	}, nil
}

// CannotUpdate returns why the install the settings name cannot update itself under stamp, empty when it can.
func CannotUpdate(settings Settings, stamp string) Reason {
	r := &run{job: Job{Settings: settings, Stamp: stamp}, layout: settings.Layout()}
	for _, check := range []func() error{r.official, r.rooted} {
		if err := check(); err != nil {
			return ReasonOf(err)
		}
	}
	return ""
}

// lastUpdate returns the update the status names with its failure, nil when it names neither an update nor a failure.
func (s Status) lastUpdate() *LastUpdate {
	held := s.Update
	if held.Step == "" && held.Failure == nil {
		return nil
	}
	return &LastUpdate{Step: held.Step, From: held.From, To: held.To, Failure: held.Failure}
}

// backupCount returns how many backups the backups folder keeps, the limit given and when the newest was taken.
func (l Layout) backupCount(limit int) BackupCount {
	count := BackupCount{Limit: limit}
	if l.Root == "" {
		return count
	}
	backups := datedBackups(l.Backups())
	count.Kept = len(backups)
	if len(backups) > 0 {
		count.Newest = &backups[0].at
	}
	return count
}

// Status is what update:status shows of an install root: the builds its links name, the last update and the backups.
type Status struct {
	Root     string   `json:"root"`
	Data     string   `json:"data"`
	Current  string   `json:"current"`
	Previous string   `json:"previous"`
	Update   State    `json:"update"`
	Restore  string   `json:"restore"`
	Backups  []string `json:"backups"`
}

// ReadStatus returns the status of the install root the settings name, an empty one for an install with no root.
func ReadStatus(settings Settings) (Status, error) {
	if settings.Root == "" {
		return Status{Update: State{}.WithEmptyLists(), Backups: []string{}}, nil
	}
	layout := settings.Layout()
	state, err := layout.ReadState()
	if err != nil {
		return Status{}, err
	}
	status := Status{Root: layout.Root, Data: layout.Kept(), Update: state.WithEmptyLists(), Backups: layout.backups()}
	status.Current, _ = layout.Named(Current)
	status.Previous, _ = layout.Named(Previous)
	if state.Step == StepFailed {
		status.Restore = layout.restoreText(state)
	}
	return status, nil
}

// backups returns each backup the backups folder keeps, relative to the folder that keeps it, the newest first.
func (l Layout) backups() []string {
	dated := datedBackups(l.Backups())
	names := make([]string, 0, len(dated))
	for _, backup := range dated {
		names = append(names, filepath.Join(backupsFolder, backup.name))
	}
	return names
}
