// SPDX-License-Identifier: Apache-2.0

package content

import "errors"

// Details holds the parts of an error a client reads as data rather than as prose.
type Details map[string]any

// Error is a sentinel carrying its condition code and the dynamic parts of the message.
type Error struct {
	Err     error
	Code    string
	Message string
	Held    Details
}

// Error returns the message the error reads as.
func (e *Error) Error() string {
	return e.Message
}

// Unwrap returns the sentinel the error stands for.
func (e *Error) Unwrap() error {
	return e.Err
}

// Refuse returns the sentinel carrying its condition code, its message and the details a client reads.
func Refuse(err error, code, message string, held Details) error {
	return &Error{Err: err, Code: code, Message: message, Held: held}
}

// CodeOf returns the condition code an error names, reporting false when it names none.
func CodeOf(err error) (string, bool) {
	var named *Error
	if !errors.As(err, &named) {
		return "", false
	}
	return named.Code, true
}

// DetailsOf returns the details an error carries, reporting false when it carries none.
func DetailsOf(err error) (Details, bool) {
	var named *Error
	if !errors.As(err, &named) {
		return nil, false
	}
	return named.Held, true
}
