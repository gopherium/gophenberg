// SPDX-License-Identifier: Apache-2.0

// Package sdk defines the contract between the Gophenberg core and its
// plugins. It is the only Gophenberg package a plugin may import.
package sdk

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/pluginkit"
)

// Plugin is an independently addable unit of functionality with a
// managed lifecycle.
type Plugin = pluginkit.Plugin

// Migrator is implemented by plugins that own database schema, which
// the host migrates before starting any plugin.
type Migrator = pluginkit.Migrator

// RouteProvider is implemented by plugins that expose HTTP endpoints
// under their own namespace.
type RouteProvider = pluginkit.RouteProvider

// PublicPathProvider is implemented by plugins declaring session-exempt public paths.
type PublicPathProvider = pluginkit.PublicPathProvider

// Deps carries the host-provided dependencies a plugin's Register function receives at registration.
type Deps struct {
	DatabaseURL string
	Content     ContentReader
	Getenv      func(string) string
}

// Item is a published content item as plugins see it: the Content
// field holds Gutenberg-serialized block HTML, sanitized for public delivery.
type Item struct {
	ID          uuid.UUID
	Type        string
	Path        string
	Slug        string
	Title       string
	Excerpt     string
	Content     string
	PublishedAt time.Time
	UpdatedAt   time.Time
}

// ContentReader gives plugins read access to published content.
type ContentReader interface {
	ListPublished(ctx context.Context, contentType string, limit int) ([]Item, error)
}
