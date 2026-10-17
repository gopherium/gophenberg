// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/signature"
)

// releaseDownloads is the address a release serves its files under, before its version.
const releaseDownloads = "https://github.com/gopherium/gophenberg/releases/download/v"

// releaseFilesKey keys the release files scenario in a scenario's context.
type releaseFilesKey struct{}

// releaseFilesScenario holds the folder, the key, the version and the archives one release files scenario works on.
type releaseFilesScenario struct {
	dir      string
	public   ed25519.PublicKey
	version  string
	archives []string
}

// recordEntry is one archive the release record lists, as its JSON names it.
type recordEntry struct {
	ID          string `json:"id"`
	Platform    string `json:"platform"`
	ContentType string `json:"content-type"`
	URL         string `json:"url"`
	Checksum    string `json:"checksum"`
	Signature   string `json:"signature"`
}

// provisionReleaseFiles gives a scenario an empty folder for its release files.
func provisionReleaseFiles(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
	dir, err := os.MkdirTemp("", "release-files")
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, releaseFilesKey{}, &releaseFilesScenario{dir: dir}), nil
}

// removeReleaseFiles removes the folder a scenario wrote its release files in.
func removeReleaseFiles(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
	s, held := releaseFilesOf(ctx)
	if held != nil {
		return ctx, errors.Join(err, held)
	}
	return ctx, errors.Join(err, os.RemoveAll(s.dir))
}

// releaseFilesOf returns the release files scenario the context carries.
func releaseFilesOf(ctx context.Context) (*releaseFilesScenario, error) {
	s, ok := ctx.Value(releaseFilesKey{}).(*releaseFilesScenario)
	if !ok {
		return nil, errors.New("the scenario has no release files state")
	}
	return s, nil
}

// theSignedArchives writes an archive of the version for each platform and signs each with a fresh release key.
func theSignedArchives(ctx context.Context, version, first, second, third string) error {
	s, err := releaseFilesOf(ctx)
	if err != nil {
		return err
	}
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}
	s.public, s.version = public, version
	for _, platform := range []string{first, second, third} {
		path := filepath.Join(s.dir, "gophenberg_"+version+"_"+strings.Replace(platform, "/", "_", 1)+".tar.gz")
		if err := writeSignedArchive(path, platform, private); err != nil {
			return err
		}
		s.archives = append(s.archives, path)
	}
	return nil
}

// writeSignedArchive writes an archive for the platform at path and its signature under key beside it.
func writeSignedArchive(path, platform string, key ed25519.PrivateKey) error {
	body := []byte("the " + platform + " archive\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return err
	}
	digest, err := signature.Digest(bytes.NewReader(body))
	if err != nil {
		return err
	}
	return os.WriteFile(path+".sig", []byte(signature.Sign(key, digest)+"\n"), 0o600)
}

// recordPath names the release record the scenario's release publishes.
func (s *releaseFilesScenario) recordPath() string {
	return filepath.Join(s.dir, "gophenberg_"+s.version+"_release-record.json")
}

// releasetoolWritesTheRecordAndSums builds releasetool and writes the release record, then SHA256SUMS over it all.
func releasetoolWritesTheRecordAndSums(ctx context.Context) error {
	s, err := releaseFilesOf(ctx)
	if err != nil {
		return err
	}
	binary := filepath.Join(s.dir, "releasetool")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/releasetool")
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("building releasetool: %w\n%s", err, out)
	}
	recordArgs := append([]string{"record", "-version", s.version, "-download-url", releaseDownloads + s.version},
		s.archives...)
	if err := runReleasetool(ctx, binary, s.recordPath(), recordArgs...); err != nil {
		return err
	}
	sumsArgs := append(append([]string{"sums"}, s.archives...), s.recordPath())
	return runReleasetool(ctx, binary, filepath.Join(s.dir, "SHA256SUMS"), sumsArgs...)
}

// runReleasetool runs releasetool with the arguments and writes what it prints to out.
func runReleasetool(ctx context.Context, binary, out string, args ...string) error {
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, binary, args...)
	command.Stderr = &stderr
	printed, err := command.Output()
	if err != nil {
		return fmt.Errorf("releasetool %s: %w: %s", args[0], err, stderr.String())
	}
	return os.WriteFile(out, printed, 0o600)
}

// sumsListsEveryFile fails unless SHA256SUMS lists each archive and the record by its SHA-256, sorted by name.
func sumsListsEveryFile(ctx context.Context) error {
	s, err := releaseFilesOf(ctx)
	if err != nil {
		return err
	}
	listed, err := os.ReadFile(filepath.Join(s.dir, "SHA256SUMS"))
	if err != nil {
		return err
	}
	files := sortedByName(append(slices.Clone(s.archives), s.recordPath()))
	var want strings.Builder
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		want.WriteString(hex.EncodeToString(sum[:]) + "  " + filepath.Base(path) + "\n")
	}
	if string(listed) != want.String() {
		return fmt.Errorf("SHA256SUMS holds %q, want %q", listed, want.String())
	}
	return nil
}

// recordListsEachArchive fails unless the record lists every archive by its platform, address, checksum and signature.
func recordListsEachArchive(ctx context.Context) error {
	s, err := releaseFilesOf(ctx)
	if err != nil {
		return err
	}
	entries, err := s.recordEntries()
	if err != nil {
		return err
	}
	var want []recordEntry
	for _, path := range sortedByName(s.archives) {
		entry, err := s.expectedEntry(path)
		if err != nil {
			return err
		}
		want = append(want, entry)
	}
	if !slices.Equal(entries, want) {
		return fmt.Errorf("the release record lists %+v, want %+v", entries, want)
	}
	return nil
}

// expectedEntry returns the record entry the archive at path should have.
func (s *releaseFilesScenario) expectedEntry(path string) (recordEntry, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return recordEntry{}, err
	}
	line, err := os.ReadFile(path + ".sig")
	if err != nil {
		return recordEntry{}, err
	}
	sum, err := signature.Checksum(bytes.NewReader(body))
	if err != nil {
		return recordEntry{}, err
	}
	name := filepath.Base(path)
	parts := strings.Split(strings.TrimSuffix(name, ".tar.gz"), "_")
	return recordEntry{
		ID: parts[2] + "-" + parts[3], Platform: parts[2] + "/" + parts[3], ContentType: "application/gzip",
		URL: releaseDownloads + s.version + "/" + name, Checksum: sum, Signature: strings.TrimSuffix(string(line), "\n"),
	}, nil
}

// eachRecordSignatureHolds fails unless every signature the record carries holds for its archive under the key.
func eachRecordSignatureHolds(ctx context.Context) error {
	s, err := releaseFilesOf(ctx)
	if err != nil {
		return err
	}
	entries, err := s.recordEntries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(s.dir, filepath.Base(entry.URL)))
		if err != nil {
			return err
		}
		digest, err := signature.Digest(bytes.NewReader(body))
		if err != nil {
			return err
		}
		if err := signature.Verify(s.public, digest, entry.Signature); err != nil {
			return fmt.Errorf("the signature of %s does not hold: %w", entry.ID, err)
		}
	}
	return nil
}

// recordEntries returns the archives the scenario's release record lists.
func (s *releaseFilesScenario) recordEntries() ([]recordEntry, error) {
	raw, err := os.ReadFile(s.recordPath())
	if err != nil {
		return nil, err
	}
	var record struct {
		Version   string `json:"version"`
		Artifacts struct {
			Package []recordEntry `json:"package"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, fmt.Errorf("reading the release record %q: %w", raw, err)
	}
	if record.Version != s.version {
		return nil, fmt.Errorf("the release record names version %q, want %q", record.Version, s.version)
	}
	return record.Artifacts.Package, nil
}

// sortedByName returns the paths sorted by their base names.
func sortedByName(paths []string) []string {
	sorted := slices.Clone(paths)
	slices.SortFunc(sorted, func(a, b string) int { return strings.Compare(filepath.Base(a), filepath.Base(b)) })
	return sorted
}

// initializeReleaseFiles registers the steps that write and read SHA256SUMS and the release record.
func initializeReleaseFiles(sc *godog.ScenarioContext) {
	sc.Before(provisionReleaseFiles)
	sc.After(removeReleaseFiles)
	sc.Given(`^the archives of version "([^"]*)" for "([^"]*)", "([^"]*)" and "([^"]*)", each signed by a fresh `+
		`release key$`, theSignedArchives)
	sc.When(`^releasetool writes the release record and SHA256SUMS$`, releasetoolWritesTheRecordAndSums)
	sc.Then(`^SHA256SUMS lists every archive and the release record by its SHA-256, sorted by name$`,
		sumsListsEveryFile)
	sc.Then(`^the release record lists each archive by its platform, with its address, its checksum and its `+
		`signature$`, recordListsEachArchive)
	sc.Then(`^each signature the release record carries holds for its archive under the release key$`,
		eachRecordSignatureHolds)
}
