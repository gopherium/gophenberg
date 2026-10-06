// SPDX-License-Identifier: Apache-2.0

// Package sdk defines the contract between the Gophenberg core and its
// plugins. It is the only Gophenberg package a plugin may import.
package sdk

import (
	"context"
	"net/http"
	"time"
)

// Plugin is an independently addable unit of functionality with a
// managed lifecycle.
type Plugin interface {
	ID() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Migrator is implemented by plugins that own database schema, which
// the host migrates before starting any plugin.
type Migrator interface {
	Migrate(ctx context.Context) error
}

// RouteProvider is implemented by plugins that expose HTTP endpoints
// under their own namespace.
type RouteProvider interface {
	Routes() http.Handler
}

// PublicPathProvider is implemented by plugins declaring session-exempt public paths.
type PublicPathProvider interface {
	PublicPaths() []string
}

// Deps carries the host-provided dependencies a plugin's Register function receives at registration.
type Deps struct {
	DatabaseURL string
	Content     ContentReader
	Getenv      func(string) string
	Env         Env
}

// Item is a published content item as plugins see it: the Content field holds sanitized block HTML, and
// Fields holds the values the content API serves, decoded the way encoding/json decodes them.
type Item struct {
	ID          ID
	Type        string
	Path        string
	Slug        string
	Title       string
	Excerpt     string
	Content     string
	Fields      map[string]any
	PublishedAt time.Time
	UpdatedAt   time.Time
}

// ContentReader gives plugins read access to published content.
type ContentReader interface {
	ListPublished(ctx context.Context, contentType string, limit int) ([]Item, error)
}
