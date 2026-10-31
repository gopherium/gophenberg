---
title: Users and signing in
description: Accounts, the login screen, and adding, changing and disabling users.
---

Everyone who writes in Gophenberg, and every member of the site, has
an account with an email and a password. An account holds one role,
and the role decides what it may do. Accounts made before roles
existed are the one exception.
They hold none until you give them one, and the last section covers
that.

## Roles

Gophenberg comes with four roles, and a plugin may declare more.
A role carries capabilities, such as `manage_users` or
`manage_themes`, and each one opens a part of the site. The admin,
the editor and the author carry `access_admin`, which lets them into
the admin at all.

An **admin** runs the site. It manages accounts, installs and
switches themes, reshapes the content model, writes the
[site settings](/guides/settings/), checks for a newer Gophenberg
with `update_core`, and changes anyone's work.

An **editor** works everyone's content and media, including work
another account wrote, but does not manage accounts, themes, types
or settings.

An **author** writes and works only its own content and media.

A **member** signs in but never enters the admin. It keeps its own
session and its language, and a plugin decides what else it may do,
such as posting in a forum. A member who opens `/admin` reads *Sorry,
you are not allowed to access this page.*, with a link to the site
and a Log out button.

Nothing else changes with the role. Every role signs in the same way,
and every role carrying `access_admin` sees the same admin, minus the
screens its capabilities do not open. A role a plugin declares with
`access_admin` and `manage_themes`, for example, sees the Themes
screen and none of the other administration screens. Each further
capability the role carries opens its own screen too.

## Signing in

The admin lives at `/admin`. When a login fails, the message says
why: *Invalid email or password.*, *Too many attempts. Please wait
a minute and try again.*, or *Login failed, please try again.* for
anything else, including the server being unreachable.

Once signed in, the navigation shows your name with a log out
control, and the running version at the bottom.

## Managing accounts

Open **Users** in the admin menu. It shows to every role carrying
`manage_users`, which `admin` carries. The screen is a list like the
[posts list](/guides/posts/), drawn the way WordPress draws its
lists. Under the title it says what the screen is for: *Manage who
can sign in and what they can do.*

**Reading the list.** Each row shows the account's name with its
email under it, then its Status, its Role, how many posts it wrote,
and when it was created. Status is a badge, Active or Disabled. An
account holding no role reads None. A role a plugin declares reads
as its name with a capital first letter, such as Archivist for
`archivist`. Created shows the date and time the way the posts list
writes them.

A small round avatar sits before each name, drawn the way the
site's [avatar settings](/self-hosting/configuration/#avatars) say,
as in the posts list. When the site shows no avatars, the list has
none at all. Otherwise you can hide it under **View options**, as
you can every column but the name.

The Posts column counts each account's published and private posts.
A count above 0 links to the posts list showing that account's
posts.
It takes its title from the Posts type, so it reads Posts until
someone renames that type. While that type is deactivated or
deleted, the column is gone.

On a phone the list shows one account per row, with its avatar, its
name, its email, its status, its role and when it was created. The
posts count is not on the row. Turned on under **View options**, it
shows as plain text there, with no link.

**Finding an account.** The search box reads `Search users…` and
matches names and emails. **Add filter** narrows the list by
**Role** or by **Status**, and each one offers only the values the
accounts hold. **View options** sorts by Name, Email or Created, and
offers the page sizes the site allows. The list opens newest first,
at the page size the site sets, 20 by default.

The page, the page size, the sort, the search and the filters all
live in the address of the screen. Reload it or share the link and
the list opens the same way. When nothing matches, the list says
`No users found.`

The screen reads every account at once, then searches, sorts and
pages them in your browser. On a site with very many accounts, it
takes longer to open.

**The actions on an account.** The actions menu of a row offers
**Change role…**, and **Enable** on a disabled account or
**Disable** on an active one. Enable is the main action. While the
pointer rests on the row of a disabled account, Enable also shows as
a button at the end of the row. On a phone it shows beside the menu.

A row you may not change shows no menu, and its name is plain text.
That is your own row, and the row of any account whose role carries
a capability your role lacks. Enable shows only on an account whose
role you may give.

Tick several rows and the bar under the list offers the same three
actions for all of them at once. A phone list has no tick boxes.

Each finished action shows a short message at the bottom of the
screen that names the account, such as `"Maria Perez" disabled.`,
or counts the accounts when you acted on several. When the server
refuses, the message says why instead. When some of several fail,
it counts the ones that failed.

**Changing a role** starts from **Change role…** in the row's menu.
You can also click the name of an account you may change, or tap
its row on a phone. A small dialog opens with the line *Choose the
role of Maria Perez.* Its menu offers only the roles your own role
reaches, the one with the fewest capabilities first, so a role
carrying a capability yours lacks is not there. For one account the
menu starts on that account's role when you may give it. Otherwise,
and for several accounts, it starts on *Change role to…*, and
**Change role** stays off until you pick a role.

The dialog closes the moment you press **Change role**. The message
then says *Role changed for "Maria Perez".*, or *Changed roles.* for
several accounts. When the server turns a change down, the message
says why, such as *Sorry, you are not allowed to give users that
role.* or *Somebody must be able to manage users.* The dialog then
opens on the role you picked the next time. Once it closes, the
focus goes back to what opened it.

**Disabling a user** blocks them from signing in and ends their
existing sessions immediately, so they are signed out everywhere.
You cannot disable your own account, which keeps the last account
that manages users from locking everyone out. **Enable** lets a
disabled account sign in again. An account whose role nothing
declares any more needs a new role from **Change role…** before
Enable shows.

**Adding a user** starts from **Add User** at the top of the list.
The page says *Create a brand new user and add them to this site.*
and asks for an email, a name, a password and a role. Email and Name
are marked *(Required)*.

The password field starts with a generated password, 24 characters
long, in plain sight. **Generate password** puts a new one in, and
**Hide** hides it. A meter under the field rates the password as
you type. A weak one asks you to tick **Confirm use of weak password**.
Press **Add User** before you tick it and the field says *Please
confirm the use of a weak password.* The password must be at least
12 characters.

The role menu offers the roles your own role reaches, the one with
the fewest capabilities first. It starts on the site's default role,
`GOPHENBERG_DEFAULT_ROLE` in the
[configuration](/self-hosting/configuration/), which is `member`
unless set. When you may not give that role, it starts on the role
with the fewest capabilities you may give.

Press **Add User** to create the account. An empty field, a taken
email or a password that is too short shows its message under that
field, such as *This email is already registered. Please choose
another one.* Any other failure shows a message at the bottom of the
screen, and the form keeps what you typed. Once the account exists,
the screen goes back to the list and says *New user created.*

Changing a password is not something the admin offers.

**Managing accounts from a shell** works too. `account:role` sets
one account's role, and `account:disable` and `account:enable`
switch one account off and back on. Each one names your own account
with `-as`, and that account must be enabled and hold a role carrying
`manage_users`, or the command refuses before changing anything.
Each one only says what it would change until you add `-yes`.
`account:role` and `account:disable` refuse to act on your own
account, and the last enabled account whose role carries
`manage_users` always stays. Ask another account holding
`manage_users` to change your role or disable your account.
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

Accounts made before roles existed hold no role, so the admin turns
them away with *Sorry, you are not allowed to access this page.*
until one is given. So does an account whose role belonged to a
plugin that is gone. That account still holds the role's name, so
`account:grant-role` skips it: find it with `account:list` and give
it a role with `account:role`. The `account:grant-role` command gives a
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
