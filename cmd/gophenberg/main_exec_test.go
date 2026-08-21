// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
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
	cmd := exec.Command(
		binary, "createadmin", "-email", "admin@example.com", "-name", "Admin", "-rank", "admin",
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

func TestMainBinarySeedsTheDemoData(t *testing.T) {
	t.Parallel()

	binary, env := coverBinary(t)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binary, "seed")
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

	binary, env := coverBinary(t)
	var stderr bytes.Buffer
	cmd := exec.Command(binary, "seed")
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

	binary, env := coverBinary(t)
	url := emptyDatabaseURL(t)
	port := freePort(t)
	cmd := exec.Command(binary)
	cmd.Dir = t.TempDir()
	cmd.Env = append(env,
		"GOPHENBERG_DATABASE_URL="+url,
		"GOPHENBERG_ADDR=localhost:"+port,
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
	waitForListening(t, stderr)
	storeSiteLocale(t, url, "es-ES")

	answered := readLocale(t, "http://localhost:"+port+"/api/locale")

	if answered != "es-ES" {
		t.Errorf("the binary answers %q, want the stored site default, so run.go wires the settings store", answered)
	}
}

// freePort returns a port nothing is listening on.
func freePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}
	defer func() { _ = listener.Close() }()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("reading the free port: %v", err)
	}
	return port
}

// waitForListening blocks until the binary reports it is serving.
func waitForListening(t *testing.T, stderr io.Reader) {
	t.Helper()
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "listening") {
			go func() { _, _ = io.Copy(io.Discard, stderr) }()
			return
		}
	}
	t.Fatal("binary never reported listening")
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
