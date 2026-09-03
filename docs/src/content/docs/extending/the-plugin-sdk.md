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
| `Content` | A read view of published content |
| `Getenv` | Reads environment variables, for the plugin's own configuration |

## The lifecycle interfaces

`sdk.Plugin` is required: `ID`, `Start`, `Stop`. Four optional
interfaces add capabilities the host discovers automatically:
`Migrator` for database migrations, `RouteProvider` for HTTP under
`/api/plugins/{id}`, `PublicPathProvider` for the exact paths that
answer without a login, and `TypeDeclarer` for content types,
field groups and fields the plugin brings with it.

## Reading content

```go
posts, err := deps.Content.ListPublished(ctx, "post", 10)
```

Each `sdk.Item` carries `ID`, `Type`, `Path`, `Slug`, `Title`,
`Excerpt`, `Content`, `Fields`, `PublishedAt`, and `UpdatedAt`.
`Path` is the item's public address, so a plugin building links
prefixes it with `/` and nothing else. The `Content` has the same
HTML filter applied that the public API uses, block markers intact.
`Title` and `Excerpt` arrive as stored, so if your plugin serves
HTML, escaping everything but `Content` is your job.

`Fields` holds the item's field values keyed by field key, exactly
as stored. They are data, not markup, so escape them too before
serving them as HTML. A relation field is not among them.

## Declaring content

A plugin that implements `TypeDeclarer` is handed a `TypeRegistrar`
once at every start, before anything serves:

```go
func (p plugin) DeclareTypes(ctx context.Context, types sdk.TypeRegistrar) error {
	if err := types.DeclareType(ctx, sdk.TypeDeclaration{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
	}); err != nil {
		return err
	}
	return types.DeclareGroup(ctx, sdk.GroupDeclaration{
		Key:      "event-details",
		Title:    "Event details",
		Location: [][]sdk.Rule{{{Source: "content_type", Operator: "==", Value: "event"}}},
		Fields:   []sdk.FieldDeclaration{{Key: "venue", Label: "Venue", Kind: "text"}},
	})
}
```

Declaring is safe to repeat. A definition that is not there yet is
created, one that is there is left alone, and a changed label,
required flag, setting or location is carried onto it. Two things
are refused: changing a field's kind, and changing a type's route
word, because both would strand stored content. A definition the
plugin stops declaring stays in place.

What a plugin declares belongs to that plugin. The admin shows it
with a badge naming the plugin and offers no way to change or delete
it, though it can still be turned off. If the site already holds a
type or group under the same key, the plugin's declaration is
skipped and the start log says so.

## What the SDK withholds

The absences are deliberate, so build against them:

- **No content writes.** Plugins read published content, the
  editor is the one writer.
- **No user or session API.** The host guards your routes, and
  the SDK gives you nothing to act on accounts with.
- **No shared database pool.** You get the URL, you own your
  connections and your schema.

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
