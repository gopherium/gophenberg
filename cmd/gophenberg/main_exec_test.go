// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestMainBinaryFailsWithoutDatabaseURL(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	var stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), binary)
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

	for testName, args := range map[string][]string{"no command": nil, "the serve command": {"serve"}} {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
			cmd := exec.CommandContext(t.Context(), binary, args...)
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

			testkit.WaitForListening(t, stderr)
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatalf("signalling: %v", err)
			}

			if err := cmd.Wait(); err != nil {
				t.Fatalf("binary exit: %v, want a clean shutdown", err)
			}
		})
	}
}

func TestMainBinaryPrintsOnlyTheSeedAnswer(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args   []string
		code   int
		stdout string
		stderr string
	}{
		"help":            {[]string{"seed", "-h"}, 0, seedUsage, ""},
		"an unknown flag": {[]string{"seed", "-now"}, 2, "", "gophenberg: seed: flag provided but not defined: -now\n"},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			var stdout, stderr bytes.Buffer
			cmd := exec.CommandContext(ctx, binary, tc.args...)
			cmd.Dir = t.TempDir()
			cmd.Env = env
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			var exitErr *exec.ExitError
			if err != nil && !errors.As(err, &exitErr) {
				t.Fatalf("gophenberg %q: %v", tc.args, err)
			}
			if cmd.ProcessState.ExitCode() != tc.code || stdout.String() != tc.stdout || stderr.String() != tc.stderr {
				t.Errorf("gophenberg %q = %d, stdout %q, stderr %q, want %d, %q and %q", tc.args,
					cmd.ProcessState.ExitCode(), stdout.String(), stderr.String(), tc.code, tc.stdout, tc.stderr)
			}
		})
	}
}

func TestMainBinaryRefusesAnUnknownCommandBeforeServing(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binary, "hepl")
	cmd.Dir = t.TempDir()
	cmd.Env = append(env,
		"GOPHENBERG_DATABASE_URL="+emptyDatabaseURL(t),
		"GOPHENBERG_ADDR=localhost:0",
	)
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("gophenberg hepl: %v with stderr %q, want exit code 2 before any serving", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), `unknown command "hepl"`) {
		t.Errorf("stderr = %q, want it to name the unknown command", stderr.String())
	}
}

func TestMainBinaryCreateAdminProvisionsAUser(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(),
		binary, "createadmin", "-email", "admin@example.com", "-name", "Admin", "-role", "admin",
	)
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

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	var stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), binary, "createadmin", "-email", "admin@example.com", "-name", "Admin")
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

func TestMainBinarySeedsTheDemoData(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), binary, "seed")
	cmd.Dir = t.TempDir()
	cmd.Env = append(env, "GOPHENBERG_DATABASE_URL="+emptyDatabaseURL(t))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("seed: %v, stderr: %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "seeded demo data") {
		t.Errorf("stdout = %q, want the seeding confirmation", stdout.String())
	}
}

func TestMainBinarySeedFailsWithoutDatabaseURL(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	var stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), binary, "seed")
	cmd.Dir = t.TempDir()
	cmd.Env = env
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("seed without a database url: %v, want exit code 1", err)
	}
	if !strings.Contains(stderr.String(), "GOPHENBERG_DATABASE_URL") {
		t.Errorf("stderr = %q, want it to name the missing variable", stderr.String())
	}
}

func TestMainBinaryAnswersTheSiteDefaultLocale(t *testing.T) {
	t.Parallel()

	binary, env := testkit.CoverBinary(t, "GOPHENBERG_", "gophenberg")
	url := emptyDatabaseURL(t)
	addr := testkit.FreeAddr(t)
	cmd := exec.CommandContext(t.Context(), binary)
	cmd.Dir = t.TempDir()
	cmd.Env = append(env,
		"GOPHENBERG_DATABASE_URL="+url,
		"GOPHENBERG_ADDR="+addr,
	)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting binary: %v", err)
	}
	defer func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
	}()
	testkit.WaitForListening(t, stderr)
	storeSiteLocale(t, url, "es-ES")

	answered := readLocale(t, "http://"+addr+"/api/locale")

	if answered != "es-ES" {
		t.Errorf("the binary answers %q, want the stored site default, so run.go wires the settings store", answered)
	}
}

// storeSiteLocale writes the site default language straight into the database.
func storeSiteLocale(t *testing.T, url, locale string) {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	defer pool.Close()
	_, err = pool.Exec(t.Context(),
		`INSERT INTO core.settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		content.LocaleSettingKey, locale)
	if err != nil {
		t.Fatalf("storing the site locale: %v", err)
	}
}

// readLocale asks the running binary which language it answers in.
func readLocale(t *testing.T, url string) string {
	t.Helper()
	var answered struct {
		Locale string `json:"locale"`
	}
	client := &http.Client{Timeout: 2 * time.Second}
	for attempt := range 50 {
		response, err := client.Get(url)
		if err != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		defer func() { _ = response.Body.Close() }()
		if err := json.NewDecoder(response.Body).Decode(&answered); err != nil {
			t.Fatalf("reading the locale on attempt %d: %v", attempt, err)
		}
		return answered.Locale
	}
	t.Fatal("the binary never answered the locale")
	return ""
}
