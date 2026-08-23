// SPDX-License-Identifier: Apache-2.0

package mediahost

import "fmt"

// Error reports an upload turned away, carrying the reason an operator reads.
type Error struct {
	// Code names the rule the upload broke, for a client that translates it.
	Code string
	// Reason names the rule the upload broke, in words fit for an admin screen.
	Reason string
	// Detail is what went wrong underneath, for the log.
	Detail error
}

// Error returns the reason followed by the detail behind it.
func (e *Error) Error() string { return e.Reason + ": " + e.Detail.Error() }

// Unwrap returns the error it was raised over.
func (e *Error) Unwrap() error { return e.Detail }

// refuse returns an error naming its code and reason over the formatted detail.
func refuse(code, reason, format string, args ...any) error {
	return &Error{Code: code, Reason: reason, Detail: fmt.Errorf(format, args...)}
}
