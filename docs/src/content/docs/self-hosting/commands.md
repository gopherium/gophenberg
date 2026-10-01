---
title: Commands
description: Every command the binary runs, how to read its help, and who may change an account.
---

The `gophenberg` binary serves the site, and it also runs the jobs an
operator does beside it, such as creating the first account or
applying the schema. Every command reads the same environment
variables and `.env` file as the server, listed in
[configuration](/self-hosting/configuration/).

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
  listing still exits with code 0, but `serve`, `check`, `migrate`
  and `seed -yes` fail until the setting is fixed.

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

Every error goes to standard error, on a line that starts with
`gophenberg:`. When the command itself was found, a misuse is followed
by its help page.

## Dry runs and -yes

Five commands only show what they would change until you add `-yes`:
`seed`, `account:grant-role`, `account:role`, `account:disable` and
`account:enable`. This line, for example:

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

## Who may change an account

`account:grant-role`, `account:role`, `account:disable` and
`account:enable` need `-as` with the email address of the person
running the command. That account must exist, be enabled and
activated, and hold a role that may manage users, which only `admin`
does. The check runs before anything changes, on a dry run too. When
it fails, the command exits with code 1 and says why:

```text
gophenberg: the account editor@example.com holds the role editor, which lacks manage_users
```

Leaving out `-as` exits with code 2.

`-as` asks for no password. It names who answers for the change.
Anyone who can run commands against the database can name any
account, so keep access to the server as tight as before.

The check needs the table the records are kept in. On a database
updated from an earlier release, start the new server or run
`migrate` first, or the command stops with
`the command records are missing, run migrate first`.

## The records

Every change those four commands apply with `-yes` is kept as one
record in the `gonsole.records` table of the database, so the usual
database backup carries it. A record names the time, the account in
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

### check

```text
gophenberg check
```

Reads every setting, registers the plugins and checks the command
names, without touching the database. It prints
`settings, plugins and command names are valid`, or names the first
bad setting and exits with code 1. Run it before a rollout.

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

### seed

```text
gophenberg seed [flags]
```

Stores demo data to try Gophenberg on. Without `-yes` it only prints
`would store the demo data`. With `-yes` it applies every schema
step, then stores the demo content, the accounts
`admin@example.com`, `editor@example.com` and `author@example.com`,
and each plugin's demo data. It prints `created` or `kept` beside each
account, and the accounts it created sign in with `password1234`.
Never seed a production database.

### serve

```text
gophenberg serve
```

Applies every schema step, then serves the site until it is stopped,
see [stopping the server](/self-hosting/configuration/#stopping-the-server).
The container image runs `serve` when it is given no command. A
compose file or deployment that sets its own `command` must start it
with `serve`, since a run with no command only prints the listing.

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
`-name`, and the role in `-role`, which is `admin`, `editor` or
`author`. It reads the password from standard input, keeping it out of
your shell history, and applies the `accounts`, `records` and `core`
schema steps first. It needs no `-as`, since it is how a site gets its
first admin:

```sh
docker compose run --rm -T gophenberg \
  account:create-admin -email admin@example.com -name "Maria Perez" -role admin
```

### account:disable

```text
gophenberg account:disable [flags] <email>
```

Disables one account, so it can no longer sign in. It refuses to
disable the last enabled admin.

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

Lists every account with its role, or a dash when it holds none, and
whether it is enabled. With `-json` it answers one document holding
each account's `id`, `email`, `name`, `role` and `disabled`.

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

Sets the role of one account to `admin`, `editor` or `author`. It
refuses to take the admin role from the last enabled admin.
