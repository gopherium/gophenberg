// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/gopherium/gophenberg/internal/signature"
)

// signatureSuffix names the file beside each signed file that holds its signature.
const signatureSuffix = ".sig"

// releaseFile is a file a release publishes, whose signature sits in the .sig beside it.
type releaseFile string

// signOptions holds the flags of one sign run.
type signOptions struct {
	key            string
	passphraseFile string
}

// sign writes beside each file a .sig holding its FAIR signature under the release key.
func sign(args []string, stdout, stderr io.Writer) error {
	var opts signOptions
	flags := newFlagSet("sign", stderr)
	flags.StringVar(&opts.key, "key", "", "the release key file")
	flags.StringVar(&opts.passphraseFile, "passphrase-file", "", "a file whose first line opens a sealed release key")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if opts.key == "" || flags.NArg() == 0 {
		return usageError{"sign needs -key, the release key file, and at least one file to sign"}
	}
	key, err := opts.releaseKey()
	if err != nil {
		return err
	}
	return signAll(releaseFiles(flags.Args()), key, stdout)
}

// signAll signs every file, writing no .sig until each file is read and none has a .sig already.
func signAll(files []releaseFile, key ed25519.PrivateKey, stdout io.Writer) error {
	if err := refuseSigned(files); err != nil {
		return err
	}
	lines, err := signatureLines(files, key)
	if err != nil {
		return err
	}
	for i, file := range files {
		if err := file.writeSignature(lines[i]); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "signed %s\n", file)
	}
	return nil
}

// signatureLines returns the FAIR signature line of every file under the key.
func signatureLines(files []releaseFile, key ed25519.PrivateKey) ([]string, error) {
	lines := make([]string, len(files))
	for i, file := range files {
		digest, err := file.digest()
		if err != nil {
			return nil, err
		}
		lines[i] = signature.Sign(key, digest)
	}
	return lines, nil
}

// verify checks the .sig beside each file under the release public key.
func verify(args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("verify", stderr)
	public := flags.String("public", "", "the release public key: a PEM file or a Multikey")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if *public == "" || flags.NArg() == 0 {
		return usageError{"verify needs -public, the release public key, and at least one file to verify"}
	}
	key, err := loadPublicKey(*public)
	if err != nil {
		return err
	}
	for _, file := range releaseFiles(flags.Args()) {
		if err := file.verify(key); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		_, _ = fmt.Fprintf(stdout, "verified %s\n", file)
	}
	return nil
}

// releaseFiles returns the paths a command was given as release files.
func releaseFiles(paths []string) []releaseFile {
	files := make([]releaseFile, len(paths))
	for i, path := range paths {
		files[i] = releaseFile(path)
	}
	return files
}

// releaseKey reads the release key file, opening a sealed one with the passphrase file's first line.
func (o signOptions) releaseKey() (ed25519.PrivateKey, error) {
	var passphrase phrase
	if o.passphraseFile != "" {
		read, err := readPassphrase(o.passphraseFile)
		if err != nil {
			return nil, err
		}
		passphrase = read
	}
	raw, err := os.ReadFile(o.key)
	if err != nil {
		return nil, fmt.Errorf("reading the release key: %w", err)
	}
	key, err := readKey(bytes.NewReader(raw), passphrase)
	if err != nil {
		return nil, err
	}
	release, isEd25519 := key.(ed25519.PrivateKey)
	if !isEd25519 {
		return nil, fmt.Errorf("refused %s, which holds a %T: release files are signed with an Ed25519 key", o.key, key)
	}
	return release, nil
}

// loadPublicKey reads the release public key from the text as a Multikey, or else from the PEM file of that name.
func loadPublicKey(value string) (ed25519.PublicKey, error) {
	key, err := signature.ParseMultikey(value)
	if err != nil {
		key, err = readPublicKeyFile(value, err)
	}
	if err != nil {
		return nil, err
	}
	release, isEd25519 := key.(ed25519.PublicKey)
	if !isEd25519 {
		return nil, fmt.Errorf("refused the public key, a %T: release files are checked under an Ed25519 key", key)
	}
	return release, nil
}

// readPublicKeyFile reads the public key file at path, returning the Multikey error when no such file exists.
func readPublicKeyFile(path string, notMultikey error) (crypto.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, notMultikey
	}
	if err != nil {
		return nil, fmt.Errorf("reading the public key: %w", err)
	}
	return signature.ReadPublicKey(bytes.NewReader(raw))
}

// refuseSigned refuses the run when any file already has a .sig, since a published signature never changes.
func refuseSigned(files []releaseFile) error {
	for _, file := range files {
		if _, err := os.Lstat(file.signaturePath()); err == nil {
			return fmt.Errorf("refused to replace %s: a published signature never changes", file.signaturePath())
		}
	}
	return nil
}

// signaturePath names the .sig beside the file.
func (f releaseFile) signaturePath() string {
	return string(f) + signatureSuffix
}

// writeSignature writes the signature line to a new .sig beside the file, leaving none when the write fails.
func (f releaseFile) writeSignature(line string) error {
	file, err := createNew(f.signaturePath(), publicMode)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(file, line)
	return removeIfFailed(file, errors.Join(err, file.Close()))
}

// removeIfFailed removes the file a failed write left behind, so a retry is not refused, and returns the failure.
func removeIfFailed(file *os.File, failure error) error {
	if failure == nil {
		return nil
	}
	return errors.Join(failure, os.Remove(file.Name()))
}

// verify checks the .sig beside the file under the key.
func (f releaseFile) verify(key ed25519.PublicKey) error {
	line, err := f.signatureLine()
	if err != nil {
		return err
	}
	digest, err := f.digest()
	if err != nil {
		return err
	}
	return signature.Verify(key, digest, line)
}

// signatureLine returns the one line the file's .sig holds, refusing a .sig with more.
func (f releaseFile) signatureLine() (string, error) {
	raw, err := os.ReadFile(f.signaturePath())
	if err != nil {
		return "", fmt.Errorf("reading the signature: %w", err)
	}
	text := strings.TrimSuffix(string(raw), "\n")
	if lines := strings.Count(text, "\n") + 1; lines > 1 {
		return "", fmt.Errorf("refused %s: it holds %d lines, and a release signature holds exactly one",
			f.signaturePath(), lines)
	}
	return text, nil
}

// digest returns the SHA-384 digest of the file.
func (f releaseFile) digest() ([signature.DigestSize]byte, error) {
	file, err := os.Open(string(f))
	if err != nil {
		return [signature.DigestSize]byte{}, fmt.Errorf("reading %s: %w", f, err)
	}
	digest, err := signature.Digest(file)
	return digest, errors.Join(err, file.Close())
}
