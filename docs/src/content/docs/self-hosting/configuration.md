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
| `GOPHENBERG_DATABASE_URL` | Yes | | The PostgreSQL connection string, or `sqlite:` and the absolute path of a SQLite file. [The PostgreSQL address](#the-postgresql-address) lists what a connection string may not hold. [A SQLite database](#a-sqlite-database) says what runs on one and lists its settings |
| `GOPHENBERG_DATABASE_MAX_CONNS` | No | The larger of 4 and the number of CPUs | How many connections to the database stay open at most, from 2 to 2147483647, see [the database connections](#the-database-connections) |
| `GOPHENBERG_ADDR` | No | `localhost:8081` | Address the server listens on. The container image sets `0.0.0.0:8081` |
| `GOPHENBERG_WEB_DIR` | No | | A built admin folder to serve in place of the admin built into the binary. It must hold `index.html` |
| `GOPHENBERG_SITE_TITLE` | No | `Gophenberg` | The site name shown by the built-in renderer |
| `GOPHENBERG_TRUSTED_PROXIES` | No | | Comma-separated CIDR ranges allowed to set forwarded headers |
| `GOPHENBERG_PUBLIC_URL` | No | | The address people reach the site at, such as `https://example.com`. When set, a write sent to any other address is refused |
| `GOPHENBERG_THEMES_DIR` | No | | The directory themes are installed in, which uploads write to. The image sets `/themes` |
| `GOPHENBERG_MEDIA_DIR` | No | | The directory uploaded media is stored in and served from. The image sets `/media` |
| `GOPHENBERG_THEME` | No | | Pins one theme, overriding the admin. Empty lets the admin choose |
| `GOPHENBERG_NODE_BIN` | No | `node` | The Node binary themes run on. The image sets its own |
| `GOPHENBERG_MEDIA_UPLOAD_CAP_MB` | No | `128` | The largest upload the media library takes, in megabytes |
| `GOPHENBERG_MEDIA_CONCURRENT_IMAGES` | No | `2` | How many uploaded images the server decodes and resizes at once, from 1 to 64. The rest wait their turn |
| `GOPHENBERG_UPLOAD_TIMEOUT` | No | `5m` | How long a media or theme upload has to arrive, in place of `GOPHENBERG_HTTP_READ_TIMEOUT` |
| `GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB` | No | `256` | The largest definitions file an import takes, in kilobytes, from 1 to 1024 |
| `GOPHENBERG_FIELD_DEPTH` | No | `32` | How many containers a field may stand inside, from 1 to 1000. A Flexible content layout counts as one |
| `GOPHENBERG_PLUGIN_DB_CONNS` | No | `4` | How many statements, open result sets and transactions each plugin adds to the plugins' database share, from 1 to 1000 |
| `GOPHENBERG_PLUGIN_DB_RESERVE` | No | `2` | How many of the database pool's connections the plugins' share always leaves to the site, from 1 to 1000. The share holds at most `GOPHENBERG_DATABASE_MAX_CONNS` less this reserve. The server refuses to start when the reserve leaves plugins no connection, so a cap of 2 needs a reserve of 1 |
| `GOPHENBERG_PLUGIN_QUERY_TIMEOUT` | No | `5s` | How long one plugin statement may run, its wait for a slot included. It also ends a plugin's read of account names |
| `GOPHENBERG_PLUGIN_TX_TIMEOUT` | No | `30s` | How long one plugin transaction may stay open |
| `GOPHENBERG_AVATARS` | No | `initials` | How the site draws avatars: `initials`, `gravatar` or `off`, see [avatars](#avatars) |
| `GOPHENBERG_AVATAR_DEFAULT` | No | `mystery` | The picture Gravatar shows for an email address it does not know: `mystery`, `blank`, `gravatar_default`, `identicon`, `wavatar`, `monsterid`, `retro`, `robohash` or `color`. WordPress's `initials` is refused, see [avatars](#avatars) |
| `GOPHENBERG_AVATAR_RATING` | No | `G` | The most mature Gravatar picture an avatar may show: `G`, `PG`, `R` or `X` |
| `GOPHENBERG_LIST_PAGE_SIZES` | No | `10,20,50,100` | The page sizes an admin list offers in its Items per page menu, from 2 to 6 of them, see [the admin lists](#the-admin-lists) |
| `GOPHENBERG_LIST_PAGE_SIZE` | No | `20` | The page size an admin list opens at, one of the sizes above |
| `GOPHENBERG_LIST_PAGE_CAP` | No | `100` | The most items one admin list page carries. A request for more gets this many |
| `GOPHENBERG_PICKER_PAGE_SIZE` | No | `50` | The page size the media picker opens at, one of the sizes above. Unset and with no `50` among them, the size a list opens at |
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
| `GOPHENBERG_CACHE_ADMIN_ASSET_MAX_AGE` | No | `8760h` | How long a browser may keep an admin file whose name carries its content hash, one year by default. A new build gives a changed file a new name, and the admin page itself is checked on every load |
| `GOPHENBERG_HTTP_READ_HEADER_TIMEOUT` | No | `10s` | How long a visitor has to send a request's headers |
| `GOPHENBERG_HTTP_READ_TIMEOUT` | No | `30s` | How long a visitor has to send a whole request |
| `GOPHENBERG_HTTP_IDLE_TIMEOUT` | No | `2m` | How long an idle connection waits for its next request |
| `GOPHENBERG_SHUTDOWN_GRACE` | No | `10s` | How long a stop gives running requests to finish, see [stopping the server](#stopping-the-server) |
| `GOPHENBERG_SHUTDOWN_CANCEL_GRACE` | No | `5s` | How long the requests cancelled after that grace get to end |
| `GOPHENBERG_SHUTDOWN_STOP_GRACE` | No | `5s` | How long the plugins get to stop after that, and after any other command |
| `GOPHENBERG_CRON_SECRET` | No | | The secret the host's cron sends to run the [cron jobs](/self-hosting/cron/), at least 32 characters. Empty leaves the cron address off |
| `GOPHENBERG_CRON_ON_VISITS` | No | `true` | Whether a visit may start the cron jobs once its page is answered |
| `GOPHENBERG_CRON_VISIT_PACE` | No | `1m` | How long after one check a visit waits before it checks again |
| `GOPHENBERG_CRON_TIME_LIMIT` | No | `30s` | How long one cron job run may take before it is cut short |
| `GOPHENBERG_CRON_LEASE` | No | `1m` | How long a run holds its job so no other run takes it, above the time limit. Also the longest the server waits for the database to list or hand over a job |
| `GOPHENBERG_CRON_STOP_GRACE` | No | `2s` | How long a stop gives a running cron job to end and record itself |
| `GOPHENBERG_SESSION_SWEEP_INTERVAL` | No | `1h` | How often expired sessions and expired account links are swept |
| `GOPHENBERG_COMMAND_RECORD_TIMEOUT` | No | `5s` | How long an account command has to store the record of a change it applied, see [the commands](/self-hosting/commands/) |
| `GOPHENBERG_COMMAND_RECORDS_LIMIT` | No | `50` | How many records `account:records` lists when its `-limit` flag names no other |
| `GOPHENBERG_FEED_TITLE` | No | `Gophenberg` | The RSS channel title |
| `GOPHENBERG_FEED_ITEMS` | No | `20` | How many posts the RSS feed carries |

Three rows deserve a warning:

- `GOPHENBERG_WEB_DIR` replaces the admin built into the binary,
  and the stylesheets every public page loads with it. Leave it
  unset to serve both from the binary. Older images set it to
  `/web`, a folder the image no longer holds, so delete that line if
  your compose file copied it, or the server stops at start.
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

## The database connections

`GOPHENBERG_DATABASE_MAX_CONNS` caps how many connections to the
database stay open at once. It takes a whole number from 2 to
2147483647. Left unset, it is the larger of 4 and the number of CPUs.

On PostgreSQL Gophenberg opens one pool of that size. Everything the
site does draws from it, the cron jobs and the plugins' share
included. `seed -yes` stores the demo data through the same pool, and
the core migration opens its connections under the same cap. Two
things open short-lived connections of their own, outside the cap:

- The migration of the account and record tables, at every start and
  in every command that migrates, such as `migrate`, `seed -yes` and
  `account:create-admin`.
- The `account:` commands, while they read or change an account.

PostgreSQL turns away connections past its own `max_connections`, 100
by default, so keep the caps of every copy of Gophenberg on one
database server added together below it, with room left for those.
On a SQLite file each connection keeps its own page cache, see
[the SQLite settings](#the-sqlite-settings).

On PostgreSQL, the plugins' share always leaves
`GOPHENBERG_PLUGIN_DB_RESERVE` of these connections to the site, 2 by
default. At a cap of 2 that leaves the plugins nothing, so set the
reserve to 1 there. Otherwise the server does not start, and `check`
says why:

```text
gophenberg: GOPHENBERG_PLUGIN_DB_RESERVE=2 leaves plugins no connection in a pool of 2, raise GOPHENBERG_DATABASE_MAX_CONNS or lower the reserve
```

A cap outside its range stops the server too, for example:

```text
gophenberg: GOPHENBERG_DATABASE_MAX_CONNS: must be 2 or more, got "1"
```

### The PostgreSQL address

A PostgreSQL connection string takes no `pool_` setting, because
`GOPHENBERG_DATABASE_MAX_CONNS` sizes the pool. Every key that starts
with `pool_` is refused by name, whatever its value, such as
`pool_max_conns` or `pool_max_conn_lifetime`:

```text
gophenberg: GOPHENBERG_DATABASE_URL: remove pool_max_conns from the address, GOPHENBERG_DATABASE_MAX_CONNS sizes the pool
```

Gophenberg also refuses a connection string that two versions of its
PostgreSQL driver read differently, so that an update can never send
the site to another server, database or account. The message names
the setting and the reason:

```text
gophenberg: GOPHENBERG_DATABASE_URL: dbkit: refused database address: the address holds a #, write it as %23
```

`check` runs these checks, and the check of
`GOPHENBERG_DATABASE_MAX_CONNS`, without connecting. `serve` runs them
before it connects. `migrate`, `seed -yes` and every `account:`
command but `account:list` and `account:records` run them before they
touch the database, so a value they refuse changes nothing there. A
command that migrates names its first schema step in front of the
message, though that step applied nothing:

```text
gophenberg: migrate accounts: GOPHENBERG_DATABASE_MAX_CONNS: must be 2 or more, got "1"
```

`account:list` and `account:records` only read, and connect without
these checks.

The connection string is refused when it:

- Lists several hosts with an empty one among them, such as
  `db1,,db2`.
- Cannot be read at all, such as a port that is not a number.

A `postgres://` address is also refused when it:

- Holds an `@` inside the user name or the password. Write it as `%40`.
- Holds a `?` or a `#` before the `@`, such as in the password. Write
  them as `%3F` and `%23`.
- Holds a sign in the user name or the password other than an
  unaccented letter, a digit, a semicolon or one of `-._:~!$&'()*+,=%`,
  such as `"`, `^`, `|` or an accented letter. Write it as its `%`
  code.
- Holds a `#` anywhere else, a space or a control character. Write
  them as `%23` and `%20`.
- Holds a `%` that starts no `%` code, or `%00`. Write a `%` as `%25`.
- Gives a port to some hosts of its host list only.
- Names a database that starts with a `/`. Give it as
  `?dbname=%2Fname` instead.
- Holds a `+` or a semicolon in the part after the `?`. Write them as
  `%2B` and `%3B`.
- Holds a pair after the `?` with no `=` or with two, sets `ssl`
  instead of `sslmode`, or sets one key twice. `dbname` and `database`
  count as one key.

A connection string written as `key=value` pairs, such as
`host=db user=postgres dbname=gophenberg`, is also refused when it
holds a backslash. Write it as a `postgres://` address with `%5C`
instead.

The safest password holds only letters and digits. In a
`postgres://` address, write every other sign in it as its `%` code,
such as `%40` for `@`, `%23` for `#` and `%3F` for `?`.

## A SQLite database

Gophenberg can also keep its data in one SQLite file instead of a
PostgreSQL server. Only part of Gophenberg runs on SQLite so far:
`check`, `migrate` and the account commands work, but the site cannot
serve from the file yet.

### The address

Write `sqlite:` and the absolute path of the file:

```sh
GOPHENBERG_DATABASE_URL=sqlite:/srv/gophenberg/site.db
```

- The path must be absolute, so `sqlite:site.db` is refused.
- Write no `//` after `sqlite:`, and no `?`, `#` or `%` anywhere in
  the address. It takes no options. The settings below do that job.
- An address that does not start with `sqlite:` is read as a
  PostgreSQL one, as before.

### Before the first run

Gophenberg never creates the database file. Create an empty one first:

```sh
touch /srv/gophenberg/site.db
```

Without it, each command that opens the file, such as `check`,
`migrate`, `seed -yes` and every account command, stops with:

```text
gophenberg: open the database: sqlite: the database file does not exist: /srv/gophenberg/site.db, create an empty file at that path to start a new site
```

SQLite runs on Linux and macOS only, and the file must sit on a local
disk. A file on a network file system, such as NFS, SMB or a Windows
folder seen from WSL2 like `/mnt/c`, is refused, because SQLite's locks
are not reliable there.

The folder holding the file must stay writable for the user that runs
Gophenberg, because SQLite keeps more files beside it:

- `site.db-wal` and `site.db-shm` while a command has the database
  open. SQLite removes them when the command ends cleanly. If a
  command failed or was cut short, they can stay behind. Leave them
  in place. The next run reads the changes they still hold.
- `site.db.migrate.lock`, which every migration takes so that two
  runs never migrate at once. It stays after the run. Never delete it.

All three sit beside the file the path finally points at, so when
the path is a symbolic link, they sit beside the file the link points
at, not beside the link.

### What runs on SQLite

[On a SQLite database](/self-hosting/commands/#on-a-sqlite-database)
lists what each command does on a SQLite file, and the error a
command gives when it stops.

[Cron jobs](/self-hosting/cron/), the session sweep included, run only
while `serve` runs, so none run on a SQLite site yet. `check` still
checks the cron settings.

Plugins get no database on SQLite. Their database address is empty,
their share of the database answers every statement with an error,
and they cannot read posts yet, see
[the plugin SDK](/extending/the-plugin-sdk/#the-database). They can
read account names, see
[reading account names](/extending/the-plugin-sdk/#reading-account-names).

### The SQLite settings

Only a `sqlite:` address reads these. A PostgreSQL address ignores
them, so a bad value beside one stops nothing.
[`GOPHENBERG_DATABASE_MAX_CONNS`](#the-database-connections) caps the
connections to the file, as it caps a PostgreSQL pool.

| Variable | Default | Purpose |
| --- | --- | --- |
| `GOPHENBERG_SQLITE_BUSY_TIMEOUT` | `5s` | How long a connection waits for a lock another one holds, in whole milliseconds, up to `596h31m23.647s` |
| `GOPHENBERG_SQLITE_CACHE_SIZE` | `2000` | The page cache of each connection, in kibibytes, up to 2147483647. All the caches together may take this times `GOPHENBERG_DATABASE_MAX_CONNS` |
| `GOPHENBERG_SQLITE_SYNCHRONOUS` | `full` | `full` writes the log to disk at every commit. `normal` writes it only at each checkpoint, which is faster, but a power cut can lose the last commits |
| `GOPHENBERG_SQLITE_JOURNAL_SIZE_LIMIT` | | The size in bytes the write-ahead log is cut back to after a checkpoint. Unset sets no limit, so the log keeps the largest size it reached |
| `GOPHENBERG_SQLITE_MIGRATE_LOCK_WAIT` | `30s` | How long a migration waits for the lock file before it stops with an error |
| `GOPHENBERG_SQLITE_MIGRATE_LOCK_POLL` | `250ms` | How often a waiting migration tries the lock file again |

`check` refuses a bad value and names it, for example
`GOPHENBERG_SQLITE_SYNCHRONOUS: must be full or normal, got "off"`.

## The admin lists

The server reads seven settings when it starts. It serves six of them
to the admin and keeps the page cap to itself, and no screen can
change them. They shape every list the same way:

- **The page sizes.** The Items per page menu offers the sizes in
  `GOPHENBERG_LIST_PAGE_SIZES`, and a list opens at
  `GOPHENBERG_LIST_PAGE_SIZE`. The menu only shows for 2 to 6 sizes,
  so the server refuses any other count rather than hide it. The
  media picker opens at `GOPHENBERG_PICKER_PAGE_SIZE`, 50 as
  WordPress's media dialog does, or the size a list opens at when
  the sizes leave out 50.
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
  WordPress, it writes the date the way WordPress does in the
  reader's language, with a short month in English and Spanish and
  a whole one in French.

## Avatars

An avatar is the small round picture beside a person's name. The
three avatar settings copy the Avatars part of WordPress's Discussion
settings. The server reads them when it starts. The author column of
the admin's content lists follows them, and so does every plugin that
shows who wrote something.

- **`initials`**, the default, draws the first letter of the name in
  a coloured circle. The site draws it, so nothing is sent to another
  service and the browser makes no extra request.
- **`gravatar`** shows the picture [Gravatar](https://gravatar.com)
  keeps for the person's email address. The site never sends the
  address itself, only a hash of it, but Gravatar still learns which
  of its pictures your visitors load. That is why it is not the
  default. `GOPHENBERG_AVATAR_DEFAULT` picks the picture Gravatar
  shows for an address it does not know, and
  `GOPHENBERG_AVATAR_RATING` the most mature picture it may show.
  WordPress also offers Initials as the default picture, which asks
  Gravatar to draw the letters and so sends it up to two letters of
  the name. Gophenberg leaves that choice out and refuses it, so the
  name never reaches Gravatar. For letters drawn without Gravatar, use
  the `initials` mode, which draws the first letter on the site.
- **`off`** shows no avatar. The admin's author column shows the
  author icon before the name, as WordPress does when Show Avatars
  is unticked.

The two Gravatar settings change nothing in the other modes.

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
3. Gives a cron job still running `GOPHENBERG_CRON_STOP_GRACE` to
   stop and record itself.
4. Gives the plugins `GOPHENBERG_SHUTDOWN_STOP_GRACE` to stop.
5. Gives the theme `GOPHENBERG_THEME_STOP_GRACE` to stop.

Whatever sends the signal also waits, and kills the server when its
own wait runs out. Docker waits 10 seconds unless the compose file
sets `stop_grace_period`. Kubernetes waits
`terminationGracePeriodSeconds`, 30 seconds unless set. Keep that wait
longer than the five settings above added together, 25 seconds at
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
- `GOPHENBERG_DATABASE_URL` names a SQLite file, which the site cannot
  serve from yet, see [a SQLite database](#a-sqlite-database).
- `GOPHENBERG_DATABASE_URL` is a PostgreSQL connection string that
  holds any `pool_` setting, which the message names, or a form
  Gophenberg refuses, see
  [the PostgreSQL address](#the-postgresql-address).
- `GOPHENBERG_DATABASE_MAX_CONNS` is not a whole number from 2 to
  2147483647.
- `GOPHENBERG_PLUGIN_DB_RESERVE` leaves the plugins no connection
  under `GOPHENBERG_DATABASE_MAX_CONNS`.
- `GOPHENBERG_TRUSTED_PROXIES` is not valid CIDR notation.
- `GOPHENBERG_PUBLIC_URL` is not an `http` or `https` address naming
  only a host.
- `GOPHENBERG_WEB_DIR` names a folder that holds no `index.html`, or
  one whose `index.html` the server cannot read, such as a page only
  another user may read. The message names the reason and tells you
  to unset the setting to serve the admin built into the binary.
- `GOPHENBERG_FEED_ITEMS` is not a positive whole number.
- `GOPHENBERG_MEDIA_UPLOAD_CAP_MB` is not a positive whole number, or
  names more megabytes than the server can count in bytes.
- `GOPHENBERG_MEDIA_CONCURRENT_IMAGES` is not a whole number from 1 to 64.
- `GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB` is not a whole number from 1 to
  1024, which is as much of a request body as the server reads.
- `GOPHENBERG_THEME_START_ATTEMPTS` is not a whole number from 1 to 1000.
- `GOPHENBERG_FIELD_DEPTH` is not a whole number from 1 to 1000.
  These four refuse rather than quietly using another value.
- Any of the cache windows is not a positive whole number of seconds.
  Write them as durations, `1h`, `90s`, `5m`. Part of a second is
  refused, because the header counts in whole seconds.
- Any of the theme timings, HTTP timeouts, shutdown graces, cron job
  timings or the upload timeout is not a positive duration. Write them
  the way Go does, `30s`, `500ms`, `1m`.
- `GOPHENBERG_THEME_MAX_BACKOFF` stands below
  `GOPHENBERG_THEME_BACKOFF`, which would leave no room to grow.
- `GOPHENBERG_CRON_LEASE` does not stand above
  `GOPHENBERG_CRON_TIME_LIMIT`, which would let a second run take a
  job that is still allowed to run.
- `GOPHENBERG_CRON_SECRET` is set but shorter than 32 characters.
  The message names its length, never the secret.
- `GOPHENBERG_CRON_ON_VISITS` is `false` while
  `GOPHENBERG_CRON_SECRET` is empty, so no cron job could ever run.
- `GOPHENBERG_LIST_PAGE_SIZES` lists fewer than 2 or more than 6
  sizes, lists a size twice or out of order, or names a size above
  `GOPHENBERG_LIST_PAGE_CAP`.
- `GOPHENBERG_LIST_PAGE_SIZE` is not one of those sizes.
- `GOPHENBERG_PICKER_PAGE_SIZE` is not one of those sizes.
- `GOPHENBERG_LIST_PAGE_CAP` is not a whole number from 1 to
  2147483647, or stands below the largest page size.
- `GOPHENBERG_TOAST_DURATION` is not a whole number of milliseconds
  from `1ms` to `2147483647ms`, the longest a browser timer holds.
- `GOPHENBERG_TOAST_NAME_LENGTH` is not a positive whole number.
- `GOPHENBERG_FORMAT_LOCALE` is not a language tag such as `es-ES` or
  `en-GB`.
- `GOPHENBERG_AVATARS`, `GOPHENBERG_AVATAR_DEFAULT` or
  `GOPHENBERG_AVATAR_RATING` names a value outside its list. The
  message names the setting and the value.
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
database. On a PostgreSQL address it also checks the connection
string, every `pool_` setting in it included, and the connection cap.
On a `sqlite:` address it also checks the SQLite settings and that the
file exists on a local disk, still without connecting to it. It
answers `settings, plugins and command names are valid`, or names the
settings it refuses and exits with code 1:

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
| `/admin/` | The admin, built into the binary. Its hashed files are cached for a year unless set otherwise, and the page is checked on every load | The screens ask for one |
| `/api/...` | The admin's JSON API | Yes, apart from signing in and out |
| `/api/content/v1/...` | The [public content API](/reference/content-api/) | No |
| `/api/cron` | Runs the [cron jobs](/self-hosting/cron/) for the host's cron, `POST` only, served only once a secret is set | The cron secret, in a header |
| `/api/plugins/feed/rss.xml` | The [RSS feed](/reference/rss-feed/) | No |
| `/gophenberg/...` | Site assets, cached for an hour unless set otherwise | No |
| `/_gophenberg/...` | Reserved for internal use | Answers 404 |

The last row answering 404 from outside is correct behavior, not
an outage.
