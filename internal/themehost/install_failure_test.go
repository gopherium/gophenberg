// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"hash/crc32"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// rawEntry is one archive entry written exactly as given, past what the writer would compute.
type rawEntry struct {
	path   string
	body   string
	method uint16
	crc    uint32
}

// rawArchiveOf returns a zip carrying the method and checksum each entry names.
func rawArchiveOf(t *testing.T, entries ...rawEntry) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, e := range entries {
		body := []byte(e.body)
		header := &zip.FileHeader{
			Name:               e.path,
			Method:             e.method,
			CRC32:              e.crc,
			CompressedSize64:   uint64(len(body)),
			UncompressedSize64: uint64(len(body)),
		}
		file, err := writer.CreateRaw(header)
		if err != nil {
			t.Fatalf("writing %s: %v", e.path, err)
		}
		if _, err := file.Write(body); err != nil {
			t.Fatalf("writing the body of %s: %v", e.path, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the archive: %v", err)
	}
	return buffer.Bytes()
}

// denyWrite takes write permission away from a directory, skipping when the user keeps it anyway.
func denyWrite(t *testing.T, path string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which writes to a directory whatever its mode says")
	}
	if err := os.Chmod(path, 0o500); err != nil {
		t.Fatalf("denying write on %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
}

func TestInstallReportsAnEntryCompressedInAWayItCannotRead(t *testing.T) {
	t.Parallel()

	const unregisteredMethod uint16 = 99
	body := "body{}\n"

	_, err := install(t, rawArchiveOf(t, rawEntry{
		path:   "client/app.css",
		body:   body,
		method: unregisteredMethod,
		crc:    crc32.ChecksumIEEE([]byte(body)),
	}))

	if !errors.Is(err, zip.ErrAlgorithm) {
		t.Fatalf("Install() = %v, want the unreadable compression reported", err)
	}
	if !strings.Contains(err.Error(), "reading client/app.css from the archive") {
		t.Errorf("Install() = %v, want the read of the entry named", err)
	}
}

func TestInstallReportsAnEntryWhoseChecksumDoesNotMatchItsBody(t *testing.T) {
	t.Parallel()

	body := "body{}\n"
	wrongChecksum := crc32.ChecksumIEEE([]byte(body)) + 1

	_, err := install(t, rawArchiveOf(t, rawEntry{
		path:   "client/app.css",
		body:   body,
		method: zip.Store,
		crc:    wrongChecksum,
	}))

	if !errors.Is(err, zip.ErrChecksum) {
		t.Fatalf("Install() = %v, want the mismatched checksum reported", err)
	}
	if !strings.Contains(err.Error(), "writing client/app.css") {
		t.Errorf("Install() = %v, want the write of the entry named", err)
	}
}

func TestInstallReportsAThemeItCannotSwapIntoPlace(t *testing.T) {
	t.Parallel()

	themesDir := t.TempDir()
	unclearableRetired := filepath.Join(themesDir, ".aurora.retired")
	if err := os.MkdirAll(unclearableRetired, 0o755); err != nil {
		t.Fatalf("planting the retired copy: %v", err)
	}
	writeFile(t, filepath.Join(unclearableRetired, "held.txt"), "held")
	denyWrite(t, unclearableRetired)
	archive := validArchive(t, "aurora")

	_, err := themehost.Install(themesDir, "aurora", bytes.NewReader(archive), int64(len(archive)))

	if err == nil {
		t.Fatal("Install() = nil, want the swap it cannot make reported")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("Install() = %v, want the denied write reported", err)
	}
	if !strings.Contains(err.Error(), "clearing "+unclearableRetired) {
		t.Errorf("Install() = %v, want the clearing of the retired copy named", err)
	}
	if _, statErr := os.Stat(filepath.Join(themesDir, "aurora")); statErr == nil {
		t.Error("the theme was installed, want the refused swap to leave it out")
	}
}
