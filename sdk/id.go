// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"encoding/hex"
	"fmt"
)

// ID identifies a content item or an account, sixteen bytes the host assigns.
type ID [16]byte

// hyphens are the positions of the hyphens in the canonical form.
var hyphens = [...]int{8, 13, 18, 23}

// String returns the id in its canonical form, 36 lowercase characters with hyphens.
func (id ID) String() string {
	var text [36]byte
	hex.Encode(text[0:8], id[0:4])
	text[8] = '-'
	hex.Encode(text[9:13], id[4:6])
	text[13] = '-'
	hex.Encode(text[14:18], id[6:8])
	text[18] = '-'
	hex.Encode(text[19:23], id[8:10])
	text[23] = '-'
	hex.Encode(text[24:], id[10:])
	return string(text[:])
}

// MarshalText returns the id in its canonical form, so JSON carries it as text.
func (id ID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

// UnmarshalText reads the id from its canonical form in either case, leaving it unchanged on a refusal.
func (id *ID) UnmarshalText(text []byte) error {
	refusal := fmt.Errorf("%q is not an id, want 36 characters such as 019fb000-0000-7000-8000-000000000001", text)
	if len(text) != 36 {
		return refusal
	}
	digits := make([]byte, 0, 32)
	start := 0
	for _, at := range hyphens {
		if text[at] != '-' {
			return refusal
		}
		digits = append(digits, text[start:at]...)
		start = at + 1
	}
	digits = append(digits, text[start:]...)
	var read ID
	if _, err := hex.Decode(read[:], digits); err != nil {
		return refusal
	}
	*id = read
	return nil
}
