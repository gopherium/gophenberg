// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/gopherium/framework/dbkit"
	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/gopherium/framework/gonsole"
)

const (
	// createOwnerName is the name of the command that sets up the role the site logs in as.
	createOwnerName = "database:create-owner"
	// siteAddressSetting is the setting that names the role the site logs in as.
	siteAddressSetting = "DATABASE_URL"
	// superuserAddressSetting is the setting that names the superuser, which only database:create-owner reads.
	superuserAddressSetting = "DATABASE_SUPERUSER_URL"
)

// errUnreadableAddress refuses an address pgx cannot read, in words that repeat none of it.
var errUnreadableAddress = fmt.Errorf("%w: pgx cannot parse the address or one of its settings", dbkit.ErrAddress)

// hostPort is one server an address tries.
type hostPort struct {
	host string
	port uint16
}

// databaseOwnerCommand returns the command that gives the role the site logs in as its database.
func databaseOwnerCommand(getenv func(string) string) gonsole.Command {
	return gonsole.Command{
		Name:    createOwnerName,
		Summary: "create the role the site logs in as and give it the database, as a superuser",
		Writes:  true,
		Run:     postgresOnly(getenv, createOwnerName, createOwner),
	}
}

// createOwner gives the role of the site's address its database, or only lists the steps on a dry run.
func createOwner(ctx context.Context, call gonsole.Call) error {
	site, superuser, err := ownerAddresses(call)
	if err != nil {
		return err
	}
	handle, err := openPostgresAt(call.Env, superuserAddressSetting, superuser)
	if err != nil {
		return err
	}
	defer func() { _ = handle.Close() }()
	owner := dbkitpg.Owner{Role: site.User, Password: site.Password}
	steps, err := ownerSteps(ctx, call.Apply, handle.DB, owner)
	if err != nil {
		return ownerFailure(call.Env, err)
	}
	_, err = io.WriteString(call.Stdout, ownerAnswer(steps))
	return err
}

// ownerAddresses returns how the site's address reads and the superuser's address, refusing two other databases.
func ownerAddresses(call gonsole.Call) (*pgx.ConnConfig, string, error) {
	address, siteErr := call.DatabaseURL()
	superuser, superuserErr := call.Env.Required(superuserAddressSetting)
	if err := errors.Join(siteErr, superuserErr); err != nil {
		return nil, "", err
	}
	site, siteErr := addressConfig(call.Env, siteAddressSetting, address)
	held, superuserErr := addressConfig(call.Env, superuserAddressSetting, superuser)
	if err := errors.Join(siteErr, superuserErr); err != nil {
		return nil, "", err
	}
	if !sameDatabase(site, held) {
		return nil, "", fmt.Errorf("%s and %s reach different databases, so name the same database on the same server",
			call.Env.Key(siteAddressSetting), call.Env.Key(superuserAddressSetting))
	}
	return site, superuser, nil
}

// sameDatabase reports whether two addresses log in to one database on the same servers.
func sameDatabase(site, held *pgx.ConnConfig) bool {
	return slices.Equal(serversOf(site), serversOf(held)) && reachedDatabase(site) == reachedDatabase(held)
}

// serversOf returns the servers config tries in order, with no server twice in a row.
func serversOf(config *pgx.ConnConfig) []hostPort {
	servers := []hostPort{{config.Host, config.Port}}
	for _, fallback := range config.Fallbacks {
		servers = append(servers, hostPort{fallback.Host, fallback.Port})
	}
	return slices.Compact(servers)
}

// reachedDatabase returns the database config logs in to, which is the user's own when the address names none.
func reachedDatabase(config *pgx.ConnConfig) string {
	if config.Database == "" {
		return config.User
	}
	return config.Database
}

// addressConfig returns how pgx reads address, blaming the setting it came from.
func addressConfig(env gonsole.Env, setting, address string) (*pgx.ConnConfig, error) {
	config, err := readAddress(address)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", env.Key(setting), err)
	}
	return config, nil
}

// readAddress returns how pgx reads address once dbkit finds no shape pgx misreads.
func readAddress(address string) (*pgx.ConnConfig, error) {
	if err := dbkitpg.CheckAddress(address); err != nil {
		return nil, err
	}
	config, err := pgx.ParseConfig(address)
	if err != nil {
		return nil, errUnreadableAddress
	}
	return config, nil
}

// ownerSteps runs the hand-over to owner on db and returns its steps, or only lists them when apply is false.
func ownerSteps(ctx context.Context, apply bool, db *sql.DB, owner dbkitpg.Owner) ([]string, error) {
	if apply {
		return dbkitpg.HandOver(ctx, db, owner)
	}
	return dbkitpg.OwnerSteps(ctx, db, owner.Role)
}

// ownerFailure returns err blaming the setting that caused it, when one did.
func ownerFailure(env gonsole.Env, err error) error {
	switch {
	case errors.Is(err, dbkitpg.ErrNotSuperuser):
		return fmt.Errorf("%s: %w", env.Key(superuserAddressSetting), err)
	case errors.Is(err, dbkitpg.ErrNotPlainRole), errors.Is(err, dbkitpg.ErrRoleName):
		return fmt.Errorf("%s: %w", env.Key(siteAddressSetting), err)
	}
	return err
}

// ownerAnswer returns the lines a run prints for steps, saying so when there are none.
func ownerAnswer(steps []string) string {
	if len(steps) == 0 {
		return "nothing to move, the role already owns the database\n"
	}
	return strings.Join(steps, "\n") + "\n"
}
