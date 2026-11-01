// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/updater"
)

// settingsOf returns the update settings values hold, failing t when they are refused.
func settingsOf(t *testing.T, values map[string]string) updater.Settings {
	t.Helper()
	settings, err := updater.SettingsFrom(envOf(values))
	if err != nil {
		t.Fatalf("SettingsFrom(%v) = %v, want the settings read", values, err)
	}
	return settings
}

// envOf returns the GOPHENBERG_ settings values hold.
func envOf(values map[string]string) gonsole.Env {
	return gonsole.Env{Prefix: "GOPHENBERG_", Getenv: testkit.Getenv(values)}
}

func TestSettingsFromTakesTheDefaultsOfAnInstallWithNoRoot(t *testing.T) {
	t.Parallel()

	settings := settingsOf(t, map[string]string{})

	want := updater.Settings{
		DownloadCap: 200 << 20, DownloadTimeout: 10 * time.Minute, DiskMargin: 64 << 20, BackupsKept: 3,
		PgDump: "pg_dump", Attempts: 3, VerifyTimeout: time.Minute, VersionsKept: 2,
	}
	if settings.Root != want.Root || settings.DownloadCap != want.DownloadCap ||
		settings.DownloadTimeout != want.DownloadTimeout || settings.DiskMargin != want.DiskMargin ||
		settings.BackupsKept != want.BackupsKept || settings.PgDump != want.PgDump {
		t.Errorf("SettingsFrom() of no settings = %+v, want %+v", settings, want)
	}
	if settings.Attempts != want.Attempts || settings.VerifyTimeout != want.VerifyTimeout ||
		settings.VersionsKept != want.VersionsKept {
		t.Errorf("SettingsFrom() of no settings = %+v, want the start check defaults of %+v", settings, want)
	}
}

func TestSettingsFromReadsEveryUpdateSetting(t *testing.T) {
	t.Parallel()

	settings := settingsOf(t, map[string]string{
		"GOPHENBERG_INSTALL_ROOT":            "/opt/gophenberg/",
		"GOPHENBERG_UPDATE_DOWNLOAD_CAP_MB":  "50",
		"GOPHENBERG_UPDATE_DOWNLOAD_TIMEOUT": "90s",
		"GOPHENBERG_UPDATE_DISK_MARGIN_MB":   "0",
		"GOPHENBERG_UPDATE_BACKUPS_KEEP":     "7",
		"GOPHENBERG_PG_DUMP_BIN":             "/usr/lib/postgresql/18/bin/pg_dump",
		"GOPHENBERG_UPDATE_ATTEMPTS":         "5",
		"GOPHENBERG_UPDATE_VERIFY_TIMEOUT":   "2m",
		"GOPHENBERG_UPDATE_VERSIONS_KEEP":    "4",
	})

	if settings.Root != "/opt/gophenberg" || settings.DownloadCap != 50<<20 ||
		settings.DownloadTimeout != 90*time.Second || settings.DiskMargin != 0 || settings.BackupsKept != 7 ||
		settings.PgDump != "/usr/lib/postgresql/18/bin/pg_dump" {
		t.Errorf("SettingsFrom() = %+v, want every value the settings name", settings)
	}
	if settings.Attempts != 5 || settings.VerifyTimeout != 2*time.Minute || settings.VersionsKept != 4 {
		t.Errorf("SettingsFrom() = %+v, want every start check value the settings name", settings)
	}
}

func TestSettingsFromRefusesAValueOutOfItsRange(t *testing.T) {
	t.Parallel()

	for _, setting := range [][2]string{
		{"GOPHENBERG_INSTALL_ROOT", "opt/gophenberg"},
		{"GOPHENBERG_UPDATE_DOWNLOAD_CAP_MB", "0"},
		{"GOPHENBERG_UPDATE_DOWNLOAD_CAP_MB", "1073741825"},
		{"GOPHENBERG_UPDATE_DOWNLOAD_TIMEOUT", "soon"},
		{"GOPHENBERG_UPDATE_DISK_MARGIN_MB", "1073741825"},
		{"GOPHENBERG_UPDATE_BACKUPS_KEEP", "0"},
		{"GOPHENBERG_UPDATE_ATTEMPTS", "0"},
		{"GOPHENBERG_UPDATE_VERIFY_TIMEOUT", "0s"},
		{"GOPHENBERG_UPDATE_VERSIONS_KEEP", "none"},
	} {
		name, value := setting[0], setting[1]

		_, err := updater.SettingsFrom(envOf(map[string]string{name: value}))

		wantRefused(t, err, name+":")
	}
}

func TestSettingsFromRefusesAnInstallRootThatIsNotAbsolute(t *testing.T) {
	t.Parallel()

	_, err := updater.SettingsFrom(envOf(map[string]string{"GOPHENBERG_INSTALL_ROOT": "gophenberg"}))

	wantRefused(t, err, `GOPHENBERG_INSTALL_ROOT: must be an absolute path, got "gophenberg"`)
}

func TestSettingsFromRefusesADataFolderInTheInstallRoot(t *testing.T) {
	t.Parallel()

	for _, data := range []string{"/opt/gophenberg", "/opt/gophenberg/data", "/opt/gophenberg/..data"} {
		_, err := updater.SettingsFrom(envOf(map[string]string{
			"GOPHENBERG_INSTALL_ROOT": "/opt/gophenberg", "GOPHENBERG_DATA_DIR": data,
		}))

		wantRefused(t, err, `GOPHENBERG_DATA_DIR: must stand outside GOPHENBERG_INSTALL_ROOT, got "`+data+`"`)
	}
}

func TestSettingsFromTakesADataFolderOutsideTheInstallRoot(t *testing.T) {
	t.Parallel()

	for _, data := range []string{"/opt/gophenberg-data", "/opt/data", "/srv/gophenberg-data"} {
		_, err := updater.SettingsFrom(envOf(map[string]string{
			"GOPHENBERG_INSTALL_ROOT": "/opt/gophenberg", "GOPHENBERG_DATA_DIR": data,
		}))

		if err != nil {
			t.Errorf("SettingsFrom() with the data folder %s = %v, want nil", data, err)
		}
	}
}
