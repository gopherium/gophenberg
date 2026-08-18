// SPDX-License-Identifier: Apache-2.0

package content

import "errors"

// Details holds the parts of a refusal a client reads as data rather than as prose.
type Details map[string]any

// Refusal is a sentinel carrying its condition code and the dynamic parts of the refusal.
type Refusal struct {
	Err     error
	Code    string
	Message string
	Held    Details
}

// Error returns the message the refusal reads as.
func (r *Refusal) Error() string {
	return r.Message
}

// Unwrap returns the sentinel the refusal stands for.
func (r *Refusal) Unwrap() error {
	return r.Err
}

// Refuse returns the sentinel carrying its condition code, its message and the details a client reads.
func Refuse(err error, code, message string, held Details) error {
	return &Refusal{Err: err, Code: code, Message: message, Held: held}
}

// CodeOf returns the condition code a refusal names, reporting false when it names none.
func CodeOf(err error) (string, bool) {
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		return "", false
	}
	return refusal.Code, true
}

// DetailsOf returns the details a refusal carries, reporting false when it carries none.
func DetailsOf(err error) (Details, bool) {
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		return nil, false
	}
	return refusal.Held, true
}
