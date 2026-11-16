// SPDX-License-Identifier: Apache-2.0

package pluginsettings

import (
	"log/slog"
	"testing"

	"github.com/gopherium/framework/gonsole/testkit"
)

func TestANilLoggerFallsBackToTheDiscardHandler(t *testing.T) {
	t.Parallel()

	set := New(nil, testkit.Getenv(nil), nil)

	if got := set.logger.Handler(); got != slog.DiscardHandler {
		t.Errorf("a nil logger leaves the handler %T, want slog.DiscardHandler", got)
	}
}
