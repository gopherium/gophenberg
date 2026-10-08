// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"go/build"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestModuleRequiresNoOtherModule(t *testing.T) {
	t.Parallel()

	declared, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("reading the module file: %v", err)
	}

	var directives []string
	for line := range strings.Lines(string(declared)) {
		if word, _, _ := strings.Cut(strings.TrimSpace(line), " "); word != "" {
			directives = append(directives, word)
		}
	}
	if !strings.HasPrefix(string(declared), "module github.com/gopherium/gophenberg/sdk\n") ||
		!slices.Equal(directives, []string{"module", "go"}) {
		t.Errorf("go.mod reads %q, want the module github.com/gopherium/gophenberg/sdk and its go version alone, "+
			"so a plugin requiring the sdk pulls in no other module", declared)
	}
}

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
