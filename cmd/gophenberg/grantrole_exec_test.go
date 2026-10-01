// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole/testkit"
)

func TestMainBinaryGrantRoleReachesEveryAccountHoldingNone(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	databaseURL := emptyDatabaseURL(t)
	env = append(env, "GOPHENBERG_DATABASE_URL="+databaseURL)
	for _, role := range []string{"admin", "author"} {
		provision := exec.CommandContext(t.Context(),
			binary, "account:create-admin", "-email", role+"@example.com", "-name", "Holder", "-role", role,
		)
		provision.Dir = t.TempDir()
		provision.Env = env
		provision.Stdin = strings.NewReader(typedPassword + "\n")
		if err := provision.Run(); err != nil {
			t.Fatalf("account:create-admin %s: %v", role, err)
		}
	}
	execSQL(t, databaseURL, "UPDATE auth.users SET role = '' WHERE email = 'author@example.com'")
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), binary,
		"account:grant-role", "-role", "author", "-yes", "-as", "admin@example.com")
	cmd.Dir = t.TempDir()
	cmd.Env = env
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("account:grant-role: %v, stderr: %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "granted author to 1 account") {
		t.Errorf("stdout = %q, want it to report the account that took the role", stdout.String())
	}
	held := recordsOf(t, testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}))
	if len(held) != 1 || !strings.Contains(held[0], "admin@example.com  account:grant-role") {
		t.Errorf("records = %q, want the one grant admin@example.com applied", held)
	}
}

func TestMainBinaryGrantRoleFailsWithoutDatabaseURL(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	var stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), binary,
		"account:grant-role", "-role", "admin", "-yes", "-as", "admin@example.com")
	cmd.Dir = t.TempDir()
	cmd.Env = env
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("account:grant-role without a database url: %v, want exit code 1", err)
	}
	if !strings.Contains(stderr.String(), "GOPHENBERG_DATABASE_URL") {
		t.Errorf("stderr = %q, want it to name the missing variable", stderr.String())
	}
}
