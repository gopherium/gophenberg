// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gopherium/gophenberg/internal/signature"
)

// sums prints the SHA256SUMS line of every file, sorted by name, refusing a signature file.
func sums(args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("sums", stderr)
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return usageError{"sums needs at least one file to list"}
	}
	files := releaseFiles(flags.Args())
	sortByName(files)
	var listed strings.Builder
	for _, file := range files {
		line, err := file.sumLine()
		if err != nil {
			return err
		}
		listed.WriteString(line)
	}
	_, err := io.WriteString(stdout, listed.String())
	return err
}

// sortByName sorts the files by their base names.
func sortByName(files []releaseFile) {
	slices.SortFunc(files, func(a, b releaseFile) int { return strings.Compare(a.name(), b.name()) })
}

// name returns the file's base name.
func (f releaseFile) name() string {
	return filepath.Base(string(f))
}

// sumLine returns the file's SHA256SUMS line, its SHA-256 and its name, refusing a signature file.
func (f releaseFile) sumLine() (string, error) {
	if strings.HasSuffix(f.name(), signatureSuffix) {
		return "", fmt.Errorf("refused %s: SHA256SUMS lists release files, never their signatures", f)
	}
	sum, err := f.checksum()
	if err != nil {
		return "", err
	}
	_, hexadecimal, _ := strings.Cut(sum, ":")
	return hexadecimal + "  " + f.name() + "\n", nil
}

// checksum returns the file's FAIR checksum, sha256: and its SHA-256 in hexadecimal.
func (f releaseFile) checksum() (string, error) {
	file, err := os.Open(string(f))
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", f, err)
	}
	sum, err := signature.Checksum(file)
	return sum, errors.Join(err, file.Close())
}
