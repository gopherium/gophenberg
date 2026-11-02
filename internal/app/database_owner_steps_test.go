// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/jackc/pgx/v5"

	"github.com/gopherium/gophenberg/internal/testdb"
)

const (
	// databaseOwnerFeature is the feature database:create-owner answers to.
	databaseOwnerFeature = "../../test/features/features/database-owner.feature"
	// ownerFirstPassword is the password a scenario's role starts with.
	ownerFirstPassword = "first-role-password"
	// ownerSecondPassword is the password a scenario's role moves to.
	ownerSecondPassword = "second-role-password"
	// ownerLeftOutQuery lists what the role $1 does not own in the database of the connection.
	ownerLeftOutQuery = `
SELECT 'schema ' || nspname FROM pg_namespace
WHERE nspname !~ '^pg_' AND nspname NOT IN ('information_schema', 'public') AND pg_get_userbyid(nspowner) <> $1
UNION ALL
SELECT 'relation ' || relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE nspname !~ '^pg_' AND nspname <> 'information_schema' AND pg_get_userbyid(relowner) <> $1
UNION ALL
SELECT 'database ' || datname FROM pg_database WHERE datname = current_database() AND pg_get_userbyid(datdba) <> $1`
)

// ownerScenario is what one database owner scenario keeps beside the operator scenario.
type ownerScenario struct {
	*operatorScenario
	address string
	plain   string
	role    string
}

// initializeDatabaseOwner binds the steps of the database:create-owner scenarios to s.
func initializeDatabaseOwner(sc *godog.ScenarioContext, s *operatorScenario) {
	o := &ownerScenario{operatorScenario: s}
	sc.After(o.dropRole)
	sc.Given(`^a database whose tables a superuser owns$`, o.migrateAsSuperuser)
	sc.Given(`^the settings name a role the server does not have yet$`, o.nameNewRole)
	sc.Given(`^the settings name the superuser's address$`, o.nameSuperuser)
	sc.Given(`^the settings name no superuser address$`, o.nameNoSuperuser)
	sc.Given(`^the superuser's address names a plain role$`, o.nameRoleAsSuperuser)
	sc.Given(`^the superuser's address names another database$`, o.nameOtherDatabase)
	sc.Given(`^the superuser's address names another server$`, o.nameOtherServer)
	sc.Given(`^the settings name the superuser as the site's login$`, o.logInAsSuperuser)
	sc.Given(`^the server has the role, which can create roles$`, o.holdRoleThatCreatesRoles)
	sc.Given(`^the server has the role, which runs programs on the server$`, o.holdRoleThatRunsPrograms)
	sc.Given(`^the server has the role, which belongs to a superuser$`, o.holdRoleInASuperuser)
	sc.Given(`^the role can log in$`, o.letRoleLogIn)
	sc.Given(`^the settings name a new password for the role$`, o.nameNewPassword)
	sc.Given(`^the settings point at a SQLite database$`, o.pointAtSQLite)
	sc.Given(`^(GOPHENBERG_[A-Z_]+) is "([^"]*)"$`, o.setSetting)
	sc.Then(`^the answer lists the step that creates the role$`, o.listsRoleStep)
	sc.Then(`^the server has no such role$`, o.hasNoRole)
	sc.Then(`^the role owns the database and everything inside it$`, o.ownsEverything)
	sc.Then(`^the role logs in with the password the settings name$`, o.logsIn)
	sc.Then(`^the first password no longer logs in$`, o.firstPasswordFails)
	sc.Then(`^the answer says there was nothing to move$`, o.saysNothingToMove)
	sc.Then(`^neither answer holds a password$`, o.holdsNoPassword)
}

// migrateAsSuperuser fills a fresh database as the test server's own user, so a superuser owns its tables.
func (o *ownerScenario) migrateAsSuperuser() error {
	o.plain = o.engine.empty(o.t)
	o.address = testdb.AsServer(o.t, o.plain)
	o.env["GOPHENBERG_DATABASE_URL"] = o.address
	o.run("", "migrate")
	if o.result.Code != 0 {
		return fmt.Errorf("migrate as the superuser exited %d: %s", o.result.Code, o.result.Stderr)
	}
	return nil
}

// addressAs returns the scenario's address logging in as role with password.
func (o *ownerScenario) addressAs(role, password string) (string, error) {
	parsed, err := url.Parse(o.address)
	if err != nil {
		return "", err
	}
	parsed.User = url.UserPassword(role, password)
	return parsed.String(), nil
}

// nameNewRole points the site's address at a role name no role holds yet.
func (o *ownerScenario) nameNewRole() error {
	o.role = "site_owner_" + strings.ToLower(rand.Text())
	address, err := o.addressAs(o.role, ownerFirstPassword)
	o.env["GOPHENBERG_DATABASE_URL"] = address
	return err
}

// nameSuperuser sets the superuser address to the test server's own user.
func (o *ownerScenario) nameSuperuser() {
	o.env["GOPHENBERG_DATABASE_SUPERUSER_URL"] = o.address
}

// nameNoSuperuser leaves the superuser address out.
func (o *ownerScenario) nameNoSuperuser() {
	delete(o.env, "GOPHENBERG_DATABASE_SUPERUSER_URL")
}

// nameRoleAsSuperuser points the superuser address at the plain role the test database belongs to.
func (o *ownerScenario) nameRoleAsSuperuser() {
	o.env["GOPHENBERG_DATABASE_SUPERUSER_URL"] = o.plain
}

// nameOtherDatabase points the superuser address at a second fresh database in place of the site's.
func (o *ownerScenario) nameOtherDatabase() {
	o.env["GOPHENBERG_DATABASE_SUPERUSER_URL"] = testdb.AsServer(o.t, o.engine.empty(o.t))
}

// nameOtherServer points the superuser address at the same database on another port.
func (o *ownerScenario) nameOtherServer() error {
	parsed, err := url.Parse(o.address)
	if err != nil {
		return err
	}
	parsed.Host = parsed.Hostname() + ":1"
	o.env["GOPHENBERG_DATABASE_SUPERUSER_URL"] = parsed.String()
	return nil
}

// logInAsSuperuser points the site's address at the superuser.
func (o *ownerScenario) logInAsSuperuser() {
	o.env["GOPHENBERG_DATABASE_URL"] = o.address
}

// holdRoleThatCreatesRoles makes the scenario's role on the server, able to create roles.
func (o *ownerScenario) holdRoleThatCreatesRoles(ctx context.Context) error {
	return o.exec(ctx, "CREATE ROLE "+pgx.Identifier{o.role}.Sanitize()+" CREATEROLE")
}

// holdRoleThatRunsPrograms makes the scenario's role on the server, a member of the role that runs server programs.
func (o *ownerScenario) holdRoleThatRunsPrograms(ctx context.Context) error {
	name := pgx.Identifier{o.role}.Sanitize()
	return errors.Join(o.exec(ctx, "CREATE ROLE "+name), o.exec(ctx, "GRANT pg_execute_server_program TO "+name))
}

// holdRoleInASuperuser makes the scenario's role on the server, a member of the superuser.
func (o *ownerScenario) holdRoleInASuperuser(ctx context.Context) error {
	super, err := url.Parse(o.address)
	if err != nil {
		return err
	}
	name := pgx.Identifier{o.role}.Sanitize()
	granted := "GRANT " + pgx.Identifier{super.User.Username()}.Sanitize() + " TO " + name
	return errors.Join(o.exec(ctx, "CREATE ROLE "+name), o.exec(ctx, granted))
}

// letRoleLogIn lets the scenario's role log in with the first password.
func (o *ownerScenario) letRoleLogIn(ctx context.Context) error {
	return o.exec(ctx, "ALTER ROLE "+pgx.Identifier{o.role}.Sanitize()+" LOGIN PASSWORD '"+ownerFirstPassword+"'")
}

// nameNewPassword changes the password of the site's address.
func (o *ownerScenario) nameNewPassword() error {
	address, err := o.addressAs(o.role, ownerSecondPassword)
	o.env["GOPHENBERG_DATABASE_URL"] = address
	return err
}

// pointAtSQLite points the site's address at a fresh SQLite file.
func (o *ownerScenario) pointAtSQLite() {
	o.env["GOPHENBERG_DATABASE_URL"] = emptySQLiteAddress(o.t)
}

// setSetting sets one setting by its full name.
func (o *ownerScenario) setSetting(name, value string) {
	o.env[name] = value
}

// exec runs statement on the scenario's database as the superuser.
func (o *ownerScenario) exec(ctx context.Context, statement string) error {
	conn, err := pgx.Connect(ctx, o.address)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(context.Background()) }()
	_, err = conn.Exec(ctx, statement)
	return err
}

// dropRole gives what the scenario's role owns back to the superuser and drops it.
func (o *ownerScenario) dropRole(ctx context.Context, _ *godog.Scenario, failed error) (context.Context, error) {
	if o.role == "" {
		return ctx, failed
	}
	held, err := o.roleExists(ctx)
	if err != nil || !held {
		return ctx, errors.Join(failed, err)
	}
	super, err := url.Parse(o.address)
	if err != nil {
		return ctx, errors.Join(failed, err)
	}
	name := pgx.Identifier{o.role}.Sanitize()
	given := o.exec(ctx, "REASSIGN OWNED BY "+name+" TO "+super.User.Username())
	dropped := o.exec(ctx, "DROP ROLE "+name)
	return ctx, errors.Join(failed, given, dropped)
}

// roleExists reports whether the server has the scenario's role.
func (o *ownerScenario) roleExists(ctx context.Context) (bool, error) {
	return postgresAnswers(ctx, o.address, "SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = $1)", o.role)
}

// listsRoleStep fails unless the answer lists the statement that creates the scenario's role.
func (o *ownerScenario) listsRoleStep() error {
	if step := "CREATE ROLE " + o.role + " LOGIN"; !strings.Contains(o.result.Stdout, step) {
		return fmt.Errorf("the answer %q does not list %q", o.result.Stdout, step)
	}
	return nil
}

// hasNoRole fails unless the server has no role of the scenario's name.
func (o *ownerScenario) hasNoRole(ctx context.Context) error {
	held, err := o.roleExists(ctx)
	if err != nil {
		return err
	}
	if held {
		return fmt.Errorf("the server has the role %q, want nothing changed", o.role)
	}
	return nil
}

// ownsEverything fails unless the database and every schema, table and sequence inside it belong to the role.
func (o *ownerScenario) ownsEverything(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, o.address)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(context.Background()) }()
	rows, err := conn.Query(ctx, ownerLeftOutQuery, o.role)
	if err != nil {
		return err
	}
	left, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(left) != 0 {
		return fmt.Errorf("the role does not own %q", left)
	}
	return nil
}

// logsIn fails unless the site's address logs in.
func (o *ownerScenario) logsIn(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, o.env["GOPHENBERG_DATABASE_URL"])
	if err != nil {
		return err
	}
	return conn.Close(ctx)
}

// firstPasswordFails fails when the first password of the role still logs in.
func (o *ownerScenario) firstPasswordFails(ctx context.Context) error {
	address, err := o.addressAs(o.role, ownerFirstPassword)
	if err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, address)
	if err == nil {
		_ = conn.Close(ctx)
		return errors.New("the first password still logs in, want it replaced")
	}
	return nil
}

// saysNothingToMove fails unless the answer says the role already owned everything.
func (o *ownerScenario) saysNothingToMove() error {
	if !strings.Contains(o.result.Stdout, "nothing to move") {
		return fmt.Errorf("the answer %q does not say there was nothing to move", o.result.Stdout)
	}
	return nil
}

// holdsNoPassword fails when the answer or the error holds the role's password or the superuser's login.
func (o *ownerScenario) holdsNoPassword() error {
	super, err := url.Parse(o.address)
	if err != nil {
		return err
	}
	for _, held := range []string{ownerFirstPassword, super.User.String()} {
		if strings.Contains(o.result.Stdout+o.result.Stderr, held) {
			return fmt.Errorf("the run printed the password %q", held)
		}
	}
	return nil
}

func TestDatabaseOwner(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, databaseOwnerFeature, postgresOperators())
}
