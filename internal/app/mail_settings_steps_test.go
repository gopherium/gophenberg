// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

// mailSettingsFeature is the feature the built binary answers to with mail settings named in its .env.
const mailSettingsFeature = "../../test/features/features/mail-settings.feature"

// initializeMailSettings binds the steps of the mail settings feature to s.
func initializeMailSettings(sc *godog.ScenarioContext, s *limitsScenario) {
	initializeRuntimeLimits(sc, s)
	sc.Then(`^the site logged that invitations answer their link$`, s.loggedNoRelay)
	sc.Then(`^the site logged the relay "([^"]*)" on the port (\d+)$`, s.loggedRelay)
}

// loggedNoRelay fails unless the run logged that invitations answer their link.
func (s *limitsScenario) loggedNoRelay() error {
	return s.logged(`msg="no mail relay configured, invitations answer their activation link"`)
}

// loggedRelay fails unless the run logged the relay at host and port.
func (s *limitsScenario) loggedRelay(host string, port int) error {
	return s.logged(fmt.Sprintf(`msg="mail relay configured" host=%s port=%d`, host, port))
}

// logged fails unless stderr holds the line.
func (s *limitsScenario) logged(line string) error {
	if !strings.Contains(s.stderr, line) {
		return fmt.Errorf("stderr = %q, want %q", s.stderr, line)
	}
	return nil
}

func TestMailSettings(t *testing.T) {
	t.Parallel()

	runBinaryFeature(t, mailSettingsFeature, initializeMailSettings)
}
