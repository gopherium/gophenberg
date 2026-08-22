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
switches themes, reshapes the content model, writes the site
settings, and changes anyone's work.

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
nothing until one is given. The `grantrole` command gives a role to
every account that holds none, and says how many it changed.

```sh
docker compose run --rm -T gophenberg grantrole -role admin
```

Run it once after upgrading. It only touches accounts holding no
role, so running it again changes nothing, and an account that
already holds a role keeps it.

Pick the role you want those accounts to have. On a site where the
existing accounts are the people running it, `admin` is the usual
answer. On a larger site, give `author`, then change the few
accounts that need to do more.

The login machinery comes from the Gopherium authentication
bricks, documented at
[docs.gopherium.org](https://docs.gopherium.org/authentication/overview/).
