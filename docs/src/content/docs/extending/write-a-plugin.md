---
title: Write a plugin
description: A backend plugin from empty directory to compiled in, with the RSS feed as the model.
---

A Gophenberg plugin is a Go package compiled into the binary, with
its own route namespace, its own database schema if it wants one,
and a read view of published posts. The built-in RSS feed in
`plugins/feed` is the reference to keep open.

## 1. The manifest

A plugin is a directory under `plugins/` with a `plugin.json`:

```json
{
  "id": "hello",
  "name": "Hello",
  "backend": "github.com/gopherium/gophenberg/plugins/hello"
}
```

| Field | Rules |
| --- | --- |
| `id` | Required. Lowercase letters, digits, and hyphens, starting with a letter. Must equal the directory name and what the plugin's `ID()` returns. The id of a plugin with settings must not hold two hyphens in a row or end with one |
| `name` | Required. The human readable name |
| `description` | One short sentence saying what the plugin does, such as `Serves your published posts as an RSS feed.` The [Plugins screen](/guides/plugins/) shows it on one line, so keep it short |
| `version` | The plugin's version as text, such as `0.0.1`. The Plugins screen shows it as written |
| `backend` | The Go import path of the plugin package |
| `frontend` | The package name of an admin screen, if any |
| `hooks` | The [hooks](/extending/the-plugin-sdk/#hooks) the plugin joins, each one a method it has. `serve` pauses a plugin whose list does not match its methods |
| `settings` | The [settings](/extending/the-plugin-sdk/#settings) the plugin reads. Each has a `key`, a `type` and, for most types, a `default` written as text. A `count` may add `min` and `max`, a `choice` lists its `options`, and `label` and `help` describe it in plain words. `serve` pauses a plugin whose settings break the rules |

At least one of `backend` and `frontend` is required.

For a plugin that ships inside Gophenberg, the `name`, the
`description` and each setting's `label` and `help` reach the
translators. Run `make pot` after you change one, as
[Add a translatable string](/contributing/add-a-translatable-string/#what-the-gates-check)
says.

Spell each key exactly as listed here, in lower case, and only once,
or `make generate` stops with an error such as:

```text
pluginwire: plugins/hello/plugin.json: the key "Version" must be spelled "version"
```

A plugin that reads settings lists them under `settings`. The feed
lists its two, after its description and its version:

```json
{
  "id": "feed",
  "name": "Feed",
  "description": "Serves your published posts as an RSS feed.",
  "version": "0.0.1",
  "backend": "github.com/gopherium/gophenberg/plugins/feed",
  "hooks": ["public.head-tags"],
  "settings": [
    { "key": "title", "type": "text", "default": "Gophenberg", "label": "Feed title", "help": "The channel title feed readers display" },
    { "key": "items", "type": "count", "default": "20", "min": 1, "label": "Posts in the feed", "help": "How many posts the feed carries" }
  ]
}
```

The site reads `title` from `GOPHENBERG_PLUGIN_FEED__TITLE` and
`items` from `GOPHENBERG_PLUGIN_FEED__ITEMS`, and takes each default
when the variable is unset.

A few ids are refused because they would clash with names in the
generated wiring, such as a Go keyword, `err` or `plugins`. The
[pluginkit docs](https://docs.gopherium.org/plugins/wiring-and-manifests/#ids-the-generator-refuses)
list them all. The id also names the plugin's commands, so
`help`, `list`, `version`, `check`, `serve`, `migrate`, `seed`,
`account`, `about`, `update`, `server`, `plugin`, `theme`, `setup`,
`build` and `health` are refused too. Those names belong to the
command line.

The ids `db`, `query`, `tx`, `start`, `hook`, `hook-save` and
`hook-timed` are rejected as well. A plugin under one of them would
read its settings one underscore away from one of the core's
`GOPHENBERG_PLUGIN_` settings. A plugin `hook` with the setting
`budget` would read `GOPHENBERG_PLUGIN_HOOK__BUDGET`, beside the
core's `GOPHENBERG_PLUGIN_HOOK_BUDGET`, so one typo in `.env` would
set the wrong one.

## 2. The package

The entry point is `Register(deps sdk.Deps) (sdk.Plugin, error)`,
receiving the [SDK's Deps](/extending/the-plugin-sdk/). A minimal
plugin serving one route:

```go
package hello

import (
	"context"
	"net/http"

	"github.com/gopherium/gophenberg/sdk"
)

type Plugin struct{}

// Register builds the plugin from its dependencies.
func Register(deps sdk.Deps) (sdk.Plugin, error) {
	return &Plugin{}, nil
}

// ID names the plugin.
func (p *Plugin) ID() string { return "hello" }

// Start begins serving.
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop ends serving.
func (p *Plugin) Stop(ctx context.Context) error { return nil }

// Routes serves the plugin's namespace.
func (p *Plugin) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/greeting", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})
	return mux
}
```

`Register` reads settings and builds the plugin, nothing more.
Commands such as `list` and `check` register every plugin and stop
it again without starting it, so open connections in `Start`, and
make `Stop` work when `Start` never ran.

## 3. Wire it in

```sh
make generate
```

This regenerates the wiring from every manifest, and the next
build compiles your plugin in. There is no list to edit by hand.
Then `gophenberg check` registers your plugin and checks its
settings and command names without touching the database.

## What the host gives you

- **Routes** mount at `/api/plugins/hello`, prefix stripped, and
  require a login by default. A successful answer on those routes
  is marked `private, no-store`. A failed one, a refused login
  included, is marked `no-store`. Either way no cache keeps it,
  whatever header your plugin sets. Public paths keep your own
  header. `serve` asks for your routes before `Start` runs, so
  `Routes` must build its handler without anything `Start` opens.
  A panic in a route answers the site's internal error, status 500
  with the code `internal`, and `serve` logs your plugin's name with
  the stack.
- **Public paths**: declare `PublicPaths() []string` and those
  exact paths answer without a session, for every method. Exact
  match, never a prefix. This is how the feed serves
  `/api/plugins/feed/rss.xml` publicly. A write a browser sends
  from a page on another site is refused before it reaches you,
  with the code `request_cross_origin`. When the site names its
  public address, so is any write sent to another address, a
  webhook included. A read never is, so a public path must not
  change anything on a GET.
- **Migrations**: implement `Migrate(ctx) error` and it runs
  before anything starts, and again on `gophenberg migrate` and
  `gophenberg seed -yes`. Keep your tables and your migration
  record in a schema of your own. If you run goose, turn on its
  session locker with `goose.WithSessionLocker` and
  `lock.NewPostgresSessionLocker()`. Every core schema step waits
  for that same Postgres lock, so two processes never apply your
  migrations together.
- **Configuration** arrives through `deps.Settings`, which reads the
  settings the plugin lists in its `plugin.json`. The feed keeps it
  and reads `Text("title")` and `Count("items")` on each request. An
  operator sets them as `GOPHENBERG_PLUGIN_FEED__TITLE` and
  `GOPHENBERG_PLUGIN_FEED__ITEMS`. The
  [plugin SDK](/extending/the-plugin-sdk/#settings) page lists the
  seven types and the rules.
- **Commands**: implement `Commands() []sdk.Command` and the
  `gophenberg` command line offers them as `hello:<command>`. The
  [plugin SDK](/extending/the-plugin-sdk/#commands) page shows one.
- **Content declarations**: implement `DeclareTypes` and the
  content types, field groups and fields your plugin needs exist
  on every site it is compiled into. The
  [plugin SDK](/extending/the-plugin-sdk/) page shows one.

## One honest boundary

Compiling a plugin in is a trust decision. The SDK is a clean
interface, not a sandbox: `deps.DB` and `deps.DatabaseURL` reach the
same database the core uses. Review what you compile in.
