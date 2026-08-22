// SPDX-License-Identifier: Apache-2.0

// Command gophenberg runs the Gophenberg CMS server.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/joho/godotenv"
)

// main runs the gophenberg server, or one of its subcommands.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	_ = godotenv.Load()
	if err := dispatch(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "gophenberg:", err)
		os.Exit(1)
	}
}

// dispatch runs the subcommand the arguments name, serving the site when they name none.
func dispatch(ctx context.Context) error {
	subcommands := map[string]func(context.Context) error{
		"createadmin": createAdmin,
		"grantrole":   grantRole,
		"seed":        seedFromEnvironment,
	}
	if len(os.Args) > 1 {
		if subcommand, named := subcommands[os.Args[1]]; named {
			return subcommand(ctx)
		}
	}
	return run(ctx, os.Getenv, os.Stderr, registerPlugins)
}

// seedFromEnvironment runs the seed subcommand over the configured environment.
func seedFromEnvironment(ctx context.Context) error {
	return seedDemoData(ctx, os.Getenv, os.Stdout)
}

// createAdmin runs the createadmin subcommand.
func createAdmin(ctx context.Context) error {
	databaseURL := os.Getenv("GOPHENBERG_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("GOPHENBERG_DATABASE_URL is required")
	}
	return authkitpg.RunCreateAdmin(ctx, databaseURL, os.Args[2:], os.Stdin, os.Stdout)
}

// grantRole gives the named role to every account holding none.
func grantRole(ctx context.Context) error {
	databaseURL := os.Getenv("GOPHENBERG_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("GOPHENBERG_DATABASE_URL is required")
	}
	return authkitpg.RunGrantRole(ctx, databaseURL, os.Args[2:], os.Stdout)
}
