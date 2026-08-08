---
title: Installing a theme
description: Building the artifact, the layout Gophenberg validates, and activating it.
---

A theme deploys as a built artifact, never as source. You build it
on your machine, copy three things to the server, and point two
environment variables at them.

## 1. Build

```sh
astro build
```

The kit builds a self-contained artifact: everything the theme
needs ends up inside `dist/`, with no `node_modules` on the
server. Keep your theme's dependencies to pure JavaScript, since
compiled native code cannot load from the artifact.

## 2. Lay out the artifact

A theme install is one directory holding three things:

```text
themes/
  mytheme/
    theme.json
    server/     (from dist/server)
    client/     (from dist/client)
```

`theme.json` is one line you write once:

```json
{ "name": "mytheme", "version": "1.0.0", "kit": "%VERSION%" }
```

Gophenberg validates the install at startup and **refuses to
start** with a broken one, naming the rule it hit:

- `theme.json` present and valid, its `name` equal to the
  directory name
- `server/entry.mjs` present as a file, `client/` as a directory
- no symlinks anywhere, and the tree under 64 MiB

A broken theme takes the server down, admin included, so check the
site after installing or switching.

## 3. Activate

```yaml
    environment:
      GOPHENBERG_THEMES_DIR: /themes
      GOPHENBERG_THEME: mytheme
    volumes:
      - ./themes:/themes:ro
```

Restart and watch the logs for `theme starting`, then
`theme ready`. Setting `GOPHENBERG_THEME` back to empty returns
the site to the built-in renderer.

The container image already carries the Node runtime themes run
on, so the mount is all there is to it.
