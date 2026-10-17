// SPDX-License-Identifier: Apache-2.0

package app

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/webui"
)

func TestServerConfigServesTheWebFolderTheEnvironmentNamed(t *testing.T) {
	t.Parallel()

	settings := timedConfig()
	settings.webDir = t.TempDir()
	writeAdminPage(t, settings.webDir)

	held := serverConfig(settings, site{}, pluginkit.NewHost(), definitions.Walked{}, testLogger(io.Discard))

	if held.Web == nil {
		t.Fatal("Web = nil, want the folder GOPHENBERG_WEB_DIR named")
	}
	if page, err := fs.ReadFile(held.Web, "index.html"); err != nil || string(page) != "admin" {
		t.Errorf("Web serves %q, %v, want the folder's own index.html", page, err)
	}
}

func TestServerConfigServesTheBuiltInAdminWhenNoFolderIsNamed(t *testing.T) {
	t.Parallel()

	held := serverConfig(timedConfig(), site{}, pluginkit.NewHost(), definitions.Walked{}, testLogger(io.Discard))

	if held.Web == nil {
		t.Fatal("Web = nil with no folder named, want the admin built into this binary")
	}
	page, err := fs.ReadFile(held.Web, "index.html")
	builtIn, builtErr := fs.ReadFile(webui.FS(), "index.html")
	if err != nil || builtErr != nil || string(page) != string(builtIn) {
		t.Errorf("Web serves %q (%v), want the built-in admin page %q (%v)", page, err, builtIn, builtErr)
	}
}

func TestLoadRunConfigRefusesAWebFolderWithoutAnIndex(t *testing.T) {
	t.Parallel()

	pageFolder := t.TempDir()
	if err := os.Mkdir(filepath.Join(pageFolder, "index.html"), 0o755); err != nil {
		t.Fatalf("making a folder named index.html: %v", err)
	}
	for name, tc := range map[string]struct {
		folder string
		why    string
	}{
		"an empty folder":                  {t.TempDir(), "holds no index.html"},
		"a missing folder":                 {filepath.Join(t.TempDir(), "web"), "holds no index.html"},
		"a file, no folder":                {writeAdminPage(t, t.TempDir()), "not a directory"},
		"a folder whose index.html is one": {pageFolder, "is a directory"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			refuseWebFolder(t, tc.folder, tc.why)
		})
	}
}

func TestLoadRunConfigNamesWhyItCannotReadTheAdminPage(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root reads a page whatever its mode, so an unreadable page cannot be staged here")
	}
	folder := t.TempDir()
	if err := os.Chmod(writeAdminPage(t, folder), 0o000); err != nil {
		t.Fatalf("making the admin page unreadable: %v", err)
	}

	refuseWebFolder(t, folder, "permission denied")
}

// refuseWebFolder fails unless the settings refuse folder, naming the setting, the folder, why and the way out.
func refuseWebFolder(t *testing.T, folder, why string) {
	t.Helper()
	_, err := loadRunConfig(testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		"GOPHENBERG_WEB_DIR":      folder,
	}))

	if err == nil {
		t.Fatalf("loadRunConfig() error = nil, want %s refused", folder)
	}
	for _, want := range []string{"GOPHENBERG_WEB_DIR", folder, why, "built into this binary"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to name %q", err, want)
		}
	}
}

func TestRunConfigTrimsTheWebFolder(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	writeAdminPage(t, folder)

	settings, err := loadRunConfig(testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		"GOPHENBERG_WEB_DIR":      " " + folder + " ",
	}))

	if err != nil || settings.webDir != folder {
		t.Errorf("loadRunConfig() = %q, %v, want the padded folder %q read trimmed", settings.webDir, err, folder)
	}
}

// writeAdminPage writes an admin page into folder and returns the page's path.
func writeAdminPage(t *testing.T, folder string) string {
	t.Helper()
	page := filepath.Join(folder, "index.html")
	if err := os.WriteFile(page, []byte("admin"), 0o644); err != nil {
		t.Fatalf("writing the admin page: %v", err)
	}
	return page
}

func TestLoadRunConfigTakesAMaxBackoffEqualToTheBackoff(t *testing.T) {
	t.Parallel()

	held, err := loadRunConfig(testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL":      unreachableDatabaseURL,
		"GOPHENBERG_THEME_BACKOFF":     "10s",
		"GOPHENBERG_THEME_MAX_BACKOFF": "10s",
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want an equal max backoff taken", err)
	}
	if held.themeMaxBackoff != 10*time.Second {
		t.Errorf("themeMaxBackoff = %v, want 10s", held.themeMaxBackoff)
	}
}
