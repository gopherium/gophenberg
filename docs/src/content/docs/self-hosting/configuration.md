---
title: Configuration
description: Every environment variable, what the binary serves where, and what stops startup.
---

Gophenberg is configured entirely through environment variables. A
`.env` file in the working directory is read at startup, and real
environment variables win over it. Every command reads the same
settings, the `.env` file included, so `migrate` or an account
command run beside the server finds the database the server uses.

Every value is trimmed of the spaces around it. A padded value works
as if it had none, and a value made only of spaces counts as unset,
so it takes the default.

## The variables

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `GOPHENBERG_DATABASE_URL` | Yes | | The PostgreSQL connection string |
| `GOPHENBERG_ADDR` | No | `localhost:8081` | Address the server listens on. The container image sets `0.0.0.0:8081` |
| `GOPHENBERG_WEB_DIR` | No | | Where the built admin and the public stylesheets live. The image sets `/web` |
| `GOPHENBERG_SITE_TITLE` | No | `Gophenberg` | The site name shown by the built-in renderer |
| `GOPHENBERG_TRUSTED_PROXIES` | No | | Comma-separated CIDR ranges allowed to set forwarded headers |
| `GOPHENBERG_PUBLIC_URL` | No | | The address people reach the site at, such as `https://example.com`. When set, a write sent to any other address is refused |
| `GOPHENBERG_THEMES_DIR` | No | | The directory themes are installed in, which uploads write to. The image sets `/themes` |
| `GOPHENBERG_MEDIA_DIR` | No | | The directory uploaded media is stored in and served from. The image sets `/media` |
| `GOPHENBERG_THEME` | No | | Pins one theme, overriding the admin. Empty lets the admin choose |
| `GOPHENBERG_NODE_BIN` | No | `node` | The Node binary themes run on. The image sets its own |
| `GOPHENBERG_MEDIA_UPLOAD_CAP_MB` | No | `128` | The largest upload the media library takes, in megabytes |
| `GOPHENBERG_UPLOAD_TIMEOUT` | No | `5m` | How long a media or theme upload has to arrive, in place of `GOPHENBERG_HTTP_READ_TIMEOUT` |
| `GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB` | No | `256` | The largest definitions file an import takes, in kilobytes, from 1 to 1024 |
| `GOPHENBERG_FIELD_DEPTH` | No | `32` | How many containers a field may stand inside, from 1 to 1000. A Flexible content layout counts as one |
| `GOPHENBERG_PLUGIN_DB_CONNS` | No | `4` | How many statements, open result sets and transactions each plugin adds to the plugins' database share, from 1 to 1000 |
| `GOPHENBERG_PLUGIN_DB_RESERVE` | No | `2` | How many of the database pool's connections the plugins' share always leaves to the site, from 1 to 1000. The share holds at most the pool's size less this reserve. The pool's size is the `pool_max_conns` of `GOPHENBERG_DATABASE_URL`, the larger of 4 and the number of CPUs when unset. The server refuses to start when the reserve leaves plugins no connection |
| `GOPHENBERG_PLUGIN_QUERY_TIMEOUT` | No | `5s` | How long one plugin statement may run, its wait for a slot included |
| `GOPHENBERG_PLUGIN_TX_TIMEOUT` | No | `30s` | How long one plugin transaction may stay open |
| `GOPHENBERG_LIST_PAGE_SIZES` | No | `10,20,50,100` | The page sizes an admin list offers in its Items per page menu, from 2 to 6 of them, see [the admin lists](#the-admin-lists) |
| `GOPHENBERG_LIST_PAGE_SIZE` | No | `20` | The page size an admin list opens at, one of the sizes above |
| `GOPHENBERG_LIST_PAGE_CAP` | No | `100` | The most items one admin list page carries. A request for more gets this many |
| `GOPHENBERG_TOAST_DURATION` | No | `6s` | How long a confirmation message stays on screen, in whole milliseconds from `1ms` to `2147483647ms` |
| `GOPHENBERG_TOAST_NAME_LENGTH` | No | `45` | How many characters of an item's name a confirmation message shows before it cuts the name short |
| `GOPHENBERG_FORMAT_LOCALE` | No | `es-ES` | The locale the admin writes dates, times and numbers in, whatever language its screens are in |
| `GOPHENBERG_THEME_READY_TIMEOUT` | No | `30s` | How long a starting theme has to answer before it is given up on |
| `GOPHENBERG_THEME_START_ATTEMPTS` | No | `5` | How many times in a row a theme that will not start is started before it is given up on, from 1 to 1000 |
| `GOPHENBERG_THEME_BACKOFF` | No | `500ms` | How long to wait before the first retry, doubling after each one |
| `GOPHENBERG_THEME_MAX_BACKOFF` | No | `30s` | The longest that wait grows to |
| `GOPHENBERG_THEME_STOP_GRACE` | No | `3s` | How long a theme has to stop before it is killed |
| `GOPHENBERG_THEME_PROXY_TIMEOUT` | No | `10s` | How long a running theme has to start answering one request |
| `GOPHENBERG_CACHE_ASSET_MAX_AGE` | No | `1h` | How long a browser may keep a site stylesheet or icon. A proxy setting its own header wins |
| `GOPHENBERG_CACHE_MEDIA_MAX_AGE` | No | `1h` | How long a browser may keep an uploaded file |
| `GOPHENBERG_CACHE_CONTENT_SHARED_MAX_AGE` | No | `1m` | How long a shared cache may serve a content API answer. The language answer is never shared, since it is resolved per reader |
| `GOPHENBERG_CACHE_CONTENT_STALE_WHILE_REVALIDATE` | No | `5m` | How much longer that cache may serve the old answer while fetching a fresh one |
| `GOPHENBERG_HTTP_READ_HEADER_TIMEOUT` | No | `10s` | How long a visitor has to send a request's headers |
| `GOPHENBERG_HTTP_READ_TIMEOUT` | No | `30s` | How long a visitor has to send a whole request |
| `GOPHENBERG_HTTP_IDLE_TIMEOUT` | No | `2m` | How long an idle connection waits for its next request |
| `GOPHENBERG_SHUTDOWN_GRACE` | No | `10s` | How long a stop gives running requests to finish, see [stopping the server](#stopping-the-server) |
| `GOPHENBERG_SHUTDOWN_CANCEL_GRACE` | No | `5s` | How long the requests cancelled after that grace get to end |
| `GOPHENBERG_SHUTDOWN_STOP_GRACE` | No | `5s` | How long the plugins get to stop after that, and after any other command |
| `GOPHENBERG_COMMAND_RECORD_TIMEOUT` | No | `5s` | How long an account command has to store the record of a change it applied, see [the commands](/self-hosting/commands/) |
| `GOPHENBERG_COMMAND_RECORDS_LIMIT` | No | `50` | How many records `account:records` lists when its `-limit` flag names no other |
| `GOPHENBERG_FEED_TITLE` | No | `Gophenberg` | The RSS channel title |
| `GOPHENBERG_FEED_ITEMS` | No | `20` | How many posts the RSS feed carries |

Three rows deserve a warning:

- `GOPHENBERG_WEB_DIR` also holds the stylesheets every public
  page loads. Unset, the public site loses its block styling,
  theme or not.
- `GOPHENBERG_SITE_TITLE` only affects the built-in renderer. A
  theme names the site in its own source.
- `GOPHENBERG_MEDIA_DIR` holds files no database backup carries.
  Leave it unset and the media library refuses every upload. Point
  it at a volume that survives a restart, and back it up alongside
  the database.

Every cache window applies only to an answer that succeeded. An
address nothing holds, a file that is missing or a request the server
refused is never kept by any cache, however the windows are set. An
answer to a signed in account is never kept by any cache either, not
even that person's own browser.

## The admin lists

The server reads six settings when it starts. It serves five of them
to the admin and keeps the page cap to itself, and no screen can
change them. They shape every list the same way:

- **The page sizes.** The Items per page menu offers the sizes in
  `GOPHENBERG_LIST_PAGE_SIZES`, and a list opens at
  `GOPHENBERG_LIST_PAGE_SIZE`. The menu only shows for 2 to 6 sizes,
  so the server refuses any other count rather than hide it.
- **The page cap.** No list page carries more than
  `GOPHENBERG_LIST_PAGE_CAP` items, whatever a request asks for. The
  largest page size has to fit under it.
- **The confirmation messages.** `GOPHENBERG_TOAST_DURATION` sets how
  long a message after an action stays on screen.
  `GOPHENBERG_TOAST_NAME_LENGTH` sets how many characters of an item's
  name such a message shows before it cuts the name short with "…".
- **The format.** `GOPHENBERG_FORMAT_LOCALE` sets the locale for
  dates, times and numbers, whatever language a person reads the
  admin in. The tag `es-ES` writes 04/10/2026, 14:05 and 1.234,56.
  The date column of a content list is the one exception. Like
  WordPress, it writes the month short in the reader's language.

## Which theme serves

Two things can name a theme, and they do not carry equal weight:

- **`GOPHENBERG_THEME`**, when set, wins. The admin refuses to
  activate, deactivate or roll back while it is set, and a pinned
  theme that fails to load stops the server from starting.
- **The theme chosen in the admin**, stored in the database, governs
  when no pin is set. If it fails to load, the server still starts,
  the built-in renderer serves, and the admin shows the theme as
  broken.

Leaving `GOPHENBERG_THEME` unset is the normal way to run. Pin it
when you want the theme fixed by deployment rather than by whoever
is logged in.

## Trusted proxies

Behind a proxy, requests reach Gophenberg from the proxy's
address, and the headers naming the real visitor and the real
`https` address can be written by anyone. Gophenberg believes them
only from addresses inside `GOPHENBERG_TRUSTED_PROXIES`.

Leaving it unset behind a proxy causes three quiet problems: any
absolute address a theme builds comes out as `http`, the login
rate limiter sees all visitors as one client, so a few failed
logins by anyone can lock out everyone, and a browser too old to
say where a request came from is refused when it saves anything.
Your proxy should also pass `X-Forwarded-Proto`, which Caddy and
nginx both do, or those browsers are refused the same way. Naming
the site's public address, below, spares those browsers either way.

## The public address

Set `GOPHENBERG_PUBLIC_URL` to the address people type to reach the
site, such as `https://example.com`, with nothing after the host.
Gophenberg then takes a write only when it was sent to that address,
and a browser's write only when its page stands at that address too.
That changes two things:

- A browser too old to say where a request came from is judged
  against that address, so it can save behind any proxy.
- Another domain pointed at your server's address can no longer make
  a visitor's browser post to the site.

Every write has to go through that address, including the ones a
script or another server sends, so a program that writes to
Gophenberg at its internal address is refused once the setting is
on. Reads are never refused. Behind a proxy, the proxy has to pass
the host the visitor typed, in the `Host` header or in
`X-Forwarded-Host` from an address inside
`GOPHENBERG_TRUSTED_PROXIES`. Caddy passes it as it is. With nginx,
add `proxy_set_header Host $host`.

The server logs every write it refuses and names the reason: `host`
when the write was sent to another address, `origin` when its page
stood at another address, `fetch-site` when the browser said the page
was on another site, and `scheme` when a proxy did not say whether the
visitor used `http` or `https`.

## Stopping the server

A stop begins when the server receives `SIGTERM`, or `Ctrl-C` in a
terminal. From then on it takes no new request, and it:

1. Gives the requests still running `GOPHENBERG_SHUTDOWN_GRACE` to
   finish.
2. Cancels the ones still running after that, gives them
   `GOPHENBERG_SHUTDOWN_CANCEL_GRACE` to end, and then closes their
   connections.
3. Gives the plugins `GOPHENBERG_SHUTDOWN_STOP_GRACE` to stop.
4. Gives the theme `GOPHENBERG_THEME_STOP_GRACE` to stop.

Whatever sends the signal also waits, and kills the server when its
own wait runs out. Docker waits 10 seconds unless the compose file
sets `stop_grace_period`. Kubernetes waits
`terminationGracePeriodSeconds`, 30 seconds unless set. Keep that wait
longer than the four settings above added together, 23 seconds at
their defaults, or the kill lands in the middle of the stop.

The plugins get the same `GOPHENBERG_SHUTDOWN_STOP_GRACE` to stop
after every other command that loads them, such as `migrate` or
`seed -yes`, not only after serving.

For a planned stop that needs a long wrap-up, raise both clocks. For
example, set `GOPHENBERG_SHUTDOWN_GRACE=4m` and
`stop_grace_period: 5m`. While it wraps up, the server takes no new
request, so on a site that runs one copy that time is downtime.

A stop can also come with no warning, from a power cut or a kill, and
then none of these steps runs.

## Uploads a stop cut short

A stop in the middle of an upload can leave its files in the media
folder with no item in the library pointing at them. Every start
deletes those files before the site serves, and logs each one as
`unsaved upload deleted`. A file an item points at is never deleted.
A file the server cannot delete, such as one in a folder it may not
write to, is logged as `unsaved uploads kept for the next start`, and
the next start tries again.

To know which files those are, every upload leaves a note in a
`.uploading` folder inside the media folder until its item is saved.
That folder is never served. Leave it where it is.

Run one Gophenberg per media folder. A start treats every note it
finds as an upload a stop cut short, so a second copy sharing the
folder could lose an upload it is saving at that moment.

## What stops startup

The server refuses to start, and says why, when:

- `GOPHENBERG_DATABASE_URL` is missing.
- `GOPHENBERG_TRUSTED_PROXIES` is not valid CIDR notation.
- `GOPHENBERG_PUBLIC_URL` is not an `http` or `https` address naming
  only a host.
- `GOPHENBERG_FEED_ITEMS` is not a positive whole number.
- `GOPHENBERG_MEDIA_UPLOAD_CAP_MB` is not a positive whole number, or
  names more megabytes than the server can count in bytes.
- `GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB` is not a whole number from 1 to
  1024, which is as much of a request body as the server reads.
- `GOPHENBERG_THEME_START_ATTEMPTS` is not a whole number from 1 to 1000.
- `GOPHENBERG_FIELD_DEPTH` is not a whole number from 1 to 1000.
  These three refuse rather than quietly using another value.
- Any of the cache windows is not a positive whole number of seconds.
  Write them as durations, `1h`, `90s`, `5m`. Part of a second is
  refused, because the header counts in whole seconds.
- Any of the theme timings, HTTP timeouts, shutdown graces or the upload
  timeout is not a positive duration. Write them the way Go does, `30s`,
  `500ms`, `1m`.
- `GOPHENBERG_THEME_MAX_BACKOFF` stands below
  `GOPHENBERG_THEME_BACKOFF`, which would leave no room to grow.
- `GOPHENBERG_LIST_PAGE_SIZES` lists fewer than 2 or more than 6
  sizes, lists a size twice or out of order, or names a size above
  `GOPHENBERG_LIST_PAGE_CAP`.
- `GOPHENBERG_LIST_PAGE_SIZE` is not one of those sizes.
- `GOPHENBERG_LIST_PAGE_CAP` is not a whole number from 1 to
  2147483647, or stands below the largest page size.
- `GOPHENBERG_TOAST_DURATION` is not a whole number of milliseconds
  from `1ms` to `2147483647ms`, the longest a browser timer holds.
- `GOPHENBERG_TOAST_NAME_LENGTH` is not a positive whole number.
- `GOPHENBERG_FORMAT_LOCALE` is not a language tag such as `es-ES` or
  `en-GB`.
- `GOPHENBERG_THEME` pins a theme that fails to load, see
  [installing a theme](/themes/installing-a-theme/). A theme chosen
  in the admin does not stop startup.

`GOPHENBERG_COMMAND_RECORD_TIMEOUT` has to be a positive duration and
`GOPHENBERG_COMMAND_RECORDS_LIMIT` a positive whole number. A bad one
does not stop the server, but the account commands that read it
refuse to run.

Run `check` before every rollout, with the new image and the settings
it will run with. It reads every setting, the two above included,
and loads the plugins, without starting anything or touching the
database. It answers `settings, plugins and command names are valid`,
or names the settings it refuses and exits with code 1:

```sh
docker compose run --rm -T gophenberg check
```

## What the binary serves where

| Path | What | Login needed |
| --- | --- | --- |
| `/` | The public site, the default type's newest items | No |
| `/{path}` | One published item at its stored address | No |
| `/{routeword}` | A [content type](/guides/content-types/)'s archive | No |
| `/page/{n}`, `/{routeword}/page/{n}` | Older items behind the page word | No |
| `/media/...` | Uploaded files and their derived sizes | No |
| `/admin/` | The admin | The screens ask for one |
| `/api/...` | The admin's JSON API | Yes, apart from signing in and out |
| `/api/content/v1/...` | The [public content API](/reference/content-api/) | No |
| `/api/plugins/feed/rss.xml` | The [RSS feed](/reference/rss-feed/) | No |
| `/gophenberg/...` | Site assets, cached for an hour unless set otherwise | No |
| `/_gophenberg/...` | Reserved for internal use | Answers 404 |

The last row answering 404 from outside is correct behavior, not
an outage.
