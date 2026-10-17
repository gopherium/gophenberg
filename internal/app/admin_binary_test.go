// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/webui"
)

// adminBinaryFeature is the feature the admin built into the binary answers to.
const adminBinaryFeature = "../../test/features/features/admin-in-the-binary.feature"

// adminScenario is what one built-in admin scenario keeps between its steps.
type adminScenario struct {
	t      *testing.T
	env    map[string]string
	folder string
	base   string
	stop   context.CancelFunc
	done   chan error
	result testkit.Result
}

// initializeAdminInTheBinary returns the binding of the built-in admin steps, t holding their databases.
func initializeAdminInTheBinary(t *testing.T) func(*godog.ScenarioContext) {
	return func(sc *godog.ScenarioContext) {
		s := &adminScenario{t: t, env: map[string]string{}}
		sc.Given(`^the settings name no admin folder$`, s.nameNoFolder)
		sc.Given(`^the settings name an admin folder whose page says "([^"]*)"$`, s.nameFolderSaying)
		sc.Given(`^the settings name an admin folder that holds no index\.html$`, s.nameEmptyFolder)
		sc.Given(`^the settings name an admin folder holding the script "([^"]*)"$`, s.nameFolderHolding)
		sc.Given(`^the settings keep the hashed admin files for "([^"]*)"$`, s.keepFor)
		sc.When(`^the site serves$`, s.serve)
		sc.When(`^the operator runs "([^"]*)"$`, s.runCommand)
		sc.Then(`^"([^"]*)" answers the admin page built into this binary$`, s.answersTheBuiltInPage)
		sc.Then(`^"([^"]*)" answers a page saying "([^"]*)"$`, s.answersAPageSaying)
		sc.Then(`^the command fails, naming the folder and the admin built into this binary$`, s.failsNamingTheFolder)
		sc.Then(`^"([^"]*)" may be kept for (\d+) seconds and never changes$`, s.mayBeKeptFor)
		sc.Then(`^"([^"]*)" is checked again on every load$`, s.isCheckedOnEveryLoad)
		sc.After(s.stopServing)
	}
}

// nameNoFolder leaves the admin folder setting out.
func (s *adminScenario) nameNoFolder() {
	delete(s.env, "GOPHENBERG_WEB_DIR")
}

// nameFolder names a new admin folder holding each file given by its path inside the folder.
func (s *adminScenario) nameFolder(files map[string]string) error {
	s.folder = s.t.TempDir()
	s.env["GOPHENBERG_WEB_DIR"] = s.folder
	for name, body := range files {
		path := filepath.Join(s.folder, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// nameFolderSaying names an admin folder whose page carries text.
func (s *adminScenario) nameFolderSaying(text string) error {
	return s.nameFolder(map[string]string{"index.html": "<!doctype html><p>" + text + "</p>"})
}

// nameEmptyFolder names an admin folder that holds no file at all.
func (s *adminScenario) nameEmptyFolder() error {
	return s.nameFolder(nil)
}

// nameFolderHolding names an admin folder holding a page and the script at name.
func (s *adminScenario) nameFolderHolding(name string) error {
	return s.nameFolder(map[string]string{"index.html": "<!doctype html><p>admin</p>", name: "console.log('admin')"})
}

// keepFor names how long a client may keep a hashed admin file.
func (s *adminScenario) keepFor(window string) {
	s.env["GOPHENBERG_CACHE_ADMIN_ASSET_MAX_AGE"] = window
}

// serve starts the site over a fresh database and waits until it answers.
func (s *adminScenario) serve() {
	address := testkit.FreeAddr(s.t)
	s.env["GOPHENBERG_DATABASE_URL"] = emptyDatabaseURL(s.t)
	s.env["GOPHENBERG_ADDR"] = address
	ctx, cancel := context.WithCancel(s.t.Context())
	s.stop, s.done = cancel, make(chan error, 1)
	go func() { s.done <- run(ctx, testkit.Getenv(s.env), io.Discard, noPlugins) }()
	s.base = "http://" + address
	awaitServer(s.t, s.base)
}

// stopServing stops the site a scenario started and reports how it stopped.
func (s *adminScenario) stopServing(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
	if s.stop == nil {
		return ctx, err
	}
	s.stop()
	return ctx, errors.Join(err, <-s.done)
}

// runCommand runs the command line over the scenario's settings and a database nothing answers at.
func (s *adminScenario) runCommand(line string) {
	s.env["GOPHENBERG_DATABASE_URL"] = unreachableDatabaseURL
	s.result = testkit.Run(s.t, Program(testkit.Getenv(s.env), noPlugins), "", strings.Fields(line)...)
}

// get returns the headers and the body the site answers at path, failing on any status but 200.
func (s *adminScenario) get(path string) (http.Header, string, error) {
	response, err := http.Get(s.base + path)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err == nil && response.StatusCode != http.StatusOK {
		err = fmt.Errorf("GET %s answered %d", path, response.StatusCode)
	}
	return response.Header, string(body), err
}

// answersTheBuiltInPage fails unless path answers the admin page built into this binary.
func (s *adminScenario) answersTheBuiltInPage(path string) error {
	builtIn, err := fs.ReadFile(webui.FS(), "index.html")
	if err != nil {
		return err
	}
	_, body, err := s.get(path)
	if err != nil || body != string(builtIn) {
		return fmt.Errorf("GET %s = %q (%v), want the built-in admin page %q", path, body, err, builtIn)
	}
	return nil
}

// answersAPageSaying fails unless path answers a page carrying text.
func (s *adminScenario) answersAPageSaying(path, text string) error {
	_, body, err := s.get(path)
	if err != nil || !strings.Contains(body, text) {
		return fmt.Errorf("GET %s = %q (%v), want a page saying %q", path, body, err, text)
	}
	return nil
}

// failsNamingTheFolder fails unless the command failed, naming the folder and the admin built into this binary.
func (s *adminScenario) failsNamingTheFolder() error {
	if s.result.Code == 0 || !strings.Contains(s.result.Stderr, s.folder) ||
		!strings.Contains(s.result.Stderr, "built into this binary") {
		return fmt.Errorf("the command exited with %d and stderr %q, want a failure naming %s and the built-in admin",
			s.result.Code, s.result.Stderr, s.folder)
	}
	return nil
}

// mayBeKeptFor fails unless path may be kept for seconds and never changes.
func (s *adminScenario) mayBeKeptFor(path string, seconds int) error {
	return s.carries(path, fmt.Sprintf("public, max-age=%d, immutable", seconds))
}

// isCheckedOnEveryLoad fails unless a client must check path again on every load.
func (s *adminScenario) isCheckedOnEveryLoad(path string) error {
	return s.carries(path, "no-cache")
}

// carries fails unless path answers with the Cache-Control want.
func (s *adminScenario) carries(path, want string) error {
	header, _, err := s.get(path)
	if err != nil {
		return err
	}
	if held := header.Get("Cache-Control"); held != want {
		return fmt.Errorf("GET %s carries Cache-Control %q, want %q", path, held, want)
	}
	return nil
}

func TestAdminInTheBinary(t *testing.T) {
	tags := "~@wip"
	if os.Getenv("GOPHENBERG_BDD_WIP") != "" {
		tags = ""
	}
	suite := godog.TestSuite{
		ScenarioInitializer: initializeAdminInTheBinary(t),
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{adminBinaryFeature},
			Tags:     tags,
			Strict:   true,
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Error("the feature did not pass")
	}
}
