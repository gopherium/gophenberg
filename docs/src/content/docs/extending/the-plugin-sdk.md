---
title: The plugin SDK
description: Everything the sdk package gives a plugin, and what it deliberately withholds.
---

The `sdk` package is the only Gophenberg package a plugin imports.
It is small on purpose, and this page is all of it.

## Deps

`Register` receives one value:

| Field | What it is |
| --- | --- |
| `DatabaseURL` | The PostgreSQL connection string. A plugin opens its own connection and owns its own schema |
| `Posts` | A read view of published posts |
| `Getenv` | Reads environment variables, for the plugin's own configuration |

## The lifecycle interfaces

`sdk.Plugin` is required: `ID`, `Start`, `Stop`. Three optional
interfaces add capabilities the host discovers automatically:
`Migrator` for database migrations, `RouteProvider` for HTTP under
`/api/plugins/{id}`, and `PublicPathProvider` for the exact paths
that answer without a login.

## Reading content

```go
posts, err := deps.Content.ListPublished(ctx, "post", 10)
```

Each `sdk.Item` carries `ID`, `Type`, `Slug`, `Title`, `Excerpt`,
`Content`, `PublishedAt`, and `UpdatedAt`. The `Content` has the
same HTML filter applied that the public API uses, block markers
intact. `Title` and `Excerpt` arrive as stored, so if your plugin
serves HTML, escaping everything but `Content` is your job.

## What the SDK withholds

The absences are deliberate, so build against them:

- **No content writes.** Plugins read published content, the
  editor is the one writer.
- **No user or session API.** The host guards your routes, and
  the SDK gives you nothing to act on accounts with.
- **No shared database pool.** You get the URL, you own your
  connections and your schema.
- **No content type registration.** A plugin cannot add its own
  content type.

## Admin screens for plugins

A plugin can add screens to the admin. Name the package in the
manifest's `frontend` field and export an object called `plugin`:

```ts
export const plugin = {
	id: 'hello',
	routes: (parent) => [/* routes, as children of parent */],
	nav: [{ label: 'Hello', to: '/hello', icon: someIcon }],
}
```

`make generate` wires it in: routes mount inside the admin layout
and nav rows appear after the built-in ones. No built-in plugin
uses this path yet, so expect to be the first through it.
