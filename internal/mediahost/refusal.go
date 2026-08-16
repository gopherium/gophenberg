// SPDX-License-Identifier: Apache-2.0

package mediahost

import "fmt"

// Refusal reports an upload turned away, carrying the reason an operator reads.
type Refusal struct {
	// Code names the rule the upload broke, for a client that translates it.
	Code string
	// Reason names the rule the upload broke, in words fit for an admin screen.
	Reason string
	// Detail is what went wrong underneath, for the log.
	Detail error
}

// Error returns the reason followed by the detail behind it.
func (r *Refusal) Error() string { return r.Reason + ": " + r.Detail.Error() }

// Unwrap returns the error the refusal was raised over.
func (r *Refusal) Unwrap() error { return r.Detail }

// refuse returns a refusal naming its code and reason over the formatted detail.
func refuse(code, reason, format string, args ...any) error {
	return &Refusal{Code: code, Reason: reason, Detail: fmt.Errorf(format, args...)}
}
