// SPDX-License-Identifier: Apache-2.0

// Command gophenberg runs the Gophenberg CMS server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"syscall"

	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/joho/godotenv"
)

// usage is the command list the program prints when asked for help.
const usage = `Usage:
  gophenberg              serve the site
  gophenberg serve        serve the site
  gophenberg createadmin  create the first administrator
  gophenberg grantrole    give a role to every account holding none
  gophenberg seed         store the demo data
  gophenberg help         print this text

Pass -h to a subcommand for its own flags.
`

// seedUsage is the help the seed command prints.
const seedUsage = `Usage:
  gophenberg seed

Stores the demo data. It takes no flags or arguments.
`

// main runs the gophenberg server, or one of its subcommands.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	_ = godotenv.Load()
	code := dispatch(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// misuse is a command line the program refuses before doing anything.
type misuse struct {
	message string
}

// Error returns the reason the command line was refused.
func (m misuse) Error() string {
	return m.message
}

// dispatch runs the command the arguments name and answers its exit code.
func dispatch(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return report(command(ctx, args, stdin, stdout, stderr), stderr)
}

// command runs the command the arguments name, serving the site when they name none.
func command(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return run(ctx, os.Getenv, stderr, registerPlugins)
	}
	switch args[0] {
	case "serve":
		return serveCommand(ctx, args[1:], stderr)
	case "createadmin":
		return createAdmin(ctx, args[1:], stdin, stdout)
	case "grantrole":
		return grantRole(ctx, args[1:], stdout)
	case "seed":
		return seedCommand(ctx, args[1:], stdout)
	case "help", "-h", "-help", "--help":
		_, err := fmt.Fprint(stdout, usage)
		return err
	}
	return misuse{fmt.Sprintf("unknown command %q, want createadmin, grantrole, seed or serve", args[0])}
}

// report prints a failure to stderr and answers the exit code it earns.
func report(err error, stderr io.Writer) int {
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return 0
	}
	_, _ = fmt.Fprintln(stderr, "gophenberg:", err)
	var refused misuse
	if errors.As(err, &refused) {
		return 2
	}
	return 1
}

// serveCommand serves the site, refusing any argument.
func serveCommand(ctx context.Context, args []string, stderr io.Writer) error {
	if len(args) > 0 {
		return misuse{"serve takes no arguments"}
	}
	return run(ctx, os.Getenv, stderr, registerPlugins)
}

// seedCommand stores the demo data, refusing any flag or argument.
func seedCommand(ctx context.Context, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("seed", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	err := flags.Parse(args)
	switch {
	case errors.Is(err, flag.ErrHelp):
		_, err := fmt.Fprint(stdout, seedUsage)
		return err
	case err != nil:
		return misuse{"seed: " + err.Error()}
	case len(args) > 0:
		return misuse{"seed takes no arguments"}
	}
	return seedDemoData(ctx, os.Getenv, stdout)
}

// asksForHelp reports whether the arguments ask a command for its flags.
func asksForHelp(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		return arg == "-h" || arg == "-help" || arg == "--help"
	})
}

// createAdmin runs the createadmin subcommand.
func createAdmin(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	databaseURL := os.Getenv("GOPHENBERG_DATABASE_URL")
	if databaseURL == "" && !asksForHelp(args) {
		return errors.New("GOPHENBERG_DATABASE_URL is required")
	}
	return authkitpg.RunCreateAdmin(ctx, databaseURL, args, stdin, stdout)
}

// grantRole gives the named role to every account holding none.
func grantRole(ctx context.Context, args []string, stdout io.Writer) error {
	databaseURL := os.Getenv("GOPHENBERG_DATABASE_URL")
	if databaseURL == "" && !asksForHelp(args) {
		return errors.New("GOPHENBERG_DATABASE_URL is required")
	}
	return authkitpg.RunGrantRole(ctx, databaseURL, args, stdout)
}
