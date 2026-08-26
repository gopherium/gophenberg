// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// entry is one file an archive carries.
type entry struct {
	path    string
	body    string
	symlink bool
}

// archiveOf returns a zip carrying the given entries.
func archiveOf(t *testing.T, entries ...entry) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, e := range entries {
		header := &zip.FileHeader{Name: e.path, Method: zip.Deflate}
		if e.symlink {
			header.SetMode(os.ModeSymlink)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("writing %s: %v", e.path, err)
		}
		if _, err := file.Write([]byte(e.body)); err != nil {
			t.Fatalf("writing the body of %s: %v", e.path, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the archive: %v", err)
	}
	return buffer.Bytes()
}

// validArchive returns an archive holding a theme that passes every rule.
func validArchive(t *testing.T, name string) []byte {
	t.Helper()

	return archiveOf(t,
		entry{path: "theme.json", body: manifestFor(name, "1.0.0")},
		entry{path: "server/entry.mjs", body: "export default {}\n"},
		entry{path: "client/app.css", body: "body{}\n"},
	)
}

// install unpacks an archive as the theme "aurora" and returns the themes directory.
func install(t *testing.T, archive []byte) (string, error) {
	t.Helper()

	themesDir := t.TempDir()
	_, err := themehost.Install(themesDir, "aurora", bytes.NewReader(archive), int64(len(archive)))
	return themesDir, err
}

func TestInstallLaysOutAThemeTheServerCanLoad(t *testing.T) {
	t.Parallel()

	themesDir, err := install(t, validArchive(t, "aurora"))

	if err != nil {
		t.Fatalf("Install() = %v, want the theme installed", err)
	}
	loaded, err := themehost.Load(themesDir, "aurora")
	if err != nil {
		t.Fatalf("the installed theme does not load: %v", err)
	}
	if loaded.Name != "aurora" {
		t.Errorf("Name = %q, want the installed theme", loaded.Name)
	}
}

func TestInstallRefusesAnArchiveWithoutAManifest(t *testing.T) {
	t.Parallel()

	_, err := install(t, archiveOf(t,
		entry{path: "server/entry.mjs", body: "export default {}\n"},
		entry{path: "client/app.css", body: "body{}\n"},
	))

	if err == nil {
		t.Fatal("want an archive with no manifest refused")
	}
}

func TestInstallRefusesAManifestNamingAnotherTheme(t *testing.T) {
	t.Parallel()

	_, err := install(t, validArchive(t, "driftwood"))

	if err == nil {
		t.Fatal("want a manifest naming another theme refused")
	}
}

func TestInstallRefusesAnArchiveWithoutAServerEntry(t *testing.T) {
	t.Parallel()

	_, err := install(t, archiveOf(t,
		entry{path: "theme.json", body: manifestFor("aurora", "1.0.0")},
		entry{path: "client/app.css", body: "body{}\n"},
	))

	if err == nil {
		t.Fatal("want an archive with no server entry refused")
	}
}

func TestInstallRefusesAnArchiveWithoutClientAssets(t *testing.T) {
	t.Parallel()

	_, err := install(t, archiveOf(t,
		entry{path: "theme.json", body: manifestFor("aurora", "1.0.0")},
		entry{path: "server/entry.mjs", body: "export default {}\n"},
	))

	if err == nil {
		t.Fatal("want an archive with no client assets refused")
	}
}

func TestInstallRefusesASymlink(t *testing.T) {
	t.Parallel()

	_, err := install(t, archiveOf(t,
		entry{path: "theme.json", body: manifestFor("aurora", "1.0.0")},
		entry{path: "server/entry.mjs", body: "export default {}\n"},
		entry{path: "client/app.css", body: "body{}\n"},
		entry{path: "client/escape", body: "/etc/passwd", symlink: true},
	))

	if err == nil {
		t.Fatal("want an archive carrying a symlink refused")
	}
}

func TestInstallRefusesAnEntryEscapingTheThemeDirectory(t *testing.T) {
	t.Parallel()

	themesDir := t.TempDir()
	archive := archiveOf(t,
		entry{path: "theme.json", body: manifestFor("aurora", "1.0.0")},
		entry{path: "server/entry.mjs", body: "export default {}\n"},
		entry{path: "client/app.css", body: "body{}\n"},
		entry{path: "../escaped.txt", body: "owned"},
	)

	_, err := themehost.Install(themesDir, "aurora", bytes.NewReader(archive), int64(len(archive)))

	if err == nil {
		t.Fatal("want an archive escaping its directory refused")
	}
	if _, statErr := os.Stat(filepath.Join(themesDir, "escaped.txt")); statErr == nil {
		t.Error("the escaping entry was written outside the theme directory")
	}
}

func TestInstallRefusesAThemeOverTheSizeCap(t *testing.T) {
	t.Parallel()

	_, err := install(t, archiveOf(t,
		entry{path: "theme.json", body: manifestFor("aurora", "1.0.0")},
		entry{path: "server/entry.mjs", body: "export default {}\n"},
		entry{path: "client/app.css", body: strings.Repeat("a", int(themehost.MaxSize)+1)},
	))

	if err == nil {
		t.Fatal("want a theme over the size cap refused")
	}
}

func TestInstallAcceptsAnArchiveCarryingDirectoryEntries(t *testing.T) {
	t.Parallel()

	themesDir, err := install(t, archiveOf(t,
		entry{path: "server/"},
		entry{path: "client/"},
		entry{path: "theme.json", body: manifestFor("aurora", "1.0.0")},
		entry{path: "server/entry.mjs", body: "export default {}\n"},
		entry{path: "client/app.css", body: "body{}\n"},
	))

	if err != nil {
		t.Fatalf("Install() = %v, want directory entries accepted", err)
	}
	if _, err := themehost.Load(themesDir, "aurora"); err != nil {
		t.Errorf("the installed theme does not load: %v", err)
	}
}

func TestInstallRefusesAnArchiveCarryingMoreFilesThanTheCap(t *testing.T) {
	t.Parallel()

	entries := []entry{
		{path: "theme.json", body: manifestFor("aurora", "1.0.0")},
		{path: "server/entry.mjs", body: "export default {}\n"},
		{path: "client/app.css", body: "body{}\n"},
	}
	for i := range themehost.MaxEntries {
		entries = append(entries, entry{path: fmt.Sprintf("client/%d", i)})
	}

	_, err := install(t, archiveOf(t, entries...))

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Install() = %v, want an archive of too many files refused", err)
	}
	if refused.Reason != "the archive holds too many files" {
		t.Errorf("Reason = %q, want the file count refused", refused.Reason)
	}
}

func TestInstallRefusesADeclaredEntryFloodBeforeReadingTheDirectory(t *testing.T) {
	t.Parallel()

	blob := bytes.Repeat([]byte{0xAA}, 4096)
	record := make([]byte, 22)
	binary.LittleEndian.PutUint32(record[0:], 0x06054b50)
	binary.LittleEndian.PutUint16(record[8:], 60000)
	binary.LittleEndian.PutUint16(record[10:], 60000)
	binary.LittleEndian.PutUint32(record[12:], 4096)

	_, err := install(t, append(blob, record...))

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Install() = %v, want the declared flood refused", err)
	}
	if refused.Code != "archive_too_many_entries" {
		t.Errorf("Code = %q, want the count refused before the directory is ever read", refused.Code)
	}
}

func TestInstallStillCountsTheEntriesWhenTheClosingRecordWrapsAround(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for i := range 65536 + 2 {
		header := &zip.FileHeader{Name: fmt.Sprintf("client/%d", i), Method: zip.Store}
		if _, err := writer.CreateHeader(header); err != nil {
			t.Fatalf("adding entry %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the archive: %v", err)
	}
	archive := buffer.Bytes()
	at := bytes.LastIndex(archive, binary.LittleEndian.AppendUint32(nil, 0x06054b50))
	binary.LittleEndian.PutUint16(archive[at+8:], 3)
	binary.LittleEndian.PutUint16(archive[at+10:], 3)

	_, err := install(t, archive)

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Install() = %v, want the parsed count refused despite the wrapped record", err)
	}
	if refused.Code != "archive_too_many_entries" {
		t.Errorf("Code = %q, want archive_too_many_entries", refused.Code)
	}
}

func TestInstallFindsTheClosingRecordBehindTrailingJunk(t *testing.T) {
	t.Parallel()

	blob := bytes.Repeat([]byte{0xAA}, 4096)
	record := make([]byte, 22)
	binary.LittleEndian.PutUint32(record[0:], 0x06054b50)
	binary.LittleEndian.PutUint16(record[8:], 60000)
	binary.LittleEndian.PutUint16(record[10:], 60000)
	decoy := make([]byte, 22)
	binary.LittleEndian.PutUint32(decoy[0:], 0x06054b50)
	binary.LittleEndian.PutUint16(decoy[20:], 65535)
	blob = append(append(blob, record...), decoy...)

	_, err := install(t, blob)

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Install() = %v, want the declared flood refused past the decoy", err)
	}
	if refused.Code != "archive_too_many_entries" {
		t.Errorf("Code = %q, want the record behind the junk read rather than the decoy", refused.Code)
	}
}

func TestInstallReportsALongBlobHoldingNoClosingRecord(t *testing.T) {
	t.Parallel()

	_, err := install(t, bytes.Repeat([]byte{0xAA}, 4096))

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Install() = %v, want the recordless blob refused", err)
	}
	if refused.Code != "archive_unreadable" {
		t.Errorf("Code = %q, want archive_unreadable", refused.Code)
	}
}

// unreadableArchive is a reader that fails every read.
type unreadableArchive struct{}

// ReadAt fails as a broken upload would.
func (unreadableArchive) ReadAt([]byte, int64) (int, error) {
	return 0, errors.New("the upload is broken")
}

func TestInstallReportsAnArchiveItCannotReadTheTailOf(t *testing.T) {
	t.Parallel()

	_, err := themehost.Install(t.TempDir(), "aurora", unreadableArchive{}, 4096)

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Install() = %v, want the unreadable archive refused", err)
	}
	if refused.Code != "archive_unreadable" {
		t.Errorf("Code = %q, want archive_unreadable", refused.Code)
	}
}

func TestInstallRefusesAnArchiveOfDirectoriesThatUnpacksPastTheCap(t *testing.T) {
	t.Parallel()

	deep := strings.Repeat("d/", 400)
	entries := []entry{
		{path: "theme.json", body: manifestFor("aurora", "1.0.0")},
		{path: "server/entry.mjs", body: "export default {}\n"},
		{path: "client/app.css", body: "body{}\n"},
	}
	for i := range 50 {
		entries = append(entries, entry{path: fmt.Sprintf("client/x%d/%s", i, deep)})
	}

	themesDir, err := install(t, archiveOf(t, entries...))

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Install() = %v, want an archive of directories refused", err)
	}
	if refused.Reason != "the theme is too large" {
		t.Errorf("Reason = %q, want the size cap to catch the directories", refused.Reason)
	}
	if !strings.Contains(err.Error(), "unpacks to more than") {
		t.Errorf("error = %v, want it refused while unpacking rather than after the tree exists", err)
	}
	left, _ := os.ReadDir(themesDir)
	if len(left) != 0 {
		t.Errorf("the themes directory holds %d entries, want the refused bomb to leave nothing", len(left))
	}
}

func TestInstallRefusesSomethingThatIsNotAnArchive(t *testing.T) {
	t.Parallel()

	_, err := install(t, []byte("this is not a zip"))

	if err == nil {
		t.Fatal("want something that is not an archive refused")
	}
}

func TestInstallLeavesNothingBehindWhenItRefuses(t *testing.T) {
	t.Parallel()

	themesDir := t.TempDir()
	archive := validArchive(t, "driftwood")

	_, err := themehost.Install(themesDir, "aurora", bytes.NewReader(archive), int64(len(archive)))

	if err == nil {
		t.Fatal("want the mismatched manifest refused")
	}
	left, readErr := os.ReadDir(themesDir)
	if readErr != nil {
		t.Fatalf("reading the themes directory: %v", readErr)
	}
	if len(left) != 0 {
		t.Errorf("the themes directory holds %d entries, want the refused upload to leave nothing", len(left))
	}
}

func TestInstallLeavesAThemeNamedLikeTheScratchPathAlone(t *testing.T) {
	t.Parallel()

	themesDir := t.TempDir()
	scratchLikeName := "driftwood.retired"
	first := validArchive(t, scratchLikeName)
	if _, err := themehost.Install(themesDir, scratchLikeName, bytes.NewReader(first), int64(len(first))); err != nil {
		t.Fatalf("installing the scratch-like name: %v", err)
	}
	second := validArchive(t, "driftwood")

	if _, err := themehost.Install(themesDir, "driftwood", bytes.NewReader(second), int64(len(second))); err != nil {
		t.Fatalf("installing the plain name: %v", err)
	}

	if _, err := themehost.Load(themesDir, scratchLikeName); err != nil {
		t.Errorf("the scratch-like theme was destroyed: %v", err)
	}
	if _, err := themehost.Load(themesDir, "driftwood"); err != nil {
		t.Errorf("the plain theme does not load: %v", err)
	}
}

func TestInstallReplacesAnInstalledTheme(t *testing.T) {
	t.Parallel()

	themesDir := t.TempDir()
	first := validArchive(t, "aurora")
	if _, err := themehost.Install(themesDir, "aurora", bytes.NewReader(first), int64(len(first))); err != nil {
		t.Fatalf("installing the first copy: %v", err)
	}
	second := archiveOf(t,
		entry{path: "theme.json", body: manifestFor("aurora", "2.0.0")},
		entry{path: "server/entry.mjs", body: "export default {}\n"},
		entry{path: "client/app.css", body: "body{}\n"},
	)

	if _, err := themehost.Install(themesDir, "aurora", bytes.NewReader(second), int64(len(second))); err != nil {
		t.Fatalf("replacing the theme: %v", err)
	}

	loaded, err := themehost.Load(themesDir, "aurora")
	if err != nil {
		t.Fatalf("loading the replaced theme: %v", err)
	}
	if loaded.Version != "2.0.0" {
		t.Errorf("Version = %q, want the newer theme", loaded.Version)
	}
	left, _ := os.ReadDir(themesDir)
	if len(left) != 1 {
		t.Errorf("the themes directory holds %d entries, want only the replaced theme", len(left))
	}
}

func TestInstallReportsAStagingDirectoryItCannotMake(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("in the way"), 0o644); err != nil {
		t.Fatalf("planting the blocking file: %v", err)
	}
	archive := validArchive(t, "aurora")

	_, err := themehost.Install(blocked, "aurora", bytes.NewReader(archive), int64(len(archive)))

	if err == nil {
		t.Error("Install() error = nil, want the staging directory reported")
	}
}

func TestInstallReportsAnArchiveHoldingAFileWhereADirectoryBelongs(t *testing.T) {
	t.Parallel()

	for name, entries := range map[string][]entry{
		"a file a later entry nests under": {
			{path: "server", body: "a file where the directory belongs"},
			{path: "server/entry.mjs", body: "export const handler = () => {}"},
		},
		"a file a later directory takes over": {
			{path: "client", body: "a file where the directory belongs"},
			{path: "client/", body: ""},
		},
		"a directory a later file takes over": {
			{path: "client/", body: ""},
			{path: "client", body: "a file where the directory belongs"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			archive := archiveOf(t, entries...)

			_, err := themehost.Install(t.TempDir(), "aurora", bytes.NewReader(archive), int64(len(archive)))

			if err == nil {
				t.Error("Install() error = nil, want the clashing entry reported")
			}
		})
	}
}
