// SPDX-License-Identifier: Apache-2.0

package columns

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/gopherium/gophenberg/internal/content"
)

// Values is a content item's field values stored as JSON text.
type Values content.Values

// Value returns the map as the JSON text json.Marshal writes, or no value for a nil map.
func (v Values) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	text, err := json.Marshal(content.Values(v))
	return string(text), err
}

// Scan replaces the map with the one the JSON text holds, nil for null, every number a float64.
func (v *Values) Scan(src any) error {
	text, ok := src.(string)
	if !ok {
		return fmt.Errorf("columns: the field values read JSON text, got %T", src)
	}
	var read content.Values
	if err := json.Unmarshal([]byte(text), &read); err != nil {
		return fmt.Errorf("columns: read the field values: %w", err)
	}
	*v = Values(read)
	return nil
}
