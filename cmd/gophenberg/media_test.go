// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestLoadRunConfigReadsTheMediaDirectory(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
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

func TestLoadRunConfigLeavesMediaOffWithoutADirectory(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	if settings.mediaDir != "" {
		t.Errorf("mediaDir = %q, want it empty when the environment names none", settings.mediaDir)
	}
}
