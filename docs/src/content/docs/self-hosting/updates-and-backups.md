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
docker compose run --rm -T gophenberg check
docker compose up -d
```

`check` reads your settings with the new version while the old one
still serves, and names the ones it refuses, see
[what stops startup](/self-hosting/configuration/#what-stops-startup).
Migrations run automatically when the new version starts, and
`docker compose run --rm -T gophenberg migrate` applies them without
starting the server. Two copies that migrate at once never apply the
same migration twice, the second waits for the first and then finds
nothing left to do. While Gophenberg is below 1.0, read the release
notes first, since a release can change behavior.

Updating to %VERSION% from an earlier %FEATURE_VERSION% release runs no
migration, changes no setting and needs no theme rebuild. It fixes
**Empty Trash**, which on any list but Posts deleted the trashed posts
for good and left the trash on screen as it was. Rolling back within
%FEATURE_VERSION% deletes nothing, and brings that defect back.

Updating to %FEATURE_VERSION% from the release before it runs one
migration when the server starts, or when you run `migrate`. It adds
the `gonsole` schema, with one table that keeps a record of each
account change applied with `-as`. It changes no row you wrote. The
database account needs the right to create a schema, the same right
the site used when it first started. If the migration fails, the
server stops at start, and the older image still starts on that
database. Until it runs, `account:records` and the commands that take
`-as` refuse with `the command records are missing, run migrate first`.
This release serves the same theme kits as the release before it, so
no theme needs rebuilding. Back up the database before the first
start on this release, as before any update.

Some things work differently after this update:

- A run with no command lists the commands and stops, instead of
  serving. A word that is not a command stops with exit code 2 before
  it touches the database. The release before this one started the
  full server for any word it did not know. The image runs `serve`
  when it is given no command, so the compose file from
  [Install](/self-hosting/install/) keeps serving. A setup that passes
  the image its own arguments, or runs the binary itself, has to name
  `serve` first.
- A stop takes up to 23 seconds at the default settings. It gives the
  running requests `GOPHENBERG_SHUTDOWN_GRACE` to finish, cancels the
  rest, and then gives the plugins and the theme their own time.
  Docker kills the server after 10 seconds unless the compose file
  sets `stop_grace_period`, so set it above 23 seconds, as the compose
  file from [Install](/self-hosting/install/) does with `30s`. See
  [stopping the server](/self-hosting/configuration/#stopping-the-server).
- `createadmin` is now `account:create-admin`, and `grantrole` is now
  `account:grant-role`. The old names stop with exit code 2.
  `account:create-admin` takes only the roles `admin`, `editor` and
  `author`.
- `seed` and `account:grant-role` only say what they would change
  until you add `-yes`. `account:grant-role` also needs `-as` with the
  address of an enabled admin, and records each change it applies.
  Update any script that calls them, see
  [the commands page](/self-hosting/commands/).
- Every setting is trimmed of the spaces around it, and a value made
  only of spaces counts as unset, so it takes the default. The HTTP
  timeouts and the time an upload has to arrive are now settings. Their
  defaults are the fixed values the release before this one used. See
  [configuration](/self-hosting/configuration/).
- The new `GOPHENBERG_PUBLIC_URL` names the address people reach the
  site at. Left unset, it changes nothing. Once set, every write sent
  to another address is refused, a script writing to the server's
  internal address included, and nginx needs
  `proxy_set_header Host $host`. Every refused write is now logged as
  `write refused`, with its reason, whether the setting is set or not.
  See [the public address](/self-hosting/configuration/#the-public-address).
- Every start deletes the files of an upload that a stop cut short, and
  logs each one. Run one Gophenberg per media folder, see
  [uploads a stop cut short](/self-hosting/configuration/#uploads-a-stop-cut-short).
  An upload over 32 MB no longer leaves a scratch file of its own size
  in the temporary folder.
- A still picture over the pixel limit is refused with
  `image_pixel_budget_exceeded`, where the release before this one
  answered `image_frame_too_large`. A picture that declares no pixels
  is refused with `image_unreadable`. A program that reads those codes
  through the API needs updating.
- A post in the trash is read only. An edit, an autosave or a revision
  delete on it is refused with `content_trashed`, and the admin opens
  it with a **Restore** button. Moving a post to the trash deletes the
  unsaved work parked on it, from every author.
- Changes made at the same moment no longer slip past the checks on a
  page's place. A restore that lands while another admin restores and
  publishes the same post is refused with `restore_not_trashed`,
  instead of turning it back into a draft. A page filed under a parent
  that moves to the trash at that moment is refused with
  `parent_trashed`. Of two opposite page moves, the second is refused
  with `parent_cycle`, instead of nesting the pages inside each other.
- **Rules** on a group can move the group onto other content and point
  each of its Linked from fields anew in the same save. The release
  before this one needed such a field deleted and declared again. An
  import with a move you left unticked lists that field, and
  everything inside it, as left alone. It no longer creates a group
  that would stand empty.
- On a site that builds its own plugins, commands such as `list` and
  `check` register every plugin and stop it again without starting
  it, so a plugin has to open nothing before it starts, and its `Stop`
  has to work when `Start` never ran. A plugin that imports only the
  `sdk` package builds unchanged. See
  [the plugin SDK](/extending/the-plugin-sdk/).

Updating from further back also runs the migrations of every release
you skip, so read each of their notes. Coming from four releases back
or older, write any `@` inside the password in
`GOPHENBERG_DATABASE_URL` as `%40`, or the server does not start. The
`%40` form works on those releases too, so change the address first
and update after.

Coming from two releases back or older, a browser that saves anything
from a page on another site is refused with `request_cross_origin`,
so a plugin form has to sit on this site's own pages. Behind a reverse
proxy, set `GOPHENBERG_PUBLIC_URL`, or set
`GOPHENBERG_TRUSTED_PROXIES` and let the proxy pass
`X-Forwarded-Proto`. Without one of them, a browser too old to say
where a request came from is refused on this site too.
**Move** also deletes the values a field leaves on the content its
new group does not reach, where those releases left them hidden.

If a type stopped nesting on an older release while its items still
sat inside other items, this release refuses to save those items.
Press **Let items nest** on that type. For a type a plugin declared,
move those items to the top level instead.

Coming from three releases back or older, the update also runs the
migration that two releases back added. On most sites it only adds a
database index, a rule that stops one container from holding two
fields of the same name, and changes no row. It never changes what
your items, revisions and autosaves store. The migration runs as one
step. If it fails, the server stops at start and the database stays
as it was, so the older image starts again.

The migration does more on a site where one container held two fields
of the same name. That can only happen where a release five back or
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
older image tag back, the newest patch of that release, and start it.
It runs on this release's database as it stands. Accounts keep
signing in, and the database password needs no change. It serves the
same theme kits, so no theme needs rebuilding. It does not bring back
what this release deleted, such as the work parked on a post moved to
the trash. While you run it:

- It knows three commands, `createadmin`, `grantrole` and `seed`.
  `seed` stores the demo data at once, with no dry run, and
  `grantrole` refuses `-as` and `-yes`. Any other word, `serve`,
  `check`, `migrate`, `list` and every `account:` command included,
  starts the full server and runs its migrations. Under
  `docker compose run`, that server keeps running until you stop it.
- It ignores the `gonsole` schema and keeps no record of the account
  changes it makes. The records kept before the rollback stay. The
  roles and disabled accounts set with the `account:` commands still
  hold.
- It ignores the settings this release added. Its HTTP timeouts and
  the time an upload has to arrive are fixed at this release's
  defaults. A stop gives the requests and the plugins one window of 10
  seconds together, and never cancels a running request.
- It reads every setting as it is written, spaces included. A value
  this release took only because it trimmed the spaces around it can
  stop it from starting, or be read as another value, and a value made
  only of spaces counts as set. Take those spaces out before you roll
  back.
- It ignores `GOPHENBERG_PUBLIC_URL` and logs no refused write. Where
  that setting was on, a browser too old to say where a request came from
  is refused again behind a proxy that does not pass
  `X-Forwarded-Proto`. Another domain pointed at your server can again
  make a visitor's browser post to the site.
- A stop in the middle of an upload leaves its files in the media
  folder, and updating again does not delete them. An upload over
  32 MB leaves a scratch file of its own size in the temporary folder,
  and a refused upload answers the old codes.
- A post in the trash takes edits, autosaves and revision deletes
  again. A restore that lands while another admin restores and
  publishes the same post can turn it back into a draft, a page filed
  as its parent moves to the trash can land under it, and two opposite
  page moves made at the same moment can nest the pages inside each
  other.
- Leaving the editor for another admin screen loses the unsaved words.
  Going Back to a post can show another post's content, which an
  autosave can then store as the first post's unsaved work.
- Two admins saving at the same moment can take a field deeper than
  `GOPHENBERG_FIELD_DEPTH`, or leave a Linked from field reading a
  relation that is gone. Moving a group with a Linked from field needs
  that field deleted and declared again, and an import with a move you
  left unticked creates a group that stands empty.

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

- Two releases back accepts what a browser saves from a page on
  another site. Its **Move** leaves a field's values hidden on the
  content the new group does not reach, and updating again does not
  clean them up. Its imports delete the values of a field they move to
  another group, and some imports write part of the file before they
  refuse the rest. It lets you delete a relation inside a Section that
  a Linked from field reads, which leaves that field reading nothing.
  It refuses to delete a group holding both a relation and the Linked
  from field reading it. A site whose Linked from field reads a
  relation inside a Section cannot import its own export. It also lets
  a block delimiter inside an allowed attribute hide an event handler
  from the content sanitizer, which the newest patch of the release
  before this one fixed.
- Three releases back cannot move a field into or out of a container,
  and fields a newer release moved stay where they are. It has no
  buttons to let a type nest or stop nesting, and it lets the API or
  an import turn nesting off while items still sit inside other items.
  After you update again, those items cannot be saved until you press
  **Let items nest**. A type a plugin declared, which newer releases
  keep nesting, turns flat when it starts and stays flat after you
  update again, so move its nested items to the top level. Deleting a
  field from a group that is turned off or **Shadowed** clears the
  values another active group serves at the same place.
- Four releases back ignores `GOPHENBERG_FIELD_DEPTH` and holds the
  limit at 32. It refuses a new field more than 32 deep, and any
  import file that holds one.
- Five releases back moves a Section, a Repeater or a Flexible
  content field to another group without the fields inside it. After
  such a move, adding a field to that container under the name of a
  field left behind fails with an internal error. Deleting the old
  group fails the same way. It also lets a shared cache keep a failed
  content API answer, and does not mark answers to a signed in account
  as private.
- Six releases back does not know a relation inside a container. Its
  editor refuses to save a change to a container holding one, and a
  change sent through the API that leaves the relation out deletes its
  value. It keeps what an item points at in a separate list, while
  newer releases read the item itself, so a relation you change there
  is lost when you update again.
- Seven releases back does not know the newer field kinds. It shows
  their values but refuses a change to one. It runs only themes built
  on theme kit 0.12.0 or older. A newer theme does not load, and the
  site shows the built-in pages instead. If `GOPHENBERG_THEME` names
  that theme, the server does not start.
- Eight releases back knows only the six original kinds and does not
  know containers at all. It cannot create a field group. Deleting a
  field there also deletes every field of that name inside the group's
  containers, and deleting a group also deletes its fields that stand
  inside other groups' containers. It runs only themes built on theme
  kit 0.9.0.

Running the migrations backwards by hand, as eight releases back would
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

**The database**, which holds everything you wrote, and the record of
each account change applied with `-as`:

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

Restore into a database Gophenberg has never touched, because the
tables it creates make the restore fail. Starting the server creates
them, and so do `migrate`, `seed -yes` and `account:create-admin`, so
run none of them before the restore:

```sh
docker compose up -d db
docker compose exec -T db pg_restore -U postgres -d gophenberg < gophenberg.dump
docker compose up -d
```

Starting only the database first is what keeps Gophenberg out of
the way until the restore is done.
