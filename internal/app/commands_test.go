// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/sdk"
)

// commanding is a plugin offering the one command it holds and noting the call that command ran with.
type commanding struct {
	command sdk.Command
	ran     sdk.Call
}

// ID returns the plugin's identifier.
func (*commanding) ID() string {
	return "commanding"
}

// Start readies nothing.
func (*commanding) Start(context.Context) error {
	return nil
}

// Stop stops nothing.
func (*commanding) Stop(context.Context) error {
	return nil
}

// Commands returns the command the plugin holds.
func (c *commanding) Commands() []sdk.Command {
	return []sdk.Command{c.command}
}

// noteCall returns a command run that keeps the call it is given in c and answers err.
func (c *commanding) noteCall(err error) func(context.Context, sdk.Call) error {
	return func(_ context.Context, call sdk.Call) error {
		c.ran = call
		return err
	}
}

// offeredBy returns the commands the command line walks for the plugins.
func offeredBy(t *testing.T, plugins ...sdk.Plugin) []gonsole.Command {
	t.Helper()
	groups, err := gonsole.Walk(offerings(plugins))
	if err != nil {
		t.Fatalf("Walk() error = %v, want nil", err)
	}
	var commands []gonsole.Command
	for _, group := range groups {
		commands = append(commands, group.Commands...)
	}
	return commands
}

func TestAPluginCommandReachesTheCommandLineWithEveryField(t *testing.T) {
	t.Parallel()

	plugin := &commanding{command: sdk.Command{
		Name: "commanding:show", Summary: "show what was asked", Args: []string{"<email>"},
		Flags: func(fs *flag.FlagSet) { fs.String("format", "text", "how to print") },
		Needs: []string{"format"}, Writes: true, JSON: true, Capability: "manage_users",
	}}

	commands := offeredBy(t, plugin)

	if len(commands) != 1 {
		t.Fatalf("offered %d commands, want the one the plugin holds", len(commands))
	}
	held := commands[0]
	if held.Name != "commanding:show" || held.Summary != "show what was asked" ||
		!slices.Equal(held.Args, []string{"<email>"}) || !slices.Equal(held.Needs, []string{"format"}) ||
		!held.Writes || !held.JSON || held.Capability != "manage_users" || held.Migrates {
		t.Errorf("command = %+v, want every field the plugin set and Migrates left off", held)
	}
	flags := flag.NewFlagSet("commanding:show", flag.ContinueOnError)
	held.Flags(flags)
	if flags.Lookup("format") == nil {
		t.Error("Flags() set no format flag, want the plugin's own flags")
	}
}

func TestAPluginCommandRunsWithTheCallTheCommandLineGives(t *testing.T) {
	t.Parallel()

	plugin := &commanding{}
	plugin.command = sdk.Command{Name: "commanding:show", Run: plugin.noteCall(nil)}
	var stdout, stderr bytes.Buffer
	given := gonsole.Call{
		Args: []string{"maria@example.com"}, Flags: map[string]string{"format": "json"},
		Stdin: strings.NewReader("yes\n"), Stdout: &stdout, Stderr: &stderr,
		JSON: true, Apply: true, Actor: "admin@example.com",
	}

	err := offeredBy(t, plugin)[0].Run(t.Context(), given)

	ran := plugin.ran
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !slices.Equal(ran.Args, given.Args) || ran.Flags["format"] != "json" || ran.Stdin != given.Stdin ||
		ran.Stdout != given.Stdout || ran.Stderr != given.Stderr || !ran.JSON || !ran.Apply ||
		ran.Actor != "admin@example.com" {
		t.Errorf("the plugin ran with %+v, want every part of the call it was given", ran)
	}
}

func TestAPluginCommandAnswersItsOwnError(t *testing.T) {
	t.Parallel()

	refused := errors.New("the reservations could not be read")
	plugin := &commanding{}
	plugin.command = sdk.Command{Name: "commanding:show", Run: plugin.noteCall(refused)}

	err := offeredBy(t, plugin)[0].Run(t.Context(), gonsole.Call{})

	if !errors.Is(err, refused) || errors.Is(err, gonsole.ErrMisused) {
		t.Errorf("Run() error = %v, want the plugin's own error, not a misused command line", err)
	}
}

func TestAPluginCommandMisuseExitsAsAMisusedCommandLine(t *testing.T) {
	t.Parallel()

	plugin := &commanding{}
	plugin.command = sdk.Command{
		Name: "commanding:show", Summary: "show what was asked",
		Run: plugin.noteCall(sdk.Misuse(errors.New("commanding:show takes no arguments"))),
	}
	env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL})

	got := testkit.Run(t, Program(env, listOf(plugin)),
		"", "commanding:show")

	if got.Code != gonsole.ExitMisused || !strings.Contains(got.Stderr, "commanding:show takes no arguments") {
		t.Errorf("commanding:show = %d with stderr %q, want 2 and the plugin's message", got.Code, got.Stderr)
	}
}

func TestAPluginCommandWithoutANeededFlagExitsAsAMisusedCommandLine(t *testing.T) {
	t.Parallel()

	for name, args := range map[string][]string{
		"left out":    {"commanding:show"},
		"only spaces": {"commanding:show", "-since", "  "},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			plugin := &commanding{}
			plugin.command = sdk.Command{
				Name: "commanding:show", Summary: "show what was asked",
				Flags: func(fs *flag.FlagSet) { fs.String("since", "", "first `day` to bring back") },
				Needs: []string{"since"}, Run: plugin.noteCall(nil),
			}
			env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL})

			got := testkit.Run(t, Program(env, listOf(plugin)),
				"", args...)

			if got.Code != gonsole.ExitMisused || !strings.Contains(got.Stderr, "commanding:show wants -since <day>") ||
				plugin.ran.Stdout != nil {
				t.Errorf("%q = %d with stderr %q, want 2, the needed flag named and the command never run",
					args, got.Code, got.Stderr)
			}
		})
	}
}

func TestCheckNamesAPluginCommandNeedingAFlagItDoesNotDeclare(t *testing.T) {
	t.Parallel()

	plugin := &commanding{}
	plugin.command = sdk.Command{
		Name: "commanding:show", Summary: "show what was asked", Needs: []string{"since"}, Run: plugin.noteCall(nil),
	}
	env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL})

	got := testkit.Run(t, Program(env, listOf(plugin)),
		"", "check")

	if got.Code != gonsole.ExitFailed ||
		!strings.Contains(got.Stderr, `command "commanding:show" needs -since, which it does not declare`) {
		t.Errorf("check = %d with stderr %q, want 1 and the undeclared needed flag named", got.Code, got.Stderr)
	}
}

func TestAPluginCommandWithoutARunStaysWithoutOne(t *testing.T) {
	t.Parallel()

	plugin := &commanding{command: sdk.Command{Name: "commanding:show"}}

	if run := offeredBy(t, plugin)[0].Run; run != nil {
		t.Error("Run is set for a command the plugin gave none, want it left nil as the plugin wrote it")
	}
}

func TestAPluginWithoutCommandsOffersNone(t *testing.T) {
	t.Parallel()

	if commands := offeredBy(t, failingPlugin{}); len(commands) != 0 {
		t.Errorf("offered %v, want no command from a plugin that offers none", commands)
	}
}

func TestAPluginCommandEncodesAsTheCommandLineEncodes(t *testing.T) {
	t.Parallel()

	document := map[string]any{"title": "Tom & Jerry <live>", "seats": 3, "venues": []string{"Hall A"}}
	var lent, wanted strings.Builder

	lentErr := sdk.Call{Stdout: &lent}.Encode(document)
	wantedErr := gonsole.Call{Stdout: &wanted}.Encode(document)

	if lentErr != nil || wantedErr != nil || lent.String() != wanted.String() {
		t.Errorf("a plugin encodes %q, %v, want %q, %v as the command line encodes it",
			lent.String(), lentErr, wanted.String(), wantedErr)
	}
}
