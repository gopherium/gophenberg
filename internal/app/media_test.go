// SPDX-License-Identifier: Apache-2.0

package app

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/gopherium/framework/gonsole/testkit"
)

func TestLoadRunConfigReadsTheMediaDirectory(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		"GOPHENBERG_MEDIA_DIR":    "/srv/media",
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	if settings.mediaDir != "/srv/media" {
		t.Errorf("mediaDir = %q, want /srv/media", settings.mediaDir)
	}
}

func TestLoadRunConfigKeepsMediaInTheDataFolderWhenNothingNamesAMediaFolder(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		"GOPHENBERG_DATA_DIR":     "/srv/gophenberg-data",
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	if want := filepath.Join("/srv/gophenberg-data", "data", "media"); settings.mediaDir != want {
		t.Errorf("mediaDir = %q, want %q", settings.mediaDir, want)
	}
}

func TestLoadRunConfigTakesTheMediaFolderTheSettingsNameOverTheDataFolder(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		"GOPHENBERG_DATA_DIR":     "/srv/gophenberg-data",
		"GOPHENBERG_MEDIA_DIR":    "/srv/media",
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	if settings.mediaDir != "/srv/media" {
		t.Errorf("mediaDir = %q, want /srv/media", settings.mediaDir)
	}
}

func TestSeedStoresTheDemoMediaInTheDataFolderWhenNothingNamesAMediaFolder(t *testing.T) {
	t.Parallel()

	data := t.TempDir()
	env := map[string]string{"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t), "GOPHENBERG_DATA_DIR": data}

	if err := seedMigrated(t.Context(), testkit.Getenv(env), io.Discard); err != nil {
		t.Fatalf("seedMigrated() error = %v, want nil", err)
	}

	stored, err := os.ReadDir(filepath.Join(data, "data", "media"))
	if err != nil || len(stored) == 0 {
		t.Errorf("the media folder of the data folder holds %v (%v), want the demo pictures", stored, err)
	}
}

func TestLoadRunConfigLeavesMediaOffWithoutADirectory(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	if settings.mediaDir != "" {
		t.Errorf("mediaDir = %q, want it empty when the environment names none", settings.mediaDir)
	}
}
