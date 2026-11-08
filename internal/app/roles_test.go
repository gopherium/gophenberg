// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/sdk"
)

// declaring is a plugin declaring the roles it holds under its id.
type declaring struct {
	id    string
	roles []sdk.RoleDeclaration
}

// ID returns the plugin's identifier.
func (d declaring) ID() string {
	return d.id
}

// Start readies nothing.
func (declaring) Start(context.Context) error {
	return nil
}

// Stop stops nothing.
func (declaring) Stop(context.Context) error {
	return nil
}

// Roles returns the roles the plugin holds.
func (d declaring) Roles() []sdk.RoleDeclaration {
	return d.roles
}

// declaringRole returns a plugin under id declaring the one role with its capabilities.
func declaringRole(id, name string, capabilities ...string) declaring {
	return declaring{id: id, roles: []sdk.RoleDeclaration{{Name: name, Capabilities: capabilities}}}
}

// declared returns a registry holding the core's roles and every role the plugins declare.
func declared(t *testing.T, plugins ...sdk.Plugin) *goncierge.Registry {
	t.Helper()
	registry := goncierge.New()
	if err := declareRoles(registry, role.Core(), plugins); err != nil {
		t.Fatalf("declareRoles() = %v, want every declaration granted", err)
	}
	return registry
}

func TestDeclareRolesGrantsAPluginsRoleAndItsCapabilityToTheAdministrator(t *testing.T) {
	t.Parallel()

	registry := declared(t, declaringRole("notes", "archivist", "notes.manage"))

	if !registry.Can("archivist", "notes.manage") || !registry.Can(role.Admin, "notes.manage") {
		t.Errorf("archivist and admin carry notes.manage %v and %v, want both to",
			registry.Can("archivist", "notes.manage"), registry.Can(role.Admin, "notes.manage"))
	}
	if registry.Can(role.Editor, "notes.manage") {
		t.Error("editor carries notes.manage, want only the declared role and the administrator")
	}
}

func TestDeclareRolesWidensACoreRoleAndTheAdministratorAlone(t *testing.T) {
	t.Parallel()

	registry := declared(t, declaringRole("notes", role.Editor, "notes.edit"))

	if !registry.Can(role.Editor, "notes.edit") || !registry.Can(role.Admin, "notes.edit") {
		t.Error("editor or admin lacks notes.edit, want the widened role and the administrator to carry it")
	}
	if registry.Can(role.Author, "notes.edit") || !registry.Can(role.Editor, string(role.ChangeOthersWork)) {
		t.Error("widening editor changed another role or dropped editor's own capability")
	}
}

func TestDeclareRolesLetsAPluginGrantACapabilityTheCoreDeclares(t *testing.T) {
	t.Parallel()

	registry := declared(t, declaringRole("teams", "team_lead", string(role.ManageUsers)))

	if held := registry.HoldersOf(string(role.ManageUsers)); !slices.Equal(held, []string{role.Admin, "team_lead"}) {
		t.Errorf("HoldersOf(ManageUsers) = %v, want admin and team_lead", held)
	}
}

func TestDeclareRolesRefusesANewCapabilityOutsideThePluginsPrefix(t *testing.T) {
	t.Parallel()

	err := declareRoles(goncierge.New(), role.Core(), []sdk.Plugin{declaringRole("notes", "archivist", "manage_archive")})

	if !errors.Is(err, goncierge.ErrOutsidePrefix) || !strings.Contains(err.Error(), "plugin notes") {
		t.Errorf("declareRoles() = %v, want the prefix error naming the plugin", err)
	}
}

func TestDeclareRolesRefusesAnAddOnRegisteredBeforeItsParent(t *testing.T) {
	t.Parallel()

	parent := declaringRole("notes", "archivist", "notes.manage")
	addOn := declaringRole("archive", role.Editor, "notes.manage")

	inOrder := declareRoles(goncierge.New(), role.Core(), []sdk.Plugin{parent, addOn})
	early := declareRoles(goncierge.New(), role.Core(), []sdk.Plugin{addOn, parent})

	if inOrder != nil {
		t.Errorf("declareRoles() with the parent first = %v, want the add-on's grant accepted", inOrder)
	}
	if !errors.Is(early, goncierge.ErrOutsidePrefix) || !strings.Contains(early.Error(), "plugin archive") {
		t.Errorf("declareRoles() with the add-on first = %v, want the prefix error naming the add-on", early)
	}
}

func TestDeclareRolesLeavesTheCoreRolesForAPluginDeclaringNone(t *testing.T) {
	t.Parallel()

	registry := declared(t, &commanding{})

	core := role.Core()
	if roles := registry.Roles(); len(roles) != len(core) {
		t.Errorf("Roles() = %v, want the %d core roles alone", roles, len(core))
	}
	for _, held := range core {
		want := slices.Sorted(slices.Values(held.Capabilities))
		if got := registry.CapabilitiesOf(held.Name); !slices.Equal(got, want) {
			t.Errorf("CapabilitiesOf(%q) = %v, want the core's %v", held.Name, got, want)
		}
	}
}

func TestDeclareRolesRefusesACoreTableNoRoleAdministers(t *testing.T) {
	t.Parallel()

	core := []goncierge.Role{{Name: role.Admin, Capabilities: []string{string(role.ManageThemes)}}}

	err := declareRoles(goncierge.New(), core, nil)

	if err == nil || !strings.Contains(err.Error(), string(role.ManageUsers)) {
		t.Errorf("declareRoles() = %v, want an error naming the capability no role carries", err)
	}
}

func TestDeclareRolesRefusesACoreTableTheRegistryRefuses(t *testing.T) {
	t.Parallel()

	err := declareRoles(goncierge.New(), []goncierge.Role{{Name: "Admin"}}, nil)

	if !errors.Is(err, goncierge.ErrInvalidName) {
		t.Errorf("declareRoles() = %v, want the registry's name error", err)
	}
}

func TestComposeHoldsTheRolesThePluginsDeclare(t *testing.T) {
	t.Parallel()

	built, err := compose(composeConfig{databaseURL: emptyDatabaseURL(t), fieldDepth: 4},
		listOf(declaringRole("notes", "archivist", "notes.manage")))
	if err != nil {
		t.Fatalf("compose() error = %v, want the plugin's role declared", err)
	}
	t.Cleanup(built.close)

	if !built.roles.Can("archivist", "notes.manage") || !built.roles.Can(role.Admin, string(role.ManageUsers)) {
		t.Error("the composed registry lacks the plugin's role or the core's, want both")
	}
}

func TestComposeKeepsTheRegistryItIsHanded(t *testing.T) {
	t.Parallel()

	handed := goncierge.New()

	built, err := compose(composeConfig{databaseURL: emptyDatabaseURL(t), fieldDepth: 4, roles: handed},
		noPlugins)
	if err != nil {
		t.Fatalf("compose() error = %v, want the core's roles declared", err)
	}
	t.Cleanup(built.close)

	if built.roles != handed || !handed.Known(role.Admin) {
		t.Error("compose declared into another registry, want the one it was handed")
	}
}

func TestComposeRefusesAPluginDeclarationTheRulesRefuse(t *testing.T) {
	t.Parallel()

	_, err := compose(composeConfig{databaseURL: emptyDatabaseURL(t), fieldDepth: 4},
		listOf(declaringRole("notes", "archivist", "manage_archive")))

	if !errors.Is(err, goncierge.ErrOutsidePrefix) || !strings.Contains(err.Error(), "plugin notes") {
		t.Errorf("compose() = %v, want the prefix error naming the plugin", err)
	}
}

// composedWithDefaultRole returns the server settings composed over the plugins while the setting names named.
func composedWithDefaultRole(t *testing.T, named string, plugins registrations) (server.Config, error) {
	t.Helper()
	getenv := testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_DEFAULT_ROLE": named,
	})
	settings, err := loadRunConfig(getenv)
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	built, err := compose(composeOf(settings, getenv), plugins)
	if err != nil {
		return server.Config{}, err
	}
	t.Cleanup(built.close)
	return configOver(settings, built), nil
}

func TestComposeHandsTheServerTheDefaultRoleTheSettingNames(t *testing.T) {
	t.Parallel()

	notes := listOf(declaringRole("notes", "archivist", "notes.manage"))
	for name, asked := range map[string]struct {
		named   string
		plugins registrations
		want    string
	}{
		"nothing set":     {named: "", plugins: noPlugins, want: role.Member},
		"a core role":     {named: role.Author, plugins: noPlugins, want: role.Author},
		"a plugin's role": {named: "archivist", plugins: notes, want: "archivist"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := composedWithDefaultRole(t, asked.named, asked.plugins)

			if err != nil || cfg.DefaultRole != asked.want {
				t.Errorf("compose() = %v with DefaultRole %q, want nil and %q", err, cfg.DefaultRole, asked.want)
			}
		})
	}
}

func TestComposeStopsTheBootOnADefaultRoleNobodyDeclares(t *testing.T) {
	t.Parallel()

	_, err := composedWithDefaultRole(t, "archivist", noPlugins)

	message := fmt.Sprint(err)
	if !strings.Contains(message, "GOPHENBERG_DEFAULT_ROLE") || !strings.Contains(message, `"archivist"`) {
		t.Errorf("compose() = %v, want an error naming GOPHENBERG_DEFAULT_ROLE and the role it names", err)
	}
}

func TestComposeKeepsARegistrationFailureBesideTheDeclarationItBroke(t *testing.T) {
	t.Parallel()

	failed := errors.New("a setting is missing")
	plugins := append(listedAs("notes", func(sdk.Deps) (sdk.Plugin, error) { return nil, failed }),
		listOf(declaringRole("notes-tags", role.Editor, "notes.manage"))...)

	_, err := compose(composeConfig{databaseURL: emptyDatabaseURL(t), fieldDepth: 4}, plugins)

	if !errors.Is(err, failed) || !errors.Is(err, goncierge.ErrOutsidePrefix) {
		t.Errorf("compose() = %v, want the registration failure beside the prefix error", err)
	}
}
