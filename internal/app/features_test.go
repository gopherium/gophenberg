// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/themehost"
	"github.com/gopherium/gophenberg/internal/version"
	"github.com/gopherium/gophenberg/sdk"
)

// operatorCommandsFeature is the feature the command line answers to.
const operatorCommandsFeature = "../../test/features/features/operator-commands.feature"

// typedPassword is the password the operator types for an account a scenario creates.
const typedPassword = "correct horse battery"

// operatorEngine is what the operator steps ask of the database engine one run of the feature keeps its data on.
type operatorEngine struct {
	empty     func(t *testing.T) string
	answers   func(ctx context.Context, address, query string, args ...any) (bool, error)
	noSchema  func(ctx context.Context, address string) error
	noContent func(ctx context.Context, address string) error
	roleHeld  string
	enabled   string
}

// postgresOperators returns the engine of the feature's run on a PostgreSQL database.
func postgresOperators() operatorEngine {
	return operatorEngine{
		empty:     emptyDatabaseURL,
		answers:   postgresAnswers,
		noSchema:  postgresHoldsNoSchema,
		noContent: postgresHoldsNoContent,
		roleHeld:  "SELECT EXISTS (SELECT FROM auth.users WHERE email = $1 AND role = $2)",
		enabled:   "SELECT EXISTS (SELECT FROM auth.users WHERE email = $1 AND NOT disabled)",
	}
}

// sqliteOperators returns the engine of the feature's run on a SQLite file.
func sqliteOperators() operatorEngine {
	return operatorEngine{
		empty:     emptySQLiteAddress,
		answers:   sqliteAnswers,
		noSchema:  untouchedSQLiteFile,
		noContent: untouchedSQLiteFile,
		roleHeld:  "SELECT EXISTS (SELECT 1 FROM auth_users WHERE email = ? AND role = ?)",
		enabled:   "SELECT EXISTS (SELECT 1 FROM auth_users WHERE email = ? AND NOT disabled)",
	}
}

// operatorScenario is what one operator scenario keeps between its steps.
type operatorScenario struct {
	t       *testing.T
	engine  operatorEngine
	env     map[string]string
	result  testkit.Result
	plugins []sdk.Plugin
	offered *commanding
	notes   *noting
}

// initializeOperatorCommands returns the binding of the operator command steps on engine, t holding their databases.
func initializeOperatorCommands(t *testing.T, engine operatorEngine) func(*godog.ScenarioContext) {
	return func(sc *godog.ScenarioContext) {
		s := &operatorScenario{t: t, engine: engine, env: map[string]string{}}
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
		sc.Then(`^the error says "([^"]*)"$`, s.errorSays)
		sc.When(`^the operator gives "([^"]*)" the role "([^"]*)" with a blank -as$`, s.giveRoleAsBlank)
		sc.Then(`^the account "([^"]*)" is still enabled$`, s.isEnabled)
		sc.Given(`^a plugin offering "([^"]*)", which needs -since$`, s.offerNeedingSince)
		sc.Given(`^a plugin offering "([^"]*)", which needs -since but declares no flags$`, s.offerNeedingUndeclared)
		sc.When(`^the operator runs "([^"]*)" with -since holding only spaces$`, s.runWithBlankSince)
		sc.Then(`^the plugin command never ran$`, s.neverRan)
		sc.Given(`^a plugin declaring the role "([^"]*)" with the capability "([^"]*)"$`, s.declareRole)
		sc.Given(`^a plugin offering "([^"]*)", which asks for "([^"]*)"$`, s.offerAsking)
		sc.Given(`^the administrator "([^"]*)" gave "([^"]*)" the role "([^"]*)"$`, s.gaveRole)
		sc.Then(`^the answer names the version, the build stamp and the theme kits of this binary$`, s.namesTheBuildFacts)
	}
}

// namesTheBuildFacts fails unless the answer is the one document that holds this binary's build facts.
func (s *operatorScenario) namesTheBuildFacts() error {
	var facts buildFacts
	if err := json.Unmarshal([]byte(s.result.Stdout), &facts); err != nil {
		return fmt.Errorf("the answer %q is no JSON document: %w", s.result.Stdout, err)
	}
	want := buildFacts{
		Schema: 1, Name: "gophenberg", Version: version.Version(),
		Commit: version.Commit(), Stamp: version.Stamp(), Kits: themehost.ServedKits(),
	}
	if !facts.equal(want) {
		return fmt.Errorf("the answer %q holds %+v, want %+v", s.result.Stdout, facts, want)
	}
	return nil
}

// declareRole has the notes plugin declare the role name carrying the capability.
func (s *operatorScenario) declareRole(name, capability string) {
	held := s.notesPlugin()
	held.roles = append(held.roles, sdk.RoleDeclaration{Name: name, Capabilities: []string{capability}})
}

// offerAsking has the notes plugin offer a writing command called name that asks for the capability.
func (s *operatorScenario) offerAsking(name, capability string) {
	s.offered = &commanding{}
	s.offered.command = sdk.Command{
		Name: name, Summary: "purge every archived note", Writes: true, Capability: capability,
		Run: s.offered.noteCall(nil),
	}
	s.notesPlugin().offered = s.offered
}

// gaveRole gives the account at email the role acting as actor, failing unless the change is applied.
func (s *operatorScenario) gaveRole(actor, email, role string) error {
	s.giveRole(email, role, actor)
	return s.succeeds()
}

// notesPlugin returns the scenario's notes plugin, offering it the first time it is asked for.
func (s *operatorScenario) notesPlugin() *noting {
	if s.notes == nil {
		s.notes = &noting{}
		s.plugins = []sdk.Plugin{s.notes}
	}
	return s.notes
}

// offerNeedingSince offers a plugin whose command called name declares and needs -since.
func (s *operatorScenario) offerNeedingSince(name string) {
	s.offer(name, func(fs *flag.FlagSet) { fs.String("since", "", "first `day` to bring back") })
}

// offerNeedingUndeclared offers a plugin whose command called name needs -since and declares no flags.
func (s *operatorScenario) offerNeedingUndeclared(name string) {
	s.offer(name, nil)
}

// offer offers a plugin holding one command called name that declares flags, needs -since and notes its run.
func (s *operatorScenario) offer(name string, flags func(*flag.FlagSet)) {
	s.offered = &commanding{}
	s.offered.command = sdk.Command{
		Name: name, Summary: "bring back what was archived", Flags: flags, Needs: []string{"since"},
		Run: s.offered.noteCall(nil),
	}
	s.notesPlugin().offered = s.offered
}

// noting is a plugin under the id notes offering the command it was given and declaring the roles it holds.
type noting struct {
	offered *commanding
	roles   []sdk.RoleDeclaration
}

// ID returns the plugin's identifier.
func (*noting) ID() string {
	return "notes"
}

// Start readies nothing.
func (*noting) Start(context.Context) error {
	return nil
}

// Stop stops nothing.
func (*noting) Stop(context.Context) error {
	return nil
}

// Commands returns the command the plugin was given, none before one is.
func (n *noting) Commands() []sdk.Command {
	if n.offered == nil {
		return nil
	}
	return n.offered.Commands()
}

// Roles returns the roles the plugin declares.
func (n *noting) Roles() []sdk.RoleDeclaration {
	return n.roles
}

// runWithBlankSince runs the command called name with -since holding only spaces.
func (s *operatorScenario) runWithBlankSince(name string) {
	s.run("", name, "-since", "  ")
}

// neverRan fails when the offered plugin command ran.
func (s *operatorScenario) neverRan() error {
	if s.offered.ran.Stdout != nil {
		return fmt.Errorf("the plugin command ran with %+v, want it never run", s.offered.ran)
	}
	return nil
}

// loaded returns the plugins the scenario offers, none until a step offers one.
func (s *operatorScenario) loaded(sdk.Deps) ([]sdk.Plugin, error) {
	return s.plugins, nil
}

// errorSays fails unless the error stream carries text.
func (s *operatorScenario) errorSays(text string) error {
	if !strings.Contains(s.result.Stderr, text) {
		return fmt.Errorf("stderr %q does not carry %q", s.result.Stderr, text)
	}
	return nil
}

// giveRoleAsBlank gives the account at email the role with -as holding only spaces.
func (s *operatorScenario) giveRoleAsBlank(email, role string) {
	s.run("", "account:role", email, role, "-yes", "-as", "  ")
}

// isEnabled fails unless the account at email is enabled.
func (s *operatorScenario) isEnabled(ctx context.Context, email string) error {
	held, err := s.answers(ctx, s.engine.enabled, email)
	if err != nil || !held {
		return fmt.Errorf("the account %s is enabled %v (%v), want it enabled", email, held, err)
	}
	return nil
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
	listed := testkit.Run(s.t, Program(testkit.Getenv(s.env), s.loaded), "", "account:records")
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
	s.env["GOPHENBERG_DATABASE_URL"] = s.engine.empty(s.t)
}

// nameNoDatabase leaves the database setting out.
func (s *operatorScenario) nameNoDatabase() {
	delete(s.env, "GOPHENBERG_DATABASE_URL")
}

// run runs the command line in process over the scenario's settings, feeding stdin.
func (s *operatorScenario) run(stdin string, args ...string) {
	s.result = testkit.Run(s.t, Program(testkit.Getenv(s.env), s.loaded), stdin, args...)
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
	return s.engine.noSchema(ctx, s.env["GOPHENBERG_DATABASE_URL"])
}

// holdsNoDemoContent fails when the database holds any content item.
func (s *operatorScenario) holdsNoDemoContent(ctx context.Context) error {
	return s.engine.noContent(ctx, s.env["GOPHENBERG_DATABASE_URL"])
}

// holdsRole fails unless the account at email holds the role.
func (s *operatorScenario) holdsRole(ctx context.Context, email, role string) error {
	held, err := s.answers(ctx, s.engine.roleHeld, email, role)
	if err != nil || !held {
		return fmt.Errorf("the account %s holds the role %s %v (%v), want it to", email, role, held, err)
	}
	return nil
}

// answers returns what the yes or no query answers on the scenario's database.
func (s *operatorScenario) answers(ctx context.Context, query string, args ...any) (bool, error) {
	return s.engine.answers(ctx, s.env["GOPHENBERG_DATABASE_URL"], query, args...)
}

// postgresHoldsNoSchema fails when the PostgreSQL database at address holds any schema Gophenberg migrates.
func postgresHoldsNoSchema(ctx context.Context, address string) error {
	for _, schema := range []string{"auth", "core", "gonsole"} {
		held, err := postgresAnswers(ctx, address, "SELECT to_regnamespace($1) IS NOT NULL", schema)
		if err != nil || held {
			return fmt.Errorf("the %s schema is held %v (%v), want no schema", schema, held, err)
		}
	}
	return nil
}

// postgresHoldsNoContent fails when the PostgreSQL database at address holds any content item.
func postgresHoldsNoContent(ctx context.Context, address string) error {
	table, err := postgresAnswers(ctx, address, "SELECT to_regclass('core.content') IS NOT NULL")
	if err != nil || !table {
		return err
	}
	stored, err := postgresAnswers(ctx, address, "SELECT EXISTS (SELECT FROM core.content)")
	if err != nil || stored {
		return fmt.Errorf("the database holds content %v (%v), want none", stored, err)
	}
	return nil
}

// postgresAnswers returns what the yes or no query answers on the PostgreSQL database at address.
func postgresAnswers(ctx context.Context, address, query string, args ...any) (bool, error) {
	pool, err := pgxpool.New(ctx, address)
	if err != nil {
		return false, err
	}
	defer pool.Close()
	var held bool
	err = pool.QueryRow(ctx, query, args...).Scan(&held)
	return held, err
}

// emptySQLiteAddress returns the sqlite: address of a new empty file, alone in a folder of its own.
func emptySQLiteAddress(t *testing.T) string {
	t.Helper()
	return sqlitePrefix + emptySQLiteFile(t)
}

// sqliteAnswers returns what the yes or no query answers on the SQLite file at address, closing its handle after.
func sqliteAnswers(ctx context.Context, address, query string, args ...any) (bool, error) {
	db, err := openDefaultSQLite(address)
	if err != nil {
		return false, err
	}
	var held bool
	err = db.QueryRowContext(ctx, query, args...).Scan(&held)
	return held, errors.Join(err, db.Close())
}

// openDefaultSQLite returns a handle on the SQLite file at address under the default SQLite settings.
func openDefaultSQLite(address string) (*sql.DB, error) {
	settings, err := sqlite.SettingsFrom(settingsEnv(testkit.Getenv(nil)))
	if err != nil {
		return nil, err
	}
	return sqlite.Open(address, settings)
}

// untouchedSQLiteFile fails unless the SQLite file at address still holds no byte, never opening it.
func untouchedSQLiteFile(_ context.Context, address string) error {
	info, err := os.Stat(strings.TrimPrefix(address, sqlitePrefix))
	if err != nil {
		return err
	}
	if info.Size() != 0 {
		return fmt.Errorf("the SQLite file holds %d bytes, want 0 since no command opened it", info.Size())
	}
	return nil
}

func TestOperatorCommands(t *testing.T) {
	t.Parallel()

	tags := "~@wip"
	if os.Getenv("GOPHENBERG_BDD_WIP") != "" {
		tags = ""
	}
	for name, engine := range map[string]operatorEngine{"postgres": postgresOperators(), "sqlite": sqliteOperators()} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			suite := godog.TestSuite{
				ScenarioInitializer: initializeOperatorCommands(t, engine),
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
		})
	}
}
