// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
)

// Command is one command a plugin offers under its id.
type Command struct {
	Name       string
	Summary    string
	Args       []string
	Flags      func(fs *flag.FlagSet)
	Writes     bool
	JSON       bool
	Capability string
	Run        func(ctx context.Context, call Call) error
}

// Call is what a plugin command receives when it runs.
type Call struct {
	Args   []string
	Flags  map[string]string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	JSON   bool
	Apply  bool
	Actor  string
}

// Encode writes v to Stdout as one indented JSON document.
func (c Call) Encode(v any) error {
	encoder := json.NewEncoder(c.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(v)
}

// CommandProvider is implemented by plugins that offer commands under their id.
type CommandProvider interface {
	Commands() []Command
}

// ErrMisused marks an error as a misused command line, which exits with code 2.
var ErrMisused = errors.New("misused command line")

// Misuse marks err as a misused command line, which exits with code 2.
func Misuse(err error) error {
	if err == nil {
		return nil
	}
	return misuse{err: err}
}

// misuse is an error a command answers for a misused command line.
type misuse struct {
	err error
}

// Error returns the text of the error it marks.
func (m misuse) Error() string {
	return m.err.Error()
}

// Unwrap returns the error it marks and ErrMisused.
func (m misuse) Unwrap() []error {
	return []error{m.err, ErrMisused}
}
