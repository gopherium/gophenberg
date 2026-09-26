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

Updating to %VERSION% from the release before this one runs no
migration, so the database stays as it is when the server starts. This
release serves the same theme kits as the release before it, so no
theme needs rebuilding. Back up the database before the first start
on this release, as before any update.

Some things work differently after this update:

- A browser that saves anything from a page on another site is
  refused with the code `request_cross_origin`. That covers the admin,
  signing in and out, and the forms a plugin serves, so a plugin form
  has to sit on this site's own pages. A program that is not a browser,
  such as a script calling the API, is not affected as long as it sends
  no `Origin` header. Behind a reverse proxy, set
  `GOPHENBERG_TRUSTED_PROXIES` and let the proxy pass
  `X-Forwarded-Proto`, or a browser too old to say where a request came
  from is refused on this site too. See
  [configuration](/self-hosting/configuration/).
- **Move** between the tops of two groups deletes the field's values
  from the content the new group does not reach, in every item and
  revision, unless another active group still serves the field there.
  The release before this one left those values hidden, and a field of
  the same name added to that content later picked them up. This
  release does not clean up values an older release left behind that
  way.
- An import that moves a field to another group, or drops a group and
  declares its fields again in another one, moves the field the way
  **Move** does. Its values stay when the field keeps its kind and the
  new group reaches the same content. Before, the import deleted them.
  The review screen says on each line whether the values follow.
- An import is judged against everything the file leaves behind once
  your ticks are counted, so it is refused before it writes anything
  or applied in full. The release before this one wrote part of some
  files and then stopped. A group you tick is now taken away before the
  file's other groups land.
- A relation inside a Section that a Linked from field reads can no
  longer be deleted. A group holding both a relation and the Linked
  from field reading it can now be deleted in one step. A site whose
  Linked from field reads a relation inside a Section can now import
  its own export.
- Every start deletes the files of an upload that a stop cut short, and
  logs each one. Run one Gophenberg per media folder, see
  [uploads a stop cut short](/self-hosting/configuration/#uploads-a-stop-cut-short).

Updating from further back also runs the migrations of every release
you skip, so read each of their notes. Coming from three releases back
or older, write any `@` inside the password in
`GOPHENBERG_DATABASE_URL` as `%40`, or the server does not start. The
`%40` form works on those releases too, so change the address first
and update after.

If a type stopped nesting on an older release while its items still
sat inside other items, this release refuses to save those items.
Press **Let items nest** on that type. For a type a plugin declared,
move those items to the top level instead.

Coming from two releases back, the update also runs that release's
one migration when the server starts. On most sites it only adds a
database index, a rule that stops one container from holding two
fields of the same name, and changes no row. It never changes what
your items, revisions and autosaves store. The migration runs as one
step. If it fails, the server stops at start and the database stays
as it was, so the older image starts again.

The migration does more on a site where one container held two fields
of the same name. That can only happen where a release four back or
older moved a Section, a Repeater or a Flexible content field to
another group without the fields inside it. The migration keeps one
copy. It keeps the copy stored in the same group as the top-level
field above it, or else the older one. The other copy is deleted with
its label and settings. If both copies are the same kind, and two
relations point at the same type, the fields inside the removed copy
move into the copy that stays. Where that copy already holds a field
of the same name, those two are settled by the same rule. Otherwise
everything inside the removed copy is deleted too. Its values stay in
your items, and the public API still serves them, even where they do
not fit the field that stays. Saving such an item can be refused
until that value is cleared. Last, every field inside a container
moves into the group of the top-level field above it.

The server logs nothing about what the migration removes, and rolling
back does not bring it back. On such a site, press **Export
definitions** before you update to keep a record, and keep the backup
until you have checked those containers. Do not delete one of the two
copies by hand first, because on that release it clears the values
both copies read.

Rolling back to the release before this one deletes nothing. Put the
older image tag back and start it. It runs on this release's database
as it stands. Accounts keep signing in, and the database password
needs no change. It serves the same theme kits, so no theme needs
rebuilding. While you run it:

- It accepts what a browser saves from a page on another site.
- Its **Move** leaves a field's values hidden on the content the new
  group does not reach, and updating again does not clean them up.
- Its imports delete the values of a field they move to another group,
  and some imports write part of the file before they refuse the rest.
- It lets you delete a relation inside a Section that a Linked from
  field reads, which leaves that field reading nothing. It refuses to
  delete a group holding both a relation
  and the Linked from field reading it. A site whose Linked from field
  reads a relation inside a Section cannot import its own export.
- A stop in the middle of an upload leaves its files in the media
  folder, and updating again does not delete them.

Updating again runs no migration, and what you changed while rolled
back stays as it is.

Going back more than one release is not supported. To run an older
release, restore the backup you took just before you first updated
past it, with the media volume from the same day. Everything written
since that backup is lost.

You can instead start an older image on this release's database.
Before you do, write every sign in the database password that is not
a letter from a to z or a digit as its `%` code, such as `%23` for
`#` and `%3F` for `?`. The older image then starts, and starting
deletes nothing. Each older release also has the problems of every
newer one, the release before this one included, unless its own line
says otherwise:

- Two releases back cannot move a field into or out of a container,
  and fields a newer release moved stay where they are. It has no
  buttons to let a type nest or stop nesting, and it lets the API or
  an import turn nesting off while items still sit inside other items.
  After you update again, those items cannot be saved until you press
  **Let items nest**. A type a plugin declared, which newer releases
  keep nesting, turns flat when it starts and stays flat after you
  update again, so move its nested items to the top level. Deleting a
  field from a group that is turned off or **Shadowed** clears the
  values another active group serves at the same place.
- Three releases back ignores `GOPHENBERG_FIELD_DEPTH` and holds the
  limit at 32. It refuses a new field more than 32 deep, and any
  import file that holds one.
- Four releases back moves a Section, a Repeater or a Flexible
  content field to another group without the fields inside it. After
  such a move, adding a field to that container under the name of a
  field left behind fails with an internal error. Deleting the old
  group fails the same way. It also lets a shared cache keep a failed
  content API answer, and does not mark answers to a signed in account
  as private.
- Five releases back does not know a relation inside a container. Its
  editor refuses to save a change to a container holding one, and a
  change sent through the API that leaves the relation out deletes its
  value. It keeps what an item points at in a separate list, while
  newer releases read the item itself, so a relation you change there
  is lost when you update again.
- Six releases back does not know the newer field kinds. It shows
  their values but refuses a change to one. It runs only themes built
  on theme kit 0.12.0 or older. A newer theme does not load, and the
  site shows the built-in pages instead. If `GOPHENBERG_THEME` names
  that theme, the server does not start.
- Seven releases back knows only the six original kinds and does not
  know containers at all. It cannot create a field group. Deleting a
  field there also deletes every field of that name inside the group's
  containers, and deleting a group also deletes its fields that stand
  inside other groups' containers. It runs only themes built on theme
  kit 0.9.0.

Running the migrations backwards by hand, as seven releases back would
need, deletes definitions, and that is a one way trip. It deletes
every field whose kind is not one of the six, every field standing
inside another field, every field's settings, and the record of which
plugin declared what. It leaves every Gallery as a single Media field
holding a list of files it cannot read. The values stay in the
database, but an item holding a value for a deleted field cannot be
saved, not even with a new title, until a field of that name exists
again. The definitions do not come back if you migrate forward again,
and neither do the copies the container migration removed.

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
