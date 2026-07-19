// SPDX-License-Identifier: Apache-2.0

// Package postgres provides the PostgreSQL data layer for the CMS core.
package postgres

import "embed"

// Migrations holds the core schema migration files applied by goose.
//
//go:embed migrations/*.sql
var Migrations embed.FS
