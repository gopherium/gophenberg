// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/gonsole/testkit"
)

// runtimeLimitsFeature is the feature the built binary answers to with limits named in its .env.
const runtimeLimitsFeature = "../../test/features/features/runtime-limits.feature"

// limitsBound is how long one run of the built binary may take.
const limitsBound = 30 * time.Second

// limitsScenario is one run of the built binary in a folder holding the .env the scenario names.
type limitsScenario struct {
	binary string
	env    []string
	dir    string
	lines  []string
	code   int
	stderr string
}

// initializeRuntimeLimits binds the steps of the runtime limits feature to s.
func initializeRuntimeLimits(sc *godog.ScenarioContext, s *limitsScenario) {
	sc.Given(`^the database refuses every connection$`, s.refuseDatabase)
	sc.Given(`^serve waits "([^"]*)" for the database$`, s.waitFor)
	sc.Given(`^Go starts with "([^"]*)"$`, s.startWith)
	sc.Given(`^the site's .env holds the line "([^"]*)"$`, s.holdLine)
	sc.When(`^the site starts$`, func() error { return s.run("serve") })
	sc.When(`^the operator runs "([^"]*)"$`, func(command string) error { return s.run(command) })
	sc.Then(`^the site logged that Go holds (\d+) processors? and a memory limit of (\d+) MB$`, s.loggedLimits)
	sc.Then(`^the site logged no runtime limits$`, s.loggedNone)
	sc.Then(`^the command exits with code (\d+)$`, s.exitsWith)
	sc.Then(`^the error says "([^"]*)"$`, s.errorSays)
	sc.Then(`^the command succeeds$`, func() error { return s.exitsWith(0) })
}

// refuseDatabase names a database address nothing answers on.
func (s *limitsScenario) refuseDatabase() {
	s.env = append(s.env, "GOPHENBERG_DATABASE_URL="+unreachableDatabaseURL)
}

// waitFor names how long serve waits for the database.
func (s *limitsScenario) waitFor(wait string) {
	s.env = append(s.env, "GOPHENBERG_DATABASE_WAIT="+wait)
}

// startWith names a variable Go reads before the site loads its .env.
func (s *limitsScenario) startWith(variable string) {
	s.env = append(s.env, variable)
}

// holdLine adds a line to the .env the next run reads.
func (s *limitsScenario) holdLine(line string) {
	s.lines = append(s.lines, line)
}

// run runs the built binary over args in the scenario's folder, keeping its exit code and stderr.
func (s *limitsScenario) run(args ...string) error {
	if err := os.WriteFile(filepath.Join(s.dir, ".env"), []byte(strings.Join(s.lines, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), limitsBound)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, s.binary, args...)
	cmd.Dir, cmd.Env, cmd.Stderr = s.dir, s.env, &stderr
	err := cmd.Run()
	s.stderr, s.code = stderr.String(), cmd.ProcessState.ExitCode()
	var exited *exec.ExitError
	if errors.As(err, &exited) {
		return nil
	}
	return err
}

// loggedLimits fails unless the run logged the processors and memory limit Go holds.
func (s *limitsScenario) loggedLimits(procs, memoryMB int) error {
	want := fmt.Sprintf(`msg="runtime limits" max_procs=%d memory_limit_mb=%d`, procs, memoryMB)
	if !strings.Contains(s.stderr, want) {
		return fmt.Errorf("stderr = %q, want %q", s.stderr, want)
	}
	return nil
}

// loggedNone fails when the run logged runtime limits.
func (s *limitsScenario) loggedNone() error {
	if strings.Contains(s.stderr, "runtime limits") {
		return fmt.Errorf("stderr = %q, want no runtime limits line", s.stderr)
	}
	return nil
}

// exitsWith fails unless the run ended with the code.
func (s *limitsScenario) exitsWith(code int) error {
	if s.code != code {
		return fmt.Errorf("exit code = %d, want %d, stderr %q", s.code, code, s.stderr)
	}
	return nil
}

// errorSays fails unless stderr holds the text.
func (s *limitsScenario) errorSays(text string) error {
	if !strings.Contains(s.stderr, text) {
		return fmt.Errorf("stderr = %q, want it to hold %q", s.stderr, text)
	}
	return nil
}

func TestRuntimeLimits(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			initializeRuntimeLimits(sc, &limitsScenario{binary: binary, env: env, dir: t.TempDir()})
		},
		Options: &godog.Options{Format: "pretty", Paths: []string{runtimeLimitsFeature}, Strict: true, TestingT: t},
	}
	if suite.Run() != 0 {
		t.Error("the feature did not pass")
	}
}
