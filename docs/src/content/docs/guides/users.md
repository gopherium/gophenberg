---
title: Users and signing in
description: Accounts, the login screen, and enabling and disabling users.
---

Everyone who writes in Gophenberg has an account with an email and
a password. Every account holds one rank, and the rank decides what
it may do.

## Ranks

There are three ranks.

An **admin** runs the site. It manages accounts, installs and
switches themes, reshapes the content model, writes the site
settings, and changes anyone's work.

An **editor** works everyone's content and media, including work
another account wrote, but does not manage accounts, themes, types
or settings.

An **author** writes and works only its own content and media.

Nothing else changes with rank. Every rank signs in the same way and
sees the same admin, minus the screens its rank cannot use.

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

## Giving a rank to accounts that hold none

Accounts made before ranks existed hold no rank, so they can do
nothing until one is given. The `grantrank` command gives a rank to
every account that holds none, and says how many it changed.

```sh
GOPHENBERG_DATABASE_URL=... gophenberg grantrank -rank admin
```

Run it once after upgrading. It only touches rankless accounts, so
running it again changes nothing, and an account that already holds
a rank keeps it.

Pick the rank you want those accounts to have. On a site where the
existing accounts are the people running it, `admin` is the usual
answer. On a larger site, give `author` and raise the few who need
more.

The login machinery comes from the Gopherium authentication
bricks, documented at
[docs.gopherium.org](https://docs.gopherium.org/authentication/overview/).
