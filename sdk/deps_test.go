// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"go/build"
	"strings"
	"testing"
)

func TestContractImportsOnlyTheStandardLibrary(t *testing.T) {
	t.Parallel()

	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("inspecting package: %v", err)
	}

	for _, imported := range pkg.Imports {
		if first, _, _ := strings.Cut(imported, "/"); strings.Contains(first, ".") {
			t.Errorf("sdk imports %q, want the standard library alone, so no outside release changes the contract",
				imported)
		}
	}
}
