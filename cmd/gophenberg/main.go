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
	if len(os.Args) > 1 && os.Args[1] == "createadmin" {
		if err := createAdmin(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "gophenberg:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(ctx, os.Getenv, os.Stderr, registerPlugins); err != nil {
		fmt.Fprintln(os.Stderr, "gophenberg:", err)
		os.Exit(1)
	}
}

// createAdmin runs the createadmin subcommand.
func createAdmin(ctx context.Context) error {
	databaseURL := os.Getenv("GOPHENBERG_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("GOPHENBERG_DATABASE_URL is required")
	}
	return authkitpg.RunCreateAdmin(ctx, databaseURL, os.Args[2:], os.Stdin, os.Stdout)
}
