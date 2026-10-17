// SPDX-License-Identifier: Apache-2.0

package app

import (
	"cmp"
	"context"
	"fmt"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/themehost"
	"github.com/gopherium/gophenberg/internal/version"
)

// shortCommit is how many characters of the commit the summary line shows.
const shortCommit = 7

// buildFacts is what about tells of this binary, the fields every release.json repeats.
type buildFacts struct {
	Schema  int      `json:"schema"`
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Commit  string   `json:"commit"`
	Stamp   string   `json:"stamp"`
	Kits    []string `json:"kits"`
}

// aboutCommand returns the command that prints what this binary is, as one document with -json.
func aboutCommand() gonsole.Command {
	return gonsole.Command{
		Name:    "about",
		Summary: "print the version, the build stamp, the commit and the theme kits of this binary",
		JSON:    true,
		Run:     printAbout,
	}
}

// printAbout prints the build facts of this binary, one line for people or one document with -json.
func printAbout(_ context.Context, call gonsole.Call) error {
	facts := buildFacts{
		Schema: 1, Name: "gophenberg", Version: version.Version(),
		Commit: version.Commit(), Stamp: version.Stamp(), Kits: themehost.ServedKits(),
	}
	if call.JSON {
		return call.Encode(facts)
	}
	_, err := fmt.Fprintln(call.Stdout, facts.summary())
	return err
}

// summary returns the build facts as the one line about prints for people.
func (f buildFacts) summary() string {
	line := fmt.Sprintf("Gophenberg %s, %s build", f.Version, cmp.Or(f.Stamp, "custom"))
	if f.Commit != "" {
		line += ", commit " + f.Commit[:min(shortCommit, len(f.Commit))]
	}
	return line + ", theme kits " + f.Kits[0] + " to " + f.Kits[len(f.Kits)-1]
}
