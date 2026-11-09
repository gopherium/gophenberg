// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

// pluginRoutePanicsFeature is the feature a site answers to while a plugin's route panics.
const pluginRoutePanicsFeature = "../../test/features/features/plugin-route-panics.feature"

// routePanicLine is the message the site logs for each panic of a plugin's route.
const routePanicLine = "plugin route panicked"

// routeBoom is a route of a pausing plugin that panics, after it began its answer or behind a session when set.
type routeBoom struct {
	text    string
	after   bool
	private bool
}

// serve panics with the boom's text, first answering 200 with part of a body sent when the boom says so.
func (b routeBoom) serve(w http.ResponseWriter, _ *http.Request) {
	if b.after {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "partial")
		_ = http.NewResponseController(w).Flush()
	}
	panic(b.text)
}

// bindRoutePanics binds the steps of the scenarios where a plugin's route panics.
func (p *pausingScenario) bindRoutePanics(sc *godog.ScenarioContext) {
	sc.Given(`^the route "([^"]*)" of "([^"]*)" (answers 200, then )?panics with "([^"]*)"$`, p.panicsOnRoute)
	sc.Given(`^the route "([^"]*)" of "([^"]*)" needs a session and panics with "([^"]*)"$`, p.panicsBehindSession)
	sc.Then(`^the route "([^"]*)" of "([^"]*)" answers (\d+) with the code "([^"]*)", kept out of every cache$`,
		p.routeAnswersCode)
	sc.Then(`^the answer of the route "([^"]*)" of "([^"]*)" is cut$`, p.routeCut)
	sc.Then(`^the site logged once that (\S+) "([^"]*)" of "([^"]*)" panicked with "([^"]*)", (answered|not answered)$`,
		p.loggedRoutePanic)
}

// panicsOnRoute gives the plugin id a public route at path that panics with text, after it answered 200 when set.
func (p *pausingScenario) panicsOnRoute(path, id, after, text string) {
	p.plugins[id].routeBooms[path] = routeBoom{text: text, after: after != ""}
}

// panicsBehindSession gives the plugin id a route at path behind a session that panics with text.
func (p *pausingScenario) panicsBehindSession(path, id, text string) {
	p.plugins[id].routeBooms[path] = routeBoom{text: text, private: true}
}

// pluginAddress returns the address of the route path of the plugin id on the served site.
func (p *pausingScenario) pluginAddress(id, path string) string {
	return p.served.base + "/api/plugins/" + id + path
}

// routeAnswersCode fails unless the route answers status with the code, names no plugin and is kept out of caches.
func (p *pausingScenario) routeAnswersCode(path, id string, status int, code string) error {
	response, err := p.served.client.Get(p.pluginAddress(id, path))
	got, err := read(response, err)
	if err != nil {
		return fmt.Errorf("the route %s of %s got no answer: %w", path, id, err)
	}
	var refused answeredError
	if err := json.Unmarshal(got.body, &refused); err != nil || got.status != status || refused.Code != code {
		return fmt.Errorf("the route %s of %s answered %d %s, want %d with the code %s", path, id, got.status,
			got.body, status, code)
	}
	if bytes.Contains(got.body, []byte(id)) {
		return fmt.Errorf("the answer %s names the plugin %s", got.body, id)
	}
	if control := response.Header.Get("Cache-Control"); !strings.Contains(control, "no-store") {
		return fmt.Errorf("the answer carries Cache-Control %q, want no-store", control)
	}
	return nil
}

// routeCut fails unless a request for the route, on a connection of its own, ends cut short and not for a timeout.
func (p *pausingScenario) routeCut(path, id string) error {
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: siteStartLimit}
	defer client.CloseIdleConnections()
	got, err := read(client.Get(p.pluginAddress(id, path)))
	var netErr net.Error
	if err == nil || errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("the route %s of %s answered %d %q (%v), want the answer cut", path, id, got.status,
			got.body, err)
	}
	return nil
}

// loggedRoutePanic fails unless the site logged one route panic, of the plugin id for the request, answered or not.
func (p *pausingScenario) loggedRoutePanic(method, path, id, text, how string) error {
	logged := p.logged()
	if count := strings.Count(logged, fmt.Sprintf("msg=%q", routePanicLine)); count != 1 {
		return fmt.Errorf("the site logged %d route panics, want 1:\n%s", count, logged)
	}
	return p.loggedLine(fmt.Sprintf("msg=%q plugin=%s method=%s path=%s answered=%t error=%q stack=\"goroutine ",
		routePanicLine, id, method, path, how == "answered", "pluginkit: "+id+" route panicked: "+text))
}

func TestPluginRoutePanics(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, pluginRoutePanicsFeature, postgresOperators())
}
