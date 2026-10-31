// SPDX-License-Identifier: Apache-2.0

package app

import (
	"log/slog"

	"github.com/gopherium/framework/gonsole"
)

// logRuntimeLimits logs the processors and the memory limit Go holds, each only when the site names that limit.
func logRuntimeLimits(logger *slog.Logger, named gonsole.RuntimeLimits, procs int, memory int64) {
	held := make([]any, 0, 4)
	if named.MaxProcs > 0 {
		held = append(held, "max_procs", procs)
	}
	if named.MemoryLimitMB > 0 {
		held = append(held, "memory_limit_mb", memory>>20)
	}
	if len(held) > 0 {
		logger.Info("runtime limits", held...)
	}
}
