// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestMainBinaryGrantRankReachesEveryAccountHoldingNone(t *testing.T) {
	t.Parallel()

	binary, env := coverBinary(t)
	databaseURL := emptyDatabaseURL(t)
	provision := exec.Command(
		binary, "createadmin", "-email", "admin@example.com", "-name", "Admin", "-rank", "admin",
	)
	provision.Dir = t.TempDir()
	provision.Env = append(env, "GOPHENBERG_DATABASE_URL="+databaseURL)
	provision.Stdin = strings.NewReader("correct horse battery\n")
	if err := provision.Run(); err != nil {
		t.Fatalf("createadmin: %v", err)
	}
	execSQL(t, databaseURL, "UPDATE auth.users SET rank = ''")
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binary, "grantrank", "-rank", "admin")
	cmd.Dir = t.TempDir()
	cmd.Env = append(env, "GOPHENBERG_DATABASE_URL="+databaseURL)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("grantrank: %v, stderr: %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "to 1 accounts") {
		t.Errorf("stdout = %q, want it to report the account that took the rank", stdout.String())
	}
}

func TestMainBinaryGrantRankFailsWithoutDatabaseURL(t *testing.T) {
	t.Parallel()

	binary, env := coverBinary(t)
	var stderr bytes.Buffer
	cmd := exec.Command(binary, "grantrank", "-rank", "admin")
	cmd.Dir = t.TempDir()
	cmd.Env = env
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("grantrank without a database url: %v, want exit code 1", err)
	}
	if !strings.Contains(stderr.String(), "GOPHENBERG_DATABASE_URL") {
		t.Errorf("stderr = %q, want it to name the missing variable", stderr.String())
	}
}
