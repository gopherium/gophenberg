---
title: Local development
description: From a fresh clone to writing posts on your machine.
---

This page takes you from a fresh clone to a running Gophenberg with
demo content and a login.

You need three tools installed: Go (the version pinned in `go.mod`),
Node 26 with pnpm, and Docker (it runs the PostgreSQL database, and
nothing else).

## 1. Clone and configure

```sh
git clone https://github.com/gopherium/gophenberg
cd gophenberg
cp .env.example .env
```

The example file points the server at the development database and
needs no editing.

## 2. Seed the database

```sh
make seed
```

This starts PostgreSQL in Docker, applies the database migrations,
and creates a development login plus demo posts. The login is
`admin@example.com` with the password `password1234`.

:::caution
The seeded login is publicly known. It is for development machines
only, never for a server anyone can reach.
:::

## 3. Run the backend and the admin

In two terminals:

```sh
make dev
```

```sh
pnpm install && pnpm --filter @gophenberg/frontend dev
```

The first runs the API on `localhost:8081`. The second runs the
admin on [localhost:5174](http://localhost:5174), which forwards
`/api` calls to the first, so the login cookie works without any
HTTPS setup.

## 4. Look around

- Open [localhost:5174](http://localhost:5174) and log in with the
  seeded account. You land in the admin with demo posts to play
  with.
- Open [localhost:8081](http://localhost:8081) for the public site,
  served by the built-in renderer.

From here, [the posts list](/guides/posts/) shows you around the
admin, and [how theming works](/themes/how-theming-works/) explains
the public site.
