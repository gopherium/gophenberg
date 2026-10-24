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

`sdk.Plugin` is required: `ID`, `Start`, `Stop`. Six optional
interfaces add capabilities the host discovers automatically:
`Migrator` for database migrations, `RouteProvider` for HTTP under
`/api/plugins/{id}`, `PublicPathProvider` for the exact paths that
answer without a login, `TypeDeclarer` for content types, field
groups and fields the plugin brings with it, `CommandProvider`
for [commands](#commands) on the `gophenberg` command line, and
`RoleProvider` for the [roles and capabilities](#roles-and-capabilities)
the plugin brings.

`Register` only checks the plugin's settings and builds it. It
opens nothing, no connection, no file and no goroutine. `Start`
does that. Commands such as `list`, `check` and `migrate` register
every plugin and stop it again without ever starting it. Only
`serve` starts the plugins. So `Stop` must work even when `Start`
never ran, and return by the time its context ends. When `list` or
`help` registers the plugins, `DatabaseURL` can be empty. On a SQLite
database it is always empty.

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
- **A declaration that breaks a rule stops** `serve`, `check`,
  `migrate`, `seed -yes`, every plugin command and every account
  command except `account:list` and `account:records`, and the error
  names your plugin:
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

The frontend SDK, `@gophenberg/frontend-sdk`, passes on a few icons
from `@wordpress/icons`, each named after the job it does in the
admin: `authorIcon`, `backIcon`, `backupIcon`, `brushIcon`,
`categoryIcon`, `closeIcon`, `customPostTypeIcon`, `downIcon`,
`languageIcon`, `listIcon`, `listViewIcon`, `mediaIcon`, `pageIcon`,
`postIcon`, `redoIcon`, `saveIcon`, `settingsIcon`, `sidebarIcon`,
`sidebarRtlIcon`, `trashIcon`, `undoIcon`, `upIcon`, `updateIcon` and
`uploadIcon`. `authorIcon` is the one an
author cell shows when the site shows no avatars, `postIcon` the one
an empty content list shows, `mediaIcon` the one an empty media
library shows, and `uploadIcon` the one an upload button carries.
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

A write that fails never shows inside a dialog. Say why in a toast
at the bottom of the screen, as WordPress does. `useToaster` from the
SDK hands you the admin's own toasts:

```ts
const toaster = useToaster()

toaster.show(failure.message)
```

To confirm an action outside a list, such as a button in your
screen's header, use `ConfirmDialog` from
`@gophenberg/frontend-sdk/dialogs`. It draws the WordPress modal, a
sheet along the bottom of the screen on a phone, with Cancel
focused. Leave `title` out on a confirm that removes something, so
the question is the only text. What `onConfirm` returns decides when
the dialog goes. For a button, make it a plain function that returns
nothing. The dialog then closes at once while your write runs, as the
editor's Move to trash does, and the button that opened it shows the
work:

```tsx
import { Button, useToaster } from '@gophenberg/frontend-sdk'
import { ConfirmDialog } from '@gophenberg/frontend-sdk/dialogs'

const remove = () => {
	setRemoving(true)
	removeItem(item.id)
		.then(() => toaster.show(done))
		.catch((failure) => toaster.show(failure.message))
		.finally(() => setRemoving(false))
}

<Button variant="outline" loading={removing} onClick={() => setAsking(true)}>{label}</Button>
{asking ? (
	<ConfirmDialog confirmLabel={label} cancelLabel={cancel} onConfirm={remove} onClose={() => setAsking(false)}>
		{question}
	</ConfirmDialog>
) : null}
```

An `async` function always returns a promise, which keeps the dialog
open, so leave `onConfirm` plain. Return the write's promise only
for a confirm on a row of a list you draw yourself, outside DataViews,
as WordPress does for a row's own menu. The dialog then stays open
with its button busy until the write ends. Put the `.catch` inside
and return what it answers, such as
`() => removeItem(item.id).catch((failure) => toaster.show(failure.message))`.

`FormDialog` from the same entry asks for a form from a button, as
WordPress opens Add Page: a title, the first field focused, and the
dialog left open after a failure so the reader keeps what they typed.

The SDK still passes on `AlertDialog` and `Dialog` from
`@wordpress/ui`. They float as a card on a phone, where WordPress
draws a sheet, so prefer the two dialogs above.

A confirm on a list row is different: WordPress draws it inside the
list's own modal. A confirm that removes something, such as Move to
trash, hides the header, so give that row action
`hideModalHeader: true`. A plain confirm, such as making an item the
default, keeps its header, which shows the action's `modalHeader` or
else its label. Either way, give the row action
`modalFocusOnMount: 'firstContentElement'` and render `RunConfirm`
with the question as its child, the words of **Cancel** and of a
button named after the action, and `run`, a function that takes the
picked rows and settles once your screen shows how it went. It keeps
Cancel inert while the run goes and closes once it settles.

Import `RunConfirm` from `@gophenberg/frontend-sdk`, beside
`useListRun` and `useBulkRun`, the hooks the admin's own lists run
their actions with. `useListRun` holds the ticked rows and the focus
after a run, and `useBulkRun` makes one call per picked row,
refreshes your list and shows one toast in the words you pass as
`BulkWords`: the failure words when a row failed, else the done
words.

For a row action that asks for more, the SDK passes on the bodies
the admin draws in a row modal. `ConfirmBody` asks a question you
run yourself. `RenameBody` asks for a new name. Give its button the
label **Save**, as WordPress does, close the modal the moment the
reader saves, then write. `FormBody` holds any fields, with `kind`
set to `edit`, `copy` or `create` to space its buttons as WordPress
does. A copy, or a change to several rows, keeps the modal open with
`busy` until the write ends. Taking all of these from the SDK keeps
your plugin on the admin's own copy.

## Versions

Each version below says what it adds and what it changes. When a
change stops a plugin from compiling, **To move** lists the steps.
To take a version, pass its number to `go get`, such as
`github.com/gopherium/gophenberg/sdk@v0.5.0`.

Your plugin compiles into Gophenberg, so it builds against the SDK
inside the Gophenberg you build. Build with one that holds the
version your plugin requires, or a later one.

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
