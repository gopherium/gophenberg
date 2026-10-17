// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

func TestSumsListsTheFilesSortedByNameWithTwoSpaces(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	record := writeFile(t, dir, "gophenberg_0.20.0_release-record.json", "{}\n")
	archive := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")

	got := runTool("sums", record, archive)

	want := sha256Of(t, archive) + "  gophenberg_0.20.0_linux_amd64.tar.gz\n" +
		sha256Of(t, record) + "  gophenberg_0.20.0_release-record.json\n"
	if got.code != 0 || got.stdout != want {
		t.Errorf("sums = %d, %q, %q, want 0 and %q", got.code, got.stdout, got.stderr, want)
	}
}

func TestSumsRefusesASignatureFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	archive := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")
	signed := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz.sig", "a signature line\n")

	got := runTool("sums", archive, signed)

	if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, signed) {
		t.Errorf("sums over a .sig = %d, %q, %q, want 1, nothing listed and the .sig named", got.code, got.stdout,
			got.stderr)
	}
}

func TestSumsReportsAFileItCannotRead(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "gophenberg_0.20.0_linux_amd64.tar.gz")

	got := runTool("sums", missing)

	if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, missing) {
		t.Errorf("sums over a missing file = %d, %q, %q, want 1 and the file named", got.code, got.stdout, got.stderr)
	}
}

func TestSumsExitsTwoWithoutAFile(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"sums"}, {"sums", "-all", "file"}} {
		got := runTool(args...)

		if got.code != 2 {
			t.Errorf("releasetool %v = %d, %q, want 2", args, got.code, got.stderr)
		}
	}
}

// sha256Of returns the SHA-256 of a file in lowercase hexadecimal.
func sha256Of(t *testing.T, path string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(readText(t, path)))
	return hex.EncodeToString(sum[:])
}
