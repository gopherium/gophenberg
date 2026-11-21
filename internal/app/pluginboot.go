// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/pluginhooks"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/sdk"
)

// registrations lists every compiled plugin's id and Register function, in the order the wiring holds them.
type registrations = []pluginkit.Registration[sdk.Deps, sdk.Plugin]

// pausedLine is the warning serve logs for each plugin a start pauses.
const pausedLine = "plugin paused"

// slowStartLine is the warning serve logs for each plugin whose Start ran past its budget.
const slowStartLine = "plugin started slower than its budget"

const (
	// stepTypes is the declaration of a plugin's content types, a step only this site runs.
	stepTypes pluginkit.Step = "types"
	// stepHooks is the check of a plugin's listed hooks against its methods, a step only this site runs.
	stepHooks pluginkit.Step = "hooks"
)

// pluginStart is what one start of the plugins left: a host over each tried plugin, the started ones' routes and hooks.
type pluginStart struct {
	host        *pluginkit.Host
	routes      map[string]http.Handler
	publicPaths map[string][]string
	hooks       *pluginhooks.Set
	walked      definitions.Walked
}

// declaredPlugins is what the declaring steps of one start kept: the plugins that passed, the types walked, routes.
type declaredPlugins struct {
	kept   []sdk.Plugin
	walked definitions.Walked
	routes pluginkit.RoutesOutcome
}

// pausing pauses the plugins one start fails on through its records, logging each and keeping their ids.
type pausing struct {
	records *pluginkit.Boot
	logger  *slog.Logger
	paused  []string
}

// startPlugins registers, declares and starts the plugins the records let in, pausing each one that fails.
func (b boot) startPlugins(ctx context.Context, plugins registrations) (pluginStart, error) {
	started := pluginStart{host: pluginkit.NewHost()}
	paused, registered, err := b.registerPlugins(ctx, plugins)
	if err != nil {
		return started, err
	}
	held, err := b.declarePlugins(ctx, paused, role.Core(), registered)
	if err != nil {
		return started, err
	}
	started.host, started.walked = hostOf(held.kept), held.walked
	running, err := b.startEach(ctx, paused, started.host, held.kept)
	started.routes, started.publicPaths = servedRoutes(held.routes, running)
	paused.mark(held.walked)
	if err != nil {
		return started, err
	}
	started.hooks, err = pluginhooks.New(running, b.settings.hookSettings(), b.logger)
	return started, err
}

// hookSettings returns the values every plugin hook runs under.
func (c runConfig) hookSettings() pluginhooks.Settings {
	return pluginhooks.Settings{
		Budget: c.pluginHookBudget, SaveBudget: c.pluginHookSaveBudget, TimedFirings: c.pluginHookTimedFirings,
	}
}

// registerPlugins reads the plugin records, then registers each plugin they let in, pausing each that fails.
func (b boot) registerPlugins(ctx context.Context, plugins registrations) (*pausing, []sdk.Plugin, error) {
	records, err := pluginkit.NewBoot(ctx, b.built.pluginRecords)
	if err != nil {
		return nil, nil, err
	}
	outcome, err := pluginkit.Register(ctx, records, b.built.checked(plugins), b.built.depsFor)
	if err != nil {
		return nil, nil, err
	}
	paused := &pausing{records: records, logger: b.logger}
	paused.logged(outcome.Failed)
	return paused, outcome.Registered, nil
}

// declarePlugins grants core, then each plugin's roles, types, hooks and routes, pausing failures, and lends a share.
func (b boot) declarePlugins(
	ctx context.Context, paused *pausing, core []goncierge.Role, registered []sdk.Plugin,
) (declaredPlugins, error) {
	if err := declareRoles(b.built.roles, core, nil); err != nil {
		return declaredPlugins{}, err
	}
	withRoles, err := paused.keep(ctx, registered, pluginkit.StepRoles, func(plugin sdk.Plugin) error {
		return declarePlugin(b.built.roles, plugin)
	})
	if err != nil {
		return declaredPlugins{}, err
	}
	held := declaredPlugins{walked: definitions.Walked{}}
	withTypes, err := paused.keep(ctx, withRoles, stepTypes, func(plugin sdk.Plugin) error {
		return b.declarePluginTypes(ctx, plugin, held.walked)
	})
	if err != nil {
		return declaredPlugins{}, err
	}
	withHooks, err := paused.keep(ctx, withTypes, stepHooks, func(plugin sdk.Plugin) error {
		return pluginhooks.Check(plugin, b.listed.byPlugin[plugin.ID()], b.listed.defined)
	})
	if err != nil {
		return declaredPlugins{}, err
	}
	if held.kept, held.routes, err = paused.routed(ctx, withHooks); err != nil {
		return declaredPlugins{}, err
	}
	if err := b.built.openLane(len(held.kept)); err != nil {
		return declaredPlugins{}, err
	}
	b.built.accounts.Open()
	return held, nil
}

// servedRoutes returns the routes and public paths read for the running plugins only.
func servedRoutes(read pluginkit.RoutesOutcome, running []sdk.Plugin) (map[string]http.Handler, map[string][]string) {
	routes, public := map[string]http.Handler{}, map[string][]string{}
	for _, plugin := range running {
		id := plugin.ID()
		if handler, ok := read.Routes[id]; ok {
			routes[id] = handler
		}
		if paths, ok := read.PublicPaths[id]; ok {
			public[id] = paths
		}
	}
	return routes, public
}

// startEach starts each kept plugin on its own, pauses each that fails and answers the ones that started.
func (b boot) startEach(
	ctx context.Context, paused *pausing, host *pluginkit.Host, kept []sdk.Plugin,
) ([]sdk.Plugin, error) {
	outcome, err := host.StartEach(ctx)
	if err := errors.Join(err, paused.records.Started(ctx, outcome)); err != nil {
		return nil, err
	}
	paused.logged(outcome.Failed)
	b.warnSlow(outcome.Started)
	for _, id := range paused.paused {
		b.built.roles.Withdraw(id)
	}
	if err := checkDefaultRole(b.built.roles, b.built.defaultRole, paused.paused); err != nil {
		return nil, err
	}
	return startedOf(kept, outcome.Started), nil
}

// warnSlow logs each started plugin whose Start ran past the budget the settings name.
func (b boot) warnSlow(started []pluginkit.StartedPlugin) {
	budget := b.settings.pluginStartBudget
	for _, plugin := range started {
		if plugin.Took > budget {
			b.logger.Warn(slowStartLine, "plugin", plugin.ID, "took", plugin.Took, "budget", budget)
		}
	}
}

// startedOf returns the kept plugins that started, in their order.
func startedOf(kept []sdk.Plugin, started []pluginkit.StartedPlugin) []sdk.Plugin {
	return slices.DeleteFunc(slices.Clone(kept), func(plugin sdk.Plugin) bool {
		return !slices.ContainsFunc(started, func(held pluginkit.StartedPlugin) bool { return held.ID == plugin.ID() })
	})
}

// logged logs each failure the plugin records paused, keeping its id.
func (p *pausing) logged(failures []pluginkit.Failure) {
	for _, failure := range failures {
		p.note(failure)
	}
}

// keep runs step on each plugin, answering the ones it passed and pausing each it failed at that step.
func (p *pausing) keep(
	ctx context.Context, plugins []sdk.Plugin, step pluginkit.Step, run func(sdk.Plugin) error,
) ([]sdk.Plugin, error) {
	kept := make([]sdk.Plugin, 0, len(plugins))
	for _, plugin := range plugins {
		failure := stepOf(plugin.ID(), step, func() error { return run(plugin) })
		if failure.Err == nil {
			kept = append(kept, plugin)
			continue
		}
		if err := p.pause(ctx, failure); err != nil {
			return nil, err
		}
	}
	return kept, nil
}

// routed reads each plugin's routes, pausing each whose routes fail to load, and answers the rest and what it read.
func (p *pausing) routed(ctx context.Context, plugins []sdk.Plugin) ([]sdk.Plugin, pluginkit.RoutesOutcome, error) {
	read := pluginkit.RoutesEach(plugins)
	for _, failure := range read.Failed {
		if err := p.pause(ctx, failure); err != nil {
			return nil, read, err
		}
	}
	kept := slices.DeleteFunc(slices.Clone(plugins), func(plugin sdk.Plugin) bool {
		return slices.ContainsFunc(read.Failed, func(failure pluginkit.Failure) bool { return failure.ID == plugin.ID() })
	})
	return kept, read, nil
}

// pause pauses the plugin the failure names through the records, then logs the pause and keeps its id.
func (p *pausing) pause(ctx context.Context, failure pluginkit.Failure) error {
	if err := p.records.Pause(ctx, failure); err != nil {
		return err
	}
	p.note(failure)
	return nil
}

// stepOf runs the step of the plugin id, answering its failure, or one with no Err when the step passed.
func stepOf(id string, step pluginkit.Step, run func() error) (failure pluginkit.Failure) {
	failure = pluginkit.Failure{ID: id, Step: step}
	defer func() {
		if caught := recover(); caught != nil {
			failure.Panicked, failure.Reason = true, fmt.Sprintf("panicked: %v", caught)
			failure.Err = fmt.Errorf("plugin %s %s %s", id, step, failure.Reason)
		}
	}()
	if err := run(); err != nil {
		failed := &stepError{front: fmt.Sprintf("plugin %s %s: ", id, step), text: fmt.Sprint(err), err: err}
		failure.Err, failure.Reason = failed, failed.text
	}
	return failure
}

// stepError is the error a plugin's step returned, its text read once and kept apart from the plugin id and the step.
type stepError struct {
	front string
	text  string
	err   error
}

// Error returns the text with the plugin id and the step in front, as the log line prints it.
func (e *stepError) Error() string {
	return e.front + e.text
}

// Unwrap returns the error the step returned.
func (e *stepError) Unwrap() error {
	return e.err
}

// mark marks each plugin the start paused as paused in walked.
func (p *pausing) mark(walked definitions.Walked) {
	for _, id := range p.paused {
		walk := walked[id]
		walk.Paused = true
		walked[id] = walk
	}
}

// note logs the pause the failure names, with its whole error, and keeps the paused plugin's id.
func (p *pausing) note(failure pluginkit.Failure) {
	p.logger.Warn(pausedLine, "plugin", failure.ID, "step", string(failure.Step), "error", failure.Err)
	p.paused = append(p.paused, failure.ID)
}

// declarePluginTypes hands a declaring plugin a registrar over the types, noting its walk and what it could not claim.
func (b boot) declarePluginTypes(ctx context.Context, plugin sdk.Plugin, walked definitions.Walked) error {
	declarer, ok := plugin.(sdk.TypeDeclarer)
	if !ok {
		return nil
	}
	registrar := definitions.New(b.built.registry, plugin.ID())
	if err := declarer.DeclareTypes(ctx, registrar); err != nil {
		return err
	}
	walked[plugin.ID()] = definitions.Walk{
		Declared: registrar.Declared(), Skipped: registrar.Skipped(), Kept: registrar.Kept(),
	}
	if kept := registrar.Kept(); len(kept) > 0 {
		b.logger.Warn("plugin declarations kept as they stand, the site's content holds them",
			"plugin", plugin.ID(), "keys", kept)
	}
	if skipped := registrar.Skipped(); len(skipped) > 0 {
		b.logger.Warn("plugin declarations skipped, another owner holds the key",
			"plugin", plugin.ID(), "keys", skipped)
	}
	return nil
}

// pausedNote returns the tail of an error naming the plugins a start paused, empty when it paused none.
func pausedNote(paused []string) string {
	if len(paused) == 0 {
		return ""
	}
	return ", and this start paused " + strings.Join(paused, ", ")
}
