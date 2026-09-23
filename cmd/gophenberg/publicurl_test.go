// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// lockedLog keeps what the server logs while the test reads it back.
type lockedLog struct {
	mu   sync.Mutex
	held bytes.Buffer
}

// Write keeps one log line.
func (l *lockedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.held.Write(p)
}

// String returns every line kept so far.
func (l *lockedLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.held.String()
}

func TestLoadRunConfigReadsThePublicAddress(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		"GOPHENBERG_PUBLIC_URL":   "https://cms.example/",
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	if settings.publicURL == nil || settings.publicURL.String() != "https://cms.example" {
		t.Errorf("publicURL = %v, want https://cms.example", settings.publicURL)
	}
}

func TestLoadRunConfigLeavesThePublicAddressUnsetByDefault(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	if settings.publicURL != nil {
		t.Errorf("publicURL = %v, want none", settings.publicURL)
	}
}

func TestLoadRunConfigRefusesAPublicAddressWithAPath(t *testing.T) {
	t.Parallel()

	_, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		"GOPHENBERG_PUBLIC_URL":   "https://cms.example/blog",
	}))

	if err == nil || !strings.Contains(err.Error(), "GOPHENBERG_PUBLIC_URL") {
		t.Errorf("loadRunConfig() error = %v, want the setting named", err)
	}
}

func TestRunRefusesAndLogsAWriteSentAwayFromThePublicAddress(t *testing.T) {
	t.Parallel()

	address := freeAddress(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t),
		"GOPHENBERG_ADDR":         address,
		"GOPHENBERG_WEB_DIR":      t.TempDir(),
		"GOPHENBERG_PUBLIC_URL":   "https://cms.example",
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	logs := &lockedLog{}
	done := make(chan error, 1)
	go func() { done <- run(ctx, testGetenv(env), logs, noPlugins) }()
	base := "http://" + address
	awaitServer(t, base)

	response, err := http.Post(base+"/api/auth/login", "application/json",
		strings.NewReader(`{"email":"admin@example.com","password":"password1234"}`))
	if err != nil {
		t.Fatalf("posting the sign in: %v", err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()

	if response.StatusCode != http.StatusForbidden || !strings.Contains(string(body), "request_cross_origin") {
		t.Errorf("sign in = %d %s, want it refused as a write sent away from the public address",
			response.StatusCode, body)
	}
	if line := logs.String(); !strings.Contains(line, `msg="write refused"`) || !strings.Contains(line, "reason=host") {
		t.Errorf("log = %q, want the refused write logged with the reason host", line)
	}
	cancel()
	<-done
}
