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
No, a date reads in your language, and a choice reads its label
rather than its stored value. These columns do not sort. Hide any of
them from **View options**.

The date cell reads `Published <date>` once a post has ever been
published, and `Last Modified <date>` before that. A post you
returned to draft keeps its publication date, so its badge and its
date can disagree. Ordering follows that same date, so editing an
old draft does not move it up the list.

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

**Move to Trash** appears on every row, and in the editor's
Document panel. You can trash several posts at once, and the
confirmation notice carries one **Undo** that brings the whole
batch back.

In the Trash view, each row offers two actions:

- **Restore** brings the post back **as a draft**, whatever it was
  before. Publish it again to put it back on your site.
- **Delete Permanently** removes it forever, after a confirmation.

Both work one post at a time. **Empty Trash**, which only appears
here, clears everything at once.
