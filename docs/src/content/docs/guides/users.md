---
title: Users and signing in
description: Accounts, the login screen, and enabling and disabling users.
---

Everyone who writes in Gophenberg has an account with an email and
a password. An account holds one role, and the role decides what it
may do. Accounts made before roles existed are the one exception.
They hold none until you give them one, and the last section covers
that.

## Roles

There are three roles.

An **admin** runs the site. It manages accounts, installs and
switches themes, reshapes the content model, writes the
[site settings](/guides/settings/), and changes anyone's work.

An **editor** works everyone's content and media, including work
another account wrote, but does not manage accounts, themes, types
or settings.

An **author** writes and works only its own content and media.

Nothing else changes with the role. Every role signs in the same way
and sees the same admin, minus the screens its role cannot use.

## Signing in

The admin lives at `/admin`. When a login fails, the message says
why: *Invalid email or password.*, *Too many attempts. Please wait
a minute and try again.*, or *Login failed, please try again.* for
anything else, including the server being unreachable.

Once signed in, the navigation shows your name with a log out
control, and the running version at the bottom.

## Managing accounts

The Users screen lists every account with an Active or Disabled
badge.

**Creating a user** asks for email, name, and password. The
password must be at least 12 characters, and a taken email says
so.

**Disabling a user** blocks them from signing in and ends their
existing sessions immediately, so they are signed out everywhere.
You cannot disable your own account, which keeps the last
administrator from locking everyone out.

Changing a password is not something the admin offers.

**Managing accounts from a shell** works too. `account:role` sets
one account's role, and `account:disable` and `account:enable`
switch one account off and back on. Each one names your own account
with `-as`, and that account must be an enabled admin, or the
command refuses before changing anything. Each one only says what it
would change until you add `-yes`. `account:role` and
`account:disable` refuse to act on your own account, so the site
always keeps an enabled admin. Ask another admin to change your role
or disable your account.
With the Docker setup from [Install](/self-hosting/install/), this
makes an author an editor:

```sh
docker compose run --rm -T gophenberg \
  account:role -as admin@example.com -yes author@example.com editor
```

Every change applied this way is kept on record, naming who ran it.
`account:records` lists them, the newest first, and `account:list`
shows every account with its role. Every flag is on
[the commands page](/self-hosting/commands/).

## Upgrading a site that ran an earlier version

Two steps, in this order, and only on a site that ran a version
before this one.

The commands below match the Docker setup from
[Install](/self-hosting/install/), where the database service is
called `db`. If you run Gophenberg another way, drop the
`docker compose` wrappers and call `psql "$GOPHENBERG_DATABASE_URL"`
and `gophenberg` directly.

**First, rename the column, if your site still has the old one.**
Versions before this one stored the role in a column called `rank`.
The rename ships as an edit to the migration that creates it, and a
database that already ran the old one keeps the old name. Nothing
detects this, so the site starts, reports no error, and then fails
the first time anyone signs in. Check which name your database has:

```sh
docker compose exec -T db psql -U postgres -d gophenberg -tAc \
  "select column_name from information_schema.columns where table_schema='auth' and table_name='users' and column_name in ('rank','role');"
```

If it answers `role`, or nothing at all, skip this step. If it
answers `rank`, stop the site, rename the column, and start the new
version. With the new image tag already in your `compose.yaml`:

```sh
docker compose stop gophenberg
docker compose exec -T db psql -U postgres -d gophenberg \
  -v ON_ERROR_STOP=1 --single-transaction \
  -c "ALTER TABLE auth.users RENAME COLUMN rank TO role;" \
  -c "ALTER INDEX auth.users_rank_idx RENAME TO users_role_idx;"
docker compose up -d
```

Both renames happen together or neither does, so a failure halfway
leaves the database as it was. Every account keeps the role it held.

**Second, give a role to the accounts that hold none.**

Accounts made before roles existed hold no role, so they can do
nothing until one is given. The `account:grant-role` command gives a
role to every account that holds none, and says how many it changed.

Like `account:role`, `account:disable` and `account:enable`, it names
the admin running it with `-as`. While no account holds a role, there
is no admin to name, so create one first with `account:create-admin`,
which takes no `-as`, using an address none of the existing accounts
has. It waits for you to type the password. Then give the role,
acting as that admin:

```sh
docker compose run --rm -T gophenberg \
  account:create-admin -email admin@example.com -name "Maria Perez" -role admin
docker compose run --rm -T gophenberg \
  account:grant-role -role admin -yes -as admin@example.com
```

Without `-yes`, `account:grant-role` only says how many accounts it
would change. Run it once after upgrading. It only touches accounts holding no
role, so running it again changes nothing, and an account that
already holds a role keeps it.

Pick the role you want those accounts to have. On a site where the
existing accounts are the people running it, `admin` is the usual
answer. On a larger site, give `author`, then change the few
accounts that need to do more.

The login machinery comes from the Gopherium authentication
bricks, documented at
[docs.gopherium.org](https://docs.gopherium.org/authentication/overview/).
