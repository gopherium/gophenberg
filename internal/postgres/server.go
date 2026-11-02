// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// OldestMajor is the oldest PostgreSQL major version this release supports.
const OldestMajor = 15

const (
	// majorStep is how far apart two PostgreSQL majors sit in server_version_num.
	majorStep = 10000
	// serverLookup reads the server's version number and the version it shows.
	serverLookup = "SELECT current_setting('server_version_num')::int, current_setting('server_version')"
)

// CheckMajor returns an error naming the server's version when it runs a PostgreSQL major older than OldestMajor.
func CheckMajor(ctx context.Context, db *sql.DB) error {
	number, shown, err := serverVersion(ctx, db)
	if err != nil {
		return err
	}
	return majorRefusal(number, shown)
}

// ServerMajor returns the PostgreSQL major the server db connects to runs.
func ServerMajor(ctx context.Context, db *sql.DB) (int, error) {
	number, _, err := serverVersion(ctx, db)
	return number / majorStep, err
}

// serverVersion returns the version number of the server db connects to and the version it shows.
func serverVersion(ctx context.Context, db *sql.DB) (int, string, error) {
	var number int
	var shown string
	if err := db.QueryRowContext(ctx, serverLookup).Scan(&number, &shown); err != nil {
		return 0, "", fmt.Errorf("postgres: read the server version: %w", err)
	}
	return number, shown, nil
}

// majorRefusal returns an error naming shown when the version number belongs to a major older than OldestMajor.
func majorRefusal(number int, shown string) error {
	if number/majorStep >= OldestMajor {
		return nil
	}
	return fmt.Errorf("this database runs PostgreSQL %s and Gophenberg needs PostgreSQL %d or newer", shown, OldestMajor)
}
