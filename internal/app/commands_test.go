// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

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
		Flags:  func(fs *flag.FlagSet) { fs.String("format", "text", "how to print") },
		Writes: true, JSON: true, Capability: "manage_users",
	}}

	commands := offeredBy(t, plugin)

	if len(commands) != 1 {
		t.Fatalf("offered %d commands, want the one the plugin holds", len(commands))
	}
	held := commands[0]
	if held.Name != "commanding:show" || held.Summary != "show what was asked" ||
		!slices.Equal(held.Args, []string{"<email>"}) || !held.Writes || !held.JSON ||
		held.Capability != "manage_users" || held.Migrates {
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

	got := testkit.Run(t, Program(env, func(sdk.Deps) ([]sdk.Plugin, error) { return []sdk.Plugin{plugin}, nil }),
		"", "commanding:show")

	if got.Code != gonsole.ExitMisused || !strings.Contains(got.Stderr, "commanding:show takes no arguments") {
		t.Errorf("commanding:show = %d with stderr %q, want 2 and the plugin's message", got.Code, got.Stderr)
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

// settingValues are the texts the settings readers are compared over, malformed ones included.
var settingValues = []string{
	"", "   ", "7", " 7 ", "banana", "2.5", "0", "-3", "9223372036854775807", "99999999999999999999",
	"-99999999999999999999", "+5", "0x10", "1e3", "30s", " 1m30s ", "0s", "-1m", "soon", "true", "FALSE", "1",
	"t", "maybe", "https://example.com/feed", "10,50,100", " 10 , 50 ", "50,10", "10,10", "10,,50", "10,",
	",10", "10,banana", "10,0", "10,-5", "10,99999999999999999999",
}

// readings returns what every settings reader answers for the setting ITEMS, errors as text.
func readings(env interface {
	Value(name string) string
	Key(name string) string
	Required(name string) (string, error)
	Count(name string, fallback int) (int, error)
	Counts(name string, fallback []int) ([]int, error)
	Flag(name string, fallback bool) (bool, error)
	Duration(name string, fallback time.Duration) (time.Duration, error)
},
) []string {
	required, requiredErr := env.Required("ITEMS")
	count, countErr := env.Count("ITEMS", 20)
	counts, countsErr := env.Counts("ITEMS", []int{5})
	flagged, flagErr := env.Flag("ITEMS", true)
	waited, durationErr := env.Duration("ITEMS", 5*time.Second)
	return []string{
		env.Key("ITEMS"), env.Value("ITEMS"), required, errorText(requiredErr),
		strconv.Itoa(count), errorText(countErr), fmt.Sprintf("%#v", counts), errorText(countsErr),
		strconv.FormatBool(flagged), errorText(flagErr), waited.String(), errorText(durationErr),
	}
}

// gonsoleReader adapts the command line's settings to the readers the plugins' settings offer.
type gonsoleReader struct {
	gonsole.Env
}

// Count reads a whole number with no bound.
func (r gonsoleReader) Count(name string, fallback int) (int, error) {
	return r.Env.Count(name, fallback)
}

// Counts reads rising whole numbers with no bound.
func (r gonsoleReader) Counts(name string, fallback []int) ([]int, error) {
	return r.Env.Counts(name, fallback)
}

// Duration reads a duration with no bound.
func (r gonsoleReader) Duration(name string, fallback time.Duration) (time.Duration, error) {
	return r.Env.Duration(name, fallback)
}

func TestPluginSettingsReadAsTheCommandLineReadsThem(t *testing.T) {
	t.Parallel()

	for _, value := range settingValues {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			getenv := testkit.Getenv(map[string]string{"GOPHENBERG_FEED_ITEMS": value})

			lent := readings(sdk.Env{Prefix: settingsPrefix, Getenv: getenv}.Within("FEED_"))
			wanted := readings(gonsoleReader{settingsEnv(getenv).Within("FEED_")})

			if !slices.Equal(lent, wanted) {
				t.Errorf("plugins read %q, want %q as the command line reads it", lent, wanted)
			}
		})
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

// errorText returns the text of err, empty when it is nil.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
