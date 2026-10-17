// SPDX-License-Identifier: Apache-2.0

// Command releasetool makes release keys, signs release files, checks their signatures, and lists them in
// SHA256SUMS and the release record.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// standardStream names standard output where a flag takes a file.
const standardStream = "-"

// usage lists the commands releasetool runs and their flags.
const usage = `usage:
  releasetool keygen -curve ed25519|p256 -out <file|-> [-public <file>] [-passphrase-file <file> -iterations <n>]
  releasetool sign   -key <file> [-passphrase-file <file>] <file>...
  releasetool verify -public <file|multikey> <file>...
  releasetool sums   <file>...
  releasetool record -version <version> -download-url <https address> <archive>...
`

// command runs one releasetool command over its arguments.
type command func(args []string, stdout, stderr io.Writer) error

// commands maps each command name to what runs it.
var commands = map[string]command{
	"keygen": keygen,
	"sign":   sign,
	"verify": verify,
	"sums":   sums,
	"record": record,
}

// errFlags reports flags the flag package refused and has already described.
var errFlags = errors.New("the flags do not parse")

// usageError reports a command misused, which exits 2 as the flag package does.
type usageError struct {
	message string
}

// Error returns what the misuse was.
func (e usageError) Error() string {
	return e.message
}

// main runs the command the arguments name and exits with its code.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run runs the command args name and returns its exit code: 0 done, 1 refused or failed, 2 misused.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}
	named, known := commands[args[0]]
	if !known {
		_, _ = fmt.Fprintf(stderr, "releasetool: unknown command %q\n%s", args[0], usage)
		return 2
	}
	return exitCode(named(args[1:], stdout, stderr), stderr)
}

// exitCode prints a command's error and returns the code it exits with.
func exitCode(err error, stderr io.Writer) int {
	switch {
	case err == nil || errors.Is(err, flag.ErrHelp):
		return 0
	case errors.Is(err, errFlags):
		return 2
	}
	_, _ = fmt.Fprintln(stderr, "releasetool:", err)
	if errors.As(err, new(usageError)) {
		return 2
	}
	return 1
}

// newFlagSet returns the flags of a command, which describe their misuse on stderr.
func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet("releasetool "+name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags
}

// parseFlags parses the arguments, turning a parse error the flag package has described into errFlags.
func parseFlags(flags *flag.FlagSet, args []string) error {
	err := flags.Parse(args)
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return err
	}
	return errFlags
}
