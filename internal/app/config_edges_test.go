// SPDX-License-Identifier: Apache-2.0

package app

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/internal/definitions"
)

func TestServerConfigServesTheWebFolderTheEnvironmentNamed(t *testing.T) {
	t.Parallel()

	settings := timedConfig()
	settings.webDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(settings.webDir, "index.html"), []byte("admin"), 0o644); err != nil {
		t.Fatalf("writing the admin page: %v", err)
	}

	held := serverConfig(settings, site{}, pluginkit.NewHost(), definitions.Walked{}, testLogger(io.Discard))

	if held.Web == nil {
		t.Fatal("Web = nil, want the folder GOPHENBERG_WEB_DIR named")
	}
	if page, err := fs.ReadFile(held.Web, "index.html"); err != nil || string(page) != "admin" {
		t.Errorf("Web serves %q, %v, want the folder's own index.html", page, err)
	}
}

func TestServerConfigServesNoWebFolderWhenNoneIsNamed(t *testing.T) {
	t.Parallel()

	held := serverConfig(timedConfig(), site{}, pluginkit.NewHost(), definitions.Walked{}, testLogger(io.Discard))

	if held.Web != nil {
		t.Errorf("Web = %v with no folder named, want nil", held.Web)
	}
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
