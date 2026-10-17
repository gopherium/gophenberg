// SPDX-License-Identifier: Apache-2.0

package makefile_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// releaseEpoch is the time a test release names for every archive entry.
const releaseEpoch = 1760000000

// fullCommit is the commit a test release names for its binaries.
const fullCommit = "4c3ba6000f6e3b3a1a9d2c8f7e5b4a3c2d1e0f9a"

// archiveEntry is one entry of a release archive, as the tests read it.
type archiveEntry struct {
	header *tar.Header
	body   []byte
}

// archiveFacts is the release.json an archive carries, as the tests read it.
type archiveFacts struct {
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Stamp    string `json:"stamp"`
	Platform string `json:"platform"`
	Notes    string `json:"notes"`
}

func TestMakeReleaseArchivesPacksEachTargetFlatTheSameWayEveryTime(t *testing.T) {
	t.Parallel()

	root := packingRepo(t)
	host := runtime.GOOS + "/" + runtime.GOARCH
	other := anotherPlatform(host)
	args := []string{"release-archives", "RELEASE_TARGETS=" + host + " " + other, "RELEASE_STAMP=official",
		"RELEASE_COMMIT=" + fullCommit, "RELEASE_EPOCH=" + strconv.Itoa(releaseEpoch)}

	first := packedArchive(t, root, host, args...)

	entries := readArchive(t, first)
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.header.Name
		if entry.header.Uid != 0 || entry.header.Gid != 0 || entry.header.ModTime.Unix() != releaseEpoch {
			t.Errorf("%s belongs to %d:%d at %v, want 0:0 at %d", entry.header.Name, entry.header.Uid,
				entry.header.Gid, entry.header.ModTime, releaseEpoch)
		}
	}
	layout := []string{"LICENSE", "LICENSES/", "LICENSES/GPL-3.0.txt", "NOTICE", "gophenberg", "release.json"}
	if !slices.Equal(names, layout) {
		t.Errorf("the archive holds %q, want %q at its root, sorted by name", names, layout)
	}
	if mode := entryNamed(t, entries, "gophenberg").header.Mode; mode != 0o755 {
		t.Errorf("the binary's entry has mode %o, want 755", mode)
	}
	want := archiveFacts{Version: "0.20.0", Commit: fullCommit, Stamp: "official", Platform: host,
		Notes: "https://github.com/gopherium/gophenberg/releases/tag/v0.20.0"}
	if facts := factsOf(t, entries); facts != want {
		t.Errorf("release.json = %+v, want %+v", facts, want)
	}
	if facts := factsOf(t, readArchive(t, archiveBytes(t, root, other))); facts.Platform != other {
		t.Errorf("the %s archive's release.json names %q", other, facts.Platform)
	}
	if again := packedArchive(t, root, host, args...); !bytes.Equal(again, first) {
		t.Error("a second run packed different bytes, want the same archive every time")
	}
}

func TestMakeReleaseArchivesStampsNothingAndDatesTheLastCommitUnlessTold(t *testing.T) {
	t.Parallel()

	root := packingRepo(t)
	host := runtime.GOOS + "/" + runtime.GOARCH

	entries := readArchive(t, packedArchive(t, root, host, "release-archives", "RELEASE_TARGETS="+host))

	committed, err := inRepo(t, root, "git", "log", "-1", "--format=%ct")
	if err != nil {
		t.Fatalf("reading the last commit's time: %v", err)
	}
	for _, entry := range entries {
		if stamp := strconv.FormatInt(entry.header.ModTime.Unix(), 10); stamp != strings.TrimSpace(committed) {
			t.Errorf("%s is dated %s, want the last commit's time %s", entry.header.Name, stamp, committed)
		}
	}
	if facts := factsOf(t, entries); facts.Stamp != "" || facts.Commit != "" {
		t.Errorf("release.json = %+v, want no stamp and no commit in a build the release workflow did not make", facts)
	}
}

func TestMakeReleaseArchivesPacksATargetThisMachineCannotRun(t *testing.T) {
	t.Parallel()

	root := packingRepo(t)
	host := runtime.GOOS + "/" + runtime.GOARCH
	other := anotherPlatform(host)

	entries := readArchive(t, packedArchive(t, root, other, "release-archives", "RELEASE_TARGETS="+other))

	if facts := factsOf(t, entries); facts.Platform != other {
		t.Errorf("the %s archive's release.json names %q", other, facts.Platform)
	}
	hostArchive := filepath.Join(root, "dist", "gophenberg_0.20.0_"+platformFolder(host)+".tar.gz")
	if _, err := os.Stat(hostArchive); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("make release-archives packed %s, a platform the targets leave out: %v", host, err)
	}
}

func TestMakeReleaseArchivesStopsBeforeBuildingWithoutGNUTar(t *testing.T) {
	t.Parallel()

	root := archivesRepo(t)
	bsdtar := filepath.Join(root, "bin", "bsdtar")
	writeFile(t, bsdtar, "#!/bin/sh\necho 'bsdtar 3.5.3 - libarchive 3.5.3'\n")
	if err := os.Chmod(bsdtar, 0o755); err != nil {
		t.Fatalf("making the stand-in bsdtar runnable: %v", err)
	}

	out, err := releaseIn(t, root, "release-archives", "TAR=bsdtar")

	if err == nil || !strings.Contains(out, "needs GNU tar") || !strings.Contains(out, "TAR=gtar") {
		t.Errorf("make release-archives with bsdtar = %v with output %q, want it stopped, naming GNU tar and TAR=gtar",
			err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "pnpm-args")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("make release-archives built the admin before it found no GNU tar: %v", err)
	}
}

func TestMakeReleaseArchivesBuildsItsTargetsAfterABuildRelease(t *testing.T) {
	t.Parallel()

	root := packingRepo(t)
	other := anotherPlatform(runtime.GOOS + "/" + runtime.GOARCH)

	packed := packedArchive(t, root, other, "build-release", "release-archives", "RELEASE_TARGETS="+other)

	if facts := factsOf(t, readArchive(t, packed)); facts.Platform != other {
		t.Errorf("the %s archive's release.json names %q", other, facts.Platform)
	}
}

func TestMakeReleaseArchivesPacksTheSameBytesWhateverTheUmask(t *testing.T) {
	t.Parallel()

	open := packedUnderUmask(t, "022")
	closed := packedUnderUmask(t, "077")

	if !bytes.Equal(open, closed) {
		t.Error("the archive packed under umask 077 differs from the one packed under 022, want the same bytes")
	}
	for _, entry := range readArchive(t, closed) {
		want := int64(0o644)
		if entry.header.Typeflag == tar.TypeDir || entry.header.Name == "gophenberg" {
			want = 0o755
		}
		if entry.header.Mode != want {
			t.Errorf("%s has mode %o under umask 077, want %o", entry.header.Name, entry.header.Mode, want)
		}
	}
}

// packedUnderUmask packs this machine's archive in a new release repository under the file mode mask, and returns it.
func packedUnderUmask(t *testing.T, mask string) []byte {
	t.Helper()
	root := packingRepo(t)
	platform := runtime.GOOS + "/" + runtime.GOARCH
	args := []string{"release-archives", "RELEASE_TARGETS=" + platform, "RELEASE_EPOCH=" + strconv.Itoa(releaseEpoch)}
	path := "PATH=" + filepath.Join(root, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	command := exec.Command("sh", append([]string{"-c", `umask "$0" && exec make "$@"`, mask}, append(args, path)...)...)
	command.Dir = root
	command.Env = append(isolated(), "GOWORK=off", "GOFLAGS=", "MAKEFLAGS=", "MFLAGS=", "MAKELEVEL=")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("make %v under umask %s = %v with output %q", args, mask, err, out)
	}
	return archiveBytes(t, root, platform)
}

// archivesRepo returns a release repository, skipping when a tool release-archives runs is missing.
func archivesRepo(t *testing.T) string {
	t.Helper()
	for _, tool := range []string{"jq", "tar", "gzip"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH, so make release-archives cannot be exercised here", tool)
		}
	}
	return releaseRepo(t, "<p>the built admin</p>")
}

// packingRepo returns a release repository for a test that packs with the system tar, skipping when it is not GNU tar.
func packingRepo(t *testing.T) string {
	t.Helper()
	root := archivesRepo(t)
	version, err := exec.Command("tar", "--version").Output()
	if err != nil || !strings.Contains(string(version), "GNU tar") {
		t.Skip("tar is not GNU tar, which make release-archives packs with, so it cannot be exercised here")
	}
	return root
}

// anotherPlatform returns a platform a release builds other than host.
func anotherPlatform(host string) string {
	if host == "linux/arm64" {
		return "linux/amd64"
	}
	return "linux/arm64"
}

// packedArchive runs make with the arguments and returns the bytes of the platform's archive.
func packedArchive(t *testing.T, root, platform string, args ...string) []byte {
	t.Helper()
	if out, err := releaseIn(t, root, args...); err != nil {
		t.Fatalf("make %v = %v with output %q", args, err, out)
	}
	return archiveBytes(t, root, platform)
}

// archiveBytes returns the bytes of the platform's archive in the repository's release folder.
func archiveBytes(t *testing.T, root, platform string) []byte {
	t.Helper()
	packed, err := os.ReadFile(filepath.Join(root, "dist", "gophenberg_0.20.0_"+platformFolder(platform)+".tar.gz"))
	if err != nil {
		t.Fatalf("reading the %s archive: %v", platform, err)
	}
	return packed
}

// readArchive returns the entries of a gzipped tar archive, failing when its gzip header names a file or a time.
func readArchive(t *testing.T, packed []byte) []archiveEntry {
	t.Helper()
	unzipped, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatalf("opening the archive: %v", err)
	}
	if unzipped.Name != "" || !unzipped.ModTime.IsZero() {
		t.Errorf("the gzip header names %q at %v, want neither", unzipped.Name, unzipped.ModTime)
	}
	var entries []archiveEntry
	reader := tar.NewReader(unzipped)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return entries
		}
		if err != nil {
			t.Fatalf("reading the archive: %v", err)
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("reading %s: %v", header.Name, err)
		}
		entries = append(entries, archiveEntry{header: header, body: body})
	}
}

// entryNamed returns the archive entry called name.
func entryNamed(t *testing.T, entries []archiveEntry, name string) archiveEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.header.Name == name {
			return entry
		}
	}
	t.Fatalf("the archive holds no %s", name)
	return archiveEntry{}
}

// factsOf returns the release.json the archive's entries carry.
func factsOf(t *testing.T, entries []archiveEntry) archiveFacts {
	t.Helper()
	var facts archiveFacts
	if err := json.Unmarshal(entryNamed(t, entries, "release.json").body, &facts); err != nil {
		t.Fatalf("reading release.json: %v", err)
	}
	return facts
}
