---
title: Commands
description: Every command the binary runs, how to read its help, and who may change an account.
---

The `gophenberg` binary serves the site, and it also runs the jobs an
operator does beside it, such as creating the first account or
applying the schema. Every command reads the same environment
variables and `.env` file as the server, listed in
[configuration](/self-hosting/configuration/). The two exceptions are
[`server:install`](#serverinstall) and
[`server:uninstall`](#serveruninstall). They run as root and take none
of their settings from a `.env` file, see
[the server settings](/self-hosting/configuration/#the-server-settings).

With Docker Compose, put the command after the service name:

```sh
docker compose run --rm -T gophenberg list
```

## The listing

Run `gophenberg` with no command, or `gophenberg list`, and it prints
every command it knows, then exits:

```text
Gophenberg Version %VERSION%

Usage:
  gophenberg <command> [flags] [arguments]

Every command answers -h. A command that offers -json answers one JSON document. A command that offers -yes is a dry run until -yes.

Available commands:
  about                 print the version, the build stamp, the commit and the theme kits of this binary
  check                 check every setting, every plugin and every command name
  help                  print the help of one command
  list                  list every command
  migrate               apply every schema step
  seed                  store the demo data
  serve                 run the server
  version               print the version
 account
  account:create-admin  create an account under a role
  account:disable       disable one account
  account:enable        enable one disabled account
  account:grant-role    give a role to every account holding none
  account:list          list every account with its role
  account:records       list who applied which change, the newest first
  account:role          set one account's role
 server
  server:install        install Gophenberg on this Linux server with PostgreSQL, Caddy and a systemd service
  server:uninstall      remove what server:install set up on this server, keeping the site's data unless -delete-data
 update
  update:apply          update this server install to a newer Gophenberg, then ask for a restart
  update:check          check the update site for a newer Gophenberg and keep what it found
  update:rollback       take back the last update, pointing current at the build before it
  update:status         show the builds of this server install, its last update and its backups

Every command is described at https://docs.gophenberg.org/self-hosting/commands/
```

## Reading the listing

- A name without a colon is one of the site's own commands.
- A name with a colon sits under a heading, the part before the
  colon, such as `account`. You always type the full name, such as
  `account:list`.
- A heading with the word `plugin` beside it holds the commands one
  plugin offers. They are named after the plugin's id.
- A `Not loaded:` part at the end names each plugin that failed to
  register, and why, such as a bad `GOPHENBERG_FEED_ITEMS`. The
  listing still exits with code 0, but `serve`, `check`, `migrate`,
  `seed -yes` and every account command except `account:list` and
  `account:records` fail until the setting is fixed.

Every command answers `-h`. `gophenberg account:role -h` and
`gophenberg help account:role` print the same page, and neither needs
a database:

```text
set one account's role

Usage:
  gophenberg account:role [flags] <email> <role>

Flags:
  -as email
        email address of the account acting
  -yes
        apply the change, a dry run without it
```

A word in angle brackets is an argument the command needs, in that
order. `[flags]` means the command takes the flags listed below it.
The word after a flag, such as `email` in `-as email`, says what value
that flag wants. Flags can go before or after the arguments.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | The command finished, printed a help page, or ended a dry run |
| `1` | The command ran and failed |
| `2` | The command line was misused, such as an unknown command, a missing argument or a missing flag |
| `75` | `update:apply` or `update:rollback` pointed `current` at another build, or `serve` stopped once an update started from the admin did, so restart Gophenberg to start it |
| `76` | `serve` started a new build that failed its start check after an update, see [the boot guard](/self-hosting/updates-and-backups/#the-boot-guard) |

Every error goes to standard error, on a line that starts with
`gophenberg:`. When the command itself was found, a misuse is followed
by its help page.

## On a SQLite database

When `GOPHENBERG_DATABASE_URL` names a SQLite file, such as
`sqlite:/srv/gophenberg/site.db`, only some commands run so far:

| Command | On a SQLite database |
| --- | --- |
| `list`, `help` and `version` | Work as on PostgreSQL |
| `check` | Works, and also checks the SQLite settings and the file |
| `migrate` | Works |
| `seed` | Without `-yes` it still prints `would store the demo data` and exits with code 0. With `-yes` it migrates the file, then exits with code 1 |
| `serve` | Exits with code 1 before it opens anything |
| `account:create-admin` | Works as on PostgreSQL. It migrates the file first, then asks for the password |
| Every other `account:` command | Works as on PostgreSQL, on the accounts and the records kept in the file |
| A plugin command that names a capability | Checks the account in `-as`, then runs, as on PostgreSQL |
| `update:apply` | Exits with code 1 before it opens anything |
| `update:rollback` and `update:status` | Work as on PostgreSQL, since they never open the database |

`serve`, `seed -yes` and `update:apply` stop and name what they need:

```text
gophenberg: serve: content and types do not run on SQLite yet, so this command needs GOPHENBERG_DATABASE_URL to name a PostgreSQL database
```

`seed -yes` says `seed:` in place of `serve:`, and `update:apply`
says `update:apply:`.

`seed -yes` migrates the file before it stops, `account:create-admin`
migrates it first, and every other account command and every plugin
command opens it. So a missing file, a bad address or a bad SQLite
setting stops each of them with its own error, as it stops `migrate`.
On a file that was never migrated, run `migrate` first. Until then
`account:records` and every command that takes `-as` stop with
`the command records are missing, run migrate first`, and
`account:list` stops with SQLite's own error.
[A SQLite database](/self-hosting/configuration/#a-sqlite-database)
covers the address, the file and its settings.

## Dry runs and -yes

Six commands only show what they would change until you add `-yes`:
`seed`, `account:grant-role`, `account:role`, `account:disable`,
`account:enable` and `server:uninstall`. This line, for example:

```sh
gophenberg account:role author@example.com editor -as admin@example.com
```

changes nothing and prints:

```text
would set author@example.com to editor
gophenberg: dry run, nothing changed, pass -yes to apply
```

A dry run exits with code 0. Add `-yes` to the same line to apply the
change, and it prints `set author@example.com to editor` instead.

`migrate` and `account:create-admin` have no dry run. They act at
once.

`server:install` takes no `-yes`. Its dry run is the `-dry-run` flag,
see [server:install](#serverinstall).

## Who may change an account

`account:grant-role`, `account:role`, `account:disable` and
`account:enable` need `-as` with the email address of the person
running the command. That account must exist, be enabled and
activated, and hold a role carrying `manage_users`. `admin` carries
it, and so does any role a plugin gives it. The check runs before
anything changes, on a dry run too. When it fails, the command exits
with code 1 and says why:

```text
gophenberg: the account editor@example.com holds the role editor, which lacks manage_users
```

Leaving out `-as`, or giving it only spaces, exits with code 2. The
message names the command, so for `account:role` it reads:

```text
gophenberg: account:role wants -as <email>
```

`account:role` and `account:disable` refuse to act on the account
named in `-as`, on a dry run too, even when another admin is left.
The command exits with code 1 and says why:

```text
gophenberg: the account admin@example.com cannot disable itself
```

`account:role` says
`the account admin@example.com cannot change its own role` instead.
Name another admin in `-as` to make that change.

An account may hand out only a role whose capabilities its own role
carries, and may change only an account whose role it covers that
way. So a role a plugin gives only `manage_users` can make members,
but never an author or an admin. Going past that exits with code 1:

```text
gophenberg: the role admin carries access_admin, which the account lead@example.com lacks
```

A role is one of the core's, `admin`, `editor`, `author` and
`member`, or one a plugin declares. A `member` signs in but never
enters the admin. Every account command except `account:list` and
`account:records` registers the plugins before it runs, so
`account:role` and `-role` take a plugin's roles too.
Naming a role nobody declares exits with code 2 and lists the roles
there are.

The last enabled account whose role carries `manage_users` always
stays. If two changes run at the same time and would together leave
no such account, one of them fails with
`<address> is the last enabled privileged account`.

`-as` asks for no password. It names who answers for the change.
Anyone who can run commands against the database can name any
account, so keep access to the server as tight as before.

The check needs the table the records are kept in. On a database
updated from an earlier release, start the new server or run
`migrate` first, or the command stops with
`the command records are missing, run migrate first`.

## The records

Every change those four commands apply with `-yes` is kept as one
record, in the `gonsole.records` table on PostgreSQL or in the
`gonsole_records` table of a SQLite file, so a backup of the database
carries it. A record names the time, the account in
`-as`, the command, its arguments and its other flags. A dry run
records nothing, and neither do `account:create-admin` and `seed`.

`account:records` lists them, the newest first, with times in UTC:

```text
2026-10-01T09:30:12Z  admin@example.com  account:role        author@example.com editor
2026-09-30T16:02:45Z  admin@example.com  account:grant-role  -role admin
```

It lists the latest 50, unless `-limit` or
`GOPHENBERG_COMMAND_RECORDS_LIMIT` names another number. Storing one
record may take up to `GOPHENBERG_COMMAND_RECORD_TIMEOUT`, 5 seconds
unless set. When a record cannot be stored, the change stays applied,
and the command exits with code 1 and says so.

## Renamed commands

Two commands took new names. The old names are gone, with no alias:

| Old name | New name |
| --- | --- |
| `createadmin` | `account:create-admin` |
| `grantrole` | `account:grant-role` |

An old name exits with code 2:

```text
gophenberg: unknown command "createadmin", run "gophenberg list" to see every command
```

Change any script that runs them. `account:grant-role` also wants
`-as`, and `-yes` to apply.

## Every command

### about

```text
gophenberg about [flags]
```

Prints what this binary is in one line, such as
`Gophenberg %VERSION%, official build, commit 4c3ba60, theme kits 0.9.0 to %KIT_VERSION%`.
With `-json` it answers one JSON document holding `schema`, `name`,
`version`, `commit`, `stamp`, `minimum_version`, `postgresql`, `node`
and `kits`. It needs no database.

- A binary from a release archive or the release image says
  `official build`, and names the commit it was built from.
- A binary you build yourself says `custom build` and names no commit.
- `kits` lists the [theme kit](/themes/how-theming-works/) versions
  this binary serves.
- `minimum_version` is the oldest version allowed to run on a
  database this binary migrated, see
  [going back to an older version](/self-hosting/updates-and-backups/#going-back-to-an-older-version).
- `postgresql` is the oldest PostgreSQL major this binary runs on.
- `node` is the oldest Node.js major this binary runs a theme on. The
  release image ships this major. Outside the image, point
  [`GOPHENBERG_NODE_BIN`](/self-hosting/configuration/) at a Node.js of
  this major or newer.

### check

```text
gophenberg check
```

Reads every setting, registers the plugins and checks the command
names, without touching the database. It prints
`settings, plugins and command names are valid`, or names what it
refuses, one line each, and exits with code 1. Run it before a
rollout. On a SQLite database it also checks the SQLite settings and
that the file exists on a local disk, still without connecting to it.
It does not open the file, so a folder, a file that is not a SQLite
database or a read-only file still passes `check` and stops at
`migrate`. A command that stops on a read-only file leaves the
`-wal` and `-shm` files beside it read-only too. When you make the
database file writable, make those two files writable as well.

### help

```text
gophenberg help
```

`gophenberg help <command>` prints the page of one command, the same
page `-h` prints. Alone, it prints the listing. It needs no database.

### list

```text
gophenberg list
```

Prints [the listing](#the-listing), the same as a run with no
command.

### migrate

```text
gophenberg migrate
```

Applies every schema step in order, `accounts`, `records` and `core`,
then every plugin's schema, and prints `migrated accounts`,
`migrated records`, `migrated core` and `migrated plugins`.
`serve` applies the same steps when it starts. Every step waits for a
lock in the database, so two servers starting together, or a server
and `migrate`, never apply the same step at once. The second waits,
then finds nothing left to do.

On a SQLite database the lock is the file `<database file>.migrate.lock`,
such as `site.db.migrate.lock`. It sits beside the file the path
finally points at, so when the path is a symbolic link, the lock sits
beside the file the link points at. A step waits for it up to
`GOPHENBERG_SQLITE_MIGRATE_LOCK_WAIT`, 30 seconds unless set, then
stops with an error. Never delete that file.

### seed

```text
gophenberg seed [flags]
```

Stores demo data to try Gophenberg on. Without `-yes` it only prints
`would store the demo data`. With `-yes` it applies every schema
step, then stores the demo content, the accounts
`admin@example.com`, `editor@example.com`, `author@example.com` and
`member@example.com`, and each plugin's demo data. It prints
`created` or `kept` beside each account, and the accounts it created
sign in with `password1234`.
A later run stops with an error naming the group when the demo's
Post fields or Category fields group is turned off or no longer
reaches its type. Never seed a production database.

On a SQLite database the dry run still prints
`would store the demo data` and exits with code 0, although `-yes`
stops with code 1 after it migrates the file, see
[on a SQLite database](#on-a-sqlite-database).

### serve

```text
gophenberg serve
```

Applies every schema step, then serves the site until it is stopped,
see [stopping the server](/self-hosting/configuration/#stopping-the-server).
The container image runs `serve` when it is given no command. A
compose file or deployment that sets its own `command` must start it
with `serve`, since a run with no command only prints the listing.

On a SQLite database `serve` exits with code 1 before it opens
anything, see [on a SQLite database](#on-a-sqlite-database).

When the PostgreSQL server is not up yet, `serve` first waits for it,
up to `GOPHENBERG_DATABASE_WAIT`, see
[a database that starts late](/self-hosting/configuration/#a-database-that-starts-late).

Once an update an administrator started from the admin switched
builds, `serve` stops as it does on `SIGTERM` and exits with code 75,
so the service manager starts the new build, see
[updating from the admin](/self-hosting/updates-and-backups/#updating-from-the-admin).

### version

```text
gophenberg version [flags]
```

Prints `gophenberg %VERSION%`. With `-json` it answers one JSON
document holding `name` and `version`.

### account:create-admin

```text
gophenberg account:create-admin [flags]
```

Creates one account with the address in `-email`, the display name in
`-name`, and the role in `-role`, which is `admin`, `editor`,
`author`, `member` or a role a plugin declares. It reads the
password from standard input, keeping it out of your shell history,
and applies the `accounts`, `records` and `core` schema steps first.
It needs no `-as`, since it is how a site gets its first admin:

```sh
docker compose run --rm -T gophenberg \
  account:create-admin -email admin@example.com -name "Maria Perez" -role admin
```

On a SQLite database it works the same way and creates the account in
the file, but `serve` still cannot serve a site from that file, see
[on a SQLite database](#on-a-sqlite-database).

### account:disable

```text
gophenberg account:disable [flags] <email>
```

Disables one account, so it can no longer sign in. It refuses to
disable the account named in `-as`, and the last enabled account
holding `manage_users`.

### account:enable

```text
gophenberg account:enable [flags] <email>
```

Enables one disabled account, so it can sign in again.

### account:grant-role

```text
gophenberg account:grant-role [flags]
```

Gives the role in `-role` to every account that holds none, such as
the accounts made before roles existed. On a site where no account
holds a role yet, create an admin with `account:create-admin` first
and name it in `-as`:

```sh
docker compose run --rm -T gophenberg \
  account:grant-role -role admin -as admin@example.com -yes
```

[Users and signing in](/guides/users/#upgrading-a-site-that-ran-an-earlier-version)
covers the whole upgrade.

### account:list

```text
gophenberg account:list [flags]
```

Lists every account, ordered by display name, with its role, or a
dash when it holds none, and whether it is enabled. With `-json` it
answers one document holding each account's `id`, `email`, `name`,
`role` and `disabled`. On a SQLite database a letter with an accent
sorts after z, so Álvaro lists after Zoe. PostgreSQL follows the
database's own collation, so a database the official PostgreSQL image
made, under `en_US.utf8`, lists Álvaro beside alba.

### account:records

```text
gophenberg account:records [flags]
```

Lists the account changes applied with `-yes`, the newest first, see
[the records](#the-records). `-limit` sets how many, and `-json`
answers one document instead.

### account:role

```text
gophenberg account:role [flags] <email> <role>
```

Sets the role of one account to `admin`, `editor`, `author`,
`member` or a role a plugin declares. It refuses to change the role
of the account named in `-as`, to hand out a role beyond the reach
of that account, and to take `manage_users` from the last enabled
account holding it.

### server:install

```text
gophenberg server:install [flags]
```

Installs Gophenberg on a fresh Linux server. It sets up PostgreSQL,
Caddy in front for HTTPS unless you keep a web server of your own,
and a systemd service that runs the site. With `-dry-run` it only
checks the server and prints what it would do.

Most people run it through `install.sh`, which downloads and checks
the archive and then runs this command, see
[install on a Linux server](/self-hosting/server-install/). By hand,
run it on the server itself as root, with the `gophenberg` binary of
a Linux release archive, see
[check a download](/self-hosting/install/#check-a-download). It is not
a command for the container image. The archive goes in
`/var/lib/gophenberg-installer`, a folder only root can open:

```sh
sudo mkdir -m 0700 /var/lib/gophenberg-installer
sudo cp gophenberg_%VERSION%_linux_amd64.tar.gz /var/lib/gophenberg-installer/
tar -xzf gophenberg_%VERSION%_linux_amd64.tar.gz gophenberg
sudo ./gophenberg server:install -dry-run -domain example.com -admin-email admin@example.com
sudo ./gophenberg server:install -domain example.com -admin-email admin@example.com \
  -archive /var/lib/gophenberg-installer/gophenberg_%VERSION%_linux_amd64.tar.gz
```

The last line installs. It asks for the first administrator's name
and password, see [the first administrator](#the-first-administrator).

- `-domain host` is the host name the site answers at, such as
  `example.com`, or `localhost` to try it on the server itself. A
  first run needs it, unless the environment or
  `/etc/gophenberg/server.env` already names `GOPHENBERG_PUBLIC_URL`.
  It must be `localhost` or a host name of two labels or more, each of
  1 to 63 lower-case letters, digits and hyphens with no hyphen at
  either end, a letter in the last label, and 253 characters at most.
  So an address such as `203.0.113.7` is not a domain. A domain that
  breaks the rule stops the run before anything else is read.
- `-admin-email email` is the email of the first administrator. Caddy
  also gives it to Let's Encrypt for the HTTPS certificate, unless
  `GOPHENBERG_INSTALL_CERTIFICATE_EMAIL` names another.
- `-admin-name name` is the first administrator's display name.
- `-admin-password-file file` is a file whose first line is the first
  administrator's password. It must be an absolute path, or the run
  stops before anything else is read.
- `-archive file` is the checked release archive, inside
  `/var/lib/gophenberg-installer`. A run needs it while `current`
  names no build, so a first run always does. A dry run does not open
  it, but an archive outside that folder stops the run before anything
  else is read, as a bad domain does.
- `-web-server name` picks the web server in front of the site:
  `caddy`, the default, has the install set Caddy up, and `none` keeps
  a web server of your own, see
  [with your own web server](#with-your-own-web-server). The first run
  keeps it in `/etc/gophenberg/server.env` as
  `GOPHENBERG_INSTALL_WEB_SERVER`, and a later run without the flag
  keeps it. A later run that asks for the other one stops. So does a
  run whose line in `server.env` was changed to `none` by hand while
  Gophenberg's Caddy site, `/etc/caddy/gophenberg.caddy`, is still
  there:

  ```text
  gophenberg: GOPHENBERG_INSTALL_WEB_SERVER: this server was set up with caddy, and a later run cannot move it to none, so run sudo gophenberg server:uninstall first, then change the line in /etc/gophenberg/server.env
  ```

  [Moving to your own web server](#moving-to-your-own-web-server)
  shows the steps. Any other name stops the run before anything else
  is read, as a bad domain does.
- `-skip-domain-check` skips the check that the domain points at this
  server, for a server behind a proxy the check cannot cross.
- `-dry-run` runs every check, then prints each step and every file,
  and changes nothing. It asks no question and opens no archive.

A run goes in this order:

1. It runs the first two checks listed below, root and the installer
   folder, before it opens any file. So a run that is not root's stops
   with the root line below.
2. Without `-dry-run`, it stops at once while another `server:install`
   or `server:uninstall -yes` is running on the server.
3. It reads [the server settings](/self-hosting/configuration/#the-server-settings)
   and stops on a bad value, naming every bad value at once.
4. It reads the install record,
   `/var/lib/gophenberg-installer/install.json`, when one is there.
   It stops when the record names a newer installer than this one, so
   an older binary never writes its files over newer ones.
5. It runs the rest of the checks listed below.
6. With `-dry-run`, it prints the steps and ends.
7. It opens the archive. Its `release.json` must name the version of
   this binary and the platform of this server, such as `linux/amd64`.
8. While the first administrator is still to be made, it asks for
   what the flags leave out.
9. It takes the steps, see [taking the steps](#taking-the-steps).

Nothing on the server changes before the steps.

It never reads the `.env` file of the folder it runs in, since the
site's own user can write such a file. Every program it starts, such
as `systemctl` or `systemd-run`, is looked up only in `/usr/sbin`,
`/usr/bin`, `/sbin` and `/bin`, and gets an environment that holds
only that `PATH` and `LC_ALL=C`, so no setting of the person running
it reaches that program. Each also runs in a session of its own, with
no terminal, so none can type into or read from the terminal of the
person running it.

#### The checks

A check that fails stops the run with exit code 1 and a line that
says why and ends with `so nothing was changed`. In order:

- **Root.** It runs as root, such as with `sudo`.
- **The installer folder.** `/var/lib/gophenberg-installer`, when it
  exists, is a real folder, not a link, owned by root and writable by
  root alone.
- **The system.** `/etc/os-release` names Ubuntu 24.04, Debian 12 or
  Debian 13. On Debian it also prints that Gophenberg's CI tests
  Ubuntu 24.04 only, such as
  `Debian GNU/Linux 12 (bookworm) is not yet tested in Gophenberg's CI, which tests Ubuntu 24.04`.
- **systemd.** systemd runs the server, so `/run/systemd/system` is a
  folder, and `systemctl --version` names systemd 251 or newer.
- **The sandbox.** systemd can set up the sandbox the site's service
  runs in. It runs `/bin/true` through `systemd-run` with
  `ProtectSystem=strict`, `PrivateTmp=yes` and `NoNewPrivileges=yes`.
  A container plan without nesting fails here, and the line names what
  `systemd-detect-virt` reports, such as `lxc`.
- **Memory.** `MemTotal` in `/proc/meminfo` is at least
  `GOPHENBERG_INSTALL_MEMORY_FLOOR_MB`, 900 MB unless set, a little
  under 1 GB, since a server sold with 1 GB reports less than that.
- **The install root.** Every parent of the install root that exists,
  such as `/opt`, is a real folder, not a link, owned by root and
  writable by root alone. The install root itself, when it exists, is
  a real folder and empty. So `GOPHENBERG_INSTALL_ROOT=/etc` stops
  here. The site's own user will own the install root, so it must not
  be or hold a folder whose files root runs or reads: the folder of
  any file the install writes, such as `/etc/gophenberg`,
  `/etc/apt/keyrings` or `/usr/local/bin`, or a folder root finds
  programs in, such as `/usr/bin`. So
  `GOPHENBERG_INSTALL_ROOT=/usr/local` stops here too.
- **The current link.** A `current` link in the install root names a
  build in `versions` that holds its `gophenberg` as a file that may
  run. Otherwise the run cannot tell which build runs.
- **Disk.** At least `GOPHENBERG_INSTALL_DISK_FLOOR_MB`, 1024 MB unless
  set, is free under the install root and under `/var/lib`, each
  measured at its nearest folder that exists.
- **Ports.** Nothing listens on TCP 80, TCP 443, UDP 443, the site's
  own address in `GOPHENBERG_ADDR` or TCP 5432. Each is checked by
  listening on it for a moment. The port of `GOPHENBERG_ADDR` must
  also not be one the install hands another piece, whatever address
  it names: 5432 for PostgreSQL, and with Caddy 80 and 443. So
  `GOPHENBERG_ADDR=127.0.0.1:443` stops here, on a later run too.
- **A fresh server.** No PostgreSQL, Caddy or `gophenberg` service is
  there yet, found by `/etc/postgresql`, `/usr/lib/postgresql`,
  `/var/lib/postgresql`, `/etc/caddy`, `/usr/bin/caddy` and the units
  `gophenberg.service` and `gophenberg-rollback.service` in
  `/etc/systemd/system`. `GOPHENBERG_DATABASE_URL` must not be set
  either, since the install sets up a PostgreSQL of its own.
- **The command on the path.** Nothing is at `/usr/local/bin/gophenberg`
  but the command an earlier run wrote.
- **The Caddyfile.** Once an earlier run wrote `/etc/caddy/Caddyfile`,
  it still holds the two lines that run wrote:
  `import /etc/caddy/gophenberg.caddy` and
  `admin unix//run/caddy/admin.sock`.
- **The domain.** The domain has an A or AAAA record. An address that
  never leaves the server, a loopback address such as `127.0.1.1` or
  an unspecified one such as `0.0.0.0`, stops the run, since Let's
  Encrypt asks for the domain from outside. Such an answer most often
  comes from a line in `/etc/hosts` that names the server after its
  domain. Once the domain's public record points at the server,
  `-skip-domain-check` passes over that line. When every
  address it names is one of the server's own, the check passes.
  Otherwise the check listens on TCP port 80 for a moment, answers a
  random token at `/.well-known/gophenberg-install/<token>`, and
  fetches that page through each address that is not the server's
  own, giving each fetch `GOPHENBERG_INSTALL_DOMAIN_CHECK_TIMEOUT`, 10
  seconds unless set. An address that gives any other answer, a
  redirect included, or none in time, stops the run. The line then
  names both likely causes: the domain points at another server, or
  the network does not route a request from the server to its own
  public address back to it. On a later run for a new domain whose
  address is not one of the server's own, the Caddy an earlier run
  set up holds TCP port 80, so the check cannot listen there and
  stops with a line that says so. Once the new domain points at the
  server, `-skip-domain-check` passes over it. The check is skipped
  for `localhost` and with `-skip-domain-check`. Whenever it runs, it
  also prints that the hosting provider's firewall must let TCP 80,
  TCP 443 and UDP 443 in, since no check can see that firewall.
- **The clock.** When `timedatectl` says the clock is not
  synchronized, it prints a warning and goes on, since HTTPS and the
  signed update index both need the right time.

When the install record says an earlier run installed a piece, such
as Caddy, the checks skip that piece and its ports. The domain check
also skips the domain the record names.

With `-web-server none`, the ports check leaves out TCP 80, TCP 443
and UDP 443, the fresh server check leaves out Caddy, and the domain
check does not run, since your own web server answers on those ports
and gets its own HTTPS certificate. The site's own address, TCP 5432
and every other check stay. The domain must still follow the host
name rule, since it goes into `GOPHENBERG_PUBLIC_URL`.

Some stop lines, as they read for `-domain example.com`:

```text
gophenberg: server:install must run as root, such as with sudo, so nothing was changed
gophenberg: this server has 512 MB of memory, less than the 900 MB GOPHENBERG_INSTALL_MEMORY_FLOOR_MB asks, so nothing was changed
gophenberg: something already listens on TCP port 80, which Caddy answers on, so nothing was changed
gophenberg: this server already has PostgreSQL at /etc/postgresql, and server:install sets up a fresh server only, so nothing was changed
gophenberg: /opt, a parent of the install root /opt/gophenberg, must be owned by root and writable by root alone, so nothing was changed
gophenberg: cannot find an A or AAAA record of example.com, which must point at this server: lookup example.com: no such host, so nothing was changed
```

A line whose message starts with `installer:`, such as
`gophenberg: installer: ...`, instead names something the run could
not read, such as a file it may not open. It is not a verdict on the
server, so it does not end that way.

#### What a dry run prints

Once every check passes, a dry run prints each step to standard
output, and the notes and warnings of the checks stay on standard
error. Under each step comes every file it would write, with its
owner and mode, then its text indented by four spaces. For
`-domain example.com -admin-email admin@example.com` on a fresh
server it begins:

```text
Every check passed, and nothing was changed. Without -dry-run, server:install would take these steps.

Step 1: start the install record /var/lib/gophenberg-installer/install.json and write the server settings
  writes /etc/gophenberg/server.env, owned by root with mode 0600:
    # Written by gophenberg server:install.
    GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
    GOPHENBERG_PUBLIC_URL=https://example.com
```

The fifteen steps of that run are these:

```text
Step 1: start the install record /var/lib/gophenberg-installer/install.json and write the server settings
Step 2: install PostgreSQL and Caddy with apt-get, Caddy from its own apt repository
Step 3: make the system user gophenberg, which runs the site and signs in to PostgreSQL
Step 4: make the install root /opt/gophenberg and its folders, owned by gophenberg
Step 5: make the database role gophenberg and its database gophenberg as the postgres user
Step 6: unpack the release archive into /opt/gophenberg/versions and point /opt/gophenberg/current at it
Step 7: keep a copy of this installer at /usr/local/lib/gophenberg/gophenberg, the one root runs for server:install and server:uninstall
Step 8: write the site's settings, then run check as gophenberg in /opt/gophenberg
Step 9: write the systemd units and Caddy's drop-in, then reload systemd and enable gophenberg.service
Step 10: write the gophenberg command on the path
Step 11: write Caddy's files, keep the package's Caddyfile as /etc/caddy/Caddyfile.before-gophenberg, check them with caddy validate and restart Caddy
Step 12: start gophenberg.service and wait until /admin/, /api/locale and / answer on 127.0.0.1:8081
Step 13: create the first administrator with account:create-admin as gophenberg
Step 14: check that https://example.com/admin/ answers within 2m0s
Step 15: print the summary and end the install record
```

And these are the files it shows:

| File | Owner and mode | What it is |
| --- | --- | --- |
| `/etc/gophenberg/server.env` | root, 0600 | The server settings, see [the server.env file](/self-hosting/configuration/#the-serverenv-file) |
| `/etc/apt/keyrings/caddy.asc` | root, 0644 | The key that signs Caddy's own apt repository |
| `/etc/apt/sources.list.d/caddy.sources` | root, 0644 | Caddy's apt repository, signed by that key |
| `/etc/apt/preferences.d/gophenberg-caddy` | root, 0644 | The pin that gives the `caddy` package priority 500 and every other package from any repository on `dl.cloudsmith.io` priority -1, so a repository you add there later needs a pin of its own |
| `/etc/apt/apt.conf.d/52gophenberg-caddy` | root, 0644 | The line that lets `unattended-upgrades` install Caddy's fixes, shown only where that package is installed |
| `/opt/gophenberg/.env` | gophenberg, 0600 | The site's settings, see [the site's .env on a server install](/self-hosting/configuration/#the-sites-env-on-a-server-install) |
| `/etc/systemd/system/gophenberg.service` | root, 0644 | The unit that runs the site |
| `/etc/systemd/system/gophenberg-rollback.service` | root, 0644 | The unit that takes back an update whose new build fails to start |
| `/etc/systemd/system/caddy.service.d/gophenberg.conf` | root, 0644 | Gives Caddy the folder of its admin socket |
| `/usr/local/bin/gophenberg` | root, 0755 | The `gophenberg` command on the path |
| `/usr/local/share/gophenberg/maintenance/index.html` | root, 0644 | The page Caddy shows while the site restarts |
| `/etc/caddy/gophenberg.caddy` | root, 0644 | Gophenberg's Caddy site |
| `/etc/caddy/Caddyfile` | root, 0644 | Caddy's own settings, which import Gophenberg's site |

For `localhost`, which Caddy serves with a certificate of its own, the
Caddyfile names no email. It names none either when the run knows no
certificate email.

In the site's `.env`, each line you added prints as its name and
`(kept as it is)`, such as `GOPHENBERG_SMTP_PASSWORD=(kept as it is)`.
So a password you keep there never reaches your terminal. The file
keeps the value.

On a later run the list follows what the server already holds, see
[the install record and later runs](#the-install-record-and-later-runs).

#### The first administrator

A first run creates the first administrator, an account under the
role `admin`. It asks on the terminal for each answer the flags leave
out:

```text
Email of the first administrator: admin@example.com
Name of the first administrator [admin]: Maria Perez
Password:
Password again:
```

- An empty name takes the part of the email before `@`, shown in the
  brackets.
- The password is typed twice and never shown.
- An answer the account rules refuse, or two passwords that differ,
  prints why and is asked again, such as
  `password shorter than 12 characters, so type it again`.
- Ctrl-C at a question ends the run with nothing changed, and the
  terminal shows what you type again.

A password file whose first line ends in a carriage return, as in a
file saved with Windows line endings, stops the run. So does a flag
whose answer the account rules refuse, such as a short password:

```text
gophenberg: -admin-password-file: password shorter than 12 characters, so nothing was changed
```

The questions need a terminal. A run with none, such as one a script
starts, stops and names the flags to give:

```text
gophenberg: this run has no terminal to ask the first administrator's name and password on, so give -admin-name and -admin-password-file: open /dev/tty: no such device or address, so nothing was changed
```

#### Taking the steps

Without `-dry-run`, the run takes the steps a dry run lists, in
order. Before each one it prints its line, such as
`Step 10: write the gophenberg command on the path`. Each file it
writes is the one a dry run prints, with that owner and mode.

The step that starts the site waits for its pages, up to
`GOPHENBERG_UPDATE_VERIFY_TIMEOUT`, 1 minute unless
[the server settings](/self-hosting/configuration/#the-server-settings)
set it. A site that never answers stops the run:

```text
gophenberg: gophenberg.service did not pass its start check within 1m0s, the time GOPHENBERG_UPDATE_VERIFY_TIMEOUT allows, as at the last try it could not ask for /admin/: dial tcp 127.0.0.1:8081: connect: connection refused, so read why with sudo journalctl -u gophenberg
```

With Caddy, the HTTPS step waits up to
`GOPHENBERG_INSTALL_HTTPS_WAIT`, 2 minutes unless set, for
`https://<domain>/admin/` to answer 200. For `localhost` it trusts
the root certificate Caddy signs `localhost` with. When no answer
comes, the site still runs, so the run only warns on standard error
and ends with exit code 0. The next run checks again:

```text
warning: https://example.com/admin/ did not answer 200 within 2m0s, the time GOPHENBERG_INSTALL_HTTPS_WAIT allows, as at the last try it could not be reached: dial tcp 203.0.113.7:443: i/o timeout. The site runs, so check that the hosting provider's firewall lets TCP 80, TCP 443 and UDP 443 in, then read sudo journalctl -u caddy, and the next sudo gophenberg server:install checks again
```

The last step prints a summary. The email shows only on the run that
created the administrator:

```text
Gophenberg %VERSION% runs at https://example.com. Sign in at https://example.com/admin/ as admin@example.com.
The hosting provider's firewall must let TCP 80, TCP 443 and UDP 443 in for visitors to reach the site.
Commands to know:
  sudo gophenberg update:status     shows the build that runs and the update on offer
  sudo gophenberg update:apply      installs the update on offer
  sudo journalctl -u gophenberg     shows what the site logged
  sudo gophenberg server:uninstall  removes Gophenberg and keeps its data
```

#### The install record and later runs

The install record, `/var/lib/gophenberg-installer/install.json`, is
how a run knows what earlier runs did. Root owns it, with mode 0600.
It marks each step started before the step runs, and ended once the
step is done. It also names what the install put on the server, such
as PostgreSQL and Caddy, the domain, and the SHA-256 of the Caddyfile
the first run wrote.

So a run cut off part way, such as by a power cut, picks up where it
stopped. Run the same line again. Each step checks what is already
done:

- After a cut in the packages step, it runs `dpkg --configure -a`
  first.
- A database role or database that already exists counts as made.
- A build folder left half unpacked is removed and unpacked again.
- After a Caddy restart that failed, it keeps `/etc/caddy/Caddyfile`
  as it is, checks its two lines and restarts Caddy.
- An administrator the cut-off run made is found with
  `account:list`, so nothing is asked.

A step an earlier run ended stays ended, even when a later run stops
inside it.

The first run puts the `gophenberg` command on the path. It runs
`server:install` and `server:uninstall` with the installer's own
copy, `/usr/local/lib/gophenberg/gophenberg`. It runs every other
command as the user `gophenberg` in the install root. So a later run
is:

```sh
sudo gophenberg server:install
```

It needs no flag, since `server.env` keeps the settings. It asks
nothing and creates no administrator. It keeps the build `current`
names, since updates go through [`update:apply`](#updateapply). It
leaves the Caddyfile alone and reloads Caddy. It writes every other
file again from the settings, then restarts the site. So to change a
setting such as `GOPHENBERG_SERVICE_RESTART_DELAY`, change its line in
`server.env` and run it again.

#### With your own web server

Gophenberg needs no given web server. It listens on its own loopback
address, `GOPHENBERG_ADDR`, and any web server on the same server can
sit in front of it. On a server that already runs nginx, Caddy,
Traefik or Apache for other sites, run the install with
`-web-server none`:

```sh
sudo ./gophenberg server:install -dry-run -domain example.com -admin-email admin@example.com -web-server none
```

The steps then leave out the Caddy apt files, Caddy's drop-in, the
Caddy files, the maintenance page, Caddy's restart and the HTTPS
check. The systemd units, the site's `.env` and the command on the
path stay as they are. So the site still listens on loopback alone,
and your web server must run on the same server. A web server in a
container needs the server's own network, such as `network_mode: host`
in Docker Compose, since a container's loopback address is its own.
Its fourteen steps are these:

```text
Step 1: start the install record /var/lib/gophenberg-installer/install.json and write the server settings
Step 2: install PostgreSQL with apt-get
Step 3: make the system user gophenberg, which runs the site and signs in to PostgreSQL
Step 4: make the install root /opt/gophenberg and its folders, owned by gophenberg
Step 5: make the database role gophenberg and its database gophenberg as the postgres user
Step 6: unpack the release archive into /opt/gophenberg/versions and point /opt/gophenberg/current at it
Step 7: keep a copy of this installer at /usr/local/lib/gophenberg/gophenberg, the one root runs for server:install and server:uninstall
Step 8: write the site's settings, then run check as gophenberg in /opt/gophenberg
Step 9: write the systemd units, then reload systemd and enable gophenberg.service
Step 10: write the gophenberg command on the path
Step 11: start gophenberg.service and wait until /admin/, /api/locale and / answer on 127.0.0.1:8081
Step 12: create the first administrator with account:create-admin as gophenberg
Step 13: print the summary and end the install record
Step 14: print the lines for nginx, Caddy, Traefik and Apache that send example.com to the site at 127.0.0.1:8081
```

The summary names the address the site answers on, since your web
server must send the domain there first:

```text
Gophenberg %VERSION% answers on 127.0.0.1:8081, the GOPHENBERG_ADDR the site listens on.
Once your own web server sends example.com there, sign in at https://example.com/admin/ as admin@example.com.
```

The last step prints, under a `prints:` line, the lines each of the
four web servers needs to send the domain to the site. The dry run
prints the same text the install will. Each block passes on the
`Host` the visitor sent, which the site builds some links from, such
as those of the RSS feed. Each also sends the three headers the site
trusts from `GOPHENBERG_TRUSTED_PROXIES`: `X-Forwarded-Host`,
`X-Forwarded-For` and `X-Forwarded-Proto`. Caddy and Traefik do both
on their own.

The nginx and Apache blocks also raise the limit on a request body to
the largest upload the site takes, since nginx takes no more than 1 MB
by default. That is the larger of `GOPHENBERG_MEDIA_UPLOAD_CAP_MB`,
which the run reads from the site's `.env`, 128 MB unless set, and the
64 MB a theme may hold, with 64 KB more for the form around the file.
nginx gets it in kilobytes and Apache in bytes. For
`-domain example.com` with the defaults, the nginx block is:

```nginx
client_max_body_size 131136k;

location / {
    proxy_pass http://127.0.0.1:8081;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

Traefik sets no size limit, but Traefik 3 cuts a request it has not
read in full within the entry point's
`transport.respondingTimeouts.readTimeout`, 60 seconds by default. So
the text asks you to raise it in Traefik's install configuration to
at least `GOPHENBERG_UPLOAD_TIMEOUT`, 5 minutes unless the site's
`.env` sets it, or to 0.

Your web server provides HTTPS for the domain, so the site's own
address stays `https://`. While the site restarts after an update,
your web server shows its own error page, unless it can hold requests
until the site answers again, as the Caddy the install sets up does.

### server:uninstall

```text
gophenberg server:uninstall [flags]
```

Removes what [`server:install`](#serverinstall) set up and keeps the
site's data. Without `-yes` it only lists what it would remove and
what it keeps. Run it as root, with the `gophenberg` command the
install put on the path:

```sh
sudo gophenberg server:uninstall
```

On a server set up for `example.com` with Caddy, it prints:

```text
server:uninstall removes:
  gophenberg.service, stopped and disabled
  /etc/systemd/system/gophenberg.service
  /etc/systemd/system/gophenberg-rollback.service
  /etc/caddy/Caddyfile, putting back /etc/caddy/Caddyfile.before-gophenberg, the one the caddy package wrote
  /etc/systemd/system/caddy.service.d/gophenberg.conf
  /etc/caddy/gophenberg.caddy
  /usr/local/share/gophenberg
  /usr/local/bin/gophenberg
  /usr/local/lib/gophenberg
  /opt/gophenberg/versions
  /opt/gophenberg/downloads
  /opt/gophenberg/current
server:uninstall keeps:
  the database gophenberg and its role gophenberg
  the install root /opt/gophenberg, with the site's .env, media, themes and backups
  /etc/gophenberg/server.env
  the system user gophenberg
  /var/lib/gophenberg-installer/install.json, so a later install picks up what is kept
  the packages server:install installed, which sudo apt-get purge postgresql caddy removes
  /etc/apt/keyrings/caddy.asc, /etc/apt/sources.list.d/caddy.sources and /etc/apt/preferences.d/gophenberg-caddy, which bring Caddy's security fixes from its own apt repository
gophenberg: dry run, nothing changed, pass -yes to apply
```

Add `-yes` to remove what it lists. The run then ends with
`server:uninstall removed what it lists and kept the rest.`

- It stops the site and removes its two units.
- A Caddyfile still as the install wrote it goes back to the one the
  caddy package wrote, and Caddy restarts. A Caddyfile you changed,
  such as with a site of your own, stays, and so does Caddy's drop-in.
  The line `import /etc/caddy/gophenberg.caddy` comes out when it is
  there. Before Caddy's files change, the install record keeps the
  SHA-256 of the Caddyfile the install wrote, so a later install never
  writes over yours, even when the uninstall stops part way. Caddy then
  reloads, or restarts when the install never ended its caddy step,
  such as after a first run that stopped at `caddy validate` or a
  Caddy restart that failed.
- The builds go, and so do the `previous` link and
  `state/update.json` when they are there.
- Each folder it empties goes too, and the list names the folder in
  place of the last thing in it, such as `/usr/local/lib/gophenberg`
  for the installer's own copy. A folder that holds anything else,
  such as a file of yours, stays with what it holds.
- Anything already gone is skipped, so a second run prints
  `server:uninstall finds nothing left to remove.` The command on the
  path is gone by then, so run it with the binary of the release
  archive, such as `sudo ./gophenberg server:uninstall`.
- A run cut off part way, such as by a lost connection, is finished by
  the next one, which restarts or reloads Caddy where the cut-off run
  did not.

It reads the install root, the domain and the web server from
`/etc/gophenberg/server.env` alone, with the defaults of
[the server settings](/self-hosting/configuration/#the-server-settings)
for a missing line. It never takes them from its environment or a
`.env` file. It stops on these lines before it changes anything:

```text
gophenberg: server:uninstall must run as root, such as with sudo, so nothing was changed
gophenberg: /var/lib/gophenberg-installer, which holds the install record, must be a real folder owned by root and writable by root alone, so nothing was changed
gophenberg: another server:install or server:uninstall is running on this server, so nothing was changed
gophenberg: this server has no install record at /var/lib/gophenberg-installer/install.json, so server:uninstall has nothing to remove
gophenberg: server:uninstall needs /etc/caddy/Caddyfile to take Gophenberg's site out of Caddy, and it is not there, so nothing was changed
gophenberg: /etc/caddy/Caddyfile is a link, and the line "import /etc/caddy/gophenberg.caddy" must come out of the file it leads to before server:uninstall runs again, so nothing was changed
```

`-delete-data` also drops the database and its role. It removes the
install root with everything in it, `backups` included, then
`server.env`, with `/etc/gophenberg` when nothing else is in it, and
the system user `gophenberg`. Nothing brings that data back, so copy a
backup off the server first. It removes an install root only when the
install record shows that `server:install` made it, and otherwise
lists the folder as kept and says why. An install root the install
made that `server.env` no longer names stays too, listed as kept. The
install record then holds only the packages the install put there and
the domain. The kept Caddy still holds TCP port 80, so a later install
for that domain skips the domain check. When a Caddyfile you changed
stays, the record also keeps the SHA-256 of the Caddyfile the install
wrote, even after a first run that stopped in its caddy step, so a
later install stops until the import line is back, as in
[installing again](#installing-again). A later install takes the
packages as its own, and needs `-domain` and the first administrator
again.

#### Installing again

A later install reuses what was kept. The `gophenberg` command is gone
after an uninstall, so run the install line again:

```sh
curl -fsSL https://updates.gophenberg.org/install.sh | sudo sh -s -- --domain example.com
```

It unpacks the build again, writes the files and starts the site. It
reuses the database and asks nothing, since the first administrator
exists. By hand, give `server:install` the release archive instead:

```sh
sudo ./gophenberg server:install -archive /var/lib/gophenberg-installer/gophenberg_%VERSION%_linux_amd64.tar.gz
```

When the uninstall kept a Caddyfile you changed, the run stops until
the import line is back. It never writes over your Caddyfile:

```text
gophenberg: /etc/caddy/Caddyfile lacks the line "import /etc/caddy/gophenberg.caddy" that an earlier run wrote, which has Caddy serve the site, and it must go back in before server:install runs again, so nothing was changed
```

#### Moving to your own web server

A server the install set Caddy up on moves to a web server of your
own in three steps:

1. Run `sudo gophenberg server:uninstall -yes`.
2. Change the line `GOPHENBERG_INSTALL_WEB_SERVER=caddy` in
   `/etc/gophenberg/server.env` to `GOPHENBERG_INSTALL_WEB_SERVER=none`.
3. Run `server:install` again, as in
   [installing again](#installing-again).

Caddy stays installed and running, with the Caddyfile the uninstall
left, and still answers on its ports. Before another web server, such
as nginx, takes ports 80 and 443, stop Caddy with
`sudo systemctl disable --now caddy`, or remove it with
`sudo apt-get purge caddy`. The run ends with the lines your web
server needs, see [with your own web server](#with-your-own-web-server).

### update:check

```text
gophenberg update:check [flags]
```

Checks the update site for a newer Gophenberg now, and keeps what it
found for the admin, as the site's own checks do. It prints one line
naming the newer version and this one, or saying this is the newest
version the update site offers. With `-json` it answers one document
holding `checked_at`, `offer`, `failed_at` and `error`. It runs the
migrations first. It installs nothing: on a server install,
[`update:apply`](#updateapply) installs the update when you run it.

It fetches three files: the key list, the timestamp file and the
index. It refuses an update site whose files the Gophenberg keys did
not sign, a file that expired, and a file older than the last one the
site took. It also refuses a missing timestamp file, and a key list or
an index older than the one the timestamp file names. Each time it
exits with code 1 naming why. When a file expired, the error names
the time the site's clock reads, since a wrong clock refuses good
files. See
[how a site hears of a new version](/self-hosting/updates-and-backups/#how-a-site-hears-of-a-new-version).

### update:apply

```text
gophenberg update:apply [flags]
```

Updates a server install to a newer Gophenberg, then asks you to
restart it. It works only where `GOPHENBERG_INSTALL_ROOT` names an
install root, see
[updating a server install](/self-hosting/updates-and-backups/#updating-a-server-install).
Run it with the binary that `current` names, as the user Gophenberg
runs as. The server keeps running the old build while it works.

It checks the signed key list, timestamp file and index again,
refuses what a check would refuse, an expired timestamp file
included, then downloads the archive into `downloads/`, checks its
checksum and its signature, unpacks it into `versions/<version>/`, backs
up the database with `pg_dump` into `backups/`, and points `previous`
at the build that runs and `current` at the new one. It ends with this
line on standard error and exit code 75:

```text
gophenberg: restart Gophenberg to start 1.4.1
```

- `-to <version>` takes that version of the update site, newer than
  the one that runs, in place of the newest one offered to this site.
  It may take a version the rollout has not reached this site yet,
  but never one the update site holds back from every site.
- `-retry` takes the version in `-to` again, although it failed its
  start check after an earlier update. Without it, `update:apply`
  never takes that version again on its own.
- `-skip-backup` switches with no backup. For a release that raised
  the minimum version, `update:apply` takes the backup anyway, since
  only the backup can take the site back then.
- `-json` answers the state the update ends at, holding `step`,
  `from`, `to`, `attempts`, `database_reached`, `long_jump`, `backup`
  and `refused`. `database_reached` turns `true` once a start of the
  new build reached the database, just before it migrates, or once any
  other command of the new build migrates it, such as `migrate` run by
  hand.

Before it changes anything it refuses, with exit code 1 and what to
do: a custom build, an install with no install root such as the
Docker image, a release with no archive for this platform, a release
that needs a newer PostgreSQL, an active theme that does not load or
that the release does not serve, a `current` link that does not name
the build that runs, an update that still waits for its first start,
an install root it cannot write to, a `pg_dump` older than the
server, and too little free disk. A second `update:apply` while one
runs stops with `another update runs`. If a step fails, it deletes
what it wrote, leaves `current` as it was and writes which update
failed, when, at which step and why, which `update:status` shows.

It needs a PostgreSQL database. It does not run the migrations, the
new build runs them when it starts.

### update:rollback

```text
gophenberg update:rollback [flags]
```

Takes back the last update. It points `current` at the build before
the update again, adds the version it left to the refused versions,
so `update:apply` never takes it again on its own, and ends asking for
a restart with exit code 75. It refuses when no update switched builds
since the last rollback, and when the update raised the minimum
version and its new build already reached the database, since the
older build cannot open the database the newer one may have migrated.
Then it prints the steps that restore the backup taken before the
update instead. It never opens the database, so it works when the database is
what broke.

`-after-failure` is the form the rollback unit of the service manager
runs. The unit acts only once systemd gave up restarting the site. It
reads `MONITOR_SERVICE_RESULT`, which systemd hands that unit, and
takes the update back only when the state says the update switched
builds and systemd reports any result but `success`. So a start that
failed its check with exit code 76 and a build that keeps crashing
both count. It then exits with code 0, so the unit starts the service
again. Otherwise it changes nothing and exits with code 1. It takes
back each update once, so two failing builds never take turns. An
update that raised the minimum version and whose new build already
reached the database stays down: it writes that the
update failed, so `update:status` shows the steps that restore the
backup, prints them and exits with code 1. One whose new build never
reached the database, such as one that started while PostgreSQL was
still down, goes back like any other update.

`-json` answers the state the rollback ends at, as `update:apply`
does.

### update:status

```text
gophenberg update:status [flags]
```

Shows the server install: the install root, the builds `current` and
`previous` name, where the last update stands, the refused versions
and the backups kept, the newest first. With `-json`, `data` names the
folder that keeps the backups, which is the install root unless
`GOPHENBERG_DATA_DIR` is set. After an update that raised
the minimum version and failed its start check, it also prints the
steps that restore the backup. For example:

```text
install root: /opt/gophenberg
current build: 1.4.1
previous build: 1.4.0
last update: from 1.4.0 to 1.4.1, kept, since the new build passed its start check
refused versions: none
backups kept: /opt/gophenberg/backups/1.4.0-20261020T120000Z.dump
```

After an update that failed at a step, it also prints which update
failed, when, at which step and why, until the next update starts:

```text
last failure: from 1.4.1 to 1.4.2, at the step download on 2026-10-22T09:12:00Z: updater: refused the archive, no release or standby key the key list names signed it
```

With `-json` it answers one document holding `root`, `current`,
`previous`, `update`, `restore` and `backups`. `update` holds the same
fields as the `-json` answer of `update:apply`, and `failure` after an
update that failed at a step, holding `at`, `from`, `to`, `step` and
`error`. On an install with no install root it says that the install
updates by hand. It never opens the database.
