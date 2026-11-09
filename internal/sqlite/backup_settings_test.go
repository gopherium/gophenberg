// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/sqlite"
)

func TestBackupSettingsFallBackToTheDocumentedDefaults(t *testing.T) {
	t.Parallel()

	got, err := sqlite.BackupSettingsFrom(environment(nil))
	if err != nil {
		t.Fatalf("BackupSettingsFrom() error = %v, want nil", err)
	}

	want := sqlite.BackupSettings{Interval: 24 * time.Hour, Kept: 3, TimeLimit: 15 * time.Minute}
	if got != want {
		t.Errorf("BackupSettingsFrom() = %+v, want %+v", got, want)
	}
}

func TestBackupSettingsReadEveryValueFromTheEnvironment(t *testing.T) {
	t.Parallel()

	got, err := sqlite.BackupSettingsFrom(environment(map[string]string{
		"GOPHENBERG_SQLITE_BACKUP_INTERVAL":   "6h",
		"GOPHENBERG_SQLITE_BACKUPS_KEEP":      "7",
		"GOPHENBERG_SQLITE_BACKUP_TIME_LIMIT": "40m",
		"GOPHENBERG_SQLITE_SKIP_BACKUP_FOR":   "0.21.0-rc.1",
	}))
	if err != nil {
		t.Fatalf("BackupSettingsFrom() error = %v, want nil", err)
	}

	want := sqlite.BackupSettings{Interval: 6 * time.Hour, Kept: 7, TimeLimit: 40 * time.Minute, SkipFor: "0.21.0-rc.1"}
	if got != want {
		t.Errorf("BackupSettingsFrom() = %+v, want %+v", got, want)
	}
}

func TestBackupSettingsTakeTheEdgeValues(t *testing.T) {
	t.Parallel()

	for _, edge := range []struct {
		env  map[string]string
		want sqlite.BackupSettings
	}{
		{
			env: map[string]string{
				"GOPHENBERG_SQLITE_BACKUP_INTERVAL": "0", "GOPHENBERG_SQLITE_BACKUPS_KEEP": "1",
				"GOPHENBERG_SQLITE_BACKUP_TIME_LIMIT": "1s",
			},
			want: sqlite.BackupSettings{Kept: 1, TimeLimit: time.Second},
		},
		{
			env:  map[string]string{"GOPHENBERG_SQLITE_BACKUP_INTERVAL": "1h"},
			want: sqlite.BackupSettings{Interval: time.Hour, Kept: 3, TimeLimit: 15 * time.Minute},
		},
		{
			env: map[string]string{
				"GOPHENBERG_SQLITE_BACKUP_INTERVAL": "744h", "GOPHENBERG_SQLITE_BACKUPS_KEEP": "100",
				"GOPHENBERG_SQLITE_BACKUP_TIME_LIMIT": "24h",
			},
			want: sqlite.BackupSettings{Interval: 744 * time.Hour, Kept: 100, TimeLimit: 24 * time.Hour},
		},
	} {
		got, err := sqlite.BackupSettingsFrom(environment(edge.env))
		if err != nil || got != edge.want {
			t.Errorf("BackupSettingsFrom(%v) = %+v with error %v, want %+v", edge.env, got, err, edge.want)
		}
	}
}

func TestBackupSettingsRefuseABadValueByItsName(t *testing.T) {
	t.Parallel()

	for _, bad := range []struct{ name, value, words string }{
		{"GOPHENBERG_SQLITE_BACKUP_INTERVAL", "10m", `must be 0 or at least 1h, got "10m"`},
		{"GOPHENBERG_SQLITE_BACKUP_INTERVAL", "59m59s", `must be 0 or at least 1h, got "59m59s"`},
		{"GOPHENBERG_SQLITE_BACKUP_INTERVAL", "745h", "must stand at or below 744h0m0s"},
		{"GOPHENBERG_SQLITE_BACKUP_INTERVAL", "-1h", "must not be negative"},
		{"GOPHENBERG_SQLITE_BACKUP_INTERVAL", "daily", "must be a duration"},
		{"GOPHENBERG_SQLITE_BACKUPS_KEEP", "0", "must stand above zero"},
		{"GOPHENBERG_SQLITE_BACKUPS_KEEP", "101", "must stand at or below 100"},
		{"GOPHENBERG_SQLITE_BACKUPS_KEEP", "three", "must be a whole number"},
		{"GOPHENBERG_SQLITE_BACKUP_TIME_LIMIT", "0s", "must stand above zero"},
		{"GOPHENBERG_SQLITE_BACKUP_TIME_LIMIT", "25h", "must stand at or below 24h0m0s"},
		{"GOPHENBERG_SQLITE_SKIP_BACKUP_FOR", "0.21", `must name a version such as 0.21.0, got "0.21"`},
	} {
		_, err := sqlite.BackupSettingsFrom(environment(map[string]string{bad.name: bad.value}))
		if err == nil || !strings.Contains(err.Error(), bad.name+": "+bad.words) {
			t.Errorf("BackupSettingsFrom() with %s=%s error = %v, want %s: %s", bad.name, bad.value, err, bad.name, bad.words)
		}
	}
}

func TestBackupSettingsNameEveryBadValue(t *testing.T) {
	t.Parallel()

	_, err := sqlite.BackupSettingsFrom(environment(map[string]string{
		"GOPHENBERG_SQLITE_BACKUP_INTERVAL":   "10m",
		"GOPHENBERG_SQLITE_BACKUPS_KEEP":      "0",
		"GOPHENBERG_SQLITE_BACKUP_TIME_LIMIT": "0s",
		"GOPHENBERG_SQLITE_SKIP_BACKUP_FOR":   "next",
	}))
	for _, name := range []string{
		"GOPHENBERG_SQLITE_BACKUP_INTERVAL", "GOPHENBERG_SQLITE_BACKUPS_KEEP",
		"GOPHENBERG_SQLITE_BACKUP_TIME_LIMIT", "GOPHENBERG_SQLITE_SKIP_BACKUP_FOR",
	} {
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("BackupSettingsFrom() error = %v, want one naming %s", err, name)
		}
	}
}

func TestTheSkipSettingMustNameAFullVersion(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"0.21", "v0.21.0", "0.21.0+build.7", "0.21.0-", "latest"} {
		_, err := sqlite.BackupSettingsFrom(environment(map[string]string{"GOPHENBERG_SQLITE_SKIP_BACKUP_FOR": value}))
		if err == nil || !strings.Contains(err.Error(), "must name a version such as 0.21.0") {
			t.Errorf("BackupSettingsFrom() with GOPHENBERG_SQLITE_SKIP_BACKUP_FOR=%s error = %v, want a refusal", value, err)
		}
	}
	for _, value := range []string{"0.21.0", "1.0.0-rc.2"} {
		got, err := sqlite.BackupSettingsFrom(environment(map[string]string{"GOPHENBERG_SQLITE_SKIP_BACKUP_FOR": value}))
		if err != nil || got.SkipFor != value {
			t.Errorf("BackupSettingsFrom() with GOPHENBERG_SQLITE_SKIP_BACKUP_FOR=%s = %+v with error %v, want it kept",
				value, got, err)
		}
	}
}
