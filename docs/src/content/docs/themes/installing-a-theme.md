---
title: Installing a theme
description: Packaging a theme, uploading it in the admin, and choosing which one serves.
---

A theme deploys as a built artifact, never as source. You build it
on your machine, package it as a zip file, and upload it in the
admin.

## 1. Build

```sh
astro build
```

The kit builds a self-contained artifact: everything the theme
needs ends up inside `dist/`, with no `node_modules` on the
server. Keep your theme's dependencies to pure JavaScript, since
compiled native code cannot load from the artifact.

## 2. Package it

The zip holds three things at its top level:

```text
theme.json
server/     (from dist/server)
client/     (from dist/client)
```

`theme.json` is one line you write once:

```json
{ "name": "mytheme", "version": "0.1.0", "kit": "%KIT_VERSION%" }
```

Name the zip after the theme. Gophenberg installs `mytheme.zip` as
`mytheme`, and the `name` inside `theme.json` has to match.

`kit` is the exact version of `@gophenberg/astro` you built against,
three numbers and nothing else. A range like `^0.1.0` is refused,
because the site has to know which shape your theme reads rather than
which ones you would accept.

## 3. Upload and activate

Open **Themes** in the admin, choose the zip, and select **Install
theme**. Once it appears in the list, select **Activate**.

The old theme keeps serving until the new one is ready, so the
site never goes quiet while you switch. **Deactivate** returns the
site to the built-in renderer, and **Roll back** returns it to the
choice before the current one.

## What Gophenberg checks

An upload is refused whole, and the admin shows the reason:

- `theme.json` present and valid, its `name` matching the zip name
- `kit` naming a version the site serves, which its
  [handshake](/reference/content-api/) lists
- `server/entry.mjs` present as a file, `client/` as a directory
- no symlinks anywhere, and nothing over 64 MiB packed or unpacked
- at most 10000 files, which is far more than a theme needs

A refused upload leaves nothing behind. You cannot replace the
theme that is active, so deactivate it first or upload it under
another name.

## Pinning a theme instead

You can also unpack a theme into a directory named after it inside
the themes directory yourself, and name it in the environment:

```yaml
    environment:
      GOPHENBERG_THEMES_DIR: /themes
      GOPHENBERG_THEME: mytheme
```

A pinned theme wins over whatever the admin chose, and the admin
refuses to change it while the pin is set. Pinning also changes
what a broken theme costs you: a pinned theme that fails to load
stops the server from starting, admin included. A theme chosen in
the admin never does that. It shows as broken in the list and the
built-in renderer keeps serving.

The container image already carries the Node runtime themes run
on, so there is nothing else to install.
