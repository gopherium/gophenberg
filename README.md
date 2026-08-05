# Gophenberg

A plugin-first CMS. Go backend exposing a JSON API, React SPA admin built on top of
the WPDS and Gutenberg block editor.

## What works today

- Block content authored in the Gutenberg editor, with autosave, revisions, trash and
  restore
- A posts list with status filters, counts and sorting, and a trash view
- Post types carrying their own revision policy, registered in the core
- Session authentication with login rate limiting, and a users screen for administrators
- A plugin contract covering lifecycle, a mounted route namespace, session-exempt public
  paths, database migrations, and read access to published posts
- A reference plugin serving the published posts as an RSS channel
- A public site at the root serving published posts, with the admin under `/admin`
- A read-only content API at `/api/content/v1` for published posts
- Optional Astro themes, served by a supervised Node process, with the built-in
  renderer taking over whenever no theme is active or a theme is down

## Requirements

Go as pinned in `go.mod`, Node 26 with pnpm, and Docker for the PostgreSQL service.

## Quick start

```sh
cp .env.example .env
make seed
```

`make seed` starts PostgreSQL, applies the migrations, and creates a development
administrator (`admin@example.com` with the password `password1234`) alongside demo
content. Then run the API and the admin in two terminals:

```sh
make dev
pnpm install && pnpm --filter @gophenberg/frontend dev
```

The API listens on `localhost:8081` and the admin on `localhost:5174`, which proxies
`/api` to the API.

## The public site

The root serves published posts. Without a theme they are rendered by the built-in Go
renderer, which needs nothing beyond the binary.

## Themes

A theme is an Astro project depending on `@gophenberg/astro`. It names itself in one line
of `astro.config.mjs` and describes itself in `src/theme.ts`:

```js
import { gophenberg } from '@gophenberg/astro/config'
import { defineConfig } from 'astro/config'

export default defineConfig({ integrations: [gophenberg()] })
```

```ts
import { defineTheme } from '@gophenberg/astro'

export default defineTheme({
  layouts: { Base, Post, Archive },
  blocks: { 'core/quote': Quote },
  seo: { siteName: 'A Site' },
})
```

The kit supplies the routes, the pagination, the 404, and the identification signals. A
theme supplies layouts, and may override how any block renders. `test/theme` is a working
example.

`astro build` produces `dist/server` and `dist/client`. An installed theme is a directory
holding those two beside a `theme.json` manifest:

```json
{ "name": "starter", "version": "0.1.0", "kit": "^0.1.0" }
```

Put that directory under `GOPHENBERG_THEMES_DIR` and name it in `GOPHENBERG_THEME`. The
server validates it, runs it, restarts it if it exits, and serves the built-in renderer
whenever it is not answering. Themes need Node, which the container image carries.

## License

Copyright (C) 2026 Manuel 'SirLouen' Camargo

Gophenberg is free and open-source software under the
[Apache License 2.0](LICENSE). Every file carries an
`SPDX-License-Identifier: Apache-2.0` header.

Built frontend artifacts bundle `@wordpress/*` packages licensed under
GPL-2.0-or-later. Those combined bundles are conveyed under the terms of the
GPLv3, while all original source files in this repository remain Apache-2.0.
