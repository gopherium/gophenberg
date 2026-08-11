# Gophenberg

A plugin-first CMS. Go backend exposing a JSON API, React SPA admin built on top of
the WPDS and Gutenberg block editor.

**[Documentation](https://docs.gophenberg.org)**

## What works today

- Block content authored in the Gutenberg editor, with autosave, revisions, trash and
  restore
- A posts list with status filters, counts and sorting, and a trash view
- A media library storing uploads with derived image sizes, feeding the editor's media
  blocks from a picker and the inserter
- Session authentication with login rate limiting, and a users screen for administrators
- A public site at the root, the admin under `/admin`, and a read-only content API at
  `/api/content/v1`
- Optional Astro themes through `@gophenberg/astro`, with the built-in renderer taking
  over whenever no theme is active or a theme is down
- Compile-time plugins owning routes, public paths, and their own schema, with a
  reference plugin serving the posts as an RSS channel

## Running it locally

Go as pinned in `go.mod`, Node 26 with pnpm, and Docker for the PostgreSQL service.

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

To self-host a real deployment instead, follow the
[install guide](https://docs.gophenberg.org/self-hosting/install/).

## Writing a theme

A theme is an Astro project depending on `@gophenberg/astro`. You write your layouts,
the kit supplies the routes and the block rendering. `test/theme` is a working example,
and [writing a theme](https://docs.gophenberg.org/themes/writing-a-theme/) walks it.

## Writing a plugin

A plugin is a Go package under `plugins/` with a `plugin.json`, compiled in by
`make generate`. `plugins/feed` is the reference, and
[write a plugin](https://docs.gophenberg.org/extending/write-a-plugin/) walks it.

## License

Copyright (C) 2026 Manuel 'SirLouen' Camargo

Gophenberg is free and open-source software under the
[Apache License 2.0](LICENSE). Hand-written source files carry an
`SPDX-License-Identifier: Apache-2.0` header.

Built frontend artifacts bundle `@wordpress/*` packages licensed under
GPL-2.0-or-later. Those combined bundles are conveyed under the terms of the
GPLv3, while all original source files in this repository remain Apache-2.0.
