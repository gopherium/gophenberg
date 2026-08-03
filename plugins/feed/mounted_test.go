// SPDX-License-Identifier: Apache-2.0

package feed_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gopherium/pluginkit"

	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/sdk"
)

func TestFeedAnswersThroughTheServerWithoutASession(t *testing.T) {
	t.Parallel()

	posts := &stubPosts{posts: []sdk.Post{samplePost("A Published Post", "<p>Body</p>")}}
	plugin := mustRegister(t, posts, map[string]string{})
	host := pluginkit.NewHost(plugin)
	handler := server.NewServer(server.Config{
		Plugins:           host.Routes(),
		PluginPublicPaths: host.PublicPaths(),
	})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plugins/feed/rss.xml", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d from a caller holding no session", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "A Published Post") {
		t.Errorf("body = %q, want the published post", recorder.Body.String())
	}
}

// mountedFeedLink returns the channel link the mounted feed serves a peer claiming HTTPS.
func mountedFeedLink(t *testing.T, trusted []string, peer string) string {
	t.Helper()
	posts := &stubPosts{posts: []sdk.Post{samplePost("A Published Post", "<p>Body</p>")}}
	host := pluginkit.NewHost(mustRegister(t, posts, map[string]string{}))
	handler := server.NewServer(server.Config{
		Plugins:           host.Routes(),
		PluginPublicPaths: host.PublicPaths(),
		TrustedProxies:    trusted,
	})
	request := httptest.NewRequest(http.MethodGet, "/api/plugins/feed/rss.xml", nil)
	request.Host = "example.com"
	request.RemoteAddr = peer
	request.Header.Set("X-Forwarded-Proto", "https")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	return recorder.Body.String()
}

func TestFeedIgnoresAForwardedProtocolFromAnUntrustedPeer(t *testing.T) {
	t.Parallel()

	body := mountedFeedLink(t, nil, "203.0.113.5:9999")

	if strings.Contains(body, "https://example.com") {
		t.Errorf("body = %q, want the spoofed protocol ignored", body)
	}
	if !strings.Contains(body, "http://example.com") {
		t.Errorf("body = %q, want the dialed protocol kept", body)
	}
}

func TestFeedHonorsAForwardedProtocolFromATrustedProxy(t *testing.T) {
	t.Parallel()

	body := mountedFeedLink(t, []string{"10.0.0.0/8"}, "10.1.2.3:9999")

	if !strings.Contains(body, "https://example.com") {
		t.Errorf("body = %q, want the trusted proxy's protocol honored", body)
	}
}

func TestFeedStartsAndStopsWithItsHost(t *testing.T) {
	t.Parallel()

	host := pluginkit.NewHost(mustRegister(t, &stubPosts{}, map[string]string{}))

	if err := host.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}

	if err := host.Stop(t.Context()); err != nil {
		t.Fatalf("Stop() error = %v, want nil", err)
	}
}

func TestServerStillGuardsTheRestOfThePlugin(t *testing.T) {
	t.Parallel()

	plugin := mustRegister(t, &stubPosts{}, map[string]string{})
	host := pluginkit.NewHost(plugin)
	handler := server.NewServer(server.Config{
		Plugins:           host.Routes(),
		PluginPublicPaths: host.PublicPaths(),
	})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plugins/feed/private", nil))

	if recorder.Code == http.StatusOK {
		t.Error("an undeclared plugin path answered without a session, want it guarded")
	}
}
