// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/sdk"
)

// offering is a registered plugin as the command line walks it for commands.
type offering struct {
	plugin sdk.Plugin
}

// offerings returns the registered plugins as the command line walks them for commands.
func offerings(plugins []sdk.Plugin) []offering {
	offered := make([]offering, 0, len(plugins))
	for _, plugin := range plugins {
		offered = append(offered, offering{plugin: plugin})
	}
	return offered
}

// ID returns the plugin's identifier.
func (o offering) ID() string {
	return o.plugin.ID()
}

// Commands returns the plugin's commands as the command line runs them, none when it offers none.
func (o offering) Commands() []gonsole.Command {
	provider, offers := o.plugin.(sdk.CommandProvider)
	if !offers {
		return nil
	}
	held := provider.Commands()
	commands := make([]gonsole.Command, 0, len(held))
	for _, command := range held {
		commands = append(commands, commandOf(command))
	}
	return commands
}

// commandOf returns a plugin command as the command line runs it.
func commandOf(command sdk.Command) gonsole.Command {
	converted := gonsole.Command{
		Name: command.Name, Summary: command.Summary, Args: command.Args, Flags: command.Flags,
		Writes: command.Writes, JSON: command.JSON, Capability: command.Capability,
	}
	if run := command.Run; run != nil {
		converted.Run = func(ctx context.Context, call gonsole.Call) error {
			return misusedAs(run(ctx, callOf(call)))
		}
	}
	return converted
}

// callOf returns the parts of a command line call a plugin command reads.
func callOf(call gonsole.Call) sdk.Call {
	return sdk.Call{
		Args: call.Args, Flags: call.Flags, Stdin: call.Stdin, Stdout: call.Stdout, Stderr: call.Stderr,
		JSON: call.JSON, Apply: call.Apply, Actor: call.Actor,
	}
}

// misusedAs returns err with a plugin's misuse marked the way the command line reads one.
func misusedAs(err error) error {
	if errors.Is(err, sdk.ErrMisused) {
		return gonsole.Misuse(err)
	}
	return err
}

// hostOf returns the plugin host over the registered plugins.
func hostOf(plugins []sdk.Plugin) *pluginkit.Host {
	hosted := make([]pluginkit.Plugin, 0, len(plugins))
	for _, plugin := range plugins {
		hosted = append(hosted, plugin)
	}
	return pluginkit.NewHost(hosted...)
}
