// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole/testkit"
)

// runResult is what one in-process releasetool run answers.
type runResult struct {
	code   int
	stdout string
	stderr string
}

// runTool runs releasetool in process with the arguments.
func runTool(args ...string) runResult {
	var stdout, stderr strings.Builder
	code := run(args, &stdout, &stderr)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestRunNamesAnUnknownCommandAndExitsTwo(t *testing.T) {
	t.Parallel()

	got := runTool("publish")

	if got.code != 2 || !strings.Contains(got.stderr, `unknown command "publish"`) {
		t.Errorf("run(publish) = %d, %q, want 2 and the unknown command named", got.code, got.stderr)
	}
}

func TestRunPrintsTheUsageWithoutACommand(t *testing.T) {
	t.Parallel()

	got := runTool()

	if got.code != 2 || !strings.Contains(got.stderr, "releasetool keygen") {
		t.Errorf("run() = %d, %q, want 2 and the usage", got.code, got.stderr)
	}
}

func TestMainBinaryMakesAKeySignsAndVerifies(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "releasetool")
	dir := t.TempDir()
	key, public := filepath.Join(dir, "release.pem"), filepath.Join(dir, "release.pub.pem")
	archive := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")

	for _, args := range [][]string{
		{"keygen", "-curve", "ed25519", "-out", key, "-public", public},
		{"sign", "-key", key, archive},
		{"verify", "-public", public, archive},
	} {
		if code, stderr := runBinary(t, binary, env, args...); code != 0 {
			t.Fatalf("releasetool %s = %d, %q, want 0", args[0], code, stderr)
		}
	}
}

func TestMainBinaryNamesTheSaltFIPSOnlyModeRefuses(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "releasetool")
	dir := t.TempDir()
	passphrase := writeFile(t, dir, "passphrase", opensslPassphrase+"\n")
	archive := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")
	env = append(env, "GODEBUG=fips140=only")

	code, stderr := runBinary(t, binary, env, "sign", "-key", "testdata/openssl-p256.pem", "-passphrase-file",
		passphrase, archive)

	if code != 1 || !strings.Contains(stderr, "salt") {
		t.Errorf("releasetool sign under FIPS 140-only mode = %d, %q, want 1 and the short salt named", code, stderr)
	}
}

func TestMainBinaryLeavesNoFileBehindWhenAWriteFails(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "releasetool")
	dir := t.TempDir()
	_, keyFile := newReleaseKey(t, dir)
	archive := writeFile(t, dir, "gophenberg_0.20.0_linux_amd64.tar.gz", "an amd64 archive\n")
	key, public := filepath.Join(dir, "release2.pem"), filepath.Join(dir, "release2.pub.pem")

	for _, args := range [][]string{
		{"sign", "-key", keyFile, archive},
		{"keygen", "-curve", "ed25519", "-out", key, "-public", public},
	} {
		noRoom := append([]string{"-c", `ulimit -f 0 && exec "$0" "$@"`, binary}, args...)
		if code, stderr := runBinary(t, "sh", env, noRoom...); code != 1 {
			t.Errorf("releasetool %s with no room to write = %d, %q, want 1", args[0], code, stderr)
		}
	}

	for _, left := range []string{archive + ".sig", key, public} {
		if _, err := os.Stat(left); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("a failed write left %s behind, which a retry refuses to replace: %v", left, err)
		}
	}
	if code, stderr := runBinary(t, binary, env, "sign", "-key", keyFile, archive); code != 0 {
		t.Errorf("sign after a failed write = %d, %q, want 0", code, stderr)
	}
}

// runBinary runs the releasetool binary with the arguments and returns its exit code and standard error.
func runBinary(t *testing.T, binary string, env []string, args ...string) (int, string) {
	t.Helper()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), binary, args...)
	cmd.Env = env
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("running releasetool: %v", err)
	}
	return cmd.ProcessState.ExitCode(), stderr.String()
}
