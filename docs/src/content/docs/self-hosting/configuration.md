---
title: Configuration
description: Every environment variable, what the binary serves where, and what stops startup.
---

Gophenberg is configured entirely through environment variables. A
`.env` file in the working directory is read at startup, and real
environment variables win over it. Every command reads the same
settings, the `.env` file included, so `migrate` or an account
command run beside the server finds the database the server uses.
The two exceptions are `server:install` and `server:uninstall`, which
read [the server settings](#the-server-settings) and take none of
their settings from a `.env` file.

Every value is trimmed of the spaces around it. A padded value works
as if it had none, and a value made only of spaces counts as unset,
so it takes the default.

## The variables

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `GOPHENBERG_DATABASE_URL` | Yes | | The PostgreSQL connection string, or `sqlite:` and the absolute path of a SQLite file. [The PostgreSQL address](#the-postgresql-address) lists what a connection string may not hold. [A SQLite database](#a-sqlite-database) says what runs on one and lists its settings |
| `GOPHENBERG_DATABASE_MAX_CONNS` | No | The larger of 4 and the number of CPUs | How many connections to the database stay open at most, from 2 to 2147483647, see [the database connections](#the-database-connections) |
| `GOPHENBERG_MAX_PROCS` | No | Go's own | How many processors run Go code at once, from 1 to the number of CPUs, see [memory and processors on a small host](#memory-and-processors-on-a-small-host) |
| `GOPHENBERG_MEMORY_LIMIT_MB` | No | | The memory Go tries to stay under, in megabytes from 1 to 1073741824, see [memory and processors on a small host](#memory-and-processors-on-a-small-host) |
| `GOPHENBERG_DATABASE_SUPERUSER_URL` | Only for `database:create-owner` | | The superuser [`database:create-owner`](/self-hosting/commands/#databasecreate-owner) logs in as. No other command reads it |
| `GOPHENBERG_SUPERUSER_POLICY` | No | `refuse` | What the server does when it connects to PostgreSQL as a role that can reach the database server, such as a superuser: `refuse` stops it, `warn` logs a warning and serves, see [the PostgreSQL server](#the-postgresql-server) |
| `GOPHENBERG_DATABASE_WAIT` | No | `60s` | How long `serve` waits for a PostgreSQL server that is not up yet before it stops. `0` does not wait, see [a database that starts late](#a-database-that-starts-late) |
| `GOPHENBERG_DATABASE_WAIT_INTERVAL` | No | `1s` | The pause between two tries while `serve` waits for the database |
| `GOPHENBERG_ADDR` | No | `localhost:8081` | Address the server listens on. The container image sets `0.0.0.0:8081` |
| `GOPHENBERG_SOCKET` | No | | A private Unix socket to listen on in place of an address, on Linux only, see [listening on a small host](#listening-on-a-small-host) |
| `PORT` | No | | The port a host names, on every interface, when neither a socket nor an address is set |
| `GOPHENBERG_CLIENT_ADDR_HEADER` | With a socket | | The header the program in front of the socket names the visitor in |
| `GOPHENBERG_IDLE_EXIT` | No | `0` | How long the site waits with nothing open before it exits with code 0. `0` keeps it serving |
| `GOPHENBERG_WEB_DIR` | No | | A built admin folder to serve in place of the admin built into the binary. It must hold `index.html` |
| `GOPHENBERG_SITE_TITLE` | No | `Gophenberg` | The site name shown by the built-in renderer |
| `GOPHENBERG_TRUSTED_PROXIES` | No | | Comma-separated CIDR ranges allowed to set forwarded headers |
| `GOPHENBERG_PUBLIC_URL` | No | | The address people reach the site at, such as `https://example.com`. When set, a write sent to any other address is refused |
| `GOPHENBERG_THEMES_DIR` | No | | The directory themes are installed in, which uploads write to. The image sets `/themes` |
| `GOPHENBERG_MEDIA_DIR` | No | `data/media` inside `GOPHENBERG_DATA_DIR` | The directory uploaded media is stored in and served from. The image sets `/media` |
| `GOPHENBERG_DATA_DIR` | No | | A folder outside the app folder for `backups/`, `state/` and `data/`, an absolute path. See [a data folder outside the install root](/self-hosting/updates-and-backups/#a-data-folder-outside-the-install-root) |
| `GOPHENBERG_THEME` | No | | Pins one theme, overriding the admin. Empty lets the admin choose |
| `GOPHENBERG_NODE_BIN` | No | `node` | The Node binary themes run on. The image sets its own |
| `GOPHENBERG_MEDIA_UPLOAD_CAP_MB` | No | `128` | The largest upload the media library takes, in megabytes |
| `GOPHENBERG_MEDIA_CONCURRENT_IMAGES` | No | `2` | How many uploaded images the server decodes and resizes at once, from 1 to 64. The rest wait their turn |
| `GOPHENBERG_MEDIA_MAX_MEGAPIXELS` | No | `80` | The most pixels a picture or one frame of a GIF may hold, in millions. A larger one is refused |
| `GOPHENBERG_UPLOAD_TIMEOUT` | No | `5m` | How long a media or theme upload has to arrive, in place of `GOPHENBERG_HTTP_READ_TIMEOUT` |
| `GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB` | No | `256` | The largest definitions file an import takes, in kilobytes, from 1 to 1024 |
| `GOPHENBERG_FIELD_DEPTH` | No | `32` | How many containers a field may stand inside, from 1 to 1000. A Flexible content layout counts as one |
| `GOPHENBERG_PLUGIN_DB_CONNS` | No | `4` | How many statements, open result sets and transactions each plugin adds to the plugins' database share, from 1 to 1000 |
| `GOPHENBERG_PLUGIN_DB_RESERVE` | No | `2` | How many of the database pool's connections the plugins' share always leaves to the site, from 1 to 1000. The share holds at most `GOPHENBERG_DATABASE_MAX_CONNS` less this reserve. The server refuses to start when the reserve leaves plugins no connection, so a cap of 2 needs a reserve of 1 |
| `GOPHENBERG_PLUGIN_QUERY_TIMEOUT` | No | `5s` | How long one plugin statement may run, its wait for a slot included. It also ends a plugin's read of account names |
| `GOPHENBERG_PLUGIN_TX_TIMEOUT` | No | `30s` | How long one plugin transaction may stay open |
| `GOPHENBERG_PLUGIN_START_BUDGET` | No | `10ms` | How long one plugin's `Start` may take before `serve` logs a warning naming it. A slower plugin still starts, see [a plugin that fails at start](#a-plugin-that-fails-at-start) |
| `GOPHENBERG_PLUGIN_HOOK_BUDGET` | No | `2ms` | How long one call of a plugin hook may take, all its plugins together. A plugin still working when it runs out is logged, see [a plugin whose hook fails](#a-plugin-whose-hook-fails) |
| `GOPHENBERG_PLUGIN_HOOK_TIMED_FIRINGS` | No | `20` | How many calls of a hook, after one ran past its budget, time each of its plugins, from 1 to 1000 |
| `GOPHENBERG_AVATARS` | No | `initials` | How the site draws avatars: `initials`, `gravatar` or `off`, see [avatars](#avatars) |
| `GOPHENBERG_AVATAR_DEFAULT` | No | `mystery` | The picture Gravatar shows for an email address it does not know: `mystery`, `blank`, `gravatar_default`, `identicon`, `wavatar`, `monsterid`, `retro`, `robohash` or `color`. WordPress's `initials` is refused, see [avatars](#avatars) |
| `GOPHENBERG_AVATAR_RATING` | No | `G` | The most mature Gravatar picture an avatar may show: `G`, `PG`, `R` or `X` |
| `GOPHENBERG_LIST_PAGE_SIZES` | No | `10,20,50,100` | The page sizes an admin list offers in its Items per page menu, from 2 to 6 of them, see [the admin lists](#the-admin-lists) |
| `GOPHENBERG_LIST_PAGE_SIZE` | No | `20` | The page size an admin list opens at, one of the sizes above |
| `GOPHENBERG_LIST_PAGE_CAP` | No | `100` | The most items one admin list page carries. A request for more gets this many, and a read of named files may name no more than this many in one request |
| `GOPHENBERG_PICKER_PAGE_SIZE` | No | `50` | The page size the media picker opens at, one of the sizes above. Unset and with no `50` among them, the size a list opens at |
| `GOPHENBERG_TOAST_DURATION` | No | `6s` | How long a confirmation message with no button stays on screen, in whole milliseconds from `1ms` to `2147483647ms` |
| `GOPHENBERG_TOAST_ACTION_DURATION` | No | `10s` | How long a confirmation message with a button stays on screen, in whole milliseconds from `1ms` to `2147483647ms` |
| `GOPHENBERG_FORMAT_LOCALE` | No | `es-ES` | The locale the admin writes dates, times and numbers in, whatever language its screens are in |
| `GOPHENBERG_DEFAULT_ROLE` | No | `member` | The role the Add User screen starts on, as WordPress's New User Default Role does. It must be a role the core or a plugin declares, see [roles](/guides/users/#roles) |
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
| `GOPHENBERG_SHUTDOWN_GRACE` | No | `10s` | How long a stop gives running requests to finish, the stop after an update started from the admin included, see [stopping the server](#stopping-the-server) |
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
| `GOPHENBERG_UPDATE_CHECKS` | No | `true` | Whether the site checks for a newer Gophenberg on its own. `update:check` and the admin still check when asked |
| `GOPHENBERG_UPDATE_CHECK_INTERVAL` | No | `12h` | How long after one check the site checks again on its own |
| `GOPHENBERG_UPDATE_URL` | No | `https://updates.gophenberg.org` | The update site. It must be `https`, or `http` on a loopback host such as `localhost` |
| `GOPHENBERG_UPDATE_CHANNEL` | No | `stable` | The channel the site follows, in lowercase letters, digits and hyphens |
| `GOPHENBERG_TRUSTED_KEYS` | No | | Root keys trusted for an update site other than the official one, as P-256 Multikeys separated by commas, beside the two built into the binary. That site's key list must also name a timestamp key, and the site must serve a timestamp file that has not expired, see [your own copy of the update site](/self-hosting/updates-and-backups/#your-own-copy-of-the-update-site) |
| `GOPHENBERG_UPDATE_TIMEOUT` | No | `30s` | How long a check has to fetch each file |
| `GOPHENBERG_UPDATES_SCREEN_CHECK_AGE` | No | `1m` | How long after one check the admin waits before it runs another when asked |
| `GOPHENBERG_UPDATES_SCREEN_PROGRESS_INTERVAL` | No | `2s` | How often the Updates screen asks for the progress of an update it started, in whole milliseconds from `1ms` to `2147483647ms` |
| `GOPHENBERG_UPDATES_SCREEN_RESTART_WAIT` | No | `5m` | How long the Updates screen waits for the site to come back after an update before it says the site has not come back, in whole milliseconds from `1ms` to `2147483647ms` |
| `GOPHENBERG_INSTALL_ROOT` | No | | The install root of a server install, an absolute path that holds each build in `versions/` beside a `current` link naming the one that runs. `update:apply` updates only an install that sets it. Empty, as in the Docker image, means the site only tells you about updates, see [updating a server install](/self-hosting/updates-and-backups/#updating-a-server-install). `server:install` takes `/opt/gophenberg` when it is empty, see [the server settings](#the-server-settings) |
| `GOPHENBERG_UPDATE_DOWNLOAD_CAP_MB` | No | `200` | The most megabytes an update downloads for the release archive |
| `GOPHENBERG_UPDATE_DOWNLOAD_TIMEOUT` | No | `10m` | How long an update has to download the release archive |
| `GOPHENBERG_UPDATE_DISK_MARGIN_MB` | No | `64` | How many megabytes of free disk an update asks for beyond `GOPHENBERG_UPDATE_DOWNLOAD_CAP_MB` and, when it takes a backup, the size of the database, so the site still has room once the update is done. `0` asks for none |
| `GOPHENBERG_UPDATE_BACKUPS_KEEP` | No | `3` | How many `pg_dump` backups the `backups` folder of the install root keeps, from 1 to 100 |
| `GOPHENBERG_PG_DUMP_BIN` | No | `pg_dump` | The `pg_dump` an update backs up the database with, a name found on the path or a full path. Its major version must be at least the server's |
| `GOPHENBERG_UPDATE_ATTEMPTS` | No | `3` | How many times the build an update switched to may try to start. The next start goes back to the build before it, or stays down when the update raised the minimum version and its new build reached the database |
| `GOPHENBERG_UPDATE_VERIFY_TIMEOUT` | No | `1m` | How long the first start of a new build has to check itself: the server listens, then `/admin/` and `/api/locale` answer 200 and `/` answers below 500 over loopback. `server:install` reads its own from [the server settings](#the-server-settings) |
| `GOPHENBERG_UPDATE_VERSIONS_KEEP` | No | `2` | How many of the newest builds the `versions` folder keeps once a new build passes its check. The builds the `current` and `previous` links name always stay |
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
  Leave it and `GOPHENBERG_DATA_DIR` unset and the media library
  refuses every upload. Point it at a volume that survives a restart,
  and back it up alongside the database.

Every cache window applies only to an answer that succeeded. An
address nothing holds, a file that is missing or a request the server
refused is never kept by any cache, however the windows are set. An
answer to a signed in account is never kept by any cache either, not
even that person's own browser.

## The PostgreSQL server

Gophenberg needs PostgreSQL 15 or newer. On an older server, `serve`,
`migrate` and every command that migrates stop before they change
anything, and name the version they found.

Connect as the database's own owner role, not as a superuser such as
`postgres`. A superuser can make the database server run programs, so
a fault in Gophenberg or in a plugin that let someone send it SQL would
reach the shell of that server. So `serve` stops before it changes
anything when its role can reach the database server. That covers a
superuser, a role that can create roles, a member of
`pg_execute_server_program`, `pg_read_server_files` or
`pg_write_server_files`, and a member of any of those. Set
`GOPHENBERG_SUPERUSER_POLICY` to `warn` to log a warning and serve
instead. A SQLite file has no roles, so the setting does nothing there.

[`database:create-owner`](/self-hosting/commands/#databasecreate-owner)
sets the owner role up for you, and the compose file from
[Install](/self-hosting/install/) runs it on every start.

### Moving a site off `postgres`

If your site still connects as `postgres`, it stops with an error that
names `GOPHENBERG_SUPERUSER_POLICY`. To move it, give your compose file
the `database-owner` service from
[Install](/self-hosting/install/), make `gophenberg` depend on it as
that file does, and point the site's `GOPHENBERG_DATABASE_URL` at the
new role. Then run:

```sh
docker compose up -d
```

The command creates the role and gives it the database and everything
inside, then the site starts on it. Nothing you wrote changes.

### A database that starts late

A server often starts Gophenberg before PostgreSQL answers, at boot or
right after an update. `serve` then tries the database again every
`GOPHENBERG_DATABASE_WAIT_INTERVAL`, 1 second by default, for up to
`GOPHENBERG_DATABASE_WAIT`, 60 seconds by default, before it checks the
role it connects as and migrates. It logs
`waiting for the database to answer` once when it starts to wait, with
the reason, and `the database answered` once it does, with how long
that took.

Only a database that is not up yet is worth the wait: a connection
refused, a socket that is not there yet, or a server that says it is
still starting up. A wrong password, a missing role or a missing
database stops `serve` at once with its own message, since waiting
cannot fix them. Each try gets only the time the wait has left, so a
server that takes the connection and never answers holds `serve` no
longer than the wait. A database that still does not answer once the
wait runs out stops `serve` with code 1, and the message ends with the
last reason:

```text
gophenberg: the database did not answer within 1m0s, the time GOPHENBERG_DATABASE_WAIT allows: failed to connect to `user=gophenberg database=gophenberg`: ...
```

`GOPHENBERG_DATABASE_WAIT=0` stops at the first try instead, and that
one try has no time bound of its own. Only
`serve` waits. `migrate`, `seed -yes` and the `account:` commands stop
at once, so a person running them sees the reason straight away.

Right after an update, a start that waits still counts as one attempt
of the new build, so a database down for longer than about three waits
takes the update back, see
[a database that starts late after an update](/self-hosting/updates-and-backups/#a-database-that-starts-late-after-an-update).

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

## Memory and processors on a small host

A small hosting plan caps the memory of a site and how many processes
and threads it may start. Go reads `GOMAXPROCS` and `GOMEMLIMIT`
before Gophenberg loads `.env`, so a line for either in `.env` does
nothing. Set `GOPHENBERG_MAX_PROCS` and `GOPHENBERG_MEMORY_LIMIT_MB`
instead. Every command hands them to Go, after `.env` for the
commands that read it, and `serve` logs `runtime limits` with the
values Go holds. A set value only ever lowers what Go holds, so the
tighter of it and `GOMAXPROCS`, `GOMEMLIMIT` or the container's CPU
quota wins. An unset one leaves Go its own default.

`GOPHENBERG_MAX_PROCS` bounds the processors that run Go code at
once. Go still starts a thread for each call that waits on the
system, so it lowers the thread count without capping it. Once it
takes effect, Go no longer follows a change to the container's CPU
quota while the site runs.

`GOPHENBERG_MEMORY_LIMIT_MB` is a soft limit. Go collects garbage
harder as the site nears it, and a site that holds more than the
limit keeps running while the collector works almost without pause.
Set it well above what the site uses at rest.

An upload is where a site needs the most memory. A 24 megapixel JPEG
from a camera needs about 60 MB while its sizes are made, the decoded
picture included, and the request holds up to two more copies of the
uploaded file. A progressive JPEG, which a web export often is, costs
more: about 205 MB at the same size, and about 385 MB with full
colour detail. `GOPHENBERG_MEDIA_CONCURRENT_IMAGES` pictures are made
at once, and `GOPHENBERG_MEDIA_MAX_MEGAPIXELS` refuses a picture past
its budget before it is decoded. Size a small host from the
progressive rows below, since the people who upload choose the file.

| Picture                                          | Memory per megapixel once decoded |
| ------------------------------------------------ | --------------------------------- |
| Baseline JPEG, the usual camera photo            | about 1.5 MB                      |
| Baseline JPEG with full colour detail (4:4:4)    | about 3 MB                        |
| PNG                                              | about 4 MB                        |
| Progressive JPEG                                 | about 7.5 MB                      |
| CMYK JPEG or 16-bit PNG                          | about 8 MB                        |
| Progressive JPEG with full colour detail (4:4:4) | about 15 MB                       |

## Listening on a small host

A site listens on the first of these that is set:

1. `GOPHENBERG_SOCKET`, a private Unix socket, on Linux only.
2. `GOPHENBERG_ADDR`, a host and port.
3. `PORT`, the bare setting a host names, on every interface.

With none set it listens on `localhost:8081`. A shell or a tool that
exports `PORT` moves a site that names no address, so set
`GOPHENBERG_ADDR` when that is not what you want.

On a server you share, every account can reach a loopback port, so a
neighbour could skip the web server and forge the headers that name a
visitor. A socket has the mode `0600`, so only your own programs can
open it. The program in front of it must keep to these rules, because
the site believes it:

- Write the header `GOPHENBERG_CLIENT_ADDR_HEADER` names on every
  request, with exactly one address, overwriting any copy the visitor
  sent. A request without it is answered with `400`.
- Set `X-Forwarded-Proto` on every request to `http` or `https`,
  overwriting any copy the visitor sent. A write that names no scheme
  is refused unless `GOPHENBERG_PUBLIC_URL` is set, so set it too.
- Pass `Host` as the visitor sent it.

The site believes no other forwarded header over a socket, whatever
`GOPHENBERG_TRUSTED_PROXIES` holds.

A theme and the first start of an update reach the site over TCP, so
`GOPHENBERG_SOCKET` stops the site when `GOPHENBERG_THEMES_DIR` or
`GOPHENBERG_INSTALL_ROOT` is set. Update a site on a socket by hand.

A host that stops idle programs starts the site again at the next
visit. `GOPHENBERG_IDLE_EXIT`, such as `15m`, lets the site leave by
itself: once nothing has been open for that long it stops as it does
on a signal and exits with code 0. A service manager that restarts
only a failed program leaves it stopped, so keep it unset there.

## A SQLite database

Gophenberg can also keep its data in one SQLite file instead of a
PostgreSQL server. Only part of Gophenberg runs on SQLite so far:
`check`, `migrate`, `database:backup` and the account commands work,
but the site cannot serve from the file yet.

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

With neither `GOPHENBERG_DATA_DIR` nor `GOPHENBERG_INSTALL_ROOT` set,
the backups of the file go into `site.db.backups`, a folder beside the
path in the address, see [backups of the file](#backups-of-the-file).

### What runs on SQLite

[On a SQLite database](/self-hosting/commands/#on-a-sqlite-database)
lists what each command does on a SQLite file, and the error a
command gives when it stops.

The backup a new version takes before it migrates the file runs now,
and so does `database:backup`, see
[backups of the file](#backups-of-the-file).

[Cron jobs](/self-hosting/cron/), the session sweep and the scheduled
backup included, run only while `serve` runs, so none run on a SQLite
site yet. `check` still checks the cron settings.

Plugins get no database on SQLite. Their database address is empty,
their share of the database answers every statement with an error,
and they cannot read posts yet, see
[the plugin SDK](/extending/the-plugin-sdk/#the-database). They can
read account names, see
[reading account names](/extending/the-plugin-sdk/#reading-account-names).

### Backups of the file

Gophenberg copies the SQLite file into a backups folder in two cases:

- **Before a new version migrates it.** The first command of a new
  version that migrates the file copies it first: `migrate`,
  `seed -yes`, `account:create-admin`, `account:grant-role -yes` or
  `update:check`. A new build of the same version counts as a new
  version. Later runs of that version take no more backup, and neither
  does a new empty file or a file a newer version migrated.
- **On demand.** [`database:backup`](/self-hosting/commands/#databasebackup)
  copies the file now and prints where the copy went.

A third kind, the scheduled backup, is the
[cron job](/self-hosting/cron/) `core/sqlite-backup`. It runs every
`GOPHENBERG_SQLITE_BACKUP_INTERVAL` and logs `backup taken` with the
path of each copy. Like every cron job it runs only while `serve`
runs, so it takes no backup yet.

Each backup is a whole SQLite file, checked before it takes its name.
It holds only the database. The media files in `GOPHENBERG_MEDIA_DIR`
and your `.env` are not in it, so keep a copy of them as well.

**Where they go.** Into `backups/` inside `GOPHENBERG_DATA_DIR` when it
is set, else inside `GOPHENBERG_INSTALL_ROOT`, the folder that keeps
the PostgreSQL dumps. With neither set, into a folder beside the path
in the address, named for it with `.backups` added, even when that path
is a symbolic link. So `sqlite:/srv/gophenberg/site.db` backs up into
`/srv/gophenberg/site.db.backups/`. Two SQLite files in one folder,
such as `site.db` and a test copy `site-test.db`, each get their own
backups folder and their own count, so a backup of one never pushes out
a backup of the other. With a data folder or an install root, every
file backs up into its one `backups` folder under one count, so give a
test copy its own `GOPHENBERG_DATA_DIR`.

Gophenberg makes a missing data folder and a missing backups folder
with the mode `0700`, and gives each backup the mode `0600`, since a
backup holds every account. A backups folder that is already there and
lets other users in, such as one made with the mode `0755`, is closed
to them the same way before each copy. When it cannot be closed, for
example because another user owns the folder, the backup stops before
it copies anything, and its error names the folder:
`keep the backups folder /srv/gophenberg/site.db.backups from other users`.
The folder also holds `.dbkit-snapshot.lock`, which every backup takes
so that two never write at once. Leave it in place. With an install
root, `update:status` lists the backups beside the dumps.

**Their names.** Each name holds the version that last migrated the
file, the time of the copy in UTC, and the kind:

```text
1.4.0-20261020T120000Z-update.sqlite      taken before 1.4.1 migrated the file
1.4.1-20261021T031500Z-scheduled.sqlite   taken by the schedule
1.4.1-20261022T101500Z-manual.sqlite      taken by database:backup
```

So a backup taken before an update names the version you updated
from. A file that records no version yet, such as one only older
releases migrated, gives a name that starts with `unknown`.

**How many stay.** `GOPHENBERG_SQLITE_BACKUPS_KEEP`, 3 unless set,
counts the update and scheduled backups together. Once a new one lands,
the oldest past that count are deleted, and the one just taken always
stays. A backup `database:backup` took stays out of the count, and
nothing deletes it, so delete it yourself once it is off the server.
Under the defaults, three daily scheduled backups push out the backup
taken before an update, so move that one off the server first.

**Move each backup off the server.** A copy on the server is a
convenience, not a backup. A disk that fails takes it along with the
file, and a web host gives little room for copies. Download each
backup with your host's File Manager, or fetch it over SFTP, then
delete it on the server.

**When a backup fails.** The command stops before it migrates
anything. Its error starts with `take the backup before migrating:`,
names the cause, such as a full disk, and ends with the way out. Free
room first, by moving backups off the server. When that is not enough,
the skip setting lets one version migrate the file with no backup:

```sh
GOPHENBERG_SQLITE_SKIP_BACKUP_FOR=1.4.1
```

- It applies only to the version it names, so a line left in `.env`
  does nothing once you move to the next version. Delete it all the
  same once the file has migrated.
- It never applies to a version that raises the minimum version of the
  file, see
  [going back to an older version](/self-hosting/updates-and-backups/#going-back-to-an-older-version).
  Such a version takes its backup all the same, since only that backup
  can take the file back, and its error says the setting does not
  apply.

**When another backup is running.** Only one backup writes into the
folder at a time. A command that needs its backup meanwhile waits up to
`GOPHENBERG_SQLITE_MIGRATE_LOCK_WAIT`, and checks again every
`GOPHENBERG_SQLITE_MIGRATE_LOCK_POLL`. When the other run took the
backup this version needs, it goes on with no copy of its own.
Otherwise it stops and asks you to run it again once that backup ends.
When two commands still copy the file at about the same time, only one
of the copies stays, and the other run deletes its own copy and goes on.

**What you see.** `migrate` prints the same lines with or without a
backup. On a large file or a slow disk the copy can take minutes before
the first line shows. Stopping the command then loses nothing, since
nothing has migrated yet.

**Run as the site's user.** Run every command as the user that runs
Gophenberg, not as root. As root, a backup stops when another user can
change the backups folder or a folder above it. It also stops when the
backups folder lets other users in, since a run as root never changes a
folder it did not make.

### Restoring a backup

A restore puts a backup in place of the file, and everything written
since that backup is lost. These steps keep the old file, so you can
undo the restore. Say the file is `/srv/gophenberg/site.db`, with no
data folder or install root set, so its backups are in
`/srv/gophenberg/site.db.backups/`, and you go back to the backup taken
before 1.4.1 migrated it.

First make sure no Gophenberg command runs on the file. Then move the
file, and the files SQLite keeps beside it, into a new folder only you
can open. Leave `site.db.migrate.lock` and the `site.db.backups` folder
where they are:

```sh
cd /srv/gophenberg
mkdir -m 700 before-restore-20261023
mv site.db site.db-wal site.db-shm before-restore-20261023/
```

`mv` names each of the three that is not there and moves the rest, so
a missing `-wal` or `-shm` file is fine. Move a `site.db-journal` too
when there is one. When the path in the address is a symbolic link,
take these steps in the folder of the file the link points at, and
name the backup by its full path, since its backups folder sits beside
the link.

Copy the backup in under a temporary name, give it the owner and the
mode the old file had, then rename it to `site.db`:

```sh
cp site.db.backups/1.4.0-20261020T120000Z-update.sqlite site.db.restoring
chown --reference=before-restore-20261023/site.db site.db.restoring
chmod --reference=before-restore-20261023/site.db site.db.restoring
mv site.db.restoring site.db
```

On macOS, which has no `--reference`, read the owner and the mode of
the old file with `ls -l` and set them with `chown` and `chmod`.

Then run `migrate` with the version the backup's name starts with,
1.4.0 here, and check the accounts with `account:list`. A newer
version migrates the file again, and backs it up first. Put back the
media files from the same day too, if you keep them apart.

The folder `before-restore-20261023` holds a whole copy of the old
file and takes room until you delete it. Download it or move it off
the server, then delete it once the restored file works.

### The SQLite settings

Only a `sqlite:` address reads these. A PostgreSQL address ignores
them, so a bad value beside one stops nothing.
[`GOPHENBERG_DATABASE_MAX_CONNS`](#the-database-connections) caps the
connections to the file, as it caps a PostgreSQL pool.

| Variable | Default | Purpose |
| --- | --- | --- |
| `GOPHENBERG_SQLITE_BUSY_TIMEOUT` | `5s` | How long a connection waits for a lock another one holds, in whole milliseconds, up to `596h31m23.647s` |
| `GOPHENBERG_SQLITE_BUSY_POLL` | `10ms` | How often opening a new connection tries the lock again while another connection holds the file, in whole milliseconds, up to `GOPHENBERG_SQLITE_BUSY_TIMEOUT`. A busy timeout under 10ms lowers the default to match it. A longer poll tries less often, so opening a connection can wait up to one poll after the lock is free. Every other wait, such as a write behind another write, tries again on SQLite's own schedule, which slows down to one try every 100ms |
| `GOPHENBERG_SQLITE_CACHE_SIZE` | `2000` | The page cache of each connection, in kibibytes, up to 2147483647. All the caches together may take this times `GOPHENBERG_DATABASE_MAX_CONNS` |
| `GOPHENBERG_SQLITE_SYNCHRONOUS` | `full` | `full` writes the log to disk at every commit. `normal` writes it only at each checkpoint, which is faster, but a power cut can lose the last commits |
| `GOPHENBERG_SQLITE_JOURNAL_SIZE_LIMIT` | | The size in bytes the write-ahead log is cut back to after a checkpoint. Unset sets no limit, so the log keeps the largest size it reached |
| `GOPHENBERG_SQLITE_MIGRATE_LOCK_WAIT` | `30s` | How long a migration waits for the lock file before it stops with an error. A command that needs its backup while another backup runs waits as long, see [backups of the file](#backups-of-the-file) |
| `GOPHENBERG_SQLITE_MIGRATE_LOCK_POLL` | `250ms` | How often a waiting migration tries the lock file again, and how often a command waiting for another backup checks again |
| `GOPHENBERG_SQLITE_BACKUP_INTERVAL` | `24h` | How often the schedule backs up the file, from `1h` to `744h`, 31 days. `0` turns the schedule off. It runs only while `serve` runs, so not yet on SQLite |
| `GOPHENBERG_SQLITE_BACKUPS_KEEP` | `3` | How many update and scheduled backups the backups folder keeps together, from 1 to 100. The backup just taken always stays, and a backup `database:backup` took stays out of the count |
| `GOPHENBERG_SQLITE_BACKUP_TIME_LIMIT` | `15m` | How long one scheduled backup may run before it is cut short, up to `24h`, in place of `GOPHENBERG_CRON_TIME_LIMIT` |
| `GOPHENBERG_SQLITE_SKIP_BACKUP_FOR` | | The one version, such as `1.4.1`, that migrates the file with no backup first. It does nothing for any other version, nor for a version that raises the minimum version of the file |

`check` refuses a bad value and names it, for example
`GOPHENBERG_SQLITE_SYNCHRONOUS: must be full or normal, got "off"`, or
`GOPHENBERG_SQLITE_BUSY_POLL: must stand at or below the 5s of GOPHENBERG_SQLITE_BUSY_TIMEOUT, got "6s"`
for a poll longer than the busy timeout. A bad backup setting reads
the same way, such as
`GOPHENBERG_SQLITE_BACKUP_INTERVAL: must be 0 or at least 1h, got "10m"`.
Every command that migrates the file reads the backup settings too, so
a bad one stops it before it migrates, and so does a bad
`GOPHENBERG_DATA_DIR` or `GOPHENBERG_INSTALL_ROOT`.

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
  largest page size has to fit under it. The admin splits a longer
  read of named files by the largest page size.
- **The confirmation messages.** `GOPHENBERG_TOAST_DURATION` sets how
  long a message with no button stays on screen.
  `GOPHENBERG_TOAST_ACTION_DURATION` sets how long a message with a
  button stays, such as the message with **Undo** after a trash from
  a list or from the editor. It is 10 seconds, as WordPress's next
  release keeps it, so there is time to reach the button. A message
  always shows the whole name of the item.
- **The format.** `GOPHENBERG_FORMAT_LOCALE` sets the locale for
  dates, times and numbers, whatever language a person reads the
  admin in. The tag `es-ES` writes 04/10/2026, 14:05 and 1.234,56.
  The date column of a content list, the Created column of the Users
  list and the two dates of the media library are the exceptions.
  Like WordPress, they write the date the way WordPress does in the
  reader's language. The two lists write a short month in English and
  Spanish and a whole one in French. The media library writes the
  whole date and time, such as
  `8 de octubre de 2026 17:54` in Spanish.

## Avatars

An avatar is the small round picture beside a person's name. The
three avatar settings copy the Avatars part of WordPress's Discussion
settings. The server reads them when it starts. The author column of
the admin's content and media lists follows them, and so do the Users
list and every plugin that shows who wrote something.

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
terminal. It also begins once an update started from the admin
switched builds, and that stop ends with exit code 75, see
[updating from the admin](/self-hosting/updates-and-backups/#updating-from-the-admin).
From then on it takes no new request, and it:

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

## The server settings

[`server:install`](/self-hosting/commands/#serverinstall) runs as root
to set up a Linux server, so it never reads a `.env` file, which the
site's own user can write. It takes each of its settings from the
first of these that gives it a value:

1. Its flags. `-domain example.com` gives
   `GOPHENBERG_PUBLIC_URL=https://example.com`.
2. Its environment. `sudo` leaves out your own environment, so name a
   setting through `env`, such as
   `sudo env GOPHENBERG_INSTALL_DISK_FLOOR_MB=2048 ./gophenberg server:install -dry-run -domain example.com`.
3. The file `/etc/gophenberg/server.env`, when it is there.
4. The default below.

| Variable | Default | Purpose |
| --- | --- | --- |
| `GOPHENBERG_INSTALL_ROOT` | `/opt/gophenberg` | The install root, an absolute path of letters, digits, dots, hyphens, underscores and slashes alone |
| `GOPHENBERG_PUBLIC_URL` | | `https://` and the domain the site answers at, with nothing after it. Every run needs it, from `-domain`, the environment or `server.env` |
| `GOPHENBERG_ADDR` | `127.0.0.1:8081` | The address the site listens on. It must be a loopback address with a port, since Caddy, or your own web server, runs on the same server and forwards to it. Its port must not be 5432, PostgreSQL's, or with Caddy 80 or 443 |
| `GOPHENBERG_INSTALL_WEB_SERVER` | `caddy` | The web server in front of the site, from the `-web-server` flag. `caddy` has the install set Caddy up, and `none` keeps a web server of your own, see [with your own web server](/self-hosting/commands/#with-your-own-web-server). A later run keeps it. It stops when a flag or the environment asks for the other one, and when the line says `none` while Gophenberg's Caddy site is still there, see [moving to your own web server](/self-hosting/commands/#moving-to-your-own-web-server) |
| `GOPHENBERG_INSTALL_CERTIFICATE_EMAIL` | The `-admin-email` flag | The email Caddy gives Let's Encrypt for the HTTPS certificate, one plain address with no braces. Empty leaves it out, and `localhost` never uses it |
| `GOPHENBERG_SERVICE_RESTART_DELAY` | `5s` | How long systemd waits before it starts the site again, `RestartSec` of the site's unit, in whole seconds |
| `GOPHENBERG_SERVICE_START_LIMIT_INTERVAL` | `300s` | The window systemd counts the site's starts in, `StartLimitIntervalSec` of the site's unit, in whole seconds |
| `GOPHENBERG_SERVICE_START_LIMIT_BURST` | `10` | How many starts systemd allows in that window before it stops starting the site, `StartLimitBurst`, from 1 to 4294967295 |
| `GOPHENBERG_SERVICE_STOP_MARGIN` | `5s` | The time the site's unit gives the site to stop beyond the five graces of [stopping the server](#stopping-the-server). `TimeoutStopSec` is all six added together and rounded up to a whole second, `30s` at the defaults |
| `GOPHENBERG_MAINTENANCE_RETRY_AFTER` | `600s` | The `Retry-After` header of the maintenance page Caddy answers with while the site is down, in whole seconds |
| `GOPHENBERG_PROXY_RETRY_DURATION` | `10s` | How long Caddy keeps trying the site for a request while the site restarts, before it answers with the maintenance page |
| `GOPHENBERG_INSTALL_MEMORY_FLOOR_MB` | `900` | The least memory the memory check takes, in megabytes, from 1 to 1073741824 |
| `GOPHENBERG_INSTALL_DISK_FLOOR_MB` | `1024` | The least free disk the disk check takes under the install root and under `/var/lib`, in megabytes, from 1 to 1073741824 |
| `GOPHENBERG_INSTALL_APT_LOCK_WAIT` | `5m` | How long the step that installs the packages waits for apt's lock, in whole seconds. `0` does not wait |
| `GOPHENBERG_INSTALL_DOMAIN_CHECK_TIMEOUT` | `10s` | How long the domain check waits for each fetch of its token |
| `GOPHENBERG_INSTALL_HTTPS_WAIT` | `2m` | How long the HTTPS step waits for `https://<domain>/admin/` to answer |
| `GOPHENBERG_INSTALL_POLL_INTERVAL` | `1s` | The pause between two tries in each wait of the steps |
| `GOPHENBERG_INSTALL_PROGRESS_INTERVAL` | `30s` | How often a step prints that a program it runs is still running, with the last line that program printed, or that it still waits for the site or for HTTPS, with why the last try failed, in whole seconds. `0` prints no such line, and each step still names the programs it runs |
| `GOPHENBERG_UPDATE_VERIFY_TIMEOUT` | `1m` | How long the step that starts the site waits for its first start to pass the same check an update's first start does. Updates take theirs from the site's `.env` |

With `GOPHENBERG_INSTALL_WEB_SERVER=none`, the run uses neither the
certificate email, `GOPHENBERG_MAINTENANCE_RETRY_AFTER`,
`GOPHENBERG_PROXY_RETRY_DURATION` nor the two domain and HTTPS waits,
since those are Caddy's. It still stops on a bad value of one, and
keeps their lines in `server.env`.

Every duration but the apt lock wait and the progress interval must
stand above zero. A bad value stops the run before the server is
checked, and the message names the setting, every bad one at once:

```text
gophenberg: GOPHENBERG_ADDR: must be a loopback address with a port, such as 127.0.0.1:8081, got "0.0.0.0:8081"
```

`server:install` takes no `GOPHENBERG_DATABASE_URL`. When one is set
in its environment, the run stops, since the install sets up a
PostgreSQL of its own.

[`server:uninstall`](/self-hosting/commands/#serveruninstall) reads
only `server.env` and the defaults, so nothing in its environment
changes what it removes.

### The server.env file

`/etc/gophenberg/server.env` holds the settings `server:install` acts
on. Root owns it, with mode 0600, and the site never reads it. A dry
run prints the text the file gets. For
`-domain example.com -admin-email admin@example.com` on a fresh
server, that is:

```text
# Written by gophenberg server:install.
GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
GOPHENBERG_PUBLIC_URL=https://example.com
GOPHENBERG_ADDR=127.0.0.1:8081
GOPHENBERG_INSTALL_WEB_SERVER=caddy
GOPHENBERG_INSTALL_CERTIFICATE_EMAIL=admin@example.com
GOPHENBERG_SERVICE_RESTART_DELAY=5s
GOPHENBERG_SERVICE_START_LIMIT_INTERVAL=300s
GOPHENBERG_SERVICE_START_LIMIT_BURST=10
GOPHENBERG_SERVICE_STOP_MARGIN=5s
GOPHENBERG_MAINTENANCE_RETRY_AFTER=600s
GOPHENBERG_PROXY_RETRY_DURATION=10s
```

When the file is there, a run reads it, and the text it prints for
the file keeps what you wrote:

- A line you changed keeps your value, and a line missing from the
  file gets its default. A `GOPHENBERG_INSTALL_WEB_SERVER` line changed
  to `none` stops the run while Gophenberg's Caddy site is still there.
- A flag or an environment value given on that run replaces its line,
  except `GOPHENBERG_INSTALL_WEB_SERVER`, where asking for the other
  web server stops the run. So does asking for another
  `GOPHENBERG_INSTALL_ROOT`.
- A line you added, such as `GOPHENBERG_INSTALL_DISK_FLOOR_MB=2048`,
  is read and kept, after the eleven lines above in the order of the
  names. A comment is not kept.
- The last seven settings of the table, from
  `GOPHENBERG_INSTALL_MEMORY_FLOOR_MB` to
  `GOPHENBERG_UPDATE_VERIFY_TIMEOUT`, are not among the eleven lines.
  Given only in the environment, one is used for that run but not
  written into the file.

### The site's .env on a server install

A dry run also prints the `.env` the site gets under the install
root, owned by the `gophenberg` user with mode 0600. On a fresh server
it is:

```text
# Written by gophenberg server:install.
GOPHENBERG_DATABASE_URL=postgres:///gophenberg?host=/var/run/postgresql&user=gophenberg
GOPHENBERG_SUPERUSER_POLICY=refuse
GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
GOPHENBERG_ADDR=127.0.0.1:8081
GOPHENBERG_PUBLIC_URL=https://example.com
GOPHENBERG_TRUSTED_PROXIES=127.0.0.1/32,::1/128
GOPHENBERG_THEMES_DIR=/opt/gophenberg/themes
GOPHENBERG_MEDIA_DIR=/opt/gophenberg/media
GOPHENBERG_DATABASE_WAIT=60s
```

- The database address holds no password. The site runs as the
  system user `gophenberg` and signs in to PostgreSQL over its local
  socket as the role of the same name.
- The first eight lines come from the server settings and from values
  the install fixes, whatever the site's `.env` held before. Change
  `GOPHENBERG_INSTALL_ROOT`, `GOPHENBERG_ADDR` and
  `GOPHENBERG_PUBLIC_URL` in `server.env`, not here.
- `GOPHENBERG_DATABASE_WAIT` keeps the value the site's `.env` holds,
  and `60s` while it holds none. Every other line is kept, after
  these, in the order of the names. A comment is not kept.
- A dry run prints each kept line as its name and `(kept as it is)`.
  The file keeps the value.

`server:install` takes none of its own settings from this file, since
the site's own user can write it, so a `GOPHENBERG_INSTALL_ROOT` line
planted there is never read. It reads the file only to keep its
lines, for the five stop graces, which go into `TimeoutStopSec` of the
site's unit, and, with `GOPHENBERG_INSTALL_WEB_SERVER=none` alone, for
`GOPHENBERG_MEDIA_UPLOAD_CAP_MB`, which sets the body limit of the
lines it prints for your own web server. A value there that `serve`
could not read stops the run, naming the file:

```text
gophenberg: /opt/gophenberg/.env: GOPHENBERG_SHUTDOWN_GRACE: must be a duration like 30s, got "soon"
```

A dry run still prints the name of every line to your terminal, so
the run also stops on a file that could reach the terminal or hold
the run instead:

- a line whose name or value holds a control character other than a
  tab, such as the escape character, or a byte that is not UTF-8
- a `.env` that is not a regular file, such as a folder or a pipe
- a `.env` with a second hard link, which may be another file

```text
gophenberg: installer: /opt/gophenberg/.env: "GOPHENBERG_SITE_TITLE" holds a control character or a byte that is not UTF-8, which no setting needs
```

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
- `GOPHENBERG_SOCKET` is set without `GOPHENBERG_CLIENT_ADDR_HEADER`, or
  with `GOPHENBERG_THEMES_DIR` or `GOPHENBERG_INSTALL_ROOT`, see
  [listening on a small host](#listening-on-a-small-host).
- `PORT` is not a whole number from 1 to 65535.
- `GOPHENBERG_IDLE_EXIT` is not a duration, such as `15m`.
- `GOPHENBERG_DATA_DIR` is not an absolute path, stands inside
  `GOPHENBERG_INSTALL_ROOT`, or names a folder the server cannot make.
- `GOPHENBERG_PUBLIC_URL` is not an `http` or `https` address naming
  only a host.
- `GOPHENBERG_WEB_DIR` names a folder that holds no `index.html`, or
  one whose `index.html` the server cannot read, such as a page only
  another user may read. The message names the reason and tells you
  to unset the setting to serve the admin built into the binary.
- `GOPHENBERG_MEDIA_UPLOAD_CAP_MB` is not a positive whole number, or
  names more megabytes than the server can count in bytes.
- `GOPHENBERG_MEDIA_CONCURRENT_IMAGES` is not a whole number from 1 to 64.
- `GOPHENBERG_MEDIA_MAX_MEGAPIXELS` is not a positive whole number, or
  names more megapixels than the server can count in pixels.
- `GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB` is not a whole number from 1 to
  1024, which is as much of a request body as the server reads.
- `GOPHENBERG_THEME_START_ATTEMPTS` is not a whole number from 1 to 1000.
- `GOPHENBERG_PLUGIN_HOOK_TIMED_FIRINGS` is not a whole number from 1 to
  1000.
- `GOPHENBERG_FIELD_DEPTH` is not a whole number from 1 to 1000.
- `GOPHENBERG_MAX_PROCS` is not a whole number from 1 to the number of
  CPUs.
- `GOPHENBERG_MEMORY_LIMIT_MB` is not a whole number from 1 to
  1073741824. Each of these refuses rather than quietly using another
  value.
- Any of the cache windows is not a positive whole number of seconds.
  Write them as durations, `1h`, `90s`, `5m`. Part of a second is
  refused, because the header counts in whole seconds.
- Any of the theme timings, HTTP timeouts, shutdown graces, cron job
  timings, update timings, the upload timeout,
  `GOPHENBERG_PLUGIN_START_BUDGET`, `GOPHENBERG_PLUGIN_HOOK_BUDGET` or
  `GOPHENBERG_DATABASE_WAIT_INTERVAL` is not a positive duration.
  Write them the way Go does, `30s`, `500ms`, `1m`.
- `GOPHENBERG_DATABASE_WAIT` is not a duration of `0` or more.
- `GOPHENBERG_UPDATE_URL` is not an `https` address, or an `http` one
  on a loopback host, or it names a user, a query or a fragment.
- `GOPHENBERG_UPDATE_CHANNEL` holds anything but lowercase letters,
  digits and hyphens, or does not start with a letter.
- `GOPHENBERG_TRUSTED_KEYS` names a key that is not a P-256 Multikey,
  or is set while `GOPHENBERG_UPDATE_URL` is the official update site.
  Startup never fetches the update site, so a copy whose key list
  names no timestamp key, or whose timestamp file expired, fails at
  its first check instead.
- `GOPHENBERG_UPDATE_CHECKS` is not `true` or `false`.
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
- `GOPHENBERG_TOAST_DURATION` or `GOPHENBERG_TOAST_ACTION_DURATION`
  is not a whole number of milliseconds from `1ms` to `2147483647ms`,
  the longest a browser timer holds.
- `GOPHENBERG_UPDATES_SCREEN_PROGRESS_INTERVAL` or
  `GOPHENBERG_UPDATES_SCREEN_RESTART_WAIT` is not a whole number of
  milliseconds from `1ms` to `2147483647ms`, for the same reason.
- `GOPHENBERG_FORMAT_LOCALE` is not a language tag such as `es-ES` or
  `en-GB`.
- `GOPHENBERG_AVATARS`, `GOPHENBERG_AVATAR_DEFAULT` or
  `GOPHENBERG_AVATAR_RATING` names a value outside its list. The
  message names the setting and the value.
- `GOPHENBERG_DEFAULT_ROLE` names a role neither the core nor a plugin
  declares. The message names the setting and the value. When the
  start paused a plugin, the message ends with the plugins it paused,
  since their roles are left out, see
  [a plugin that fails at start](#a-plugin-that-fails-at-start).
- `GOPHENBERG_THEME` pins a theme that fails to load, see
  [installing a theme](/themes/installing-a-theme/). A theme chosen
  in the admin does not stop startup.
- `GOPHENBERG_SUPERUSER_POLICY` is not `warn` or `refuse`.
- The database runs PostgreSQL 14 or older, see
  [the PostgreSQL server](#the-postgresql-server).
- `GOPHENBERG_SUPERUSER_POLICY` is `refuse` and the server connects
  to PostgreSQL as a role that can reach the database server, such as
  a superuser.
- A newer Gophenberg migrated the database past what this version
  serves. The message names the version the database needs, see
  [going back to an older version](/self-hosting/updates-and-backups/#going-back-to-an-older-version).

`check` never connects to the database, so the last three wait for
`serve`, `migrate` and the commands that migrate.

`GOPHENBERG_MAX_PROCS` and `GOPHENBERG_MEMORY_LIMIT_MB` reach Go in
every command. A bad one stops `check` and `serve`, and every other
command runs without either of them.

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

### A plugin that fails at start

The site keeps one record for each plugin compiled into it, in its
database. A record holds the plugin's state, which is active,
paused, inactive or not installed, and the error that paused it.
The first start that sees a plugin records it as active.

At every start, `serve` takes the active and paused plugins one by
one. It pauses a plugin at the first of these steps that returns an
error or panics, and the rest of the site starts and serves:

- `register`, when its `Register` runs, or when its `ID()` returns
  anything but the `id` in its manifest
- `roles`, when it declares its roles
- `types`, when it declares its content types
- `hooks`, when the hooks its manifest lists under `hooks` do not
  match its methods, see
  [a plugin whose hook fails](#a-plugin-whose-hook-fails)
- `routes`, when its `Routes` or `PublicPaths` runs, or when `Routes`
  returns no handler
- `migrate`, when its schema is applied
- `start`, when its `Start` runs

While a plugin is paused:

- Its routes are not mounted, so its addresses under
  `/api/plugins/<id>/` answer the site's 404.
- Its roles are left out of the site.
- The content types and field groups it declared at an earlier
  start stay, since the site stores them with its content. They are
  still the plugin's own, so the **Field Groups** screen offers none
  of them to adopt.

`serve` logs each pause as a warning that names the plugin, the step
and the error. With `GOPHENBERG_FEED_ITEMS=banana`, the line reads:

```text
time=2026-11-08T09:30:12.345Z level=WARN msg="plugin paused" plugin=feed step=register error="pluginkit: feed register: GOPHENBERG_FEED_ITEMS: must be a whole number, got \"banana\""
```

Every start tries a paused plugin again. Once it starts, it is
active again and its error is cleared. So fix the setting and
restart the server, and the plugin comes back with no other step.

An inactive plugin is one turned off on purpose. It is never
registered or tried. No screen or command turns a plugin off or on
yet.

A start cut short by a shutdown, such as a `SIGTERM` while the
plugins start, pauses nothing.

The check of the first start after an update asks for the core's
pages only, so a plugin that fails at that start is paused and the
update is kept, see
[the boot guard](/self-hosting/updates-and-backups/#the-boot-guard).

A paused plugin's roles count as undeclared, so a
`GOPHENBERG_DEFAULT_ROLE` that names one of them still stops the
start. The message then ends with the plugins this start paused:

```text
gophenberg: GOPHENBERG_DEFAULT_ROLE: must name a role the core or a plugin declares, got "archivist", and this start paused notes
```

`serve` also times each plugin's `Start`. A plugin whose `Start`
takes longer than `GOPHENBERG_PLUGIN_START_BUDGET`, 10 milliseconds
unless set, gets a warning and still starts. Nothing is paused for
being slow:

```text
time=2026-11-08T09:30:12.401Z level=WARN msg="plugin started slower than its budget" plugin=feed took=52.318ms budget=10ms
```

Every other command registers every compiled plugin and never reads
or writes these records. So a plugin that fails to register shows
under `Not loaded:` in `gophenberg list`, and `check`, `migrate`,
`seed -yes` and every account command except `account:list` and
`account:records` fail until it is fixed. Run `check` before a
rollout to catch a bad plugin setting before `serve` pauses the
plugin.

### A plugin whose route panics

When a plugin's route panics while it answers a request, the
visitor gets the site's internal error, status 500 with this body,
which no cache keeps:

```json
{"error":"internal error","code":"internal"}
```

The answer never names the plugin, the way WordPress shows its
critical error page. If the route had already sent part of its
answer, the connection is cut instead, so a half page never looks
complete. Either way `serve` logs the panic as an error that names
the plugin, the method, the path, whether the visitor got the
internal error, the panic and its stack:

```text
time=2026-11-08T09:31:04.112Z level=ERROR msg="plugin route panicked" plugin=feed method=GET path=/api/plugins/feed/rss.xml answered=true error="pluginkit: feed route panicked: runtime error: index out of range [3] with length 3" stack="goroutine 87 [running]:\n..."
```

The plugin stays active, and its other requests and the other
plugins carry on. Nothing is paused for a panic in a route. A panic
in a goroutine the route starts, or a fatal error such as two
goroutines writing one map at once, still stops the server.

### A plugin whose hook fails

A hook is a moment where the site lets plugins join in, such as
`public.head-tags`, where plugins add tags to the head of every page
the built-in site draws. Each plugin lists the hooks it joins under
`hooks` in its `plugin.json`. At start, `serve` checks the list
against the plugin's methods and pauses the plugin at the step
`hooks` when a listed hook has no method, a method's hook is not
listed, a name is listed twice, or no hook of the site or of another
compiled plugin has the name. The record's error names the hook:

```text
plugin notes hooks: has a HeadTags method but does not list public.head-tags
```

A plugin that fails or panics inside a hook never breaks the page.
`serve` drops what the plugin answered, logs it under the plugin and
the hook, and the other plugins carry on. An error is a warning:

```text
time=2026-11-09T10:12:44.208Z level=WARN msg="plugin hook failed" plugin=notes hook=public.head-tags error="pluginkit: notes public.head-tags: the notes table is missing"
```

A panic is an error with its stack:

```text
time=2026-11-09T10:12:44.208Z level=ERROR msg="plugin hook panicked" plugin=notes hook=public.head-tags error="pluginkit: notes public.head-tags panicked: runtime error: invalid memory address or nil pointer dereference" stack="goroutine 87 [running]:\n..."
```

All the plugins of one call of a hook share
`GOPHENBERG_PLUGIN_HOOK_BUDGET`, 2 milliseconds unless set. A plugin
still working when that time runs out, which its context tells it, is
logged as a note rather than a warning, since a slower plugin before
it may have used the time:

```text
time=2026-11-09T10:12:44.211Z level=INFO msg="plugin hook ran out of time" plugin=notes hook=public.head-tags error="pluginkit: notes public.head-tags: context deadline exceeded"
```

The budget only tells a plugin to stop. A plugin that ignores its
context holds the page until it returns.

A head tag must be a `meta` or `link` tag whose attribute names are
lowercase letters, digits and hyphens, and whose `href` or `src` is an
`http` or `https` address or a path on the site. `serve` drops any
other tag, logs the plugin's warning naming the tag and why, and keeps
the plugin's other tags.

The plugin stays active. Nothing is paused for a failure inside a
hook.

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

The built-in renderer gives each page of a listing one address. A
visit to page 1, to a page number with leading zeros or to a
listing address that ends in a slash gets a `301` redirect to the
plain address, with the query kept. So `/page/1` goes to `/`,
`/page/02` goes to `/page/2` and `/pages/` goes to `/pages`. Only a
`GET` or `HEAD` is redirected, and a listing page stays where it
was asked when other content is stored at its plain address.

Each post and page has one address too. A visit with a trailing
slash, with repeated slashes or with a page number gets the same
`301` to the plain address, so `/hello-world/`, `//hello-world` and
`/hello-world/page/1` all go to `/hello-world`. Page 0 answers
`404`, and content stored at the address asked for stays there, so a
page stored at `/pages/about/page/1` answers at that address. While
a theme serves the site, the theme answers these addresses itself.
