// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/sdk"
)

// pluginPausingFeature is the feature a start answers to while one of its plugins fails.
const pluginPausingFeature = "../../test/features/features/plugin-pausing.feature"

// pausingRoute is the public route every compiled plugin of a pausing scenario serves.
const pausingRoute = "/hello"

// pausingScenario is what one plugin pausing scenario keeps beside the operator scenario: its compiled plugins.
type pausingScenario struct {
	*operatorScenario
	plugins map[string]*pausingPlugin
	mu      sync.Mutex
	calls   []string
}

// pausingPlugin is a compiled plugin of a pausing scenario, failing where the scenario says.
type pausingPlugin struct {
	id           string
	scenario     *pausingScenario
	registerErr  error
	booms        map[string]string
	migrateErr   error
	startErr     error
	typesErr     error
	takes        time.Duration
	stopsTheSite bool
	roles        []sdk.RoleDeclaration
}

// initializePluginPausing binds the steps of the scenarios where a start pauses a plugin that fails to s.
func initializePluginPausing(sc *godog.ScenarioContext, s *operatorScenario) {
	p := &pausingScenario{operatorScenario: s, plugins: map[string]*pausingPlugin{}}
	sc.Given(`^the compiled plugins "([^"]*)" and "([^"]*)"$`, p.compile)
	sc.Given(`^the Register of "([^"]*)" fails with "([^"]*)"$`, p.failRegister)
	sc.Given(`^the (Register|Roles|DeclareTypes) of "([^"]*)" panics with "([^"]*)"$`, p.panics)
	sc.Given(`^the migration of "([^"]*)" fails with "([^"]*)"$`, p.failMigration)
	sc.Given(`^the Start of "([^"]*)" fails with "([^"]*)"$`, p.failStart)
	sc.Given(`^the Start of "([^"]*)" takes "([^"]*)"$`, p.slowStart)
	sc.Given(`^the content types of "([^"]*)" fail with "([^"]*)"$`, p.failTypes)
	sc.Given(`^"([^"]*)" declares the role "([^"]*)" with the capability "([^"]*)"$`, p.declare)
	sc.Given(`^the site is told to stop while "([^"]*)" starts$`, p.stopWhileStarting)
	sc.Given(`^the plugin records already hold "([^"]*)" as paused with "([^"]*)"$`, p.holdPaused)
	sc.Given(`^the plugin records already hold "([^"]*)" as (inactive|not installed)$`, p.holdIn)
	sc.When(`^the site serves$`, p.serve)
	sc.Then(`^the route of "([^"]*)" answers (\d+)$`, p.routeAnswers)
	sc.Then(`^the plugin records hold "([^"]*)" as (active|inactive|not installed) with no error$`, p.holdsClean)
	sc.Then(`^the plugin records hold "([^"]*)" as paused, its error saying "([^"]*)"$`, p.holdsPaused)
	sc.Then(`^the plugin records hold no record$`, p.holdsNone)
	sc.Then(`^the site logged that "([^"]*)" is paused at the step "([^"]*)"$`, p.loggedPause)
	sc.Then(`^the (Register|Start) of "([^"]*)" never ran$`, p.neverRan)
	sc.Then(`^the site logged a warning that "([^"]*)" started slower than its budget$`, p.warnedSlow)
	sc.Then(`^the site logged no slow start for "([^"]*)"$`, p.warnedNoSlow)
	sc.Then(`^the error names the paused plugin "([^"]*)"$`, p.namesPaused)
}

// compile compiles the plugins first and second into the scenario's binary, in that order.
func (p *pausingScenario) compile(first, second string) {
	for _, id := range []string{first, second} {
		plugin := &pausingPlugin{id: id, scenario: p, booms: map[string]string{}}
		p.plugins[id] = plugin
		p.compiled = append(p.compiled, pluginkit.Registration[sdk.Deps, sdk.Plugin]{ID: id, Register: plugin.register})
	}
}

// failRegister makes the Register of the plugin id answer text as its error.
func (p *pausingScenario) failRegister(id, text string) {
	p.plugins[id].registerErr = errors.New(text)
}

// panics makes the method of the plugin id panic with value.
func (p *pausingScenario) panics(method, id, value string) {
	p.plugins[id].booms[method] = value
}

// failMigration makes the migration of the plugin id answer text as its error.
func (p *pausingScenario) failMigration(id, text string) {
	p.plugins[id].migrateErr = errors.New(text)
}

// failStart makes the Start of the plugin id answer text as its error.
func (p *pausingScenario) failStart(id, text string) {
	p.plugins[id].startErr = errors.New(text)
}

// slowStart makes the Start of the plugin id last the duration text names.
func (p *pausingScenario) slowStart(id, text string) error {
	took, err := time.ParseDuration(text)
	p.plugins[id].takes = took
	return err
}

// failTypes makes the type declaration of the plugin id answer text as its error.
func (p *pausingScenario) failTypes(id, text string) {
	p.plugins[id].typesErr = errors.New(text)
}

// declare has the plugin id declare the role name carrying the capability.
func (p *pausingScenario) declare(id, name, capability string) {
	held := p.plugins[id]
	held.roles = append(held.roles, sdk.RoleDeclaration{Name: name, Capabilities: []string{capability}})
}

// stopWhileStarting makes the Start of the plugin id tell the site to stop.
func (p *pausingScenario) stopWhileStarting(id string) {
	p.plugins[id].stopsTheSite = true
}

// holdPaused migrates the scenario's database and records the plugin id there as paused with text.
func (p *pausingScenario) holdPaused(ctx context.Context, id, text string) error {
	return p.holdRecord(ctx, pluginkit.PluginRecord{ID: id, State: pluginkit.PluginPaused, LastError: text})
}

// holdIn migrates the scenario's database and records the plugin id there in state.
func (p *pausingScenario) holdIn(ctx context.Context, id, state string) error {
	return p.holdRecord(ctx, pluginkit.PluginRecord{ID: id, State: pluginkit.PluginState(state)})
}

// holdRecord migrates the scenario's database, then stores record in it.
func (p *pausingScenario) holdRecord(ctx context.Context, record pluginkit.PluginRecord) error {
	address := p.env["GOPHENBERG_DATABASE_URL"]
	if err := migrate(ctx, address, testdb.Open(p.t, address)); err != nil {
		return fmt.Errorf("migrating the database: %w", err)
	}
	pool, err := pgxpool.New(ctx, address)
	if err != nil {
		return err
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, "INSERT INTO core.plugins (id, state, last_error) VALUES ($1, $2, $3)",
		record.ID, string(record.State), record.LastError)
	return err
}

// serve serves the site in the background over the compiled plugins until it answers.
func (p *pausingScenario) serve() error {
	address := testkit.FreeAddr(p.t)
	p.env["GOPHENBERG_ADDR"] = address
	serving, cancel := context.WithCancel(p.t.Context())
	site := &servedSite{
		base: "http://" + address, client: &http.Client{Timeout: siteStartLimit}, cancel: cancel,
		done: make(chan int, 1), stderr: &keptWatch{cancel: func() {}},
	}
	exits := &exitCodes{}
	program := commandLine(testkit.Getenv(p.env), p.loaded(), exits, p.stamp)
	go func() {
		site.done <- exits.of(program.Run(serving, []string{"serve"}, strings.NewReader(""), io.Discard, site.stderr))
	}()
	p.served = site
	return awaitAnswer(site)
}

// awaitAnswer waits within the start limit until site answers a request, failing when its run ends first.
func awaitAnswer(site *servedSite) error {
	deadline := time.Now().Add(siteStartLimit)
	for time.Now().Before(deadline) {
		select {
		case code := <-site.done:
			site.done <- code
			return fmt.Errorf("the site stopped with %d and stderr %q before it answered", code, site.stderr.String())
		case <-time.After(50 * time.Millisecond):
		}
		if _, err := read(site.client.Get(site.base + "/api/version")); err == nil {
			return nil
		}
	}
	return fmt.Errorf("the site never answered within %v", siteStartLimit)
}

// routeAnswers fails unless the route of the plugin id answers status.
func (p *pausingScenario) routeAnswers(id string, status int) error {
	got, err := read(p.served.client.Get(p.served.base + "/api/plugins/" + id + pausingRoute))
	if err != nil || got.status != status {
		return fmt.Errorf("the route of %s answered %d (%v), want %d", id, got.status, err, status)
	}
	return nil
}

// holdsClean fails unless the plugin records hold the plugin id in state with no error.
func (p *pausingScenario) holdsClean(ctx context.Context, id, state string) error {
	return p.holds(ctx, id, func(held pluginkit.PluginRecord) bool {
		return held == pluginkit.PluginRecord{ID: id, State: pluginkit.PluginState(state)}
	}, state+" with no error")
}

// holdsPaused fails unless the plugin records hold the plugin id paused with an error carrying text.
func (p *pausingScenario) holdsPaused(ctx context.Context, id, text string) error {
	return p.holds(ctx, id, func(held pluginkit.PluginRecord) bool {
		return held.State == pluginkit.PluginPaused && strings.Contains(held.LastError, text)
	}, "paused for "+text)
}

// holds fails unless the record of the plugin id passes kept, naming want.
func (p *pausingScenario) holds(
	ctx context.Context, id string, kept func(pluginkit.PluginRecord) bool, want string,
) error {
	records, err := p.records(ctx)
	if err != nil {
		return err
	}
	if held, ok := records[id]; !ok || !kept(held) {
		return fmt.Errorf("the plugin records hold %+v for %s, want it %s", held, id, want)
	}
	return nil
}

// holdsNone fails unless the plugin records hold no record at all.
func (p *pausingScenario) holdsNone(ctx context.Context) error {
	records, err := p.records(ctx)
	if err != nil || len(records) != 0 {
		return fmt.Errorf("the plugin records hold %+v (%v), want none", records, err)
	}
	return nil
}

// records returns every plugin record the scenario's database holds.
func (p *pausingScenario) records(ctx context.Context) (map[string]pluginkit.PluginRecord, error) {
	pool, err := pgxpool.New(ctx, p.env["GOPHENBERG_DATABASE_URL"])
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	return postgres.NewPluginStore(pool).Records(ctx)
}

// logged returns what the site the scenario served or started wrote to its error stream.
func (p *pausingScenario) logged() string {
	if p.served != nil {
		return p.served.stderr.String()
	}
	return p.result.Stderr
}

// loggedPause fails unless the site logged the pause of the plugin id at step.
func (p *pausingScenario) loggedPause(id, step string) error {
	return p.loggedLine(fmt.Sprintf("msg=%q plugin=%s step=%s", pausedLine, id, step))
}

// warnedSlow fails unless the site warned that the plugin id started slower than its budget.
func (p *pausingScenario) warnedSlow(id string) error {
	return p.loggedLine(fmt.Sprintf("msg=%q plugin=%s", slowStartLine, id))
}

// warnedNoSlow fails when the site warned that the plugin id started slower than its budget.
func (p *pausingScenario) warnedNoSlow(id string) error {
	if line := fmt.Sprintf("msg=%q plugin=%s", slowStartLine, id); strings.Contains(p.logged(), line) {
		return fmt.Errorf("the site logged %q, want no slow start for %s", line, id)
	}
	return nil
}

// loggedLine fails unless the site's error stream carries line.
func (p *pausingScenario) loggedLine(line string) error {
	if logged := p.logged(); !strings.Contains(logged, line) {
		return fmt.Errorf("the site logged %q, want %q among it", logged, line)
	}
	return nil
}

// neverRan fails when the step of the plugin id ran.
func (p *pausingScenario) neverRan(step, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if slices.Contains(p.calls, step+" "+id) {
		return fmt.Errorf("the %s of %s ran, want it never run", step, id)
	}
	return nil
}

// namesPaused fails unless the error the start ended with names the plugin id as paused.
func (p *pausingScenario) namesPaused(id string) error {
	return p.errorSays("this start paused " + id)
}

// note records that the step of the plugin id ran.
func (p *pausingScenario) note(step, id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, step+" "+id)
}

// register notes the call and answers the plugin, or the failure or the panic the scenario set.
func (pl *pausingPlugin) register(sdk.Deps) (sdk.Plugin, error) {
	pl.scenario.note("Register", pl.id)
	pl.boom("Register")
	if pl.registerErr != nil {
		return nil, pl.registerErr
	}
	return pl, nil
}

// ID returns the plugin's identifier.
func (pl *pausingPlugin) ID() string {
	return pl.id
}

// Migrate applies nothing, or answers the failure the scenario set.
func (pl *pausingPlugin) Migrate(context.Context) error {
	return pl.migrateErr
}

// Start notes the call, takes the time the scenario set, tells the site to stop when asked and answers its failure.
func (pl *pausingPlugin) Start(context.Context) error {
	pl.scenario.note("Start", pl.id)
	time.Sleep(pl.takes)
	if pl.stopsTheSite {
		pl.scenario.stopping()
	}
	return pl.startErr
}

// Stop stops nothing.
func (*pausingPlugin) Stop(context.Context) error {
	return nil
}

// Routes answers 200 on the plugin's one route.
func (*pausingPlugin) Routes() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET "+pausingRoute, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return routes
}

// PublicPaths opens the plugin's one route to every visitor.
func (*pausingPlugin) PublicPaths() []string {
	return []string{pausingRoute}
}

// Roles returns the roles the scenario had the plugin declare, or panics as it set.
func (pl *pausingPlugin) Roles() []sdk.RoleDeclaration {
	pl.boom("Roles")
	return pl.roles
}

// DeclareTypes declares nothing, or answers the failure or the panic the scenario set.
func (pl *pausingPlugin) DeclareTypes(context.Context, sdk.TypeRegistrar) error {
	pl.boom("DeclareTypes")
	return pl.typesErr
}

// boom panics with the value the scenario set for method, if any.
func (pl *pausingPlugin) boom(method string) {
	if value := pl.booms[method]; value != "" {
		panic(value)
	}
}

func TestPluginPausing(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, pluginPausingFeature, postgresOperators())
}
