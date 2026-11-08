// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"

	"github.com/gopherium/framework/goncierge"

	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/sdk"
)

// declareRoles grants the core's roles, then every role a registered plugin declares under its id, into registry.
func declareRoles(registry *goncierge.Registry, core []goncierge.Role, registered []sdk.Plugin) error {
	if err := registry.Replace(role.Source, core); err != nil {
		return err
	}
	for _, plugin := range registered {
		if err := declarePlugin(registry, plugin); err != nil {
			return fmt.Errorf("plugin %s: %w", plugin.ID(), err)
		}
	}
	if len(registry.HoldersOf(string(role.ManageUsers))) == 0 {
		return fmt.Errorf("no role carries %s, so no account may administer the site", role.ManageUsers)
	}
	return nil
}

// defaultRoleSetting names the setting that holds the role Add User starts on.
const defaultRoleSetting = "DEFAULT_ROLE"

// checkDefaultRole returns an error naming the setting, its value and the plugins paused when registry lacks name.
func checkDefaultRole(registry *goncierge.Registry, name string, paused []string) error {
	if registry.Known(name) {
		return nil
	}
	return fmt.Errorf("%s%s: must name a role the core or a plugin declares, got %q%s",
		settingsPrefix, defaultRoleSetting, name, pausedNote(paused))
}

// declarePlugin declares the roles a plugin provides under its id, the administrator receiving their capabilities.
func declarePlugin(registry *goncierge.Registry, plugin sdk.Plugin) error {
	provider, ok := plugin.(sdk.RoleProvider)
	if !ok {
		return nil
	}
	return goncierge.Declare(registry, goncierge.Rules{Admin: role.Admin}, plugin.ID(), rolesOf(provider.Roles()))
}

// rolesOf returns the roles a plugin declares as the registry takes them.
func rolesOf(declared []sdk.RoleDeclaration) []goncierge.Role {
	roles := make([]goncierge.Role, 0, len(declared))
	for _, held := range declared {
		roles = append(roles, goncierge.Role{Name: held.Name, Capabilities: held.Capabilities})
	}
	return roles
}
