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

[Versions](#versions), at the end of this page, says what changed in
each version and how to move your plugin to it.

## Deps

`Register` receives one value:

| Field | What it is |
| --- | --- |
| `DB` | The plugin's share of the site's database, described [below](#the-database) |
| `DatabaseURL` | The PostgreSQL connection string, for a plugin that still opens its own connection. Empty on a SQLite database |
| `Content` | A read view of published content |
| `Accounts` | Reads the names, roles and avatars of many accounts in one call, described [below](#reading-account-names) |
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

`sdk.Plugin` is required: `ID`, `Start`, `Stop`. Seven optional
interfaces add capabilities the host discovers automatically:
`Migrator` for database migrations, `RouteProvider` for HTTP under
`/api/plugins/{id}`, `PublicPathProvider` for the exact paths that
answer without a login, `TypeDeclarer` for content types, field
groups and fields the plugin brings with it, `CommandProvider`
for [commands](#commands) on the `gophenberg` command line,
`RoleProvider` for the [roles and capabilities](#roles-and-capabilities)
the plugin brings, and `HeadContributor` for tags in the head of
every page, through a [hook](#hooks).

`Register` only checks the plugin's settings and builds it. It
opens nothing, no connection, no file and no goroutine. `Start`
does that. Commands such as `list`, `check` and `migrate` register
every plugin and stop it again without ever starting it. Only
`serve` starts the plugins. So `Stop` must work even when `Start`
never ran, and return by the time its context ends. When `list` or
`help` registers the plugins, `DatabaseURL` can be empty. On a SQLite
database it is always empty.

`serve` takes the plugins one by one. When a plugin fails to
register, a panic in `Register` included, or fails to declare its
roles, to declare its types, to match the hooks it lists, to hand
over its routes, to migrate or to start, `serve` pauses that plugin
and the rest of the site starts. A paused plugin's routes are not mounted, its roles are
left out, and every start tries it again, see
[a plugin that fails at start](/self-hosting/configuration/#a-plugin-that-fails-at-start).
A panic in one of its routes while it answers a request pauses
nothing. The visitor gets the site's internal error, and `serve`
logs the plugin's name with the stack, see
[a plugin whose route panics](/self-hosting/configuration/#a-plugin-whose-route-panics).
`serve` also logs a warning when a plugin's `Start` takes longer
than `GOPHENBERG_PLUGIN_START_BUDGET`, 10 milliseconds unless set,
though the plugin still starts.

Every other command registers every plugin and pauses none. On the
command line, a plugin that fails to register shows under
"Not loaded:" in `gophenberg list`, and `check`, `migrate` and
`seed -yes` fail. The error names the plugin and the step, such as
`pluginkit: feed register:` in front of what `Register` returned.

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
  `sdk.Postgres` on PostgreSQL and `sdk.SQLite` on SQLite.
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
- On a SQLite database the share stays closed for now. `Exec`,
  `Query`, `QueryRow` and `Begin` each answer
  `the plugin database share is closed while GOPHENBERG_DATABASE_URL names a SQLite database`.
  A `Migrate` that runs through `deps.DB` gets the same error, so
  `gophenberg migrate` stops there. `Engine` tells a plugin which
  database it runs on before it sends a statement.
- On a SQLite database the content reads of `Deps` are not ready yet
  either, so check `deps.DB.Engine()` first and skip them when it
  answers `sdk.SQLite`. The account reads work there.

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

## Reading account names

`deps.Accounts` turns account ids into the names people see. Ask
for every id a page needs in one call, with the avatar size you
draw, so a page of fifteen authors costs one read:

```go
people, err := deps.Accounts.People(ctx, authorIDs, 96)
```

`People` works the same way on a SQLite database.

The answer is a map keyed by `sdk.ID`. Each `sdk.Person` carries
`ID`, `Name`, `Role`, `Avatar` and `Disabled`. `Name` is the display
name. `Role` is the stored role, such as `member`. Build profile
links from `ID`.

`Avatar` is an image address at the size you asked, from 1 to 2048
pixels. A size outside that range is refused. For sharp screens, ask
for twice the size you draw, and ask once with the largest size the
page shows. The site's [avatar settings](/self-hosting/configuration/#avatars)
decide what the address holds. By default it is a
`data:image/svg+xml;base64,` address that holds the picture itself,
so the browser makes no request. With Gravatar it points at
`secure.gravatar.com`. With avatars off it is empty, so draw no
picture then. Put the address in an `<img src>` as it is.
`html/template` writes `#ZgotmplZ` in place of a `data:` address, so
pass it as `template.URL(person.Avatar)`. The site builds every
avatar address, so it is safe to mark:

```go
data := struct{ Avatar template.URL }{template.URL(person.Avatar)}
```

`Disabled` is true for an account an administrator turned off. Show
its name, link nowhere, and show your own word for its role, such as
"Inactive".

An id the site does not hold is missing from the map, and so is an
account that never confirmed its address. Show your own words for
it, such as "Former member". An id asked twice comes back once, and
asking for no ids reads nothing. Keep `deps.Accounts` in `Register`
and use it from `Start` on. A read during `Register` is refused, and
a read ends at `GOPHENBERG_PLUGIN_QUERY_TIMEOUT`.

A test that fakes the reader by embedding `sdk.HostOnly` stops
compiling when the reader gains a method.

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
| `Capability` | A capability the core or a plugin declares, such as `manage_users` or `notes.manage`, which adds `-as <email>` |
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
record, which `gophenberg account:records` lists. Name a capability
the core declares, such as `manage_users`, or one your plugin
declares through [`RoleProvider`](#roles-and-capabilities), such as
`notes.manage`. No account carries a capability nobody declares, so
a command naming one refuses every account.

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

## Roles and capabilities

A capability is one named permission, such as `notes.manage`. A role
is a set of capabilities an account holds. The core declares four
roles, `admin`, `editor`, `author` and `member`, and seven
capabilities. A plugin adds its own through `RoleProvider`, which
returns them as data:

```go
// Roles returns the role the notes plugin brings and the capability it adds to editors.
func (plugin) Roles() []sdk.RoleDeclaration {
	return []sdk.RoleDeclaration{
		{Name: "archivist", Capabilities: []string{"notes.manage"}},
		{Name: "editor", Capabilities: []string{"notes.edit"}},
	}
}
```

The first entry brings a new role. The second adds a capability to a
role the core already has. In WordPress, these are `add_role` and
`add_cap`. The site declares every plugin's roles into one
[registry](https://docs.gopherium.org/authorization/plugin-roles/),
under these rules:

- **A new capability starts with your plugin's id and a dot,** such
  as `notes.manage`, so two plugins never share one by accident. You
  may still grant a capability that already exists, such as the
  core's `manage_users`.
- **The administrator carries every capability a plugin declares,**
  so it can hand out your roles.
- **Names** use lowercase letters, digits, `_`, `.` and `-`, start
  with a letter, and hold at most 64 characters. Role names take no
  prefix, and two plugins that declare the same role widen it
  together.
- **Order matters.** Plugins declare in the order of their folders
  under `plugins/`. A plugin that grants another plugin's capability
  must come after it, so name it after its parent, such as
  `notes-tags` after `notes`. The id `core` is reserved.
- **A declaration that breaks a rule pauses your plugin** when
  `serve` starts, so the site serves without its routes and its
  roles. It stops `check`, `migrate`, `seed -yes`, every plugin
  command and every account command except `account:list` and
  `account:records`, and the error names your plugin:
  `plugin notes: goncierge: new capability outside the source's prefix "notes.": "manage_archive"`.

Ask for a capability, never a role name: `session.Can("notes.manage")`
in a route, and `Capability: "notes.manage"` on a command. The
account commands take your roles too:

```sh
gophenberg account:role maria@example.com archivist -yes -as admin@example.com
```

An account whose role nobody declares any more, after its plugin is
removed, keeps the stored name and carries no capability.

The admin opens only for a role carrying `access_admin`, which the
core's `admin`, `editor` and `author` carry. The core's `member`
carries no capability: it signs in and keeps a session, but never
enters the admin. Your routes see members too, so ask `session.Can` before
a member acts, the way a forum asks whether it may post. A role you
declare for the admin needs `access_admin` beside its own
capabilities, and granting `access_admin` opens the admin's content
and media to every account holding that role.

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
account's role holds a capability, such as `manage_users` or one a
plugin declares.

## Hooks

A hook is a moment where the site lets plugins join in, the way
WordPress calls `wp_head` while it draws a page. A plugin joins a
hook by having its method and listing the hook's name under `hooks`
in its `plugin.json`:

```json
{
  "id": "hello",
  "name": "Hello",
  "backend": "github.com/gopherium/gophenberg/plugins/hello",
  "hooks": ["public.head-tags"]
}
```

At start, `serve` checks the list against the plugin's methods. A
listed hook with no method, a method whose hook is not listed, a
name no hook has or a name listed twice pauses the plugin at the
step `hooks`. The site then calls each hook only for the plugins
that started and implement it, in the order the plugins are
compiled.

| Hook | Interface and method | When the site calls it |
| --- | --- | --- |
| `public.head-tags` | `HeadContributor`, `HeadTags(ctx, sdk.PageMeta) ([]sdk.HeadTag, error)` | Once for each page the built-in site draws |

`PageMeta` says which page is drawn: its `Kind`, which is `archive`,
`item`, `term` or `notfound`, the content type's key in `Type`, the
address in `Path`, the listing page in `Page`, the item of an item or
term page in `Item`, with its `Content` and `Fields` left empty, and
`SiteTitle`. The feed links its channel this way:

```go
// HeadTags links the channel from the head of every page, titled with the site's name.
func (p *plugin) HeadTags(_ context.Context, page sdk.PageMeta) ([]sdk.HeadTag, error) {
	return []sdk.HeadTag{{Tag: "link", Attrs: []sdk.Attr{
		{Key: "rel", Value: "alternate"}, {Key: "type", Value: "application/rss+xml"},
		{Key: "title", Value: page.SiteTitle}, {Key: "href", Value: sdk.PluginRoutesPrefix + pluginID + feedPath},
	}}}, nil
}
```

The site writes the tags itself. A tag must be `meta` or `link`. Its
attribute names are lowercase letters, digits and hyphens with a
letter first, none `style` or starting with `on`. Its `href` or `src`
is an `http` or `https` address or a path on the site, with no space
or control character. The site drops any other tag and keeps the
plugin's other tags. Tags reach the pages the built-in site draws. A
theme draws its own head.

All the plugins of one call share `GOPHENBERG_PLUGIN_HOOK_BUDGET`, 2
milliseconds unless set, as the deadline of the context they get, so
return when it ends. Go cannot stop a running function, and a plugin
that ignores its context holds the page until it returns. An error
or a panic inside a hook drops what the plugin answered in that call,
the site logs it under the plugin's name and still draws the page,
see
[a plugin whose hook fails](/self-hosting/configuration/#a-plugin-whose-hook-fails).

## What the SDK withholds

The absences are deliberate, so build against them:

- **No content writes.** Plugins read published content, the
  editor is the one writer.
- **No account changes.** You read the signed-in account, and the
  SDK gives you nothing to change accounts with.
- **No other account's email address.** `Accounts` gives names,
  never addresses. The signed-in account's own `Email` stays on its
  `Session`.

## Admin screens for plugins

A plugin can add screens to the admin. Name the package in the
manifest's `frontend` field and export an object called `plugin`:

```ts
export const plugin = {
	id: 'hello',
	routes: (parent) => [/* routes, as children of parent */],
	nav: [{ label: 'Hello', to: '/hello', icon: someIcon, capability: 'hello.manage' }],
}
```

`make generate` wires it in: routes mount inside the admin layout
and the rows of `nav` appear in the sidebar after the built-in ones.
Each row is a `MenuItem`: the `label` it shows, the address `to`, and
an optional `icon`, `capability` and `count`. No built-in plugin uses
this path yet, so expect to be the first through it.

A `count` is `{ value, label }`: the number drawn after the label,
hidden at zero, and the translated sentence a screen reader reads for
it, such as `1 update available`. The core's Updates row counts the
update that waits this way. A plugin's `nav` is plain data, so a count
there stays the same while the admin is open.

A row naming a `capability` shows only to an account whose role
carries it. The screen still opens from a pasted link, so the Go
routes your plugin serves must check `session.Can` themselves, as
[the signed-in account](#the-signed-in-account) shows. In the
browser, a screen asks the same question through the frontend SDK's
`can(session, capability)`, which reads the capabilities the
signed-in account carries:

```ts
import { can, useSession } from '@gophenberg/frontend-sdk'

const session = useSession().data
const manages = can(session, 'hello.manage')
```

The SDK names the core's capabilities as constants, such as
`MANAGE_USERS` and `UPDATE_CORE`, so `can(session, UPDATE_CORE)`
asks whether the account may update Gophenberg.

A row can open a sub menu of its own, as the Site Editor's Navigation
row opens the list of menus. Give the route a `staticData.Sidebar`
component that draws a `NavigationScreen`. The admin swaps the main
menu for it, slides it in, and puts Back at its top, which leads to
the list the admin opens on. Put its rows in a `NavigationList`, each
one a `NavigationItem`. A sub menu on a parent route stays while the
reader moves between the screens under it:

```tsx
import { NavigationItem, NavigationList, NavigationScreen } from '@gophenberg/frontend-sdk'
import { createRoute } from '@tanstack/react-router'
import type { AnyRoute } from '@tanstack/react-router'

function HelloSidebar() {
	return (
		<NavigationScreen title="Hello">
			<NavigationList>
				<NavigationItem to="/hello/drafts">Drafts</NavigationItem>
				<NavigationItem to="/hello/archive">Archive</NavigationItem>
			</NavigationList>
		</NavigationScreen>
	)
}

export const plugin = {
	id: 'hello',
	routes: (parent: AnyRoute) => [
		createRoute({
			getParentRoute: () => parent,
			path: '/hello',
			staticData: { Sidebar: HelloSidebar },
		}),
	],
	nav: [{ label: 'Hello', to: '/hello' }],
}
```

Leave out the screen's `back`, so Back goes where the admin's own sub
menus go. `NavigationList` also takes `items`, rows as `MenuItem`
data, and hides a row whose `capability` the reader lacks. A
`NavigationItem` hides itself when its address opens a route the
reader may not open. `DrilldownItem` draws a row that opens a further
sub menu, with a chevron, and `MenuCount` draws a number at the end of
a row, such as the updates waiting.

`renderPluginAt` from `@gophenberg/frontend-sdk/testing` mounts your
plugin inside the admin's real sidebar, with Back, the focus moves and
the phone drawer. Pass `session` to seed the account a test reads, or
`null` for none. A row that names a `capability` shows only when that
account carries it. Leave `session` out and the test asks the server
for the account at `/api/auth/session`, so serve that address with
`http` and `server` from the same entry.

**To move** a plugin written for Gophenberg v0.19.0 or earlier:

1. Replace the type `NavItem` with `MenuItem` from
   `@gophenberg/frontend-sdk`. `NavItem` is gone, with no alias.
2. If a route declares `staticData.Sidebar`, draw it as a
   `NavigationScreen` with its rows in a `NavigationList`, so it gets
   its title, Back and the focus the admin moves to it.
3. In a test through `renderPluginAt`, seed a session that carries
   the capability a row names, or serve the session address. The test
   host now hides a row the reader cannot open, and asks the server
   for the session when the test seeds none.
4. Remove `StyleProvider` and keep what it wrapped. The SDK no longer
   passes it on, with no alias, since the WordPress components now
   load their own styles into the admin page.
5. In your plugin's `package.json`, take `msw` 3 and `vitest` 5.0.4
   or newer. The `http` and `server` that
   `@gophenberg/frontend-sdk/testing` hands out now need them.
6. If your plugin imports a `@wordpress` package itself, take the
   major version the SDK pins. Every one of them moved a major, such
   as `@wordpress/components` from 42 to 43 and `@wordpress/ui` from
   0.24 to 0.25.
7. If your plugin imports `@gopherium/godmin` itself, move it from
   0.18 to 0.31 as this step says, then follow the list for v0.20.1
   below. `useToaster().name` is gone, so put the whole name in the
   message. `ListEmpty` takes `description` in place of `hint`, and
   `actions` for its buttons. The SDK passes on both, so you can
   import them from it instead. `LoadingRows` is gone, so pass the
   list's loading state to `isLoading` on DataViews, or show
   `LoadingScreen`. `NavScreen` and `NavScreenProps` are now
   `NavigationScreen` and `NavigationScreenProps`, which the SDK
   passes on. `Frame.Rail` and `FrameRailProps` are `Frame.Sidebar`
   and `FrameSidebarProps`, and `RAIL_BREAKPOINT` is
   `SIDEBAR_BREAKPOINT`. godmin now needs `@tanstack/react-router`
   1.170.44 or newer.
8. If your plugin's id is `about`, `update`, `server`, `plugin`,
   `theme`, `setup`, `build` or `health`, give it another one. These
   ids are now reserved, and `make generate` stops on them with
   `id "update" is reserved`. Rename its folder under `plugins/` to
   match, and start the names of its commands and of the capabilities
   it declares with the new id.

**To move** a plugin written for Gophenberg v0.20.1 or earlier:

1. Import `ConfirmDialog` and `FormDialog` from
   `@gophenberg/frontend-sdk`. The `/dialogs` entry is gone.
2. Give `ConfirmDialog` the question as `title` and what will happen
   as `description`, in place of its children. `size` is gone.
3. `RunConfirm`, `ConfirmBody`, `FormBody`, `RenameBody`,
   `useCloseWhileOpen` and their props types are gone, with no alias.
   A dialog over list rows is now an action `callback` that stores the
   rows, and a dialog your screen renders from them, as shown below.
4. `FormDialog` has no `kind` and no `size`. It closes itself when the
   promise from `onSubmit` ends, so stop closing it from your handler.
5. In a test, find a confirm by the role `alertdialog`, not `dialog`.
   `onClose` runs once the card has left the screen, so wait for it,
   such as with `waitFor`.
6. If your plugin imports `@gopherium/godmin` itself, take 0.32.0, the
   version the SDK pins.

The frontend SDK, `@gophenberg/frontend-sdk`, passes on a few icons
from `@wordpress/icons`, each named after the job it does in the
admin: `audioIcon`, `authorIcon`, `backIcon`, `backupIcon`,
`brushIcon`, `categoryIcon`, `closeIcon`, `customPostTypeIcon`,
`downIcon`, `fileIcon`, `imageIcon`, `languageIcon`, `listIcon`,
`listViewIcon`, `mediaIcon`, `pageIcon`, `postIcon`, `redoIcon`,
`saveIcon`, `settingsIcon`, `sidebarIcon`, `sidebarRtlIcon`,
`trashIcon`, `undoIcon`, `upIcon`, `updateIcon`, `uploadIcon` and
`videoIcon`. `authorIcon` is the one an
author cell shows when the site shows no avatars, `postIcon` the one
an empty content list shows, `mediaIcon` the one an empty media
library shows, and `uploadIcon` the one an upload button carries.
`imageIcon`, `videoIcon`, `audioIcon` and `fileIcon` are the ones a
media card draws for a file that shows no picture, picked by its
file type as WordPress picks them.
`sidebarIcon` is the one the editor's Settings button carries, and
`sidebarRtlIcon` its mirror for a language written right to left.
`saveIcon` is the one Save draft shows on a phone, and `closeIcon`
the one a panel's close button carries.
The sidebar rows carry the icons WordPress draws for the same rows:
`postIcon` for Posts and every type a site adds, `pageIcon` for
Pages, `categoryIcon` for Categories, `mediaIcon` for Media,
`brushIcon` for Themes, `customPostTypeIcon` for Content Types,
`listIcon` for Field Groups, `languageIcon` for Language,
`settingsIcon` for Settings and `updateIcon` for Updates.
Import them from the SDK, so the admin keeps one copy of the icon set:

```ts
import { trashIcon } from '@gophenberg/frontend-sdk'
```

A write that fails says why. A dialog that stays open says it inside.
A dialog that closes, and a write with no dialog, say it in a toast
at the bottom of the screen, as WordPress does. `useToaster` from the
SDK hands you the admin's own toasts:

```ts
const toaster = useToaster()

toaster.show(failure.message)
```

`ConfirmDialog` and `FormDialog` from `@gophenberg/frontend-sdk` are
the admin's own dialogs. Each is a card in the middle of the screen,
a phone too. A confirm asks one question before an action, with
Cancel focused. Give it the question as `title` and what will happen
as `description`. Add `intent="irreversible"` for an action that
cannot be undone, which draws its button red. Render it only while
you ask:

```tsx
import { Button, ConfirmDialog, useToaster } from '@gophenberg/frontend-sdk'

const remove = () => {
	setRemoving(true)
	removeItem(item.id)
		.then(() => toaster.show(done))
		.catch((failure) => toaster.show(failure.message))
		.finally(() => setRemoving(false))
}

<Button variant="outline" loading={removing} onClick={() => setAsking(true)}>{label}</Button>
{asking ? (
	<ConfirmDialog
		intent="irreversible"
		title={question}
		description={warning}
		confirmLabel={label}
		cancelLabel={cancel}
		onConfirm={remove}
		onClose={() => setAsking(false)}
	/>
) : null}
```

What `onConfirm` returns decides when the dialog goes. A plain
function that returns nothing, as `remove` is, closes it at once
while your write runs, and the button that opened it shows the work.
An `async` function always returns a promise, so keep it plain. Return
the write's promise instead, and the dialog stays open with its button
busy until the promise ends. Cancel and Escape close nothing until
then. Answer `{ error }` with words, at once or through the promise,
and the dialog stays open and shows them. `{ close: false }` keeps it
open with nothing said. `DialogAnswer` types both answers. `onClose`
runs once the card has left the screen, so take the dialog away there.

`FormDialog` asks for a short form, as WordPress opens Add Page: a
title, an optional `description` before the fields, and the first
field focused. `onSubmit` answers as `onConfirm` does, and the dialog
closes itself when its promise ends, so call nothing to close it.
Answer `{ error }` to stay open with the words and what the reader
typed. Pass a function as its children to read `busy`, such as to
make a field `readOnly` while the write runs.

The SDK still passes on `AlertDialog` and `Dialog` from
`@wordpress/ui`. The two dialogs above hold the rules for a write, so
prefer them.

`Popover` from `@gophenberg/frontend-sdk` is the one from
`@wordpress/ui`. It anchors a small panel to a button, and it paints
above a dialog. The `Popover` from `@gophenberg/frontend-sdk/editor`
is a different one, from `@wordpress/components`. Reach for it inside
the block editor, and for its `Popover.Slot`, which a dialog needs to
show the controls a components overlay draws.

A block that fails at something, such as an upload the server
refused, leaves a message for the block editor and draws nothing
itself. `useBlockNotices` from `@gophenberg/frontend-sdk/editor`
hands you each message, so a screen holding the block editor can show
it, in a toast for example.

A field the reader has to fill, or that the server may refuse, draws
through `ValidatedInputControl` from `@gophenberg/frontend-sdk`, as
the Email and Name of Add User do. Give it `required` and its label
ends in `(Required)`. Its message shows under the field once the
reader leaves it or sends the form. To show the server's reason
about that field there, such as a taken email, pass
`customValidity={{ type: 'invalid', message }}`. Then call the
form's `reportValidity()` in an effect that runs once the message is
set, as Add User does. The reader may have sent the form with Enter
from that very field. The field was never left, so its message stays
hidden. `reportValidity()` shows it and moves the focus to the field.
Set `customValidity` back to `undefined` once the reader changes the
field, since the browser holds back a form while one of its fields
is invalid. A failure about no one field still goes to a toast.

A dialog over list rows lives in your screen, not in DataViews. Give
the row action a `callback` that stores the rows, and render the
dialog from them. DataViews calls the same `callback` from a row's
actions menu and from the bulk bar.

`useListRun`, `useBulkRun` and `useDialogRun` from
`@gophenberg/frontend-sdk` are the hooks the admin's own lists run
their actions with:

```tsx
const { run, refreshing } = useListRun('notes', setSelection)
const bulk = useBulkRun<Note>(run, refresh)
const write = useDialogRun<Note>(run, refresh)
```

`useListRun` takes the name of the list on show and the setter of its
ticked rows. It holds the ticked rows and the focus after a run. Pass
its `refreshing` to the `isLoading` of your DataViews beside your own
loading state, so the old rows stay on screen, dimmed, while the list
reloads. `bulk` and `write` take its `run` and the function that
reloads your list.

`bulk` makes one call per picked row. The list shows as reloading as
the first call that works ends. One toast shows as the last call
ends, in the words you pass as `BulkWords`: the failure words when a
row failed, else the done words. Your list then reloads. A confirm
over the rows returns that run from `onConfirm`, so it stays open
until the run ends.

An action that can be undone asks nothing, so a trash runs at once.
Give its words an `undo`, and the done toast offers it once every row
worked:

```tsx
callback: (notes) =>
	bulk(notes, (note) => trashNote(note.id), {
		...trashWords,
		undo: (done) => ({ label: 'Undo', onAct: () => void bulk(done, (note) => restoreNote(note.id), restoreWords) }),
	})
```

As the list starts to reload, its region takes the focus from the
table or the footer, and after a run started from the keyboard it
draws the focus ring. Once the new rows land, the focus goes back to
the control the reader pressed, or to the first control of the row in
its place. It stays on the region when that control is a primary
action, so a one click action of your plugin never takes a second
Enter meant for another row.

For a write that is no run, show its toast as the call ends, then
mark the list around your reload:

```tsx
const ended = run.onRefresh()
await refresh().then(ended, ended)
```

A form over rows writes through `write`, which takes what `bulk`
takes and answers the dialog. When its one row failed, the dialog
stays open with your failure words and what the reader typed. Over
several rows the dialog closes, and one toast says how the run went.
`RenameDialog` asks for a new name. Its button stays on, and the
browser refuses a blank name. Label it **Save**, as WordPress does:

```tsx
import { RenameDialog } from '@gophenberg/frontend-sdk'
import type { BulkWords } from '@gophenberg/frontend-sdk'
import type { Action } from '@gophenberg/frontend-sdk/dataviews'

const nameWords: BulkWords<Note> = {
	done: () => 'Name updated.',
	failed: ([{ error }]) => `Could not save the name. ${(error as Error).message}`,
}

const [renaming, setRenaming] = useState<Note>()
const rename: Action<Note> = { id: 'rename', label: 'Rename…', callback: ([note]) => setRenaming(note) }

{renaming ? (
	<RenameDialog
		title="Rename"
		name={renaming.title}
		fieldLabel="Name"
		submitLabel="Save"
		cancelLabel="Cancel"
		onSubmit={(title) => write([renaming], (note) => renameNote(note.id, title), nameWords)}
		onClose={() => setRenaming(undefined)}
	/>
) : null}
```

A dialog that writes through `write` keeps what the reader typed
after a failure, so it needs no store of its own. For a dialog that
closes before its write, the SDK still
passes on `useUnsavedText`, which keeps the typed text by item id,
and `markLatest`, which stops an earlier write that fails late from
overwriting a later one.

When a dialog goes, the focus goes back to what opened it. A dialog
your screen holds open on one row loses that button when the list
draws the row again, and the focus falls to the page. `useFocusBack`
finds the row again and puts the focus back on the `Trigger` that
`focusedTrigger()` noted when the dialog opened. Mark the title cell
of each row with the id DataViews keys the row by, as `data-item-id`.
`useListRun` reads the same mark to find an item a reload moved,
before the focus falls to the row in its place. `focusInDialog()`
tells whether a dialog holds the focus, for a write you finish
yourself outside a run.

`ListEmpty` is what your list shows when it has no rows. It titles
the empty list one level under your page's title, as the admin's own
lists do. Pass it to DataViews as `empty`, with an icon and a short
title with no full stop. An optional `description` goes under the
title, and `actions` holds the buttons under that. Draw your add
button there at its default size.

When a search or a filter emptied the list, show `ListNoResults`
instead. It takes no props, and the admin already loads its words.
`viewNarrows` tells the two cases apart:

```tsx
import { ListEmpty, ListNoResults, postIcon, viewNarrows } from '@gophenberg/frontend-sdk'
import { DataViews } from '@gophenberg/frontend-sdk/dataviews'

const empty = viewNarrows(view) ? (
	<ListNoResults />
) : (
	<ListEmpty icon={postIcon} title="No notes yet" actions={<AddNote size="default" />} />
)

<DataViews {...props} empty={empty} />
```

## Versions

Each version below says what it adds and what it changes. When a
change stops a plugin from compiling, **To move** lists the steps.
To take a version, pass its number to `go get`, such as
`github.com/gopherium/gophenberg/sdk@v0.5.0`.

Your plugin compiles into Gophenberg, so it builds against the SDK
inside the Gophenberg you build. Build with one that holds the
version your plugin requires, or a later one.

### Unreleased

On `main`, not tagged yet. A plugin that builds against it requires
a commit of the SDK, such as `github.com/gopherium/gophenberg/sdk@main`.

- New: [hooks](#hooks). The `HeadContributor` interface adds tags to
  the head of every page the built-in site draws, with the
  `PageMeta`, `PageKind`, `HeadTag` and `Attr` types and the hook's
  name in `HeadTagsHook`.
- New: `PluginRoutesPrefix`, the address every plugin's routes answer
  under, before the plugin's id.
- Nothing breaks.

### v0.5.0

Tagged 2026-10-14. A plugin can show who wrote something.

- New: the `Accounts` field on `Deps`, an `AccountReader`. Its
  `People` method takes many account ids and an avatar size, and
  returns the public face of each account in one call. See
  [reading account names](#reading-account-names).
- New: the `Person` type, with `ID`, `Name`, `Role`, `Avatar` and
  `Disabled`, and never an email address.
- Nothing breaks for a plugin that names the fields of `Deps`.

### v0.4.0

Tagged 2026-10-12. A plugin can declare roles and capabilities.

- New: the `RoleProvider` interface. A plugin with a `Roles` method
  returning `[]sdk.RoleDeclaration` brings new roles, or adds
  capabilities to roles the site already has. See
  [roles and capabilities](#roles-and-capabilities).
- New: the `RoleDeclaration` type, a role's `Name` and the
  `Capabilities` it carries.
- A command's `Capability` can now name a capability your plugin
  declares, such as `notes.manage`. Before, only the core's
  capabilities worked.
- In the frontend SDK, which ships inside Gophenberg with no versions
  of its own, `can` takes the signed-in session instead of its role,
  and a nav row can name the `capability` it needs.
- Gophenberg built after this version opens the admin only for a role
  carrying `access_admin`. Before, every role entered it.

**To move:**

1. If a screen calls `can`, pass it the session, as in
   `can(session, 'hello.manage')`, where you passed `session.role`.
2. If a role your plugin declares should work in the admin, add
   `access_admin` to its `Capabilities`.

### v0.3.0

Tagged 2026-10-10. A command can name the flags every run must set.

- New: the `Needs` field on `sdk.Command`, the names of the flags
  every run must set. The command line refuses a run that leaves one
  out before `Run` starts. See
  [flags a run must set](#flags-a-run-must-set).
- Nothing breaks. A command that looks for a missing flag inside
  `Run` can name the flag in `Needs` and drop that check.

### v0.2.0

Tagged 2026-10-09. Each plugin gets a capped share of the site's
database.

- New: the `DB` field on `Deps`, the plugin's share of the site's one
  database pool. See [the database](#the-database).
- New types for it: `DB`, `Tx`, `Querier`, `Rows`, `Row`, and
  `Engine` with the values `Postgres` and `SQLite`. The site reports
  `Postgres` today.
- New: the `HostOnly` type, which the site embeds in the share it
  lends.
- Nothing breaks. `DatabaseURL` stays, so a plugin that opens its own
  connection keeps working. To use the share instead, keep `deps.DB`
  in `Register`, use it from `Start` on, and stop opening a pool of
  your own.

### v0.1.0

Tagged 2026-10-08. The SDK becomes a Go module of its own.

Before this version, the `sdk` package lived inside the Gophenberg
module, and its last release there was Gophenberg v0.18.2, which
holds the same package as v0.18.1. The import path stays
`github.com/gopherium/gophenberg/sdk`, so your `import` lines do not
change. What changes is the module your plugin requires, and the SDK
now names only its own types and the standard library's. Compared
with Gophenberg v0.18.2:

- The SDK is the module `github.com/gopherium/gophenberg/sdk`. Its
  versions come from tags such as `sdk/v0.1.0`.
- `Item.ID` and `Session.ID` are an `sdk.ID`, no longer a `uuid.UUID`
  from `github.com/google/uuid`. `NewSession` takes an `sdk.ID` too.
  Text and JSON show the same 36 characters as before, and reading
  one from text accepts only that form.
- `Plugin`, `Migrator`, `RouteProvider`, `PublicPathProvider`,
  `Command`, `Call`, `CommandProvider` and `Env` are the SDK's own
  types now, no longer other names for pluginkit's and gonsole's. The
  lifecycle interfaces keep their methods, so a plugin that implements
  them needs no change.
- `Call` loses `Env`, `Describe`, `DatabaseURL()` and `Plugins()`.
  `Command` loses `Migrates`.
- `Env.Duration` and `Env.Count` take no bounds any more, and
  `Env.Timeouts` is gone. `Env.Counts` is new.
- `Misuse` is a function now, and `ErrMisused` is new, so
  `errors.Is(err, sdk.ErrMisused)` spots a misused command line.
- New: the `Description` field on `TypeDeclaration`.

**To move:**

1. Swap the module your plugin requires:

   ```sh
   go mod edit -droprequire=github.com/gopherium/gophenberg
   go get github.com/gopherium/gophenberg/sdk@v0.1.0
   go mod tidy
   ```

   Keep no requirement on Gophenberg v0.18.2 or earlier. Each of those
   releases holds the `sdk` package too, so Go stops the build with
   `ambiguous import`.
2. Where you still need a `uuid.UUID`, convert it. Both are sixteen
   bytes, so `uuid.UUID(item.ID)` and `sdk.ID(someUUID)` work.
3. A command reads its settings and the database address from the
   `Deps` it kept at `Register`, `deps.Env` and `deps.DatabaseURL`,
   not from the `Call`.
4. Drop `Migrates` from your commands. Check a limit such as a highest
   count yourself, after `Env` reads the value.
5. Compare with `sdk.ErrMisused` where you compared with
   `gonsole.ErrMisused`.
