// SPDX-License-Identifier: Apache-2.0

package themehost

import "fmt"

// Error reports a theme turned away, carrying the reason an operator reads.
type Error struct {
	// Code names the rule the theme broke, for a client that translates it.
	Code string
	// Reason names the rule the theme broke, in words fit for an admin screen.
	Reason string
	// Detail is what went wrong underneath, for the log.
	Detail error
	// Held names the values the reason's translation asks for.
	Held map[string]any
}

// Error returns the reason followed by the detail behind it.
func (e *Error) Error() string { return e.Reason + ": " + e.Detail.Error() }

// Unwrap returns the error it was raised over.
func (e *Error) Unwrap() error { return e.Detail }

// refuse returns an error naming its code and reason over the formatted detail.
func refuse(code, reason, format string, args ...any) error {
	return &Error{Code: code, Reason: reason, Detail: fmt.Errorf(format, args...)}
}

// refuseHolding returns an error carrying the values its translation asks for.
func refuseHolding(code, reason string, held map[string]any, format string, args ...any) error {
	return &Error{Code: code, Reason: reason, Detail: fmt.Errorf(format, args...), Held: held}
}
