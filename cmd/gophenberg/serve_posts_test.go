// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/seed"
)

func TestServedPostsAnswerAnAuthenticatedCaller(t *testing.T) {
	t.Parallel()

	address := freeAddress(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t),
		"GOPHENBERG_ADDR":         address,
		"GOPHENBERG_WEB_DIR":      t.TempDir(),
	}
	if err := seedDemoData(t.Context(), testGetenv(env), new(bytes.Buffer)); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, testGetenv(env), io.Discard, noPlugins) }()
	base := "http://" + address
	awaitServer(t, base)
	client := loggedInClient(t, base)

	for _, path := range []string{"/api/posts?per_page=20&orderby=date&order=desc", "/api/posts/counts"} {
		assertOK(t, client, base+path)
	}

	cancel()
	<-done
}

// freeAddress returns a loopback address nothing is listening on.
func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("releasing the reserved port: %v", err)
	}
	return address
}

// awaitServer blocks until the server at base answers, failing if it never does.
func awaitServer(t *testing.T, base string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(base + "/api/version")
		if err == nil {
			_ = response.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("server never started answering")
}

// loggedInClient returns an http.Client holding a session for the seeded admin.
func loggedInClient(t *testing.T, base string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("building a cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}
	body, err := json.Marshal(map[string]string{
		"email":    seed.AdminEmail,
		"password": seed.AdminPassword,
	})
	if err != nil {
		t.Fatalf("encoding credentials: %v", err)
	}
	response, err := client.Post(base+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("logging in: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	return client
}

// assertOK fails the test unless the URL answers 200 to the client.
func assertOK(t *testing.T, client *http.Client, url string) {
	t.Helper()
	response, err := client.Get(url)
	if err != nil {
		t.Fatalf("getting %s: %v", url, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Errorf("%s status = %d, want %d", url, response.StatusCode, http.StatusOK)
	}
}
