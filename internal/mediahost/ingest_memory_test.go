// SPDX-License-Identifier: Apache-2.0

package mediahost_test

import (
	"runtime"
	"testing"
)

func TestIngestTurnsAndScalesASidewaysPhotoWithinItsMemoryBudget(t *testing.T) {
	photo := orientedJPEG(t, 6, 2600, 1300)
	library := newLibrary(t)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	mustIngest(t, library, "cliff.jpg", photo)

	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 28<<20 {
		t.Errorf("Ingest() allocated %d MiB, want at most 28", allocated>>20)
	}
}
