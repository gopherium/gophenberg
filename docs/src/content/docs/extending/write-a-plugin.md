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
| `id` | Required. Lowercase letters, digits, and hyphens, starting with a letter. Must equal the directory name |
| `name` | Required. The human readable name |
| `backend` | The Go import path of the plugin package |
| `frontend` | The package name of an admin screen, if any |

At least one of `backend` and `frontend` is required.

A few ids are refused because they would clash with names in the
generated wiring, such as a Go keyword, `err` or `plugins`. The
[pluginkit docs](https://docs.gopherium.org/plugins/wiring-and-manifests/#ids-the-generator-refuses)
list them all. The id also names the plugin's commands, so
`help`, `list`, `version`, `check`, `serve`, `migrate`, `seed` and
`account` are refused too. Those names belong to the command line.

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
  header.
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
- **Configuration** arrives through `deps.Env`, which reads the
  settings under the `GOPHENBERG_` prefix. The feed reads
  `GOPHENBERG_FEED_TITLE` and `GOPHENBERG_FEED_ITEMS` through
  `deps.Env.Within("FEED_")`. `deps.Getenv` still reads any other
  environment variable.
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
