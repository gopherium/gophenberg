// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gopherium/gophenberg/internal/server"
)

// wantGenerator and wantLink are the identification a public response carries byte for byte.
const (
	wantGenerator = "Gophenberg 1.2"
	wantLink      = `</api/content/v1>; rel="https://gophenberg.org/api"`
)

func TestPublicResponsesIdentifyWhatServesThem(t *testing.T) {
	t.Parallel()

	handler := publicServer(t)

	for _, path := range []string{"/", "/post/hello-world", "/post/page/2", "/nothing/here"} {
		recorder := doRequest(t, handler, http.MethodGet, path, "")
		if got := recorder.Header().Get("X-Generator"); got != wantGenerator {
			t.Errorf("GET %s X-Generator = %q, want %q", path, got, wantGenerator)
		}
		if got := recorder.Header().Get("Link"); got != wantLink {
			t.Errorf("GET %s Link = %q, want %q", path, got, wantLink)
		}
	}
}

func TestThePageAndTheHeaderAgreeOnWhatServedIt(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, publicServer(t), http.MethodGet, "/", "")

	header := recorder.Header().Get("X-Generator")
	if header == "" {
		t.Fatal("X-Generator = empty, want the identification")
	}
	if !strings.Contains(recorder.Body.String(), `<meta name="generator" content="`+header+`">`) {
		t.Errorf("body = %q, want the meta naming the same %q the header names", recorder.Body.String(), header)
	}
}

func TestTheAPIAndTheAdminStayUnbranded(t *testing.T) {
	t.Parallel()

	handler := publicServer(t)

	for _, path := range []string{
		"/api/content/v1",
		"/api/content/v1/posts",
		"/api/content/v1/posts/post/hello-world",
		"/api/version",
		"/api/unknown",
		"/api",
		"/admin/",
		"/admin/posts",
		"/admin/assets/app.js",
	} {
		recorder := doRequest(t, handler, http.MethodGet, path, "")
		if got := recorder.Header().Get("X-Generator"); got != "" {
			t.Errorf("GET %s X-Generator = %q, want the header kept off this scope", path, got)
		}
		if got := recorder.Header().Get("Link"); got != "" {
			t.Errorf("GET %s Link = %q, want the header kept off this scope", path, got)
		}
	}
}

func TestIdentificationReportsTheBuildVersionAtMajorMinor(t *testing.T) {
	t.Parallel()

	handler := server.NewServer(server.Config{
		Users:   newFakeUserStore(),
		Posts:   newFakePostStore(),
		Version: "0.0.0",
		Web:     fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}},
	})

	recorder := doRequest(t, handler, http.MethodGet, "/", "")

	if got := recorder.Header().Get("X-Generator"); got != "Gophenberg 0.0" {
		t.Errorf("X-Generator = %q, want %q", got, "Gophenberg 0.0")
	}
}

func TestIdentificationSurvivesWithoutAStore(t *testing.T) {
	t.Parallel()

	handler := server.NewServer(server.Config{Users: newFakeUserStore(), Version: "1.2.3"})

	recorder := doRequest(t, handler, http.MethodGet, "/", "")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("X-Generator"); got != wantGenerator {
		t.Errorf("X-Generator = %q, want %q even with nothing to serve", got, wantGenerator)
	}
}

func TestIdentificationLeavesRoomForHeadersAPageAddsItself(t *testing.T) {
	t.Parallel()

	posts := newFakePostStore()
	posts.add(publishedFixture(t, "hello-world", blockMarkup, time.Now().UTC()))
	handler := server.NewServer(server.Config{Users: newFakeUserStore(), Posts: posts, Version: "1.2.3"})

	recorder := doRequest(t, handler, http.MethodGet, "/", "")

	if got := recorder.Header().Values("Link"); len(got) != 1 || got[0] != wantLink {
		t.Errorf("Link values = %v, want exactly the one this server adds", got)
	}
}
