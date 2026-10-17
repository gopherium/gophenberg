// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/signature"
)

func TestKeygenWritesAnEd25519KeyAndPrintsItsMultikey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	out, public := filepath.Join(dir, "release.pem"), filepath.Join(dir, "release.pub.pem")

	got := runTool("keygen", "-curve", "ed25519", "-out", out, "-public", public)

	if got.code != 0 {
		t.Fatalf("keygen = %d, %q, want 0", got.code, got.stderr)
	}
	release, isEd25519 := readKeyFile(t, out, nil).(ed25519.PrivateKey)
	if !isEd25519 {
		t.Fatalf("keygen -curve ed25519 wrote a %T key", release)
	}
	raw := release.Public().(ed25519.PublicKey)
	if want := keygenLines(t, raw, raw); got.stdout != want {
		t.Errorf("keygen printed %q, want %q", got.stdout, want)
	}
	if mode := fileMode(t, out); mode != 0o600 {
		t.Errorf("keygen wrote the private key with mode %o, want 600", mode)
	}
	if written := readPublicFile(t, public); !raw.Equal(written) {
		t.Errorf("keygen wrote the public key %v, want %v", written, raw)
	}
}

func TestKeygenRefusesToOverwriteAKey(t *testing.T) {
	t.Parallel()

	out := writeFile(t, t.TempDir(), "release.pem", "the key a secret already holds\n")

	got := runTool("keygen", "-curve", "ed25519", "-out", out)

	if got.code != 1 || !strings.Contains(got.stderr, out) {
		t.Errorf("keygen over an existing key = %d, %q, want 1 and the file named", got.code, got.stderr)
	}
	if kept := readText(t, out); kept != "the key a secret already holds\n" {
		t.Errorf("keygen left %q in the existing key file, want it untouched", kept)
	}
}

func TestKeygenLeavesNoKeyBehindWhenThePublicFileExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	out, public := filepath.Join(dir, "release.pem"), writeFile(t, dir, "release.pub.pem", "an older public key\n")

	got := runTool("keygen", "-curve", "ed25519", "-out", out, "-public", public)

	if got.code != 1 || !strings.Contains(got.stderr, public) {
		t.Errorf("keygen over an existing public file = %d, %q, want 1 and the file named", got.code, got.stderr)
	}
	if _, err := os.Stat(out); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("keygen left a private key at %s with no public file beside it: %v", out, err)
	}
	if kept := readText(t, public); kept != "an older public key\n" {
		t.Errorf("keygen left %q in the existing public file, want it untouched", kept)
	}
}

func TestKeygenRefusesAP256KeyWithoutAPassphraseFile(t *testing.T) {
	t.Parallel()

	out := filepath.Join(t.TempDir(), "root1.pem")

	got := runTool("keygen", "-curve", "p256", "-out", out)

	if got.code != 2 || !strings.Contains(got.stderr, "-passphrase-file") {
		t.Errorf("keygen of a plain root key = %d, %q, want 2 and -passphrase-file named", got.code, got.stderr)
	}
	if _, err := os.Stat(out); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("keygen wrote a plain root key at %s: %v", out, err)
	}
}

func TestKeygenRefusesAPassphraseWithoutIterations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	out, passphrase := filepath.Join(dir, "root1.pem"), writeFile(t, dir, "passphrase", "a root passphrase\n")

	got := runTool("keygen", "-curve", "p256", "-out", out, "-passphrase-file", passphrase)

	if got.code != 2 || !strings.Contains(got.stderr, "-iterations") {
		t.Errorf("keygen without -iterations = %d, %q, want 2 and -iterations named", got.code, got.stderr)
	}
	if _, err := os.Stat(out); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("keygen sealed a root key with no rounds at %s: %v", out, err)
	}
}

func TestKeygenRefusesFewerRoundsThanTheFloor(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	out, passphrase := filepath.Join(dir, "root1.pem"), writeFile(t, dir, "passphrase", "a root passphrase\n")

	got := runTool("keygen", "-curve", "p256", "-out", out, "-passphrase-file", passphrase, "-iterations", "1000")

	if got.code != 2 || !strings.Contains(got.stderr, "600000") {
		t.Errorf("keygen at 1000 rounds = %d, %q, want 2 and the floor named", got.code, got.stderr)
	}
	if _, err := os.Stat(out); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("keygen sealed a root key with 1000 rounds at %s: %v", out, err)
	}
}

func TestKeygenSealsARootKeyThatOpensWithItsPassphrase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	out, passphrase := filepath.Join(dir, "root1.pem"), writeFile(t, dir, "passphrase", "a root passphrase\n")

	got := runTool("keygen", "-curve", "p256", "-out", out, "-passphrase-file", passphrase, "-iterations", "600000")

	if got.code != 0 {
		t.Fatalf("keygen of a root key = %d, %q, want 0", got.code, got.stderr)
	}
	if !strings.HasPrefix(readText(t, out), "-----BEGIN ENCRYPTED PRIVATE KEY-----\n") {
		t.Errorf("keygen wrote %q, want an encrypted PKCS#8 file", readText(t, out))
	}
	root, isECDSA := readKeyFile(t, out, []byte("a root passphrase")).(*ecdsa.PrivateKey)
	if !isECDSA {
		t.Fatalf("keygen -curve p256 wrote a %T key", root)
	}
	point, err := root.PublicKey.Bytes()
	if err != nil {
		t.Fatalf("reading the root key's point: %v", err)
	}
	compressed := append([]byte{2 + point[64]%2}, point[1:33]...)
	if want := keygenLines(t, &root.PublicKey, compressed); got.stdout != want {
		t.Errorf("keygen printed %q, want %q", got.stdout, want)
	}
	if mode := fileMode(t, out); mode != 0o600 {
		t.Errorf("keygen wrote the root key with mode %o, want 600", mode)
	}
}

func TestKeygenWritesToStandardOutputForAPipe(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	got := runTool("keygen", "-curve", "ed25519", "-out", "-")

	if got.code != 0 {
		t.Fatalf("keygen -out - = %d, %q, want 0", got.code, got.stderr)
	}
	key, err := readKey(strings.NewReader(got.stdout), nil)
	if err != nil {
		t.Fatalf("readKey of standard output: %v", err)
	}
	raw := key.Public().(ed25519.PublicKey)
	if want := keygenLines(t, raw, raw); got.stderr != want {
		t.Errorf("keygen -out - printed %q on standard error, want %q", got.stderr, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "-")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("keygen -out - wrote a file named -: %v", err)
	}
}

func TestKeygenExitsTwoWhenMisused(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	out := filepath.Join(dir, "release.pem")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"a flag it does not know", []string{"-curve", "ed25519", "-out", out, "-seed", "1"}, "-seed"},
		{"a curve it does not make", []string{"-curve", "secp256k1", "-out", out}, "ed25519 or p256"},
		{"no -out", []string{"-curve", "ed25519"}, "-out"},
		{"rounds with no passphrase", []string{"-curve", "ed25519", "-out", out, "-iterations", "600000"}, "-iterations"},
	}
	for _, tc := range cases {
		got := runTool(append([]string{"keygen"}, tc.args...)...)

		if got.code != 2 || !strings.Contains(got.stderr, tc.want) {
			t.Errorf("keygen with %s = %d, %q, want 2 and %q named", tc.name, got.code, got.stderr, tc.want)
		}
	}
	if _, err := os.Stat(out); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a misused keygen wrote %s: %v", out, err)
	}
}

func TestKeygenPrintsItsFlagsForHelp(t *testing.T) {
	t.Parallel()

	got := runTool("keygen", "-h")

	if got.code != 0 || !strings.Contains(got.stderr, "-passphrase-file") {
		t.Errorf("keygen -h = %d, %q, want 0 and the flags listed", got.code, got.stderr)
	}
}

func TestKeygenRefusesAPassphraseFileItCannotUse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cases := map[string]struct {
		file string
		want string
	}{
		"an empty first line":   {writeFile(t, dir, "empty", "\nthe second line\n"), "first line is empty"},
		"a missing file":        {filepath.Join(dir, "missing"), "reading the passphrase"},
		"a Windows line ending": {writeFile(t, dir, "crlf", "a root passphrase\r\n"), "carriage return"},
		"a device":              {os.DevNull, "terminal"},
	}
	for name, tc := range cases {
		out := filepath.Join(dir, "root-"+strings.ReplaceAll(name, " ", "-")+".pem")

		got := runTool("keygen", "-curve", "p256", "-out", out, "-passphrase-file", tc.file, "-iterations", "600000")

		if got.code != 1 || !strings.Contains(got.stderr, tc.want) {
			t.Errorf("keygen with %s = %d, %q, want 1 and %q named", name, got.code, got.stderr, tc.want)
		}
		if _, err := os.Stat(out); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("keygen with %s wrote %s: %v", name, out, err)
		}
	}
}

func TestKeygenReportsAStandardOutputThatFails(t *testing.T) {
	t.Parallel()

	var stderr strings.Builder
	failure := errors.New("the pipe closed")

	code := run([]string{"keygen", "-curve", "ed25519", "-out", "-"}, failingWriter{failure}, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), failure.Error()) {
		t.Errorf("keygen into a closed pipe = %d, %q, want 1 and the failure named", code, stderr.String())
	}
}

// writeFile writes text to a new file in dir and returns its path.
func writeFile(t *testing.T, dir, name, text string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// readText returns what a file holds.
func readText(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}

// keygenLines returns the two lines keygen prints for a public key and its raw bytes.
func keygenLines(t *testing.T, key crypto.PublicKey, raw []byte) string {
	t.Helper()
	multikey, err := signature.Multikey(key)
	if err != nil {
		t.Fatalf("Multikey: %v", err)
	}
	return fmt.Sprintf("multikey %s\npublic-key-hex %x\n", multikey, raw)
}

// readKeyFile reads a private key file keygen wrote.
func readKeyFile(t *testing.T, path string, passphrase []byte) crypto.Signer {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	key, err := readKey(bytes.NewReader(raw), passphrase)
	if err != nil {
		t.Fatalf("readKey(%s): %v", path, err)
	}
	return key
}

// readPublicFile reads a public key file keygen wrote.
func readPublicFile(t *testing.T, path string) crypto.PublicKey {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()
	key, err := signature.ReadPublicKey(file)
	if err != nil {
		t.Fatalf("ReadPublicKey(%s): %v", path, err)
	}
	return key
}

// fileMode returns the permission bits of a file.
func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
