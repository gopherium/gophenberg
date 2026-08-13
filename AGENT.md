# Gophenberg

Gophenberg is an open-source plugin-first CMS. The backend is a Go service exposing a JSON API. The frontend is a React SPA admin consuming that API, with content edited in the Gutenberg block editor.

## Architecture

- **Plugin-first.** The core contains only the HTTP server, the plugin host, authentication, and the content domain (the type registry, content items, revisions). Every other feature is a plugin. Anything that can be a plugin must be a plugin.
- **Plugins live in one folder each.** A plugin is a directory under `plugins/` holding a `plugin.json` manifest, an ordinary Go package (compiled in), and an optional `frontend/` npm package for its React screens. The Go package exports `Register(sdk.Deps) (sdk.Plugin, error)`. The frontend package exports a `FrontendPlugin` object named `plugin`. `make generate` reads every manifest and regenerates both wiring files, and CI fails if they are stale. Each plugin gets a mounted route namespace under `/api/plugins/{name}/` (and `/{name}` in the SPA), may declare session-exempt public paths, and may own a Postgres schema with its own migrations. Plugins never import each other and reach the core only through the SDK.

```text
cmd/gophenberg/       main: config, db pool, auth wiring, plugin registration
cmd/pluginwire/       generator: plugins/*/plugin.json -> wiring files
internal/server       http.Handler, routes, middleware, theme reverse proxy
internal/content      content domain package
internal/postgres     data access (pgx + sqlc)
internal/contentbridge published content as the sdk ContentReader seam
internal/publichtml   sanitizer for stored block markup on public surfaces
internal/publicsite   built-in Go renderer for the public site
internal/themehost    theme artifact validation and the node process supervisor
plugins/feed          reference plugin: RSS feed
sdk/                  public plugin contract (Go), the only Gophenberg import allowed in a plugin
sdk/frontend/         frontend plugin contract, UI facade, and test harness (@gophenberg/frontend-sdk)
sdk/astro/            theme kit (@gophenberg/astro): client, block walker, integration, routes
frontend/             React SPA host (Vite), plugins import UI only via @gophenberg/frontend-sdk
test/theme/           reference Astro theme, the e2e fixture and the starter source
```

The plugin lifecycle host and wiring generator come from
[gopherium/pluginkit](https://github.com/gopherium/pluginkit).

- **The public site is two interchangeable renderers.** The root serves published content through the built-in Go renderer, or through an Astro theme when one is active. A theme is a prebuilt artifact (`theme.json`, `server/entry.mjs`, `client/`) validated by `internal/themehost`, run as a supervised node process bound to loopback, and reached through a reverse proxy that falls back to the Go renderer whenever the theme is not answering. The theme process receives exactly three environment variables and reads content over `/api/content/v1` like any other client. `/api`, `/admin`, `/media`, `/gophenberg`, and `/_gophenberg` are never proxied.

## Stack

- Backend: Go, `net/http` + chi v5, PostgreSQL (pgx/v5, sqlc, goose migrations), gouncer/authkit authentication
- Frontend: React + TypeScript, Vite, TanStack Router + Query, `@wordpress/ui` + `@wordpress/theme` (WordPress Design System), `@wordpress/block-editor` (Gutenberg)
- Themes: Astro with the `@astrojs/node` standalone adapter, built to a self-contained artifact that runs without `node_modules`
- Testing: stdlib table-driven tests, httptest, pgtestdb (backend), Vitest, Testing Library, MSW (frontend), Playwright (e2e)

## Contributing

1. Keep changes small and focused: one behavior per change.
2. Every change ships with tests, written before the implementation.
3. Every function carries a doc comment: Go in canonical form, TypeScript following tsdoc standard. Lines wrap at 120 columns.
4. Run `make test` and `make lint` before submitting. CI enforces both, plus the race detector. It reports SDK API changes without blocking while the SDK is below 1.0.

## License

Apache-2.0 ([LICENSE](LICENSE)). Every file carries an `SPDX-License-Identifier: Apache-2.0` header. Built frontend bundles include GPL-2.0-or-later `@wordpress/*` packages and are conveyed under GPLv3 terms. See the [README](README.md).
