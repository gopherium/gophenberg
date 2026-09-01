// SPDX-License-Identifier: Apache-2.0

package server

import (
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// fieldResponse is a field definition as the admin API answers it.
type fieldResponse struct {
	Key       string          `json:"key"`
	Label     string          `json:"label"`
	Kind      string          `json:"kind"`
	RelatesTo string          `json:"relates_to,omitempty"`
	Many      bool            `json:"many"`
	Required  bool            `json:"required"`
	Settings  map[string]any  `json:"settings,omitempty"`
	Fields    []fieldResponse `json:"fields,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// fieldListResponse is one type's definition list as the admin API answers it.
type fieldListResponse struct {
	Items []fieldResponse `json:"items"`
}

// newFieldResponse builds a fieldResponse, normalizing timestamps to UTC.
func newFieldResponse(f content.Field) fieldResponse {
	return fieldResponse{
		Key:       f.Key,
		Label:     f.Label,
		Kind:      string(f.Kind),
		RelatesTo: f.RelatesTo,
		Many:      f.Many,
		Required:  f.Required,
		Settings:  f.Settings,
		Fields:    newFieldResponses(f.Fields),
		CreatedAt: f.CreatedAt.UTC(),
		UpdatedAt: f.UpdatedAt.UTC(),
	}
}

// newFieldResponses builds the answers for the sub fields a container holds, however deep they run.
func newFieldResponses(held []content.Field) []fieldResponse {
	if len(held) == 0 {
		return nil
	}
	answers := make([]fieldResponse, len(held))
	for i, f := range held {
		answers[i] = newFieldResponse(f)
	}
	return answers
}
