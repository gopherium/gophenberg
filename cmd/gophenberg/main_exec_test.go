// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func coverBinary(t *testing.T) (string, []string) {
	t.Helper()
	bindir := os.Getenv("GOPHENBERG_COVER_BINDIR")
	gocoverdir := os.Getenv("GOPHENBERG_COVER_GOCOVERDIR")
	if bindir == "" || gocoverdir == "" {
		t.Skip("skipping binary test: run via make cover")
	}
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOPHENBERG_") && !strings.HasPrefix(entry, "GOCOVERDIR=") {
			env = append(env, entry)
		}
	}
	return filepath.Join(bindir, "gophenberg"), append(env, "GOCOVERDIR="+gocoverdir)
}

func TestMainBinaryFailsWithoutDatabaseURL(t *testing.T) {
	t.Parallel()

	binary, env := coverBinary(t)
	var stderr bytes.Buffer
	cmd := exec.Command(binary)
	cmd.Dir = t.TempDir()
	cmd.Env = env
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("gophenberg without a database url: %v, want exit code 1", err)
	}
	if !strings.Contains(stderr.String(), "GOPHENBERG_DATABASE_URL") {
		t.Errorf("stderr = %q, want it to name the missing variable", stderr.String())
	}
}

func TestMainBinaryServesUntilTerminated(t *testing.T) {
	t.Parallel()

	binary, env := coverBinary(t)
	cmd := exec.Command(binary)
	cmd.Dir = t.TempDir()
	cmd.Env = append(env,
		"GOPHENBERG_DATABASE_URL="+emptyDatabaseURL(t),
		"GOPHENBERG_ADDR=localhost:0",
	)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting binary: %v", err)
	}

	scanner := bufio.NewScanner(stderr)
	listening := false
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "listening") {
			listening = true
			break
		}
	}
	if !listening {
		_ = cmd.Process.Kill()
		t.Fatal("binary never reported listening")
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signalling: %v", err)
	}
	go func() { _, _ = io.Copy(io.Discard, stderr) }()

	if err := cmd.Wait(); err != nil {
		t.Fatalf("binary exit: %v, want a clean shutdown", err)
	}
}

func TestMainBinaryCreateAdminProvisionsAUser(t *testing.T) {
	t.Parallel()

	binary, env := coverBinary(t)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binary, "createadmin", "-email", "admin@example.com", "-name", "Admin")
	cmd.Dir = t.TempDir()
	cmd.Env = append(env, "GOPHENBERG_DATABASE_URL="+emptyDatabaseURL(t))
	cmd.Stdin = strings.NewReader("correct horse battery\n")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("createadmin: %v, stderr: %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "created user admin@example.com") {
		t.Errorf("stdout = %q, want the created-user confirmation", stdout.String())
	}
}

func TestMainBinaryCreateAdminFailsWithoutDatabaseURL(t *testing.T) {
	t.Parallel()

	binary, env := coverBinary(t)
	var stderr bytes.Buffer
	cmd := exec.Command(binary, "createadmin", "-email", "admin@example.com", "-name", "Admin")
	cmd.Dir = t.TempDir()
	cmd.Env = env
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("createadmin without a database url: %v, want exit code 1", err)
	}
	if !strings.Contains(stderr.String(), "GOPHENBERG_DATABASE_URL") {
		t.Errorf("stderr = %q, want it to name the missing variable", stderr.String())
	}
}
