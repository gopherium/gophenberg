// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/jackc/pgx/v5/pgxpool"
)

// operatorCommandsFeature is the feature the command line answers to.
const operatorCommandsFeature = "../../test/features/features/operator-commands.feature"

// typedPassword is the password the operator types for an account a scenario creates.
const typedPassword = "correct horse battery"

// operatorScenario is what one operator scenario keeps between its steps.
type operatorScenario struct {
	t      *testing.T
	env    map[string]string
	result testkit.Result
}

// initializeOperatorCommands returns the binding of the operator command steps, t holding their databases.
func initializeOperatorCommands(t *testing.T) func(*godog.ScenarioContext) {
	return func(sc *godog.ScenarioContext) {
		s := &operatorScenario{t: t, env: map[string]string{}}
		sc.Given(`^the settings point at an empty database$`, s.pointAtAnEmptyDatabase)
		sc.Given(`^the settings name no database$`, s.nameNoDatabase)
		sc.When(`^the operator runs gophenberg with no command$`, s.runWithNoCommand)
		sc.When(`^the operator runs "([^"]*)"$`, s.runLine)
		sc.When(`^the operator asks for the help page of "([^"]*)"$`, s.askForHelp)
		sc.When(`^the operator creates the administrator "([^"]*)" with a password on standard input$`,
			s.createAdministrator)
		sc.Then(`^the command succeeds$`, s.succeeds)
		sc.Then(`^the command exits with code (\d+)$`, s.exitsWith)
		sc.Then(`^the answer lists the commands "([^"]*)", "([^"]*)", "([^"]*)" and "([^"]*)"$`, s.listsCommands)
		sc.Then(`^the answer names the unknown command "([^"]*)"$`, s.namesUnknownCommand)
		sc.Then(`^the answer describes "([^"]*)"$`, s.describes)
		sc.Then(`^the answer says nothing changed until it is confirmed$`, s.saysNothingChanged)
		sc.Then(`^the database holds no schema$`, s.holdsNoSchema)
		sc.Then(`^the database holds no demo content$`, s.holdsNoDemoContent)
		sc.Then(`^the account "([^"]*)" holds the role "([^"]*)"$`, s.holdsRole)
		sc.Given(`^the (administrator|author|editor) "([^"]*)"$`, s.holdAccount)
		sc.When(`^the operator gives "([^"]*)" the role "([^"]*)" acting as "([^"]*)"$`, s.giveRole)
		sc.When(`^the operator previews giving "([^"]*)" the role "([^"]*)" acting as "([^"]*)"$`, s.previewRole)
		sc.Then(`^the account "([^"]*)" still holds the role "([^"]*)"$`, s.holdsRole)
		sc.Then(`^no account change is on record$`, s.recordsNothing)
		sc.Then(`^the records list "([^"]*)" applied by "([^"]*)"$`, s.recordsChange)
	}
}

// holdAccount creates the account at email under the role a scenario calls kind.
func (s *operatorScenario) holdAccount(kind, email string) error {
	held := map[string]string{"administrator": "admin", "author": "author", "editor": "editor"}[kind]
	s.run(typedPassword+"\n", "account:create-admin", "-email", email, "-name", "Holder", "-role", held)
	return s.succeeds()
}

// giveRole gives the account at email the role, acting as actor when one is named.
func (s *operatorScenario) giveRole(email, role, actor string) {
	s.changeRole(email, role, actor, "-yes")
}

// previewRole previews giving the account at email the role, acting as actor.
func (s *operatorScenario) previewRole(email, role, actor string) {
	s.changeRole(email, role, actor)
}

// changeRole runs account:role for the account at email, adding -as actor when one is named.
func (s *operatorScenario) changeRole(email, role, actor string, flags ...string) {
	args := append([]string{"account:role", email, role}, flags...)
	if actor != "" {
		args = append(args, "-as", actor)
	}
	s.run("", args...)
}

// records returns the lines account:records lists.
func (s *operatorScenario) records() ([]string, error) {
	listed := testkit.Run(s.t, Program(testkit.Getenv(s.env), noPlugins), "", "account:records")
	if listed.Code != 0 {
		return nil, fmt.Errorf("account:records exited with %d and stderr %q", listed.Code, listed.Stderr)
	}
	return strings.FieldsFunc(listed.Stdout, func(r rune) bool { return r == '\n' }), nil
}

// recordsNothing fails when any account change is on record.
func (s *operatorScenario) recordsNothing() error {
	held, err := s.records()
	if err != nil || len(held) != 0 {
		return fmt.Errorf("records %q (%v), want none", held, err)
	}
	return nil
}

// recordsChange fails unless the records list the command applied by actor.
func (s *operatorScenario) recordsChange(command, actor string) error {
	held, err := s.records()
	if err != nil || len(held) != 1 || !strings.Contains(held[0], actor+"  "+command) {
		return fmt.Errorf("records %q (%v), want the one %s %s applied", held, err, command, actor)
	}
	return nil
}

// pointAtAnEmptyDatabase points the settings at a fresh database holding no schema.
func (s *operatorScenario) pointAtAnEmptyDatabase() {
	s.env["GOPHENBERG_DATABASE_URL"] = emptyDatabaseURL(s.t)
}

// nameNoDatabase leaves the database setting out.
func (s *operatorScenario) nameNoDatabase() {
	delete(s.env, "GOPHENBERG_DATABASE_URL")
}

// run runs the command line in process over the scenario's settings, feeding stdin.
func (s *operatorScenario) run(stdin string, args ...string) {
	s.result = testkit.Run(s.t, Program(testkit.Getenv(s.env), noPlugins), stdin, args...)
}

// runWithNoCommand runs the command line naming no command.
func (s *operatorScenario) runWithNoCommand() {
	s.run("")
}

// runLine runs the command line split into words.
func (s *operatorScenario) runLine(line string) {
	s.run("", strings.Fields(line)...)
}

// askForHelp asks for the help page of the command called name.
func (s *operatorScenario) askForHelp(name string) {
	s.run("", "help", name)
}

// createAdministrator creates an administrator at email, typing its password on standard input.
func (s *operatorScenario) createAdministrator(email string) {
	s.run(typedPassword+"\n", "account:create-admin", "-email", email, "-name", "Administrator", "-role", "admin")
}

// succeeds fails unless the command exited with code 0.
func (s *operatorScenario) succeeds() error {
	return s.exitsWith(0)
}

// exitsWith fails unless the command exited with code.
func (s *operatorScenario) exitsWith(code int) error {
	if s.result.Code != code {
		return fmt.Errorf("the command exited with %d and stderr %q, want %d", s.result.Code, s.result.Stderr, code)
	}
	return nil
}

// listsCommands fails unless the listing holds a line for each command named.
func (s *operatorScenario) listsCommands(first, second, third, fourth string) error {
	for _, name := range []string{first, second, third, fourth} {
		if !strings.Contains(s.result.Stdout, "\n  "+name+" ") {
			return fmt.Errorf("the listing %q holds no line for %s", s.result.Stdout, name)
		}
	}
	return nil
}

// namesUnknownCommand fails unless the refusal names the command called name as unknown.
func (s *operatorScenario) namesUnknownCommand(name string) error {
	if !strings.Contains(s.result.Stderr, `unknown command "`+name+`"`) {
		return fmt.Errorf("stderr %q does not name the unknown command %q", s.result.Stderr, name)
	}
	return nil
}

// describes fails unless the answer is the help page of the command called name.
func (s *operatorScenario) describes(name string) error {
	if !strings.Contains(s.result.Stdout, "gophenberg "+name) {
		return fmt.Errorf("stdout %q is no help page of %s", s.result.Stdout, name)
	}
	return nil
}

// saysNothingChanged fails unless the command reported a dry run.
func (s *operatorScenario) saysNothingChanged() error {
	if !strings.Contains(s.result.Stderr, "dry run, nothing changed, pass -yes to apply") {
		return fmt.Errorf("stderr %q reports no dry run", s.result.Stderr)
	}
	return nil
}

// holdsNoSchema fails when the database holds any schema Gophenberg migrates.
func (s *operatorScenario) holdsNoSchema(ctx context.Context) error {
	for _, schema := range []string{"auth", "core", "gonsole"} {
		held, err := s.answers(ctx, "SELECT to_regnamespace($1) IS NOT NULL", schema)
		if err != nil || held {
			return fmt.Errorf("the %s schema is held %v (%v), want no schema", schema, held, err)
		}
	}
	return nil
}

// holdsNoDemoContent fails when the database holds any content item.
func (s *operatorScenario) holdsNoDemoContent(ctx context.Context) error {
	table, err := s.answers(ctx, "SELECT to_regclass('core.content') IS NOT NULL")
	if err != nil || !table {
		return err
	}
	stored, err := s.answers(ctx, "SELECT EXISTS (SELECT FROM core.content)")
	if err != nil || stored {
		return fmt.Errorf("the database holds content %v (%v), want none", stored, err)
	}
	return nil
}

// holdsRole fails unless the account at email holds the role.
func (s *operatorScenario) holdsRole(ctx context.Context, email, role string) error {
	held, err := s.answers(ctx, "SELECT EXISTS (SELECT FROM auth.users WHERE email = $1 AND role = $2)", email, role)
	if err != nil || !held {
		return fmt.Errorf("the account %s holds the role %s %v (%v), want it to", email, role, held, err)
	}
	return nil
}

// answers returns what the yes or no query answers on the scenario's database.
func (s *operatorScenario) answers(ctx context.Context, query string, args ...any) (bool, error) {
	pool, err := pgxpool.New(ctx, s.env["GOPHENBERG_DATABASE_URL"])
	if err != nil {
		return false, err
	}
	defer pool.Close()
	var held bool
	err = pool.QueryRow(ctx, query, args...).Scan(&held)
	return held, err
}

func TestOperatorCommands(t *testing.T) {
	tags := "~@wip"
	if os.Getenv("GOPHENBERG_BDD_WIP") != "" {
		tags = ""
	}
	suite := godog.TestSuite{
		ScenarioInitializer: initializeOperatorCommands(t),
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{operatorCommandsFeature},
			Tags:     tags,
			Strict:   true,
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Error("the feature did not pass")
	}
}
