---
title: Updates and backups
description: How to upgrade safely, what to back up, and how to restore it.
---

Two operational jobs: keeping Gophenberg current, and being able
to put it back after something goes wrong.

## Updating

Your compose file pins an exact version, so nothing updates behind
your back:

```sh
# 1. edit compose.yaml: change the image tag to the new version
docker compose pull
docker compose up -d
```

Migrations run automatically when the new version starts. While
Gophenberg is below 1.0, read the release notes first, since a
release can change behavior.

Check the database address in `GOPHENBERG_DATABASE_URL` before
updating to %VERSION%. It holds the database password before an `@`
and the database server's name after it. This release ends the
password at the first `@` it finds, so write any `@` inside the
password as `%40`. The password `p@ss` goes in as:

```text
postgres://postgres:p%40ss@db:5432/gophenberg?sslmode=disable
```

If you leave the `@` as it is, Gophenberg reads the rest of the
password as part of the database server's name. It then stops at
start with an error such as `lookup ss@db: no such host` or
`invalid port`. A password without an `@` needs no change for this
update. The `%40` form works on the release before this one too, so
you can change the database address first and update after.

Updating to %VERSION% from the release before this one runs one
migration when the server starts. It changes only the tables that
hold accounts. It marks every account as confirmed and adds one empty
table, and neither changes how anyone signs in. It changes no
content, field or setting, and it deletes nothing. Updating from
further back also runs the migrations of every release you skip, so
read each of their notes. Back up the database before the first start
on this release, as before any update. The site's public addresses do
not change shape, and a theme reads the same shapes it read before.

You can now delete a field group that still stores fields inside a
Section, a Repeater or a Flexible content field that belongs to
another group. Older releases left fields behind like that when they
moved the container. On the release before this one, deleting that
group failed with an internal error and changed nothing. When you
delete it now, each of those fields moves to the group of the
top-level field it stands inside, and keeps its values. Where the
container already holds a field of the same name from another group,
the deleted group's copy is removed and the other copy stays. If both
copies are the same kind, the fields inside the removed copy move
into the copy that stays, unless it already holds a field of that
name. If they are different kinds, the fields inside the removed copy
are removed with it. Deleting a group still deletes the group's own
fields, every field inside them, and the values they store, as before.

If you moved a container on an older release, move it once more on
this release to bring its fields along. If it ended up holding two
fields of the same name, that move is refused. Delete the group that
holds one of the two copies but not the container itself, then move
the container again. That also deletes that group's own fields, with
their values. Do not delete the group the container belongs to,
because that deletes the container and every field inside it.

The new setting `GOPHENBERG_FIELD_DEPTH` sets how many containers a
field may stand inside. It defaults to 32, the fixed limit of the
release before this one, so a site that does not set it sees no
change. See [configuration](/self-hosting/configuration/).

If a theme starts but never answers when the server checks that it is
ready, the server now stops the theme once
`GOPHENBERG_THEME_READY_TIMEOUT` passes and tries again. Before, the
server could wait forever. It starts the theme at most
`GOPHENBERG_THEME_START_ATTEMPTS` times in a row. If the server cannot
write to its themes folder, uploading a theme now says so and names
`GOPHENBERG_THEMES_DIR`, instead of failing with an internal error.

Rolling back to the release before this one deletes nothing. That
release reads fewer signs in the database password when they are
written as they are. Before you roll back, write every sign in that
password that is not a letter from a to z or a digit as its `%` code,
such as `%23` for `#` and `%3F` for `?`. Then put the older image tag
back and start it. Gophenberg only ever migrates forward, so that
release runs on this release's database as it stands. Accounts keep
signing in, and it serves the same theme kit, so no theme needs
rebuilding.

That release ignores `GOPHENBERG_FIELD_DEPTH` and holds the limit at
32 again. If you raised the limit and stored fields more than 32 deep,
those fields keep their values and you can still edit them on that
release. It refuses a new field more than 32 deep, and any import file
that holds one, even an export of the same site. Updating again brings
back what this release adds, and what you changed while rolled back
stays as it is.

Going back more than one release is not supported. To run an older
release, restore the backup you took just before you first updated
past it, with the media volume from the same day. Everything written
since that backup is lost. If you start an older image on this
release's database instead, it still starts once the password in
`GOPHENBERG_DATABASE_URL` is written with `%` codes as the rollback
steps above ask. It deletes nothing when it starts, and it ignores
`GOPHENBERG_FIELD_DEPTH`. Two releases back moves a Section, a
Repeater or a Flexible content field to another field group without
the fields inside it. It also lets a shared cache keep a failed
content API answer, and stops marking answers to a signed in account
as private. Three releases back does not know a relation standing
inside a Section, a Repeater or a Flexible content field. Its editor
refuses to save a change to a container holding one, and a change
sent through the API that leaves the relation out deletes its value.
That release also reads what an item points at from an index, while
this release reads the item's own values, so a relation changed on
that release is lost when you update again. Four releases back has
both of those problems, does not know the newer field kinds, and
serves an older theme kit. Five releases back knows only the six
original kinds and can no longer create a field group. No older image
deletes fields by itself. Running the migrations backwards by hand
does, and that is a one way trip. It deletes every field whose kind
is not one of the six, every field standing inside another field,
every field's settings, and the record of which plugin declared what,
and it leaves every Gallery as a single Media field holding a list of
files it cannot read. The values those fields held stay in the
database, but the definitions do not come back if you migrate forward
again.

This release serves themes built on `@gophenberg/astro`
%KIT_VERSION%, and the older kits it still answers. Ask a site which
ones through [the handshake](/reference/content-api/), and rebuild
your theme against one of them before updating.
[Theme compatibility](/themes/compatibility/) explains how long a
built theme keeps working.

## What to back up

**The database**, which holds everything you wrote:

```sh
docker compose exec -T db pg_dump -U postgres -Fc gophenberg > gophenberg.dump
```

The `-T` is required: without it Docker attaches a terminal that
corrupts the binary dump, and you find out when the restore fails.

**Your compose file and any `.env` beside it**, the only place
your configuration and passwords exist.

**The media volume**, which holds every file you uploaded. The
database records what each file is called and where it lives, but
never the file itself, so a lost volume leaves a library of broken
pictures no dump can repair:

```sh
docker compose cp gophenberg:/media ./media-backup
```

Back it up whenever you back up the database. The two have to come
back together, or the library and the files disagree.

**The themes volume**, if you upload themes in the admin. An
uploaded theme exists only there, so a lost volume means
re-uploading every theme:

```sh
docker compose cp gophenberg:/themes ./themes-backup
```

Which theme is active is stored in the database, not in the
volume, so both have to come back for the site to look the same.
Themes you install by hand need no backup, they are rebuilt from
their source projects.

## Restoring

Restore into a database Gophenberg has never started against,
because startup creates tables that make the restore fail:

```sh
docker compose up -d db
docker compose exec -T db pg_restore -U postgres -d gophenberg < gophenberg.dump
docker compose up -d
```

Starting only the database first is what keeps Gophenberg out of
the way until the restore is done.
