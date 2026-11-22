// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/server"
)

// mariaInvite is the invitation body of a new member.
const mariaInvite = `{"email":"maria@example.com","name":"Maria Perez","role":"member"}`

// invitation is what an invite or a resend answers.
type invitation struct {
	Delivered      bool   `json:"delivered"`
	ActivationLink string `json:"activation_link"`
}

// sentInvite is one invitation a recordingInvites took.
type sentInvite struct {
	to, name, link string
}

// recordingInvites keeps every invitation it is handed.
type recordingInvites struct {
	sent []sentInvite
}

// SendInvite keeps the invitation.
func (r *recordingInvites) SendInvite(_ context.Context, to, name, link string) error {
	r.sent = append(r.sent, sentInvite{to: to, name: name, link: link})
	return nil
}

// publicSite returns the public address the test requests reach.
func publicSite(t *testing.T) *url.URL {
	t.Helper()
	public, err := server.ParsePublicURL("https://example.com")
	if err != nil {
		t.Fatalf("ParsePublicURL() = %v, want the address parsed", err)
	}
	return public
}

// invitingServer returns a server over cfg with Ada stored, and her session cookie.
func invitingServer(t *testing.T, cfg server.Config) (http.Handler, *http.Cookie) {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	cfg.Users = users
	handler := server.NewServer(cfg)
	return handler, loginCookie(t, handler)
}

// invited sends body to path with the cookie and returns the invitation answered.
func invited(t *testing.T, handler http.Handler, cookie *http.Cookie, path, body string) invitation {
	t.Helper()
	recorder := ask(handler, cookie, call{http.MethodPost, path, body})
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST %s status = %d, want %d: %s", path, recorder.Code, http.StatusOK, recorder.Body)
	}
	return decodeBody[invitation](t, recorder)
}

// tokenOf returns the token an activation link carries.
func tokenOf(t *testing.T, link string) string {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parsing the activation link %q: %v", link, err)
	}
	return parsed.Query().Get("token")
}

// activateFrom posts token with a password from the peer at remoteAddr, which forwards for forwarded when set.
func activateFrom(handler http.Handler, remoteAddr, forwarded, token string) *httptest.ResponseRecorder {
	body := `{"token":"` + token + `","password":"a password Maria chose"}`
	request := httptest.NewRequest(http.MethodPost, "/api/auth/activate", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = remoteAddr
	if forwarded != "" {
		request.Header.Set("X-Forwarded-For", forwarded)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// spendDeadLinks posts count dead links from the peer at remoteAddr, each forwarded for forwarded(i).
func spendDeadLinks(t *testing.T, handler http.Handler, remoteAddr string, count int, forwarded func(int) string) {
	t.Helper()
	for i := range count {
		recorder := activateFrom(handler, remoteAddr, forwarded(i), "a-dead-link")
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("dead link %d status = %d, want %d", i+1, recorder.Code, http.StatusUnprocessableEntity)
		}
	}
}

func TestAnInvitationWithoutMailAnswersALinkUnderThePublicAddress(t *testing.T) {
	t.Parallel()

	handler, cookie := invitingServer(t, server.Config{PublicURL: publicSite(t)})

	got := invited(t, handler, cookie, "/api/users/invite", mariaInvite)

	if got.Delivered || !strings.HasPrefix(got.ActivationLink, "https://example.com/admin/activate?token=") {
		t.Errorf("invite answered %+v, want an undelivered link under https://example.com/admin/activate", got)
	}
}

func TestAnInvitationWithoutAPublicAddressAnswersALinkOnTheSite(t *testing.T) {
	t.Parallel()

	handler, cookie := invitingServer(t, server.Config{})

	got := invited(t, handler, cookie, "/api/users/invite", mariaInvite)

	if !strings.HasPrefix(got.ActivationLink, "/admin/activate?token=") {
		t.Errorf("activation link = %q, want one under /admin/activate", got.ActivationLink)
	}
}

func TestAnInvitationHandsTheSenderTheAddressTheNameAndTheLink(t *testing.T) {
	t.Parallel()

	sender := &recordingInvites{}
	handler, cookie := invitingServer(t, server.Config{PublicURL: publicSite(t), InviteSender: sender})

	got := invited(t, handler, cookie, "/api/users/invite", mariaInvite)

	if !got.Delivered || got.ActivationLink != "" {
		t.Errorf("invite answered %+v, want it delivered with no link", got)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("sender took %d invitations, want 1", len(sender.sent))
	}
	sent := sender.sent[0]
	if sent.to != "maria@example.com" || sent.name != "Maria Perez" ||
		!strings.HasPrefix(sent.link, "https://example.com/admin/activate?token=") {
		t.Errorf("sender took %+v, want Maria Perez at maria@example.com with a link under the public address", sent)
	}
}

func TestTheActivationLinkSignsTheInvitedAccountInWithoutASession(t *testing.T) {
	t.Parallel()

	handler, cookie := invitingServer(t, server.Config{})
	link := invited(t, handler, cookie, "/api/users/invite", mariaInvite).ActivationLink

	recorder := activateFrom(handler, "192.0.2.1:1234", "", tokenOf(t, link))

	if recorder.Code != http.StatusOK {
		t.Fatalf("activate status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	sessionCookie(t, recorder)
	if body := recorder.Body.String(); !strings.Contains(body, "maria@example.com") {
		t.Errorf("activate answered %s, want the identity of maria@example.com", body)
	}
}

func TestTheLifetimeTheServerIsGivenBoundsTheActivationLink(t *testing.T) {
	t.Parallel()

	handler, cookie := invitingServer(t, server.Config{InviteTTL: time.Millisecond})
	link := invited(t, handler, cookie, "/api/users/invite", mariaInvite).ActivationLink
	time.Sleep(10 * time.Millisecond)

	recorder := activateFrom(handler, "192.0.2.1:1234", "", tokenOf(t, link))

	if got := refusalOf(t, recorder); recorder.Code != http.StatusUnprocessableEntity || got.Code != "token_invalid" {
		t.Errorf("activate after the lifetime answered %d %+v, want 422 token_invalid", recorder.Code, got)
	}
}

func TestAResendAnswersAFreshLinkAndTheFirstNoLongerActivates(t *testing.T) {
	t.Parallel()

	handler, cookie := invitingServer(t, server.Config{})
	first := invited(t, handler, cookie, "/api/users/invite", mariaInvite).ActivationLink

	fresh := invited(t, handler, cookie, "/api/users/invite/resend", `{"email":"maria@example.com"}`).ActivationLink

	if fresh == "" || fresh == first {
		t.Fatalf("resend answered the link %q after %q, want a fresh one", fresh, first)
	}
	recorder := activateFrom(handler, "192.0.2.1:1234", "", tokenOf(t, first))
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("the first link answered %d after the resend, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
}

func TestDeadLinksCountPerClientATrustedProxyForwards(t *testing.T) {
	t.Parallel()

	handler := server.NewServer(server.Config{Users: newFakeUserStore(), TrustedProxies: []string{"10.0.0.0/8"}})
	spendDeadLinks(t, handler, "10.0.0.2:1234", 10, func(int) string { return "198.51.100.7" })

	slowed := activateFrom(handler, "10.0.0.2:1234", "198.51.100.7", "a-dead-link")
	other := activateFrom(handler, "10.0.0.2:1234", "198.51.100.8", "a-dead-link")

	if got := refusalOf(t, slowed); slowed.Code != http.StatusTooManyRequests || got.Code != "link_rate_limited" {
		t.Errorf("the eleventh dead link answered %d %+v, want 429 link_rate_limited", slowed.Code, got)
	}
	if other.Code != http.StatusUnprocessableEntity {
		t.Errorf("another forwarded client answered %d, want %d", other.Code, http.StatusUnprocessableEntity)
	}
}

func TestDeadLinksFromAnUntrustedPeerCountAgainstThePeer(t *testing.T) {
	t.Parallel()

	handler := server.NewServer(server.Config{Users: newFakeUserStore(), TrustedProxies: []string{"10.0.0.0/8"}})
	spendDeadLinks(t, handler, "203.0.113.5:1234", 10, func(i int) string { return "198.51.100." + strconv.Itoa(i) })

	recorder := activateFrom(handler, "203.0.113.5:1234", "198.51.100.99", "a-dead-link")

	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("the eleventh dead link from the peer answered %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
}
