---
title: The posts list
description: Finding, filtering, sorting, copying, renaming, trashing and restoring posts in the admin.
---

The Posts screen is where everything starts: a table of your posts
with search, filters, and the trash. Every
[content type](/guides/content-types/) gets this same screen under
its own menu entry, so everything here applies to Pages and to any
type you register too.

Under the title sits a short sentence saying what the list is for.
It is the description of the content type, or a sentence of the
admin's own when the type has none.

## Reading the list

The table shows Title, Author, Status and Date. A post's title links
to its editor, and an untitled post shows `(no title)`. When a post
has an excerpt, it shows under the title. A small round avatar sits
before the author's name. By default it is a circle holding the
first letter of the name. The site's
[avatar settings](/self-hosting/configuration/#avatars) can show the
Gravatar picture instead, or no avatar, and then the author icon
sits there, as in WordPress.

The Status column shows a badge such as Draft, Pending Review or
Published. It shows only on the All tab, since every other tab holds
one status.

The date cell reads `Published: <date> <time>` on a published post,
`Scheduled: <date> <time>` on a scheduled one, and
`Modified: <date> <time>` on a draft, a pending or a private post.
In the Trash tab it shows the date and time alone. The list sorts by
that same moment, so the date you read is the date the order
follows.

Like WordPress, the date cell writes the date the way WordPress does
in the language you read the admin in. English and Spanish write the
month short, `Oct 8, 2026 5:54 pm` and `8 Oct 2026 17:54`. French
keeps it whole, `08 octobre 2026 17h54`.

Slug, and Parent on a type whose items nest, are hidden at first.
Turn them on under **View options**, where you can also hide every
other column but Title.

A field the type marks **In list** gets a column of its own,
showing what each post holds under it. A switch reads Yes or No, a
date reads in the site's format, and a choice reads its label rather
than its stored value. These columns do not sort.

On a type whose items nest, such as Pages, each item sits under its
parent with a dash for every level below the top. That tree shows
only in the order the list opens in. Sort by another column and the
list turns flat, and a reload of that address stays flat too.

On a phone the list shows one item per row with its status and its
date. Tap a row to open the post.

Counts and the dates of an **In list** field follow the format the
site runs with, whatever language you read the admin in, so every
reader sees 04/10/2026 and 1.234 by default. Whoever runs the site
picks that format, see
[the admin lists](/self-hosting/configuration/#the-admin-lists).

## Pages and sorting

The list shows 20 posts per page by default. **View options** offers
the other page sizes the site allows, and the arrows under the list
move between pages.

Click a column title, or use **View options**, to sort by Title,
Author, Date or Slug, and by Parent on a type whose items nest. The
list opens newest first.

The page, the page size, the sort, the search and the filters all
live in the address of the screen. Reload it or share the link and
the list opens the same way.

## Filtering and searching

The tabs above the table narrow the list by status: All, Published,
Draft, Pending, Private and Trash. They show no counts. All lists
every post that is not in the trash. Picking a tab goes back to the
first page and clears the search, and keeps the sort, the filters
and the page size.

The search box waits for you to stop typing, then matches titles
and post content, so a result can be matching something in its
body.

**Add filter** narrows the list further:

- **Author** keeps the posts of the people you pick, or with **Is
  none of** leaves them out.
- **Date** keeps the posts dated before or after a moment you pick.
  You pick it on your own clock, the clock the date cells use. Its
  calendar names the months in the language you read the admin in,
  and its weeks start on Monday. Its chip writes the moment in full,
  such as `October 8, 2026 5:54 pm`, as WordPress does.
- A switch or a choice field marked In list keeps the posts holding
  the value you pick.

Filters add up, so a post has to match every one of them.

When a search or a filter matches nothing, the list says `No results
found`. A status tab with nothing in it says so in its own words,
such as `No drafts` or `Trash is empty`. A list that holds nothing at
all says `No items yet` and offers the add button.

## Creating a post

Press the button at the top of the list that names the type, such as
**Add Post** or **Add Page**. Gophenberg creates a draft
and opens it in [the editor](/guides/editor/). The button waits until
the screen has read the content type, so the draft starts on the
defaults its fields name. If it fails, a message at the bottom of the
screen says why.

Each content type is one screen. Its menu entry opens the list
straight away, with the status tabs above the table, and the menu
stays as it is beside it. The admin's own address opens the list of
the default content type.

## The actions on a row

The actions menu of a row offers:

- **View** opens a published post on your site in a new tab.
- **Duplicate…** makes a new draft holding the post's title with
  `(Copy)` after it, its content, its excerpt, its parent and the
  field values it shows. A value under a field its conditions hide
  stays behind, and so does a Linked from field, which the site
  fills on its own. You can change the title in the dialog first.
  Anyone who can write may duplicate any post not in the trash.
- **Rename…** changes the title in a dialog, sent with **Save**. It
  writes over the newest version of the post, so the last rename
  wins.
- **Trash** moves the post to the trash at once, and the message
  offers **Undo**.

View is the main action. While the pointer rests on the row of a
published post, View also shows as a button at the end of the row,
as in WordPress. On a touch phone, the first main action that
applies shows beside the menu of each row instead: View on a
published post, Restore in the Trash tab.

An action ending in `…` opens a dialog first, as in WordPress.
**Trash** has none, since it opens nothing. Rename… and Trash show
only on the posts you may change.

The rename and duplicate dialogs stay open with their button busy
until the server answers. Cancel closes nothing while they save. The
rename's **Save** stays on, and a blank name is refused before
anything is sent. When the save fails, the dialog says why inside it
and keeps what you typed.

## Trash

**Trash** sits last in the actions menu of every row not in the
trash yet. It moves the post to the trash at once, with no question.
Tick several rows and the **Trash** button under the list moves them
all at once.

The editor's Document panel offers **Move to trash** under the
excerpt, above the item's fields. It is a red button, as in the
WordPress editor. It moves the post at once too, shows the post
moving and takes no second press, then goes back to the list, which
shows the same message.

Each trash shows a short message at the bottom of the screen that
names the post, such as `"Hello world" moved to the trash.` The
message shows the whole title. When you trash several posts at once,
the message counts them instead. The message offers **Undo**, which
brings the posts back as **Restore** does. It stays for 10 seconds,
unless whoever runs the site sets another length, see
[the admin lists](/self-hosting/configuration/#the-admin-lists). A
trashed post also comes back from the Trash tab.

When the server refuses, the message at the bottom of the screen
gives the reason instead, from the list and from the editor alike. A
page that still holds pages nested inside it cannot go to the trash
until those move or go first. When you act on several posts and some
fail, the one message counts the ones that failed and offers no Undo.
Rows the server refused stay ticked, so you can try them again. The
posts that moved lose their tick, so **Undo** brings them back
unticked.

In the Trash tab, each row offers two actions, and both also work
on several ticked rows at once:

- **Restore** brings the post back **as a draft**, whatever it was
  before. Publish it again to put it back on your site. It is the
  main action here, so it also shows on the row the pointer rests
  on.
- **Permanently delete…** removes the post forever, with its
  revisions and autosaves, after a question. The dialog's title
  names the post, such as `Delete "Hello world"?`, and under it
  reads
  `The item is deleted for good, with its revisions and autosaves. This action cannot be undone.`
  For several posts the title and the sentence count them. Its
  **Delete permanently** button is red.

**Empty Trash** appears at the top of the Trash tab while it lists
something, for an admin or an editor. It asks in a dialog titled
**Empty the trash?** that counts every trashed post of the type, not
only the ones a search shows, and warns that this cannot be undone.
Its **Empty trash** button is red. Confirm, and the dialog closes at
once while **Empty Trash** shows the work running. Every trashed post
of this type goes in one step. If some posts stay in the trash, such
as ones trashed while it ran, the message counts them too. If the
server refuses, the message says why and the trash stays as it was.

Opening a trashed post from its title shows it read only. Its
**Restore** control brings it back as a draft, names it in a
message, and opens the editor.
