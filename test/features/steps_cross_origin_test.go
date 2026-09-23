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

// pageSites names every site a scenario's browser page can stand on.
const pageSites = "this site|another site|a sibling site|a domain pointed at this server"

// browserOrigin is what a browser tells the server about the page a request comes from, and where it sends it.
type browserOrigin struct {
	origin    string
	fetchSite string
	host      string
}

// originOf returns the headers a browser sends for a page on the named site.
func (w *world) originOf(site string) (browserOrigin, error) {
	switch site {
	case "this site":
		return browserOrigin{origin: w.ownOrigin(), fetchSite: "same-origin"}, nil
	case "another site":
		return browserOrigin{origin: "https://elsewhere.example", fetchSite: "cross-site"}, nil
	case "a sibling site":
		return browserOrigin{origin: "https://www.example.com", fetchSite: "same-site"}, nil
	case "a domain pointed at this server":
		return browserOrigin{origin: "https://rebound.example", fetchSite: "same-origin", host: "rebound.example"}, nil
	}
	return browserOrigin{}, fmt.Errorf("no browser stands on %q", site)
}

// ownOrigin returns the address the site's own pages stand at, the public one when the site names it.
func (w *world) ownOrigin() string {
	if w.publicURL != "" {
		return w.publicURL
	}
	return w.site.URL
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
	if from.host != "" {
		request.Host = from.host
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

// aRunningGophenbergAtThePublicAddress starts a site naming its public address and serving one plugin form.
func aRunningGophenbergAtThePublicAddress(ctx context.Context, address, id, path string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	w.publicURL = address
	return aRunningGophenbergServingThePlugin(ctx, id, path)
}

// anOlderBrowserPostsAFormFrom posts a form carrying only the origin of a page on the site named.
func anOlderBrowserPostsAFormFrom(ctx context.Context, path, site string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	from, err := w.originOf(site)
	if err != nil {
		return err
	}
	from.fetchSite = ""
	return w.sentAs(w.browserOf(), http.MethodPost, path, "application/x-www-form-urlencoded", signupForm(), from)
}

// anotherServerPostsAFormAt posts a form with no browser headers to another address of the server.
func anotherServerPostsAFormAt(ctx context.Context, path, address string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.sentAs(w.browserOf(), http.MethodPost, path, "application/x-www-form-urlencoded", signupForm(),
		browserOrigin{host: address})
}

// theServerLoggedAWriteRefused asserts the server logged a refused write carrying the reason.
func theServerLoggedAWriteRefused(ctx context.Context, reason string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(w.logs.String(), "\n") {
		if strings.Contains(line, `msg="write refused"`) && strings.Contains(line, "reason="+reason) {
			return nil
		}
	}
	return fmt.Errorf("the server logged %q, want a write refused for the reason %q", w.logs.String(), reason)
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

// anotherServerPostsAForm posts a form with no browser headers at all.
func anotherServerPostsAForm(ctx context.Context, path string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.sentAs(
		w.browserOf(), http.MethodPost, path, "application/x-www-form-urlencoded", signupForm(), browserOrigin{})
}

// aPageReads reads a path from a page on the named site through a visitor's browser.
func aPageReads(ctx context.Context, site, path string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	from, err := w.originOf(site)
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
	sc.Given(
		`^a running Gophenberg at the public address "([^"]*)" serving the plugin "([^"]*)" with the public form "([^"]*)"$`,
		aRunningGophenbergAtThePublicAddress,
	)
	sc.Given(`^the administrator account exists$`, theAdministratorAccountExists)
	sc.When(`^a page on (`+pageSites+`) posts a form to "([^"]*)" through the visitor's browser$`, aPagePostsAForm)
	sc.When(
		`^an older browser posts a form to "([^"]*)" from a page on (this site|another site)$`,
		anOlderBrowserPostsAFormFrom,
	)
	sc.When(`^another server posts a form to "([^"]*)"$`, anotherServerPostsAForm)
	sc.When(`^another server posts a form to "([^"]*)" at the address "([^"]*)"$`, anotherServerPostsAFormAt)
	sc.When(
		`^a page on (another site|a domain pointed at this server) reads "([^"]*)" through the visitor's browser$`,
		aPageReads,
	)
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
	sc.Then(`^the server logged a write refused for the reason "([^"]*)"$`, theServerLoggedAWriteRefused)
}
