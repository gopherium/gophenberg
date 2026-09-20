// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/cucumber/godog"
)

// formPlugin stands in for a plugin with a public form, counting the forms posted to it.
type formPlugin struct {
	posts atomic.Int64
}

// ServeHTTP counts a posted form and answers it.
func (p *formPlugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		p.posts.Add(1)
	}
	w.WriteHeader(http.StatusOK)
}

// browserOrigin is what a browser tells the server about the page a request comes from.
type browserOrigin struct {
	origin    string
	fetchSite string
}

// originOf returns the headers a browser sends for a page on the named site.
func (w *world) originOf(site string) (browserOrigin, error) {
	switch site {
	case "this site":
		return browserOrigin{origin: w.site.URL, fetchSite: "same-origin"}, nil
	case "another site":
		return browserOrigin{origin: "https://elsewhere.example", fetchSite: "cross-site"}, nil
	case "a sibling site":
		return browserOrigin{origin: "https://www.example.com", fetchSite: "same-site"}, nil
	}
	return browserOrigin{}, fmt.Errorf("no browser stands on %q", site)
}

// browserOf returns a client holding no session, as a visitor's browser would.
func (w *world) browserOf() *http.Client {
	anonymous := *w.site.Client()
	anonymous.Jar = nil
	return &anonymous
}

// sentAs sends a request the way a browser on a page would, recording what came back.
func (w *world) sentAs(
	client *http.Client, method, path, contentType string, body io.Reader, from browserOrigin,
) error {
	if err := w.running(); err != nil {
		return err
	}
	request, err := http.NewRequest(method, w.site.URL+path, body)
	if err != nil {
		return fmt.Errorf("building the %s %s request: %w", method, path, err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if from.origin != "" {
		request.Header.Set("Origin", from.origin)
	}
	if from.fetchSite != "" {
		request.Header.Set("Sec-Fetch-Site", from.fetchSite)
	}
	got, err := w.answerFrom(client, request)
	if err != nil {
		return err
	}
	w.answer = got
	return nil
}

// signupForm returns the body a subscription form posts.
func signupForm() io.Reader {
	return strings.NewReader(url.Values{"email": {"maria@example.com"}}.Encode())
}

// aRunningGophenbergServingThePlugin starts a server mounting a plugin with one public form.
func aRunningGophenbergServingThePlugin(ctx context.Context, id, path string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	w.forms = map[string]*formPlugin{id: {}}
	w.plugins = map[string]http.Handler{id: w.forms[id]}
	w.publicPaths = map[string][]string{id: {path}}
	return w.start(ctx)
}

// aPagePostsAForm posts a form from a page on the named site through a visitor's browser.
func aPagePostsAForm(ctx context.Context, site, path string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	from, err := w.originOf(site)
	if err != nil {
		return err
	}
	return w.sentAs(w.browserOf(), http.MethodPost, path, "application/x-www-form-urlencoded", signupForm(), from)
}

// anOlderBrowserPostsAForm posts a form from a page on another site, naming only its origin.
func anOlderBrowserPostsAForm(ctx context.Context, path string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	from := browserOrigin{origin: "https://elsewhere.example"}
	return w.sentAs(w.browserOf(), http.MethodPost, path, "application/x-www-form-urlencoded", signupForm(), from)
}

// anotherServerPostsAForm posts a form with no browser headers at all.
func anotherServerPostsAForm(ctx context.Context, path string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.sentAs(
		w.browserOf(), http.MethodPost, path, "application/x-www-form-urlencoded", signupForm(), browserOrigin{})
}

// aPageOnAnotherSiteReads reads a path from a page on another site through a visitor's browser.
func aPageOnAnotherSiteReads(ctx context.Context, path string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	from, err := w.originOf("another site")
	if err != nil {
		return err
	}
	return w.sentAs(w.browserOf(), http.MethodGet, path, "", nil, from)
}

// aPageSetsThePageSize patches the page size from a page on the named site through the administrator's browser.
func aPageSetsThePageSize(ctx context.Context, site string, size int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	from, err := w.originOf(site)
	if err != nil {
		return err
	}
	body := strings.NewReader(fmt.Sprintf(`{"content_per_page":%d}`, size))
	return w.sentAs(w.client, http.MethodPatch, settingsPath, "application/json", body, from)
}

// aPageOnAnotherSiteSignsIn posts the administrator's credentials from a page on another site.
func aPageOnAnotherSiteSignsIn(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	from, err := w.originOf("another site")
	if err != nil {
		return err
	}
	body := strings.NewReader(fmt.Sprintf(`{"email":%q,"password":%q}`, adminEmail, adminPassword))
	return w.sentAs(w.browserOf(), http.MethodPost, "/api/auth/login", "text/plain", body, from)
}

// aPageOnAnotherSiteSignsOut posts a sign out from a page on another site through the administrator's browser.
func aPageOnAnotherSiteSignsOut(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	from, err := w.originOf("another site")
	if err != nil {
		return err
	}
	return w.sentAs(w.client, http.MethodPost, "/api/auth/logout", "", nil, from)
}

// theAdministratorIsStillSignedIn asserts the session still answers.
func theAdministratorIsStillSignedIn(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.get("/api/auth/session"); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// thePluginReceivedForms asserts how many forms reached the plugin.
func thePluginReceivedForms(ctx context.Context, id string, count int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.forms[id]
	if !found {
		return fmt.Errorf("no plugin %q is served", id)
	}
	if got := int(held.posts.Load()); got != count {
		return fmt.Errorf("the plugin %q received %d forms, want %d", id, got, count)
	}
	return nil
}

// initializeCrossOrigin binds the steps the cross-origin writes feature uses.
func initializeCrossOrigin(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.Given(
		`^a running Gophenberg serving the plugin "([^"]*)" with the public form "([^"]*)"$`,
		aRunningGophenbergServingThePlugin,
	)
	sc.Given(`^the administrator account exists$`, theAdministratorAccountExists)
	sc.When(
		`^a page on (this site|another site|a sibling site) posts a form to "([^"]*)" through the visitor's browser$`,
		aPagePostsAForm,
	)
	sc.When(`^an older browser posts a form to "([^"]*)" from a page on another site$`, anOlderBrowserPostsAForm)
	sc.When(`^another server posts a form to "([^"]*)"$`, anotherServerPostsAForm)
	sc.When(`^a page on another site reads "([^"]*)" through the visitor's browser$`, aPageOnAnotherSiteReads)
	sc.When(
		`^a page on (this site|another site|a sibling site) sets the page size to (\d+) through the administrator's browser$`,
		aPageSetsThePageSize,
	)
	sc.When(
		`^a page on another site signs in as the administrator through the visitor's browser$`,
		aPageOnAnotherSiteSignsIn,
	)
	sc.When(
		`^a page on another site signs the administrator out through the visitor's browser$`,
		aPageOnAnotherSiteSignsOut,
	)
	sc.When(`^a visitor lists the published content$`, aVisitorListsThePublishedContent)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the request is answered$`, theRequestIsAnswered)
	sc.Then(`^the listing offers pages of (\d+)$`, theListingOffersPagesOf)
	sc.Then(`^the administrator is still signed in$`, theAdministratorIsStillSignedIn)
	sc.Then(`^the plugin "([^"]*)" received (\d+) forms?$`, thePluginReceivedForms)
}
