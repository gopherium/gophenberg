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

Updating to %VERSION% from the release before it runs one migration
on start. What every item points at is copied out of the relation
index and into the item's own values, under the field's key and in
the order it was stored, so a relation now lives with its item and
the index is rebuilt from it on every save. Updating from further
back also runs the migrations of every release you skip, so read
each of their notes. Back up the database before the first start on
this release, as before any update. Public addresses do not change
shape, and a theme reads the same shapes it read before.

A revision or an autosave written before this release holds no
relation values, because the release before kept those in the index
alone. Taking up an autosave parked before the update leaves what
the item points at alone.

Rolling back to the release before this one deletes nothing.
Gophenberg only ever migrates forward, so put the older image tag
back and start it. It runs on this release's database as it stands,
and it serves the same theme kit, so no theme needs rebuilding. That
release reads what an item points at from the index, which this
release keeps current, so every relation still shows. It writes a
changed relation to the index alone, while this release reads the
item's own values, so updating again puts back what each item
pointed at before the rollback and a relation changed in between is
lost. That release also does not know a relation standing inside a
Section, a Repeater or a Flexible content field, so it refuses to
save a change to any container holding one. Updating again brings
everything back.

Going back further is not something Gophenberg does for you. An
older image still starts on this database and deletes nothing. Two
releases back does not know the field kinds the release before this
one added, serves an older theme kit, and shares the relation caveat
above. Three releases back knows only the six original kinds and
can no longer create a field group. Restore the backup you took
before updating instead. Only running the migrations backwards by
hand deletes fields, and that is a one way trip. It deletes every
field whose kind is not one of the six, every field standing inside
another field, every field's settings, and the record of which
plugin declared what, and it leaves every Gallery as a single Media
field holding a list of files it cannot read. The values those
fields held stay in the database, but the definitions do not come
back if you migrate forward again.

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
