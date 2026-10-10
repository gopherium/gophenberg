---
title: The posts list
description: Finding, filtering, trashing and restoring posts in the admin.
---

The Posts screen is where everything starts: a table of your posts
with search, filters, and the trash. Every
[content type](/guides/content-types/) gets this same screen under
its own menu entry, so everything here applies to Pages and to any
type you register too.

## Reading the list

The table shows Title, Author, and Date, 20 posts per page. Click
Title or Date to sort by it. A post's title links to its editor,
unpublished posts carry a status badge, and an untitled post shows
`(no title)`.

A field the type marks **In list** gets a column of its own beside
these, showing what each post holds under it. A switch reads Yes or
No, a date reads in the site's format, and a choice reads its label
rather than its stored value. These columns do not sort. Hide any of
them from **View options**.

The date cell reads `Published <date>` once a post has ever been
published, and `Last Modified <date>` before that. A post you
returned to draft keeps its publication date, so its badge and its
date can disagree. Ordering follows that same date, so editing an
old draft does not move it up the list.

Dates and counts follow the format the site runs with, whatever
language you read the admin in, so every reader sees 04/10/2026 and
1.234 by default. Whoever runs the site picks that format, see
[the admin lists](/self-hosting/configuration/#the-admin-lists).

## Filtering and searching

A row of filters above the table narrows by status: All,
Published, Draft, Pending, and Trash, each with a live count. A
Private filter appears only when a private post exists.

The search box waits for you to stop typing, then matches titles
and post content, so a result can be matching something in its
body.

A switch or a choice field marked In list also gets a filter above
the table. Pick a value and the list narrows to the posts holding
it. Pick values on two fields and a post has to hold both. The
status counts stay as they are, counting every post of that status
rather than only the narrowed ones.

## Creating a post

Open the Posts section in the menu and press **Add New**.
Gophenberg creates a draft and opens it in
[the editor](/guides/editor/).

## Trash

**Trash** sits in the actions menu of every row not in the trash
yet, and the editor's Document panel offers **Move to trash**. Both
ask first, in a small dialog that names the post. Tick several rows
and the **Trash** button under the list moves them all at once,
behind one question.

Each finished action shows a short message at the bottom of the
screen that names the post, such as `"Hello world" moved to the
trash.` A long title is cut short with `…`, in the message and in
the question. When you act on several posts at once, the message
counts them instead. There is no Undo. A trashed post comes back
from the Trash view.

When the server refuses, the list shows the reason in a notice above
the rows, such as a page that still holds pages nested inside it,
and the editor shows it inside its dialog. The notice above the list
stays until your next action, or until you switch to another status
or content type. Rows the server refused stay ticked, so you can try
them again.

In the Trash view, each row offers two actions, and both also work
on several ticked rows at once:

- **Restore** brings the post back **as a draft**, whatever it was
  before. Publish it again to put it back on your site.
- **Permanently delete** removes the post forever, after a
  confirmation.

**Empty Trash**, which only appears here, clears everything at once.

Opening a trashed post from its title shows it read only. Its
**Restore** control brings it back as a draft, names it in a
message, and opens the editor.
