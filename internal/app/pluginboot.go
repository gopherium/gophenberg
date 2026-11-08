// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/sdk"
)

// registrations lists every compiled plugin's id and Register function, in the order the wiring holds them.
type registrations = []pluginkit.Registration[sdk.Deps, sdk.Plugin]

// pausedLine is the warning serve logs for each plugin a start pauses.
const pausedLine = "plugin paused"

// slowStartLine is the warning serve logs for each plugin whose Start ran past its budget.
const slowStartLine = "plugin started slower than its budget"

// pluginStart is what one start of the plugins left: a host over each plugin it tried, the started ones, their types.
type pluginStart struct {
	host   *pluginkit.Host
	served *pluginkit.Host
	walked definitions.Walked
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
	kept, walked, err := b.declarePlugins(ctx, paused, role.Core(), registered)
	if err != nil {
		return started, err
	}
	started.host, started.walked = hostOf(kept), walked
	started.served, err = b.startEach(ctx, paused, started.host, kept)
	return started, err
}

// registerPlugins reads the plugin records, then registers each plugin they let in, pausing each that fails.
func (b boot) registerPlugins(ctx context.Context, plugins registrations) (*pausing, []sdk.Plugin, error) {
	records, err := pluginkit.NewBoot(ctx, b.built.pluginRecords)
	if err != nil {
		return nil, nil, err
	}
	outcome, err := pluginkit.Register(ctx, records, plugins, b.built.depsFor)
	if err != nil {
		return nil, nil, err
	}
	paused := &pausing{records: records, logger: b.logger}
	paused.logged(outcome.Failed)
	return paused, outcome.Registered, nil
}

// declarePlugins grants core, then each plugin's roles and types, pausing each that fails, and lends the rest a share.
func (b boot) declarePlugins(
	ctx context.Context, paused *pausing, core []goncierge.Role, registered []sdk.Plugin,
) ([]sdk.Plugin, definitions.Walked, error) {
	if err := declareRoles(b.built.roles, core, nil); err != nil {
		return nil, nil, err
	}
	withRoles, err := paused.keep(ctx, registered, "roles", func(plugin sdk.Plugin) error {
		return declarePlugin(b.built.roles, plugin)
	})
	if err != nil {
		return nil, nil, err
	}
	walked := definitions.Walked{}
	kept, err := paused.keep(ctx, withRoles, "types", func(plugin sdk.Plugin) error {
		return declarePluginTypes(ctx, b.built.registry, plugin, walked, b.logger)
	})
	if err != nil {
		return nil, nil, err
	}
	if err := b.built.openLane(len(kept)); err != nil {
		return nil, nil, err
	}
	b.built.accounts.Open()
	return kept, walked, nil
}

// startEach starts each kept plugin on its own, pauses each that fails and answers a host over the ones that started.
func (b boot) startEach(
	ctx context.Context, paused *pausing, host *pluginkit.Host, kept []sdk.Plugin,
) (*pluginkit.Host, error) {
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
	return hostOf(startedOf(kept, outcome.Started)), nil
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
		p.note(failure.ID, string(failure.Step), failure.Err)
	}
}

// keep runs step on each plugin, answering the ones it passed and pausing each it failed under the step's name.
func (p *pausing) keep(
	ctx context.Context, plugins []sdk.Plugin, step string, run func(sdk.Plugin) error,
) ([]sdk.Plugin, error) {
	kept := make([]sdk.Plugin, 0, len(plugins))
	for _, plugin := range plugins {
		failed := stepOf(plugin, step, run)
		if failed == nil {
			kept = append(kept, plugin)
			continue
		}
		if err := p.records.Pause(ctx, plugin.ID(), failed); err != nil {
			return nil, err
		}
		p.note(plugin.ID(), step, failed)
	}
	return kept, nil
}

// stepOf runs step on plugin, naming both in the error it answers or in the panic it recovered.
func stepOf(plugin sdk.Plugin, step string, run func(sdk.Plugin) error) (failed error) {
	defer func() {
		if caught := recover(); caught != nil {
			failed = fmt.Errorf("plugin %s %s panicked: %v", plugin.ID(), step, caught)
		}
	}()
	if err := run(plugin); err != nil {
		return fmt.Errorf("plugin %s %s: %w", plugin.ID(), step, err)
	}
	return nil
}

// note logs the pause of the plugin id at step for err and keeps its id.
func (p *pausing) note(id, step string, err error) {
	p.logger.Warn(pausedLine, "plugin", id, "step", step, "error", err)
	p.paused = append(p.paused, id)
}

// declarePluginTypes hands a declaring plugin a registrar over registry, noting its walk and what it could not claim.
func declarePluginTypes(
	ctx context.Context, registry *content.Registry, plugin sdk.Plugin, walked definitions.Walked, logger *slog.Logger,
) error {
	declarer, ok := plugin.(sdk.TypeDeclarer)
	if !ok {
		return nil
	}
	registrar := definitions.New(registry, plugin.ID())
	if err := declarer.DeclareTypes(ctx, registrar); err != nil {
		return err
	}
	walked[plugin.ID()] = definitions.Walk{
		Declared: registrar.Declared(), Skipped: registrar.Skipped(), Kept: registrar.Kept(),
	}
	if kept := registrar.Kept(); len(kept) > 0 {
		logger.Warn("plugin declarations kept as they stand, the site's content holds them",
			"plugin", plugin.ID(), "keys", kept)
	}
	if skipped := registrar.Skipped(); len(skipped) > 0 {
		logger.Warn("plugin declarations skipped, another owner holds the key",
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
