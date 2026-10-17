// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// downloadBase is the address a test release serves its files under.
const downloadBase = "https://github.com/gopherium/gophenberg/releases/download/v0.20.0"

func TestRecordNamesEachArchiveByPlatform(t *testing.T) {
	t.Parallel()

	archives := signedArchives(t, "gophenberg_0.20.0_linux_amd64.tar.gz", "gophenberg_0.20.0_darwin_arm64.tar.gz")

	got := runTool(append([]string{"record", "-version", "0.20.0", "-download-url", downloadBase}, archives...)...)

	record := readRecord(t, got)
	want := []artifact{
		artifactOf(t, archives[1], "darwin-arm64", "darwin/arm64", "application/gzip"),
		artifactOf(t, archives[0], "linux-amd64", "linux/amd64", "application/gzip"),
	}
	if record.Version != "0.20.0" || !slices.Equal(record.Artifacts.Package, want) {
		t.Errorf("record = %+v, want version 0.20.0 and the archives sorted by name as %+v", record, want)
	}
	if !strings.HasSuffix(got.stdout, "}\n") {
		t.Errorf("record printed %q, want the document to end its line", got.stdout)
	}
}

func TestRecordReadsAnyNameOnThePattern(t *testing.T) {
	t.Parallel()

	archives := signedArchives(t, "gophenberg-shared_0.20.0_linux_amd64.zip")

	got := runTool("record", "-version", "0.20.0", "-download-url", downloadBase+"/", archives[0])

	record := readRecord(t, got)
	want := []artifact{artifactOf(t, archives[0], "shared-linux-amd64", "linux/amd64", "application/zip")}
	if !slices.Equal(record.Artifacts.Package, want) {
		t.Errorf("record lists %+v, want %+v", record.Artifacts.Package, want)
	}
}

func TestRecordRefusesAnArchiveWithoutASignature(t *testing.T) {
	t.Parallel()

	unsigned := writeFile(t, t.TempDir(), "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")

	got := runTool("record", "-version", "0.20.0", "-download-url", downloadBase, unsigned)

	if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, "reading the signature") {
		t.Errorf("record of an unsigned archive = %d, %q, %q, want 1 and the signature named", got.code, got.stdout,
			got.stderr)
	}
}

func TestRecordReportsASignatureWhoseArchiveIsGone(t *testing.T) {
	t.Parallel()

	archives := signedArchives(t, "gophenberg_0.20.0_linux_amd64.tar.gz")
	if err := os.Remove(archives[0]); err != nil {
		t.Fatalf("removing the archive: %v", err)
	}

	got := runTool("record", "-version", "0.20.0", "-download-url", downloadBase, archives[0])

	if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, "reading "+archives[0]) {
		t.Errorf("record of a missing archive = %d, %q, %q, want 1 and the archive named", got.code, got.stdout,
			got.stderr)
	}
}

func TestRecordRefusesANameOffThePattern(t *testing.T) {
	t.Parallel()

	for name, file := range map[string]string{
		"no architecture":        "gophenberg_0.20.0_linux.tar.gz",
		"another product":        "caddy_0.20.0_linux_amd64.tar.gz",
		"an unknown extension":   "gophenberg_0.20.0_linux_amd64.tgz",
		"another version":        "gophenberg_0.19.0_linux_amd64.tar.gz",
		"an empty platform part": "gophenberg_0.20.0__amd64.tar.gz",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			archives := signedArchives(t, file)

			got := runTool("record", "-version", "0.20.0", "-download-url", downloadBase, archives[0])

			if got.code != 1 || got.stdout != "" || !strings.Contains(got.stderr, file) {
				t.Errorf("record of %s = %d, %q, %q, want 1 and the name refused", file, got.code, got.stdout, got.stderr)
			}
		})
	}
}

func TestRecordExitsTwoWhenMisused(t *testing.T) {
	t.Parallel()

	archives := signedArchives(t, "gophenberg_0.20.0_linux_amd64.tar.gz")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no version", []string{"-download-url", downloadBase, archives[0]}, "-version"},
		{"no download address", []string{"-version", "0.20.0", archives[0]}, "-download-url"},
		{"no archive", []string{"-version", "0.20.0", "-download-url", downloadBase}, "archive"},
		{"an address that is not https", []string{"-version", "0.20.0", "-download-url",
			"http://github.com/gopherium/gophenberg/releases/download/v0.20.0", archives[0]}, "https"},
		{"an address with no host", []string{"-version", "0.20.0", "-download-url", "https:///releases",
			archives[0]}, "https"},
		{"an address that does not parse", []string{"-version", "0.20.0", "-download-url", "https://example.com/%zz",
			archives[0]}, "https"},
		{"a flag it does not know", []string{"-sort", archives[0]}, "-sort"},
	}
	for _, tc := range cases {
		got := runTool(append([]string{"record"}, tc.args...)...)

		if got.code != 2 || !strings.Contains(got.stderr, tc.want) {
			t.Errorf("record with %s = %d, %q, want 2 and %q named", tc.name, got.code, got.stderr, tc.want)
		}
	}
}

// signedArchives writes an archive under each name in a new folder, signs them all, and returns their paths.
func signedArchives(t *testing.T, names ...string) []string {
	t.Helper()
	dir := t.TempDir()
	_, keyFile := newReleaseKey(t, dir)
	archives := make([]string, len(names))
	for i, name := range names {
		archives[i] = writeFile(t, dir, name, "the archive "+name+"\n")
	}
	if got := runTool(append([]string{"sign", "-key", keyFile}, archives...)...); got.code != 0 {
		t.Fatalf("sign = %d, %q, want 0", got.code, got.stderr)
	}
	return archives
}

// readRecord returns the release record a run printed, failing the test unless the run succeeded.
func readRecord(t *testing.T, got runResult) releaseRecord {
	t.Helper()
	if got.code != 0 {
		t.Fatalf("record = %d, %q, want 0", got.code, got.stderr)
	}
	var record releaseRecord
	if err := json.Unmarshal([]byte(got.stdout), &record); err != nil {
		t.Fatalf("record printed %q, want one JSON document: %v", got.stdout, err)
	}
	return record
}

// artifactOf returns the entry a record should hold for the archive at path.
func artifactOf(t *testing.T, path, id, platform, contentType string) artifact {
	t.Helper()
	signatureLine, err := os.ReadFile(path + ".sig")
	if err != nil {
		t.Fatalf("reading the signature of %s: %v", path, err)
	}
	return artifact{
		ID: id, Platform: platform, ContentType: contentType,
		URL:       downloadBase + "/" + filepath.Base(path),
		Checksum:  "sha256:" + sha256Of(t, path),
		Signature: strings.TrimSuffix(string(signatureLine), "\n"),
	}
}
