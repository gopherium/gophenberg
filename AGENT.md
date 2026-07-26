# Gophenberg

Gophenberg is an open-source plugin-first CMS. The backend is a Go service exposing a JSON API. The frontend is a React SPA admin consuming that API, with posts edited in the Gutenberg block editor.

## Architecture

- **Plugin-first.** The core contains only the HTTP server, the plugin host, authentication, and the post domain (posts, revisions, post types). Every other feature is a plugin. Anything that can be a plugin must be a plugin.
- **Plugins live in one folder each.** A plugin is a directory under `plugins/` holding a `plugin.json` manifest, an ordinary Go package (compiled in), and an optional `frontend/` npm package for its React screens. The Go package exports `Register(sdk.Deps) (*Plugin, error)`. The frontend package exports a `FrontendPlugin` object named `plugin`. `make generate` reads every manifest and regenerates both wiring files, and CI fails if they are stale. Each plugin gets a mounted route namespace under `/api/plugins/{name}/` (and `/{name}` in the SPA) and its own Postgres schema with its own migrations. Plugins never import each other and reach the core only through the SDK.

```text
cmd/gophenberg/       main: config, db pool, auth wiring, plugin registration
cmd/pluginwire/       generator: plugins/*/plugin.json -> wiring files
internal/server       http.Handler, routes, middleware
internal/post         post domain package
internal/postgres     data access (pgx + sqlc)
plugins/feed          reference plugin: RSS feed
sdk/                  public plugin contract (Go), the only Gophenberg import allowed in a plugin
sdk/frontend/         frontend plugin contract, UI facade, and test harness (@gophenberg/frontend-sdk)
frontend/             React SPA host (Vite), plugins import UI only via @gophenberg/frontend-sdk
```

The plugin lifecycle host and wiring generator come from
[gopherium/pluginkit](https://github.com/gopherium/pluginkit).

## Stack

- Backend: Go, `net/http` + chi v5, PostgreSQL (pgx/v5, sqlc, goose migrations), gouncer/authkit authentication
- Frontend: React + TypeScript, Vite, TanStack Router + Query, `@wordpress/ui` + `@wordpress/theme` (WordPress Design System), `@wordpress/block-editor` (Gutenberg)
- Testing: stdlib table-driven tests, httptest, pgtestdb (backend), Vitest, Testing Library, MSW (frontend), Playwright (e2e)

## Contributing

1. Keep changes small and focused: one behavior per change.
2. Every change ships with tests, written before the implementation.
3. Every function carries a doc comment: Go in canonical form, TypeScript following tsdoc standard. Lines wrap at 120 columns.
4. Run `make test` and `make lint` before submitting. CI enforces both, plus the race detector and SDK compatibility checks.

## License

Apache-2.0 ([LICENSE](LICENSE)). Every file carries an `SPDX-License-Identifier: Apache-2.0` header. Built frontend bundles include GPL-2.0-or-later `@wordpress/*` packages and are conveyed under GPLv3 terms. See the [README](README.md).
