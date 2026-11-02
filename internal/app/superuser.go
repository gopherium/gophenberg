// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/gopherium/framework/gonsole"
)

// superuserPolicy is what serve does when it connects to PostgreSQL as a role that can reach the database server.
type superuserPolicy string

const (
	// superuserWarn logs a warning and serves.
	superuserWarn superuserPolicy = "warn"
	// superuserRefuse stops before any migration.
	superuserRefuse superuserPolicy = "refuse"
)

// reachWarning is what serve logs under warn when its role can reach the database server.
const reachWarning = "gophenberg connects to its database as a role that can reach the database server, such as " +
	"a superuser, so connect as the database's own owner role, which " + createOwnerName + " sets up"

// errReachesServer is what stops serve under refuse when its role can reach the database server.
var errReachesServer = errors.New("gophenberg connects to its database as a role that can reach the database server, " +
	"such as a superuser, and " + settingsPrefix + "SUPERUSER_POLICY is refuse, " +
	"so connect as the database's own owner role, which " + createOwnerName + " sets up")

// prepareDatabase checks the role the site connects as under its policy, then migrates the database it serves.
func prepareDatabase(ctx context.Context, settings runConfig, built site, logger *slog.Logger) error {
	db := built.pgHandle.DB
	asked := func(ctx context.Context) (bool, error) { return dbkitpg.ReachesServer(ctx, db) }
	if err := reachCheck(ctx, settings.superuser, asked, logger); err != nil {
		return err
	}
	return migrate(ctx, settings.databaseURL, db)
}

// superuserPolicyFrom reads what serve does when its role can reach the server, refuse when the setting names nothing.
func superuserPolicyFrom(env gonsole.Env) (superuserPolicy, error) {
	return gonsole.Parse(env, "SUPERUSER_POLICY", superuserRefuse, parseSuperuserPolicy)
}

// parseSuperuserPolicy returns the policy text names, refusing any word but warn and refuse.
func parseSuperuserPolicy(text string) (superuserPolicy, error) {
	switch policy := superuserPolicy(text); policy {
	case superuserWarn, superuserRefuse:
		return policy, nil
	}
	return "", fmt.Errorf("must be warn or refuse, got %q", text)
}

// reachCheck asks whether the site's role can reach the database server, then warns or refuses as policy says.
func reachCheck(
	ctx context.Context, policy superuserPolicy, asked func(context.Context) (bool, error), logger *slog.Logger,
) error {
	reaches, err := asked(ctx)
	switch {
	case err != nil:
		return err
	case !reaches:
		return nil
	case policy == superuserRefuse:
		return errReachesServer
	}
	logger.Warn(reachWarning)
	return nil
}
