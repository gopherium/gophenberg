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

## License

Copyright (C) 2026 Manuel 'SirLouen' Camargo

Gophenberg is free and open-source software under the
[Apache License 2.0](LICENSE). Every file carries an
`SPDX-License-Identifier: Apache-2.0` header.

Built frontend artifacts bundle `@wordpress/*` packages licensed under
GPL-2.0-or-later. Those combined bundles are conveyed under the terms of the
GPLv3, while all original source files in this repository remain Apache-2.0.
