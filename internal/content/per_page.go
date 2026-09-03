// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
	"strconv"
)

// ErrPerPageInvalid reports that a listing page size is not a whole number from 1 to MaxPerPage.
var ErrPerPageInvalid = errors.New("content: per page invalid")

// DefaultPerPage is how many items a public listing page carries when nothing else applies.
const DefaultPerPage = 20

// MaxPerPage is the most items one listing page may carry.
const MaxPerPage = 100

// PerPageSettingKey names the stored public listing page size.
const PerPageSettingKey = "content.per_page"

// ParsePerPage returns the page size the text names, or the reason it is not one.
func ParsePerPage(raw string) (int, error) {
	size, err := strconv.Atoi(raw)
	if err != nil || size < 1 || size > MaxPerPage {
		return 0, Refuse(ErrPerPageInvalid, "per_page_invalid",
			fmt.Sprintf("%s: %s", ErrPerPageInvalid, raw), Details{"value": raw, "max": MaxPerPage})
	}
	return size, nil
}

// ResolvePerPage returns the page size a stored setting names, or the default when it names none.
func ResolvePerPage(held string, found bool) int {
	if !found {
		return DefaultPerPage
	}
	size, err := ParsePerPage(held)
	if err != nil {
		return DefaultPerPage
	}
	return size
}
