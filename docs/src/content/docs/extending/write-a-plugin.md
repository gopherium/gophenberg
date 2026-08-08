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

## 3. Wire it in

```sh
make generate
```

This regenerates the wiring from every manifest, and the next
build compiles your plugin in. There is no list to edit by hand.

## What the host gives you

- **Routes** mount at `/api/plugins/hello`, prefix stripped, and
  require a login by default.
- **Public paths**: declare `PublicPaths() []string` and those
  exact paths answer without a session, for every method. Exact
  match, never a prefix. This is how the feed serves
  `/api/plugins/feed/rss.xml` publicly.
- **Migrations**: implement `Migrate(ctx) error` and it runs
  before anything starts. Keep your tables and your migration
  record in a schema of your own.
- **Configuration** arrives through `deps.Getenv`, the way the
  feed reads `GOPHENBERG_FEED_TITLE` and `GOPHENBERG_FEED_ITEMS`.

## One honest boundary

Compiling a plugin in is a trust decision. The SDK is a clean
interface, not a sandbox: `deps.DatabaseURL` reaches the same
database the core uses. Review what you compile in.
