---
title: The plugin SDK
description: Everything the sdk package gives a plugin, and what it deliberately withholds.
---

The `sdk` package is the only Gophenberg package a plugin imports.
It is small on purpose, and this page is all of it. It names only
its own types and the standard library's, so a release of a module
the core uses never changes what a plugin compiles against.

The SDK is a Go module of its own,
`github.com/gopherium/gophenberg/sdk`. Its versions, such as
`v0.1.0`, come from git tags such as `sdk/v0.1.0`. A plugin kept in
its own repository requires that module alone, never the whole of
Gophenberg:

```sh
go get github.com/gopherium/gophenberg/sdk@latest
```

## Deps

`Register` receives one value:

| Field | What it is |
| --- | --- |
| `DB` | The plugin's share of the site's database, described [below](#the-database) |
| `DatabaseURL` | The PostgreSQL connection string, for a plugin that still opens its own connection |
| `Content` | A read view of published content |
| `Env` | Reads the settings under the `GOPHENBERG_` prefix, for the plugin's own configuration |
| `Getenv` | Reads any environment variable by its full name, as it is set |

`Env` takes a setting's name without the prefix and trims the
spaces around its value. `Value` returns the text, empty when the
setting is unset, and `Required` refuses an empty one. `Duration`,
`Count` and `Flag` read a duration above zero, a whole number above
zero, or true or false. `Counts` reads whole numbers above zero
split by commas, each above the one before, such as `10,50,100`.
Each returns the default you pass when the setting is empty.
`Within` narrows the prefix, so
`deps.Env.Within("FEED_").Count("ITEMS", 20)` reads
`GOPHENBERG_FEED_ITEMS`. Every error names the full setting, such
as `GOPHENBERG_FEED_ITEMS: must be a whole number, got "banana"`,
so `Register` can return it as it is.

## The lifecycle interfaces

`sdk.Plugin` is required: `ID`, `Start`, `Stop`. Five optional
interfaces add capabilities the host discovers automatically:
`Migrator` for database migrations, `RouteProvider` for HTTP under
`/api/plugins/{id}`, `PublicPathProvider` for the exact paths that
answer without a login, `TypeDeclarer` for content types, field
groups and fields the plugin brings with it, and `CommandProvider`
for [commands](#commands) on the `gophenberg` command line.

`Register` only checks the plugin's settings and builds it. It
opens nothing, no connection, no file and no goroutine. `Start`
does that. Commands such as `list`, `check` and `migrate` register
every plugin and stop it again without ever starting it. Only
`serve` starts the plugins. So `Stop` must work even when `Start`
never ran, and return by the time its context ends. When `list` or
`help` registers the plugins, `DatabaseURL` can be empty.

When a plugin fails to register or to declare its types, Gophenberg
stops every plugin that registered. When one fails to start, it
stops the ones already started. On the command line, a plugin that
fails to register shows under "Not loaded:" in `gophenberg list`,
and `check`, `migrate` and `seed -yes` fail.

## The database

`deps.DB` is the plugin's share of the site's one database pool. A
plugin never needs a pool of its own, so many plugins cannot use up
the connections PostgreSQL allows.

```go
_, err := p.db.Exec(ctx, "INSERT INTO greetings (name) VALUES ($1)", name)
```

- `Exec`, `Query` and `QueryRow` take PostgreSQL's `$1`, `$2`
  placeholders. `Begin` starts a transaction that offers the same
  three, plus `Commit` and `Rollback`. `Engine` names the engine,
  `sdk.Postgres` today.
- Each statement, open result set and transaction holds one slot
  until it ends, so close every `Rows`. The share holds
  `GOPHENBERG_PLUGIN_DB_CONNS` slots for each plugin, 4 by default.
  Today all plugins draw from one share of that size.
- The share always leaves `GOPHENBERG_PLUGIN_DB_RESERVE` of the pool's
  connections to the site, 2 by default, so it may hold fewer slots
  than the plugins add up to. A statement that finds no free slot
  before its deadline fails.
- A statement ends at `GOPHENBERG_PLUGIN_QUERY_TIMEOUT`, 5 seconds by
  default, its wait for a slot included. A transaction ends at
  `GOPHENBERG_PLUGIN_TX_TIMEOUT`, 30 seconds by default.
- The share opens once every plugin registered. Keep `deps.DB` in
  `Register` and use it from `Start` on. A statement during
  `Register` is refused.

## Reading content

```go
posts, err := deps.Content.ListPublished(ctx, "post", 10)
```

Each `sdk.Item` carries `ID`, `Type`, `Path`, `Slug`, `Title`,
`Excerpt`, `Content`, `Fields`, `PublishedAt`, and `UpdatedAt`.
`ID` is an `sdk.ID`, sixteen bytes whose `String` gives the
36 character form, such as `019fb000-0000-7000-8000-000000000001`.
JSON carries an `sdk.ID` as that same text, in both directions.
`Path` is the item's public address, so a plugin building links
prefixes it with `/` and nothing else. The `Content` has the same
HTML filter applied that the public API uses, block markers intact.
`Title` and `Excerpt` arrive as stored, so if your plugin serves
HTML, escaping everything but `Content` is your job.

`Fields` holds the item's field values keyed by field key, shaped
the way the [content API](/reference/content-api/) serves them and
decoded the way `encoding/json` decodes them. A media value is an
object naming the file, a relation lists the items it points at,
and a Linked from field lists the items pointing this way. They are
data, not markup, so escape them too before serving them as HTML.

## Declaring content

A plugin that implements `TypeDeclarer` is handed a `TypeRegistrar`
once at every start, before anything serves:

```go
func (p plugin) DeclareTypes(ctx context.Context, types sdk.TypeRegistrar) error {
	if err := types.DeclareType(ctx, sdk.TypeDeclaration{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
		Description: "Gatherings near you.",
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
description, required flag, setting or location is carried onto it.
Two things are refused: changing a field's kind, and changing a
type's route word, because both would strand stored content. A
definition the plugin stops declaring stays in place.

What a plugin declares belongs to that plugin. The admin shows it
with a badge naming the plugin and offers no way to change or delete
it, though it can still be turned off. If the site already holds a
type or group under the same key, the plugin's declaration is
skipped and the start log says so.

## Commands

A plugin that implements `CommandProvider` offers commands on the
`gophenberg` command line. Each command's name is the plugin id, a
colon, then the command:

```go
func (p plugin) Commands() []sdk.Command {
	return []sdk.Command{{
		Name:    "hello:greet",
		Summary: "print a greeting for one name",
		Args:    []string{"name"},
		Run: func(ctx context.Context, call sdk.Call) error {
			name := strings.TrimSpace(call.Args[0])
			if name == "" {
				return sdk.Misuse(errors.New("hello:greet wants a name that is not blank"))
			}
			_, err := fmt.Fprintf(call.Stdout, "hello, %s\n", name)
			return err
		},
	}}
}
```

`gophenberg list` shows it under the plugin id,
`gophenberg help hello:greet` prints its page, and
`gophenberg hello:greet Maria` runs it.

| Field | What it is |
| --- | --- |
| `Name` | `<plugin id>:<command>`, in lowercase words joined by hyphens |
| `Summary` | The one line the listing prints beside the name |
| `Args` | The names of the positional arguments, in order, each one required |
| `Flags` | Declares the command's own flags on a `flag.FlagSet`. The flags `h`, `help`, `yes`, `json` and `as` belong to the command line |
| `Needs` | The names of the flags every run must set, each one declared by `Flags` and taking a value |
| `Writes` | Makes the command a dry run until `-yes` |
| `JSON` | Offers `-json` |
| `Capability` | One of the capabilities the built-in roles carry, such as `manage_users`, which adds `-as <email>` |
| `Run` | Does the work |

`Run` receives a `sdk.Call`. `Args` holds the arguments, and `Flags`
maps each of the command's own flags the line set to its value as
text. `Stdout` takes the answer, `Stderr` takes progress and
warnings, and `Stdin` holds any input. `call.JSON` reports whether
`-json` was passed, and `call.Encode` writes one JSON document to
`Stdout`. A command reads its settings and the database address
from the `Deps` the plugin kept at `Register`. `Start` never ran,
so a command opens what it needs inside `Run` and closes it before
it returns.

A command that sets `Writes` runs on every call, but `call.Apply`
stays false until `-yes`. The command line does not stop a write,
so `Run` checks `call.Apply` and, until it is true, prints what it
would change and changes nothing. The command line then adds
`dry run, nothing changed, pass -yes to apply` on stderr.

A command that names a `Capability` is refused before `Run` unless
the `-as` account exists, is enabled and activated, and holds a
role with that capability. Each run it applies is stored as one
record, which `gophenberg account:records` lists. Plugins cannot add
capabilities. Name one the roles already carry, `manage_users`,
`manage_themes`, `manage_types`, `manage_settings` or
`change_others_work`, or every account is refused.

An error from `Run` exits with code 1. Wrap it in `sdk.Misuse` when
the command line itself is wrong, a bad argument value for example.
It then exits with code 2 and prints the command's help under the
error. A missing argument or an unknown flag is already refused that
way. A command that breaks a rule, such as a name outside its
plugin's id, a malformed or repeated name, an empty summary, no
`Run`, a flag the command line owns, or a needed flag it does not
declare, is dropped, and `gophenberg check` names it. The
[commands](/self-hosting/commands/) page shows all of this from the
operator's side.

## Flags a run must set

`Flags` declares the command's own flags with Go's standard `flag`
package, and `Needs` names the ones every run must set. This command
of an `archive` plugin brings back the rows archived since the day the
line names:

```go
// restore returns archive:restore, which brings back the rows archived since the day -since names.
func (p plugin) restore() sdk.Command {
	return sdk.Command{
		Name:    "archive:restore",
		Summary: "bring back the rows archived since one day",
		Flags: func(fs *flag.FlagSet) {
			fs.String("since", "", "first `day` to bring back, such as 2026-10-01")
		},
		Needs:  []string{"since"},
		Writes: true,
		Run: func(ctx context.Context, call sdk.Call) error {
			day := call.Flags["since"]
			since, err := time.Parse(time.DateOnly, day)
			if err != nil {
				return sdk.Misuse(fmt.Errorf("archive:restore: -since %q is not a day like 2026-10-01", day))
			}
			return p.bringBack(ctx, call, since)
		},
	}
}
```

Add `p.restore()` to the list `Commands` returns. A line that leaves
`-since` out, or gives it empty text or only spaces, exits 2 with
`gophenberg: archive:restore wants -since <day>` and the help page.
The command line checks this as soon as it has read the line, before
it calls `Run`, so `Run` never has to look for a missing `-since`
itself. `gophenberg archive:restore -h` still prints the help page.

The placeholder `day` is the word in backquotes in the flag's usage.
Without backquotes it names the kind of value, such as `string` or
`int`, or just `value`.

The command line checks the text the line types for the flag, not the
value the flag reads back. So a flag declared with `fs.Func`, which
keeps no value of its own, works in `Needs` too. A default does not
count. A flag declared as `fs.Int("batch", 500, ...)` and named in
`Needs` still stops a line that leaves `-batch` out. Each name in
`Needs` must be a flag that `Flags` declares and that takes a value,
unlike a `bool` flag. Otherwise the command line drops the command,
`gophenberg check` names it, and running it exits 1.

`Needs` only checks that a value is there. To stop a bad value, such
as `-since soon`, `Run` returns the error wrapped in `sdk.Misuse`, as
above, so the run exits 2 with the help page the same way.
[Writing commands](https://docs.gopherium.org/command-line/writing-commands/)
covers `Needs` in full.

## The signed-in account

A request reaches your routes only with a login, except on the
paths `PublicPaths` names. The host files the account on the
request's context, and `sdk.SessionFrom` reads it:

```go
session, ok := sdk.SessionFrom(r.Context())
if !ok || !session.Can("manage_settings") {
	http.Error(w, "forbidden", http.StatusForbidden)
	return
}
```

`ok` is false on a public path, where nobody has to sign in. An
`sdk.Session` carries `ID`, an `sdk.ID` like an item's, and the
account's `Email`, `Name` and `Role`. `Can` reports whether the
account's role holds a capability, such as `manage_users`.

## What the SDK withholds

The absences are deliberate, so build against them:

- **No content writes.** Plugins read published content, the
  editor is the one writer.
- **No account changes.** You read the signed-in account, and the
  SDK gives you nothing to change accounts with.
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

The frontend SDK, `@gophenberg/frontend-sdk`, passes on a few icons
from `@wordpress/icons`, each named after the job it does in the
admin: `backIcon`, `backupIcon`, `downIcon`, `listViewIcon`,
`redoIcon`, `trashIcon`, `undoIcon` and `upIcon`. Import them from
the SDK, so the admin keeps one copy of the icon set:

```ts
import { trashIcon } from '@gophenberg/frontend-sdk'
```

To confirm an action, open a `Dialog` that holds the question, a
**Cancel** button and a button named after the action.
