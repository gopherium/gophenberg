// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
)

// productName is the name every release archive starts with.
const productName = "gophenberg"

// archiveTypes maps each archive extension a record knows to the content type it is published as.
var archiveTypes = []struct{ extension, contentType string }{
	{".tar.gz", "application/gzip"},
	{".zip", "application/zip"},
}

// releaseRecord is the FAIR release document of one version, listing every archive it publishes.
type releaseRecord struct {
	Version   string          `json:"version"`
	Artifacts recordArtifacts `json:"artifacts"`
}

// recordArtifacts holds a release record's files by kind.
type recordArtifacts struct {
	Package []artifact `json:"package"`
}

// artifact is one archive a release record lists, with what an updater needs to pick, fetch and check it.
type artifact struct {
	ID          string `json:"id"`
	Platform    string `json:"platform"`
	ContentType string `json:"content-type"`
	URL         string `json:"url"`
	Checksum    string `json:"checksum"`
	Signature   string `json:"signature"`
}

// archiveName is a release archive's name read on the pattern <product>[-<variant>]_<version>_<os>_<arch>.
type archiveName struct {
	variant     string
	os          string
	arch        string
	contentType string
}

// recordOptions holds the flags of one record run.
type recordOptions struct {
	version string
	base    string
}

// record prints the release record of the version over the signed archives, each served under the download address.
func record(args []string, stdout, stderr io.Writer) error {
	var opts recordOptions
	flags := newFlagSet("record", stderr)
	flags.StringVar(&opts.version, "version", "", "the version the archives carry")
	flags.StringVar(&opts.base, "download-url", "", "the https address the release serves its files under")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if err := opts.validate(flags.NArg()); err != nil {
		return err
	}
	files := releaseFiles(flags.Args())
	sortByName(files)
	entries, err := opts.artifacts(files)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(releaseRecord{Version: opts.version, Artifacts: recordArtifacts{Package: entries}})
}

// validate refuses a run with no version, no archive, or a download address that is not https.
func (o recordOptions) validate(archives int) error {
	if o.version == "" {
		return usageError{"record needs -version, the version the archives carry"}
	}
	if archives == 0 {
		return usageError{"record needs at least one archive to list"}
	}
	if !httpsAddress(o.base) {
		return usageError{fmt.Sprintf("record -download-url %q is not the https address the release serves its files under",
			o.base)}
	}
	return nil
}

// httpsAddress reports whether text is an https address naming a host.
func httpsAddress(text string) bool {
	address, err := url.Parse(text)
	if err != nil {
		return false
	}
	return address.Scheme == "https" && address.Host != ""
}

// artifacts returns the record entry of every archive, in the order given.
func (o recordOptions) artifacts(files []releaseFile) ([]artifact, error) {
	entries := make([]artifact, len(files))
	for i, file := range files {
		entry, err := o.artifact(file)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		entries[i] = entry
	}
	return entries, nil
}

// artifact returns the record entry of the archive, read from its name, its bytes and the .sig beside it.
func (o recordOptions) artifact(file releaseFile) (artifact, error) {
	parsed, err := parseArchiveName(file.name(), o.version)
	if err != nil {
		return artifact{}, err
	}
	line, err := file.signatureLine()
	if err != nil {
		return artifact{}, err
	}
	sum, err := file.checksum()
	if err != nil {
		return artifact{}, err
	}
	return artifact{
		ID: parsed.id(), Platform: parsed.platform(), ContentType: parsed.contentType,
		URL: strings.TrimSuffix(o.base, "/") + "/" + file.name(), Checksum: sum, Signature: line,
	}, nil
}

// parseArchiveName reads an archive name on the pattern, refusing another product, another version or extension.
func parseArchiveName(name, version string) (archiveName, error) {
	stem, contentType, err := splitExtension(name)
	if err != nil {
		return archiveName{}, err
	}
	parts := strings.Split(stem, "_")
	if len(parts) != 4 || slices.Contains(parts, "") {
		return archiveName{}, errors.New("refused the name: a release archive is named " +
			"<product>_<version>_<os>_<arch> and an extension")
	}
	product, variant, _ := strings.Cut(parts[0], "-")
	if product != productName || parts[1] != version {
		return archiveName{}, fmt.Errorf("refused the name: it names %s %s, and this record lists %s %s",
			parts[0], parts[1], productName, version)
	}
	return archiveName{variant: variant, os: parts[2], arch: parts[3], contentType: contentType}, nil
}

// splitExtension returns the name without its archive extension and the content type that extension is published as.
func splitExtension(name string) (string, string, error) {
	for _, known := range archiveTypes {
		if stem, found := strings.CutSuffix(name, known.extension); found {
			return stem, known.contentType, nil
		}
	}
	return "", "", errors.New("refused the name: its extension is neither .tar.gz nor .zip")
}

// id returns the record id of the archive, <os>-<arch>, after its variant when it has one.
func (a archiveName) id() string {
	id := a.os + "-" + a.arch
	if a.variant != "" {
		return a.variant + "-" + id
	}
	return id
}

// platform returns the archive's platform as <os>/<arch>.
func (a archiveName) platform() string {
	return a.os + "/" + a.arch
}
