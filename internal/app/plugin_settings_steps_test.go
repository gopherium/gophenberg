// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/framework/pluginkit/settings"
	"github.com/gopherium/framework/pluginkit/wire"

	"github.com/gopherium/gophenberg/plugins/feed"
	"github.com/gopherium/gophenberg/sdk"
)

// pluginSettingsFeature is the feature a site answers to while its plugins read the settings their manifests declare.
const pluginSettingsFeature = "../../test/features/features/plugin-settings.feature"

// settingsRoute is the public route each plugin of a settings scenario answers its reads on.
const settingsRoute = "/read"

// feedManifest is the manifest the feed plugin ships with.
const feedManifest = "../../plugins/feed/plugin.json"

// feedRoute is the address the feed plugin serves its channel at.
const feedRoute = "/api/plugins/feed/rss.xml"

// publishedTitles are the titles of the posts an administrator publishes before a feed scenario serves.
var publishedTitles = []string{"First Post", "Second Post", "Third Post"}

// settingReaders turns what each reader of a plugin's settings answers into text, by the name the read route takes.
var settingReaders = map[string]func(lent sdk.Settings, key string) string{
	settings.TypeText:     sdk.Settings.Text,
	settings.TypeCount:    func(lent sdk.Settings, key string) string { return strconv.Itoa(lent.Count(key)) },
	settings.TypeFlag:     func(lent sdk.Settings, key string) string { return strconv.FormatBool(lent.Flag(key)) },
	settings.TypeDuration: func(lent sdk.Settings, key string) string { return lent.Duration(key).String() },
	settings.TypeLines:    func(lent sdk.Settings, key string) string { return strings.Join(lent.Lines(key), ", ") },
	settings.TypeSecret:   sdk.Settings.Secret,
	"core":                sdk.Settings.Core,
}

// settingsScenario is what one plugin settings scenario keeps beside the pausing scenario: manifests and last reads.
type settingsScenario struct {
	*pausingScenario
	declared map[string][]wire.Setting
	shipped  map[string]wire.Manifest
	named    map[string]manifestWords
	last     map[string]string
}

// carriedChannel is the part of the feed's channel a scenario checks: its title and its posts.
type carriedChannel struct {
	Title string     `xml:"channel>title"`
	Items []struct{} `xml:"channel>item"`
}

// settingsPlugin is a pausing plugin that answers on its read route what the settings reader it was lent reads.
type settingsPlugin struct {
	*pausingPlugin
	settings sdk.Settings
}

// bindSettings binds the steps of the scenarios where plugins declare settings in their manifests and read them.
func (p *pausingScenario) bindSettings(sc *godog.ScenarioContext) {
	s := &settingsScenario{
		pausingScenario: p, declared: map[string][]wire.Setting{}, shipped: map[string]wire.Manifest{},
		named: map[string]manifestWords{}, last: map[string]string{},
	}
	sc.Given(`^the plugin "([^"]*)" declares these settings:$`, s.declareTable)
	sc.Given(`^the plugin "([^"]*)" declares no settings$`, s.declareNone)
	sc.Given(`^the plugin "([^"]*)" declares the count setting "([^"]*)" with the default "([^"]*)"$`, s.declareCount)
	sc.Given(`^the plugin "([^"]*)" declares (?:a|the) text setting "([^"]*)"$`, s.declareText)
	sc.Given(`^the plugin "([^"]*)" declares a setting "([^"]*)" of the type "([^"]*)"$`, s.declareTyped)
	sc.Given(`^the plugin "([^"]*)" declares a count "([^"]*)" with no default$`, s.declareCountWithoutDefault)
	sc.Given(`^the plugin "([^"]*)" declares a secret "([^"]*)" with the default "([^"]*)"$`, s.declareSecretWithDefault)
	sc.When(`^the plugin "([^"]*)" reads "([^"]*)" as text twice$`, s.readAsTextTwice)
	sc.When(`^the plugin "([^"]*)" reads the count "([^"]*)" twice$`, s.readCountTwice)
	sc.When(`^the plugin "([^"]*)" reads "([^"]*)" as a count$`, s.readAsCount)
	sc.When(`^the plugin "([^"]*)" reads the core setting "([^"]*)"$`, s.readCore)
	sc.Then(`^the plugin "([^"]*)" reads these settings:$`, s.readsTable)
	sc.Then(`^the plugin "([^"]*)" reads "([^"]*)" for "([^"]*)"$`, s.readsFor)
	sc.Then(`^the plugin "([^"]*)" reads the core setting "([^"]*)" as "([^"]*)"$`, s.readsCore)
	sc.Then(`^the plugin "([^"]*)" read "([^"]*)" for "([^"]*)"$`, s.readLast)
	sc.Then(`^the plugin "([^"]*)" read "([^"]*)" for the core setting "([^"]*)"$`, s.readLast)
	sc.Then(`^the site's log holds one line "([^"]*)" for the plugin "([^"]*)" and the (key|name) "([^"]*)"$`,
		s.loggedOnce)
	sc.Then(`^no line of the site's log holds "([^"]*)"$`, s.loggedNowhere)
	sc.Then(`^the answer lists "([^"]*)" under "Not loaded", naming "([^"]*)"$`, s.listsNotLoaded)
	sc.Given(`^the feed plugin is compiled into the site$`, s.compileFeed)
	sc.Given(`^three published posts$`, s.publishThree)
	sc.Then(`^the feed carries (\d+) posts under the title "([^"]*)"$`, s.feedCarries)
	s.bindPluginList(sc)
}

// declare compiles the plugin id into the site the first time, adds declared to its settings and writes the index.
func (s *settingsScenario) declare(id string, declared ...wire.Setting) error {
	if _, compiled := s.plugins[id]; !compiled {
		plugin := &pausingPlugin{id: id, scenario: s.pausingScenario, booms: map[string]string{}}
		s.plugins[id] = plugin
		s.compiled = append(s.compiled, pluginkit.Registration[sdk.Deps, sdk.Plugin]{ID: id, Register: plugin.reading})
	}
	s.declared[id] = append(s.declared[id], declared...)
	return s.writeIndex()
}

// writeIndex writes the manifest index of the compiled plugins, in the order they were compiled.
func (s *settingsScenario) writeIndex() error {
	manifests := make([]wire.Manifest, 0, len(s.compiled))
	for _, compiled := range s.compiled {
		manifests = append(manifests, s.manifestOf(compiled.ID))
	}
	index, err := json.Marshal(manifests)
	s.index = index
	return err
}

// manifestOf returns the manifest of the compiled plugin id, the one it ships with if any, named as a scenario said.
func (s *settingsScenario) manifestOf(id string) wire.Manifest {
	manifest, shipped := s.shipped[id]
	if !shipped {
		manifest = wire.Manifest{ID: id, Name: id, Hooks: s.plugins[id].listed, Settings: s.declared[id]}
	}
	if words, named := s.named[id]; named {
		manifest.Name, manifest.Description = words.name, words.description
	}
	return manifest
}

// compileFeed compiles the feed plugin into the site under the manifest it ships with.
func (s *settingsScenario) compileFeed() error {
	raw, err := os.ReadFile(feedManifest)
	if err != nil {
		return err
	}
	var shipped wire.Manifest
	if err := json.Unmarshal(raw, &shipped); err != nil {
		return err
	}
	s.shipped[shipped.ID] = shipped
	s.compiled = append(s.compiled, pluginkit.Registration[sdk.Deps, sdk.Plugin]{ID: shipped.ID, Register: feed.Register})
	return s.writeIndex()
}

// publishThree has an administrator write and publish three posts on the site, served until they are published.
func (s *settingsScenario) publishThree() error {
	if err := s.holdAdministrator(); err != nil {
		return err
	}
	if err := s.serve(); err != nil {
		return err
	}
	return errors.Join(s.publish(publishedTitles...), s.stopServing())
}

// publish signs the administrator in and has them write and publish a post under each title.
func (s *settingsScenario) publish(titles ...string) error {
	if err := s.signIn(); err != nil {
		return err
	}
	for _, title := range titles {
		if err := s.writesDraft(title, map[string]any{"type": "post"}); err != nil {
			return err
		}
		if err := s.patch(title, title, map[string]any{"status": "published"}); err != nil {
			return err
		}
	}
	return nil
}

// feedCarries fails unless the feed the site serves carries count posts under the channel title.
func (s *settingsScenario) feedCarries(count int, title string) error {
	got, err := s.served.get(feedRoute)
	if err != nil || got.status != http.StatusOK {
		return fmt.Errorf("the feed answered %d %q (%v), want 200", got.status, got.body, err)
	}
	var carried carriedChannel
	if err := xml.Unmarshal(got.body, &carried); err != nil {
		return fmt.Errorf("reading the feed %q: %w", got.body, err)
	}
	if carried.Title != title || len(carried.Items) != count {
		return fmt.Errorf("the feed carries %d posts under %q, want %d under %q", len(carried.Items), carried.Title,
			count, title)
	}
	return nil
}

// declareTable has the plugin id declare one setting for each row of table, by the columns its header names.
func (s *settingsScenario) declareTable(id string, table *godog.Table) error {
	rows := tableCells(table)
	declared := make([]wire.Setting, 0, len(rows)-1)
	for _, row := range rows[1:] {
		setting, err := settingOf(rows[0], row)
		if err != nil {
			return err
		}
		declared = append(declared, setting)
	}
	return s.declare(id, declared...)
}

// settingOf returns the setting a row of a declaring table describes under the columns header names.
func settingOf(header, row []string) (wire.Setting, error) {
	cells := make(map[string]string, len(header))
	for i, name := range header {
		cells[name] = row[i]
	}
	setting := wire.Setting{Key: cells["key"], Type: cells["type"], Default: cells["default"]}
	if options := cells["options"]; options != "" {
		setting.Options = strings.Split(options, ", ")
	}
	least, err := boundOf(cells["min"])
	if err != nil {
		return wire.Setting{}, err
	}
	most, err := boundOf(cells["max"])
	setting.Min, setting.Max = least, most
	return setting, err
}

// boundOf returns the bound a cell of a declaring table holds, none for an empty cell.
func boundOf(cell string) (*float64, error) {
	if cell == "" {
		return nil, nil
	}
	bound, err := strconv.ParseFloat(cell, 64)
	return &bound, err
}

// declareNone compiles the plugin id into the site with no settings in its manifest.
func (s *settingsScenario) declareNone(id string) error {
	return s.declare(id)
}

// declareCount has the plugin id declare the count setting key with the default given.
func (s *settingsScenario) declareCount(id, key, fallback string) error {
	return s.declare(id, wire.Setting{Key: key, Type: settings.TypeCount, Default: fallback})
}

// declareText has the plugin id declare the text setting key with no default.
func (s *settingsScenario) declareText(id, key string) error {
	return s.declare(id, wire.Setting{Key: key, Type: settings.TypeText})
}

// declareTyped has the plugin id declare the setting key of the type kind.
func (s *settingsScenario) declareTyped(id, key, kind string) error {
	return s.declare(id, wire.Setting{Key: key, Type: kind})
}

// declareCountWithoutDefault has the plugin id declare the count setting key with no default.
func (s *settingsScenario) declareCountWithoutDefault(id, key string) error {
	return s.declare(id, wire.Setting{Key: key, Type: settings.TypeCount})
}

// declareSecretWithDefault has the plugin id declare the secret setting key with the default given.
func (s *settingsScenario) declareSecretWithDefault(id, key, fallback string) error {
	return s.declare(id, wire.Setting{Key: key, Type: settings.TypeSecret, Default: fallback})
}

// reading notes the call and answers the plugin, keeping the settings reader deps lends it.
func (pl *pausingPlugin) reading(deps sdk.Deps) (sdk.Plugin, error) {
	pl.scenario.note("Register", pl.id)
	return settingsPlugin{pausingPlugin: pl, settings: deps.Settings}, nil
}

// Routes answers on the read route what the reader the request names reads for the key it names.
func (r settingsPlugin) Routes() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET "+settingsRoute, func(w http.ResponseWriter, req *http.Request) {
		asked := req.URL.Query()
		_, _ = io.WriteString(w, settingReaders[asked.Get("reader")](r.settings, asked.Get("key")))
	})
	return routes
}

// PublicPaths opens the read route without a session.
func (settingsPlugin) PublicPaths() []string {
	return []string{settingsRoute}
}

// read asks the plugin id on its read route what reader reads for key, keeping the answer as its last read of key.
func (s *settingsScenario) read(id, reader, key string) (string, error) {
	asked := url.Values{"reader": {reader}, "key": {key}}
	got, err := s.served.get("/api/plugins/" + id + settingsRoute + "?" + asked.Encode())
	if err != nil || got.status != http.StatusOK {
		return "", fmt.Errorf("the read route of %s answered %d %q (%v), want 200", id, got.status, got.body, err)
	}
	s.last[id+" "+key] = string(got.body)
	return string(got.body), nil
}

// readTimes has the plugin id read key with reader the number of times given.
func (s *settingsScenario) readTimes(id, reader, key string, times int) error {
	for range times {
		if _, err := s.read(id, reader, key); err != nil {
			return err
		}
	}
	return nil
}

// readAsTextTwice has the plugin id read key with the text reader twice.
func (s *settingsScenario) readAsTextTwice(id, key string) error {
	return s.readTimes(id, settings.TypeText, key, 2)
}

// readCountTwice has the plugin id read key with the count reader twice.
func (s *settingsScenario) readCountTwice(id, key string) error {
	return s.readTimes(id, settings.TypeCount, key, 2)
}

// readAsCount has the plugin id read key with the count reader once.
func (s *settingsScenario) readAsCount(id, key string) error {
	return s.readTimes(id, settings.TypeCount, key, 1)
}

// readCore has the plugin id read the core setting name once.
func (s *settingsScenario) readCore(id, name string) error {
	return s.readTimes(id, "core", name, 1)
}

// readsTable fails unless the plugin id reads each value of table for the key beside it.
func (s *settingsScenario) readsTable(id string, table *godog.Table) error {
	for _, row := range tableCells(table)[1:] {
		if err := s.readsFor(id, row[1], row[0]); err != nil {
			return err
		}
	}
	return nil
}

// readsFor fails unless the plugin id reads want for key with the reader of the type its manifest gives key.
func (s *settingsScenario) readsFor(id, want, key string) error {
	kind, err := s.typeOf(id, key)
	if err != nil {
		return err
	}
	shown, err := shownAs(kind, want)
	if err != nil {
		return err
	}
	got, err := s.read(id, readerFor(kind), key)
	if err == nil && got != shown {
		err = fmt.Errorf("the plugin %s reads %q for %s, want %q", id, got, key, shown)
	}
	return err
}

// typeOf returns the type the manifest of the plugin id gives the setting key.
func (s *settingsScenario) typeOf(id, key string) (string, error) {
	for _, setting := range s.declared[id] {
		if setting.Key == key {
			return setting.Type, nil
		}
	}
	return "", fmt.Errorf("the plugin %s declares no setting %q", id, key)
}

// shownAs returns value as the read route shows a setting of type kind, a duration as time.Duration prints it.
func shownAs(kind, value string) (string, error) {
	if kind != settings.TypeDuration {
		return value, nil
	}
	parsed, err := time.ParseDuration(value)
	return parsed.String(), err
}

// readerFor returns the name of the reader a setting of type kind is read with, the text reader for a choice.
func readerFor(kind string) string {
	if kind == settings.TypeChoice {
		return settings.TypeText
	}
	return kind
}

// readsCore fails unless the plugin id reads want for the core setting name.
func (s *settingsScenario) readsCore(id, name, want string) error {
	got, err := s.read(id, "core", name)
	if err == nil && got != want {
		err = fmt.Errorf("the plugin %s reads %q for the core setting %s, want %q", id, got, name, want)
	}
	return err
}

// readLast fails unless the last read the plugin id made of key answered want.
func (s *settingsScenario) readLast(id, want, key string) error {
	got, read := s.last[id+" "+key]
	if !read || got != want {
		return fmt.Errorf("the plugin %s last read %q for %s (read %v), want %q", id, got, key, read, want)
	}
	return nil
}

// loggedOnce fails unless the site's log holds the line message once for the plugin id and the attribute given.
func (s *settingsScenario) loggedOnce(message, id, attribute, value string) error {
	line := fmt.Sprintf("msg=%q plugin=%s %s=%s", message, id, attribute, value)
	if count := strings.Count(s.logged(), line); count != 1 {
		return fmt.Errorf("the site's log holds %q %d times, want once:\n%s", line, count, s.logged())
	}
	return nil
}

// loggedNowhere fails when any line of the site's log holds text.
func (s *settingsScenario) loggedNowhere(text string) error {
	if strings.Contains(s.logged(), text) {
		return fmt.Errorf("the site's log holds %q, want it on no line", text)
	}
	return nil
}

// listsNotLoaded fails unless the listing names the plugin id under Not loaded on a line naming text.
func (s *operatorScenario) listsNotLoaded(id, text string) error {
	_, block, found := strings.Cut(s.result.Stdout, "\nNot loaded:\n")
	block, _, _ = strings.Cut(block, "\n\n")
	for _, line := range strings.Split(block, "\n") {
		if strings.Contains(line, " "+id+" ") && strings.Contains(line, text) {
			return nil
		}
	}
	return fmt.Errorf("the listing %q names %s under Not loaded %v, want a line naming %s", s.result.Stdout, id,
		found, text)
}

func TestPluginSettings(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, pluginSettingsFeature, postgresOperators())
}
