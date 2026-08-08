---
title: Users and signing in
description: Accounts, the login screen, and enabling and disabling users.
---

Everyone who writes in Gophenberg has an account with an email and
a password. There are no roles: every account can do everything,
including managing other accounts.

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

The login machinery comes from the Gopherium authentication
bricks, documented at
[docs.gopherium.org](https://docs.gopherium.org/authentication/overview/).
