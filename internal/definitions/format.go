// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"errors"
	"slices"
	"strings"

	"github.com/gopherium/gophenberg/internal/content"
)

// ErrFormatUnreadable reports an envelope whose format marker is not three plain numbers.
var ErrFormatUnreadable = errors.New("definitions: the format marker is not a plain version")

// ErrFormatUnsupported reports an envelope written under a format this release does not read.
var ErrFormatUnsupported = errors.New("definitions: the format is not one this release reads")

// servedFormats names the envelope formats this release reads, oldest first.
var servedFormats = []string{Format}

// ReadableFormat returns the reason the declared format cannot be read, or nothing when it can.
func ReadableFormat(declared string) error {
	if !plainVersion(declared) {
		return content.Refuse(ErrFormatUnreadable, "definitions_format_unreadable",
			ErrFormatUnreadable.Error(), content.Details{"declared": declared})
	}
	if !slices.Contains(servedFormats, declared) {
		return content.Refuse(ErrFormatUnsupported, "definitions_format_unsupported",
			ErrFormatUnsupported.Error(),
			content.Details{"declared": declared, "served": slices.Clone(servedFormats)})
	}
	return nil
}

// plainVersion reports whether the marker is written as three parts of digits alone.
func plainVersion(declared string) bool {
	parts := strings.Split(declared, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if !onlyDigits(part) {
			return false
		}
	}
	return true
}

// onlyDigits reports whether a version part is written as digits alone, with no sign or spacing.
func onlyDigits(part string) bool {
	if part == "" {
		return false
	}
	for _, character := range part {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
