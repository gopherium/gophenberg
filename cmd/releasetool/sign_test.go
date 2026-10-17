// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/ed25519"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/signature"
)

func TestSignWritesOneFAIRLinePerFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	key, keyFile := newReleaseKey(t, dir)
	archives := []string{
		writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n"),
		writeFile(t, dir, "gophenberg_0.20.0_linux_arm64.tar.gz", "an arm64 archive\n"),
	}

	got := runTool(append([]string{"sign", "-key", keyFile}, archives...)...)

	if got.code != 0 {
		t.Fatalf("sign = %d, %q, want 0", got.code, got.stderr)
	}
	for _, archive := range archives {
		want := signature.Sign(key, digestOfFile(t, archive)) + "\n"
		if line := readText(t, archive+".sig"); line != want {
			t.Errorf("sign wrote %q beside %s, want the one FAIR line %q", line, archive, want)
		}
	}
}

func TestSignRefusesToReplaceASignature(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, keyFile := newReleaseKey(t, dir)
	fresh := writeFile(t, dir, "gophenberg_0.20.0_darwin_arm64.tar.gz", "a darwin archive\n")
	published := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")
	writeFile(t, dir, filepath.Base(published)+".sig", "the published signature\n")

	got := runTool("sign", "-key", keyFile, fresh, published)

	if got.code != 1 || !strings.Contains(got.stderr, published+".sig") {
		t.Errorf("sign over a published signature = %d, %q, want 1 and the .sig named", got.code, got.stderr)
	}
	if kept := readText(t, published+".sig"); kept != "the published signature\n" {
		t.Errorf("sign left %q in the published .sig, want it untouched", kept)
	}
	if _, err := os.Stat(fresh + ".sig"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("sign signed %s before it refused the batch: %v", fresh, err)
	}
}

func TestSignWritesNoSignatureWhenAnyFileCannotBeRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, keyFile := newReleaseKey(t, dir)
	good := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")
	missing := filepath.Join(dir, "gophenberg_0.20.0_linux_arm64.tar.gz")

	got := runTool("sign", "-key", keyFile, good, missing)

	if got.code != 1 || !strings.Contains(got.stderr, missing) || got.stdout != "" {
		t.Errorf("sign with a missing file = %d, %q, %q, want 1, %s named and nothing signed", got.code, got.stdout,
			got.stderr, missing)
	}
	if _, err := os.Stat(good + ".sig"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("sign signed %s before it found %s missing: %v", good, missing, err)
	}
}

func TestRemoveIfFailedRemovesTheFileAFailedWriteLeft(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gophenberg_0.20.0_linux_amd64.tar.gz.sig")
	file, err := createNew(path, publicMode)
	if err != nil {
		t.Fatalf("createNew: %v", err)
	}
	failure := errors.New("no room left on the device")

	got := removeIfFailed(file, errors.Join(failure, file.Close()))

	if !errors.Is(got, failure) {
		t.Errorf("removeIfFailed() = %v, want the write's failure kept", got)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("removeIfFailed left %s behind: %v", path, err)
	}
}

func TestVerifyReadsAMultikeyEvenWhenAFileHasItsName(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	key, archive := signedArchive(t, dir)
	multikey := multikeyOf(t, key)
	_, other, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generating another key: %v", err)
	}
	var otherPEM strings.Builder
	if err := writePublicKey(&otherPEM, other.Public()); err != nil {
		t.Fatalf("writePublicKey: %v", err)
	}
	writeFile(t, dir, multikey, otherPEM.String())

	got := runTool("verify", "-public", multikey, archive)

	if got.code != 0 {
		t.Errorf("verify -public %s beside a file of that name = %d, %q, want 0 under the Multikey itself", multikey,
			got.code, got.stderr)
	}
}

func TestVerifyAcceptsAGoodSignatureUnderAMultikey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	key, archive := signedArchive(t, dir)
	multikey, err := signature.Multikey(key.Public())
	if err != nil {
		t.Fatalf("Multikey: %v", err)
	}
	var publicPEM strings.Builder
	if err := writePublicKey(&publicPEM, key.Public()); err != nil {
		t.Fatalf("writePublicKey: %v", err)
	}
	publicFile := writeFile(t, dir, "release.pub.pem", publicPEM.String())

	for _, public := range []string{multikey, publicFile} {
		got := runTool("verify", "-public", public, archive)

		if got.code != 0 || got.stdout != "verified "+archive+"\n" {
			t.Errorf("verify -public %s = %d, %q, %q, want 0 and the file verified", public, got.code, got.stdout,
				got.stderr)
		}
	}
}

func TestVerifyRefusesASignatureFileWithTwoLines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	key, archive := signedArchive(t, dir)
	line := readText(t, archive+".sig")
	if err := os.WriteFile(archive+".sig", []byte(line+line), 0o600); err != nil {
		t.Fatalf("writing a second line: %v", err)
	}

	got := runTool("verify", "-public", multikeyOf(t, key), archive)

	if got.code != 1 || !strings.Contains(got.stderr, "2 lines") {
		t.Errorf("verify of a two-line .sig = %d, %q, want 1 and the lines named", got.code, got.stderr)
	}
}

func TestVerifyNamesTheFileThatFailsAndExitsOne(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	key, keyFile := newReleaseKey(t, dir)
	good := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")
	changed := writeFile(t, dir, "gophenberg_0.20.0_linux_arm64.tar.gz", "an arm64 archive\n")
	if got := runTool("sign", "-key", keyFile, good, changed); got.code != 0 {
		t.Fatalf("sign = %d, %q, want 0", got.code, got.stderr)
	}
	writeFile(t, dir, filepath.Base(changed), "an arm64 archive, changed\n")

	got := runTool("verify", "-public", multikeyOf(t, key), good, changed)

	if got.code != 1 || !strings.Contains(got.stderr, changed) {
		t.Errorf("verify of a changed file = %d, %q, want 1 and %s named", got.code, got.stderr, changed)
	}
	if got.stdout != "verified "+good+"\n" {
		t.Errorf("verify printed %q, want only %s verified", got.stdout, good)
	}
}

func TestSignAndVerifyExitTwoWhenMisused(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	key, archive := signedArchive(t, dir)
	keyFile := filepath.Join(dir, "release.pem")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"sign with no key", []string{"sign", archive}, "-key"},
		{"sign with no file", []string{"sign", "-key", keyFile}, "file to sign"},
		{"sign with a flag it does not know", []string{"sign", "-key", keyFile, "-force", archive}, "-force"},
		{"verify with no key", []string{"verify", archive}, "-public"},
		{"verify with no file", []string{"verify", "-public", multikeyOf(t, key)}, "file to verify"},
		{"verify with a flag it does not know", []string{"verify", "-strict", archive}, "-strict"},
	}
	for _, tc := range cases {
		got := runTool(tc.args...)

		if got.code != 2 || !strings.Contains(got.stderr, tc.want) {
			t.Errorf("%s = %d, %q, want 2 and %q named", tc.name, got.code, got.stderr, tc.want)
		}
	}
}

func TestSignAndVerifyRefuseARootKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root := newTestP256(t)
	rootFile := writeFile(t, dir, "root1.pem", plainPEM(t, root))
	rootMultikey, err := signature.Multikey(&root.PublicKey)
	if err != nil {
		t.Fatalf("Multikey: %v", err)
	}
	_, archive := signedArchive(t, dir)

	for _, args := range [][]string{{"sign", "-key", rootFile, archive}, {"verify", "-public", rootMultikey, archive}} {
		got := runTool(args...)

		if got.code != 1 || !strings.Contains(got.stderr, "Ed25519") {
			t.Errorf("%s with a P-256 key = %d, %q, want 1 and Ed25519 named", args[0], got.code, got.stderr)
		}
	}
}

func TestSignOpensASealedReleaseKeyWithItsPassphrase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generating a release key: %v", err)
	}
	keyFile := writeFile(t, dir, "release.pem", sealedPEM(t, key))
	passphrase := writeFile(t, dir, "passphrase", string(testSealing.passphrase)+"\n")
	archive := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")

	got := runTool("sign", "-key", keyFile, "-passphrase-file", passphrase, archive)

	if got.code != 0 {
		t.Fatalf("sign with a sealed key = %d, %q, want 0", got.code, got.stderr)
	}
	if verified := runTool("verify", "-public", multikeyOf(t, key), archive); verified.code != 0 {
		t.Errorf("verify after a sealed key signed = %d, %q, want 0", verified.code, verified.stderr)
	}
}

func TestSignReportsWhatItCannotRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, keyFile := newReleaseKey(t, dir)
	archive := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")
	longName := writeFile(t, dir, strings.Repeat("a", 252), "an archive whose .sig name is too long\n")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"a missing key", []string{"-key", filepath.Join(dir, "missing.pem"), archive}, "release key"},
		{"a key file with no key", []string{"-key", archive, archive}, "no PEM block"},
		{"a missing passphrase file", []string{"-key", keyFile, "-passphrase-file", filepath.Join(dir, "none"), archive},
			"passphrase"},
		{"a missing file", []string{"-key", keyFile, filepath.Join(dir, "missing.tar.gz")}, "missing.tar.gz"},
		{"a folder", []string{"-key", keyFile, dir}, "is a directory"},
		{"a .sig name too long to write", []string{"-key", keyFile, longName}, "refused to write"},
	}
	for _, tc := range cases {
		got := runTool(append([]string{"sign"}, tc.args...)...)

		if got.code != 1 || !strings.Contains(got.stderr, tc.want) {
			t.Errorf("sign with %s = %d, %q, want 1 and %q named", tc.name, got.code, got.stderr, tc.want)
		}
	}
}

func TestVerifyReportsWhatItCannotRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	key, archive := signedArchive(t, dir)
	multikey := multikeyOf(t, key)
	unsigned := writeFile(t, dir, "gophenberg_0.20.0_darwin_arm64.tar.gz", "a darwin archive\n")
	orphan := filepath.Join(dir, "gophenberg_0.20.0_linux_arm64.tar.gz")
	writeFile(t, dir, filepath.Base(orphan)+".sig", readText(t, archive+".sig"))
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"a key that is neither file nor Multikey", []string{"-public", "release.pub.pem", archive}, "z, the base58btc"},
		{"a folder for a key", []string{"-public", dir, archive}, "reading the public key"},
		{"a file with no .sig", []string{"-public", multikey, unsigned}, "reading the signature"},
		{"a .sig with no file", []string{"-public", multikey, orphan}, "reading " + orphan},
	}
	for _, tc := range cases {
		got := runTool(append([]string{"verify"}, tc.args...)...)

		if got.code != 1 || !strings.Contains(got.stderr, tc.want) {
			t.Errorf("verify with %s = %d, %q, want 1 and %q named", tc.name, got.code, got.stderr, tc.want)
		}
	}
}

// multikeyOf returns the Multikey of a release key's public half.
func multikeyOf(t *testing.T, key ed25519.PrivateKey) string {
	t.Helper()
	multikey, err := signature.Multikey(key.Public())
	if err != nil {
		t.Fatalf("Multikey: %v", err)
	}
	return multikey
}

// signedArchive writes an archive in dir, signs it with a new release key, and returns the key and the archive.
func signedArchive(t *testing.T, dir string) (ed25519.PrivateKey, string) {
	t.Helper()
	key, keyFile := newReleaseKey(t, dir)
	archive := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")
	if got := runTool("sign", "-key", keyFile, archive); got.code != 0 {
		t.Fatalf("sign = %d, %q, want 0", got.code, got.stderr)
	}
	return key, archive
}

// newReleaseKey writes a plain Ed25519 release key in dir and returns it with its file.
func newReleaseKey(t *testing.T, dir string) (ed25519.PrivateKey, string) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generating a release key: %v", err)
	}
	path := filepath.Join(dir, "release.pem")
	var file strings.Builder
	if err := writeKey(&file, key, sealing{}); err != nil {
		t.Fatalf("writeKey: %v", err)
	}
	return key, writeFile(t, dir, filepath.Base(path), file.String())
}

// digestOfFile returns the SHA-384 digest of a file.
func digestOfFile(t *testing.T, path string) [signature.DigestSize]byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()
	digest, err := signature.Digest(file)
	if err != nil {
		t.Fatalf("Digest(%s): %v", path, err)
	}
	return digest
}
