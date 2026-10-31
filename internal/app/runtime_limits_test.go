// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
)

func TestLogRuntimeLimitsNamesWhatGoHolds(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		named gonsole.RuntimeLimits
		want  string
	}{
		"both limits named": {gonsole.RuntimeLimits{MaxProcs: 4, MemoryLimitMB: 512},
			`msg="runtime limits" max_procs=3 memory_limit_mb=256`},
		"the processors alone": {gonsole.RuntimeLimits{MaxProcs: 3}, `msg="runtime limits" max_procs=3` + "\n"},
		"the memory alone": {gonsole.RuntimeLimits{MemoryLimitMB: 256},
			`msg="runtime limits" memory_limit_mb=256`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var logged bytes.Buffer
			logRuntimeLimits(slog.New(slog.NewTextHandler(&logged, nil)), tc.named, 3, 256<<20)

			if !strings.Contains(logged.String(), tc.want) {
				t.Errorf("logged %q, want %q", logged.String(), tc.want)
			}
		})
	}
}

func TestLogRuntimeLimitsStaysQuietWhenNoLimitIsNamed(t *testing.T) {
	t.Parallel()

	var logged bytes.Buffer
	logRuntimeLimits(slog.New(slog.NewTextHandler(&logged, nil)), gonsole.RuntimeLimits{}, 3, 256<<20)

	if logged.Len() != 0 {
		t.Errorf("logged %q, want nothing", logged.String())
	}
}
