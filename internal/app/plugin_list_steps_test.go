// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"slices"
	"testing"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/sdk"
)

// pluginListFeature is the feature an administrator reads the plugins the site holds by.
const pluginListFeature = "../../test/features/features/plugin-list.feature"

// pluginListRoute is the address the admin reads the plugins the site holds at.
const pluginListRoute = "/api/plugin-admin"

// editorEmail is the address of the editor a plugin list scenario signs in as.
const editorEmail = "editor@example.com"

// listScenario is what one plugin list scenario keeps beside the settings scenario: the last answer of the list.
type listScenario struct {
	*settingsScenario
	answer answered
}

// listedPlugin is one plugin as the admin API answers it, under the names the API gives each field.
type listedPlugin struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Version      string `json:"version"`
	Status       string `json:"status"`
	PausedStep   string `json:"paused_step"`
	PausedReason string `json:"paused_reason"`
}

// listedPlugins is the plugin list as the admin API answers it.
type listedPlugins struct {
	Plugins []listedPlugin `json:"plugins"`
}

// bindPluginList binds the steps of the scenarios where an administrator reads the plugins the site holds.
func (s *settingsScenario) bindPluginList(sc *godog.ScenarioContext) {
	l := &listScenario{settingsScenario: s}
	sc.Given(`^the manifest of "([^"]*)" names it "([^"]*)" and describes it as "([^"]*)"$`, l.nameManifest)
	sc.Given(`^an editor account exists$`, l.holdEditor)
	sc.When(`^the editor signs in$`, l.signInEditor)
	sc.When(`^the (?:administrator|editor) reads the plugin list$`, l.readList)
	sc.Then(`^the plugin list holds "([^"]*)" as "(active|inactive)"$`, l.listsIn)
	sc.Then(`^the plugin list holds "([^"]*)" as paused at the step "([^"]*)", its reason reading "(.*)"$`,
		l.listsPaused)
	sc.Then(`^the plugin list names "([^"]*)" as "([^"]*)"$`, l.listsName)
	sc.Then(`^the plugin list describes "([^"]*)" as "([^"]*)" at the version "([^"]*)"$`, l.listsDescription)
	sc.Then(`^the plugin list names "([^"]*)" and "([^"]*)", in that order$`, l.listsInOrder)
	sc.Then(`^the plugin list leaves out "([^"]*)"$`, l.leavesOut)
	sc.Then(`^the request is rejected with the code "([^"]*)"$`, l.rejectedWith)
}

// manifestWords are the name and the description a scenario gives the manifest of a compiled plugin.
type manifestWords struct {
	name, description string
}

// nameManifest gives the manifest of the compiled plugin id name and description, then writes the index again.
func (l *listScenario) nameManifest(id, name, description string) error {
	compiled := slices.ContainsFunc(l.compiled, func(held pluginkit.Registration[sdk.Deps, sdk.Plugin]) bool {
		return held.ID == id
	})
	if !compiled {
		return fmt.Errorf("the manifest of %s is named before %s is compiled, want it compiled first", id, id)
	}
	l.named[id] = manifestWords{name: name, description: description}
	return l.writeIndex()
}

// holdEditor creates the editor the scenario signs in as.
func (l *listScenario) holdEditor() error {
	return l.holdAccount("editor", editorEmail)
}

// signInEditor signs the editor in to the site the scenario serves.
func (l *listScenario) signInEditor() error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	client := &http.Client{Jar: jar, Timeout: siteStartLimit}
	login, _ := json.Marshal(map[string]string{"email": editorEmail, "password": typedPassword})
	got, err := read(client.Post(l.served.base+"/api/auth/login", "application/json", bytes.NewReader(login)))
	if err == nil && got.status != http.StatusOK {
		err = fmt.Errorf("signing the editor in answered %d and %s, want 200", got.status, got.body)
	}
	l.served.client = client
	return err
}

// readList reads the plugin list as the account the scenario signed in last.
func (l *listScenario) readList() error {
	got, err := l.served.get(pluginListRoute)
	l.answer = got
	return err
}

// listed returns the plugins the last read of the list answered, failing unless it answered 200.
func (l *listScenario) listed() ([]listedPlugin, error) {
	if l.answer.status != http.StatusOK {
		return nil, fmt.Errorf("the plugin list answered %d %s, want 200", l.answer.status, l.answer.body)
	}
	var held listedPlugins
	if err := json.Unmarshal(l.answer.body, &held); err != nil {
		return nil, fmt.Errorf("reading the plugin list %s: %w", l.answer.body, err)
	}
	return held.Plugins, nil
}

// listedOne fails unless the list holds the plugin id and kept passes it, naming want.
func (l *listScenario) listedOne(id string, kept func(listedPlugin) bool, want string) error {
	listed, err := l.listed()
	if err != nil {
		return err
	}
	at := slices.IndexFunc(listed, func(plugin listedPlugin) bool { return plugin.ID == id })
	if at < 0 || !kept(listed[at]) {
		return fmt.Errorf("the plugin list holds %+v, want %s %s", listed, id, want)
	}
	return nil
}

// stateOf returns the id, the status, the step and the reason of the listed plugin, and nothing else of it.
func stateOf(plugin listedPlugin) listedPlugin {
	return listedPlugin{
		ID: plugin.ID, Status: plugin.Status, PausedStep: plugin.PausedStep, PausedReason: plugin.PausedReason,
	}
}

// listsIn fails unless the list holds the plugin id in state, with no step and no reason.
func (l *listScenario) listsIn(id, state string) error {
	want := listedPlugin{ID: id, Status: state}
	return l.listedOne(id, func(plugin listedPlugin) bool {
		return stateOf(plugin) == want
	}, "as "+state+" with no step and no reason")
}

// listsPaused fails unless the list holds the plugin id paused at step with reason as its whole reason.
func (l *listScenario) listsPaused(id, step, reason string) error {
	want := listedPlugin{ID: id, Status: "paused", PausedStep: step, PausedReason: reason}
	return l.listedOne(id, func(plugin listedPlugin) bool {
		return stateOf(plugin) == want
	}, fmt.Sprintf("as %+v", want))
}

// listsName fails unless the list holds the plugin id under name.
func (l *listScenario) listsName(id, name string) error {
	return l.listedOne(id, func(plugin listedPlugin) bool {
		return plugin.Name == name
	}, "named "+name)
}

// listsDescription fails unless the list holds the plugin id with description at version.
func (l *listScenario) listsDescription(id, description, version string) error {
	want := listedPlugin{Description: description, Version: version}
	return l.listedOne(id, func(plugin listedPlugin) bool {
		return listedPlugin{Description: plugin.Description, Version: plugin.Version} == want
	}, fmt.Sprintf("described as %q at the version %q", description, version))
}

// listsInOrder fails unless the list holds the plugins first and second alone, in that order.
func (l *listScenario) listsInOrder(first, second string) error {
	listed, err := l.listed()
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(listed))
	for _, plugin := range listed {
		ids = append(ids, plugin.ID)
	}
	if want := []string{first, second}; !slices.Equal(ids, want) {
		return fmt.Errorf("the plugin list names %v, want %v", ids, want)
	}
	return nil
}

// leavesOut fails when the list holds the plugin id.
func (l *listScenario) leavesOut(id string) error {
	listed, err := l.listed()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(listed, func(plugin listedPlugin) bool { return plugin.ID == id }) {
		return fmt.Errorf("the plugin list holds %+v, want %s left out", listed, id)
	}
	return nil
}

// rejectedWith fails unless the last read of the list was rejected as forbidden with code.
func (l *listScenario) rejectedWith(code string) error {
	var rejected answeredError
	err := json.Unmarshal(l.answer.body, &rejected)
	if l.answer.status != http.StatusForbidden || rejected.Code != code {
		return fmt.Errorf("the plugin list answered %d %s (%v), want 403 and the code %s", l.answer.status,
			l.answer.body, err, code)
	}
	return nil
}

func TestPluginList(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, pluginListFeature, postgresOperators())
}
