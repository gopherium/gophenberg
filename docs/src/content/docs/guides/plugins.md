---
title: Plugins
description: Seeing which plugins the site runs, and why one is paused.
---

A plugin adds a feature to Gophenberg, such as the
[RSS feed](/reference/rss-feed/). Every plugin is built into the
`gophenberg` program, so there is nothing to upload, and each one
runs from the first start. The **Plugins** screen lists them, says
whether each one runs, and says why one stopped.

## The list of plugins

Open **Plugins** in the admin menu, right after **Themes**. It shows
to every role carrying `activate_plugins`, which `admin` carries,
see [roles](/guides/users/#roles). The screen lives at
`/admin/plugins` and is a list like the [posts list](/guides/posts/).
Under the title it says what the screen is for: *Every plugin this
site holds, and whether it runs.*

**Reading the list.** Each row shows the plugin's name, then its
Description, its Status and its Version. Status is a badge:

- **Active** for a plugin that runs.
- **Paused** for a plugin that failed while the site started. The
  next section explains it.
- **Inactive** for a plugin turned off on purpose. No screen or
  command turns a plugin off yet, so you will not meet this one for
  now.

The version is the one the plugin's `plugin.json` gives, such as
`0.0.1` for the feed. A plugin whose `plugin.json` gives no
description or no version shows an empty cell there.

The names and descriptions of the plugins Gophenberg ships, such as
Feed, show in the language you read the admin in. A plugin you build
in yourself shows the words its `plugin.json` gives.

On a phone the list shows one plugin per row with its name, its
description and its status. The version is not on the row. Turn it
on under **View options**.

**Finding a plugin.** The search box reads `Search plugins…` and
matches names and descriptions, never the error of a paused plugin.
The list opens sorted by name, from A. Click the Plugin column
title, or use **View options**, to turn the order around. **View
options** also offers the page sizes the site allows, and hides any
column but Plugin. The list has no filters, since the tabs do that
work.

The tabs under the title narrow the list by state: **All**,
**Active** and **Paused**. They show no counts. All lists every
plugin. Active and Paused list only the plugins in that state, so a
paused plugin never shows under Active. The Status column shows only
on the All tab, since every other tab holds one state. Picking a tab
goes back to the first page and clears the search, and keeps the
sort and the page size.

The tab, the page, the page size, the sort and the search all live
in the address of the screen, such as `/admin/plugins?status=paused`
for the Paused tab. Reload it or share the link and the list opens
the same way.

**When the list is empty.** A tab with no plugin in its state says
so, such as *No paused plugins* on a site where every plugin runs. A
search that matches nothing says `No results found`. A site built
with no plugin at all says *No plugins yet*. When the server cannot
answer, the screen says *The plugins could not be loaded.* in place
of the list.

No row has a menu or a tick box yet. You cannot turn a plugin off
or on, add one or delete one from the admin. To add or remove a
plugin, build the `gophenberg` program with or without it, as
[write a plugin](/extending/write-a-plugin/) shows.

## A paused plugin

When a plugin fails while the site starts, the site pauses that
plugin and serves everything else. While it is paused, its addresses
answer 404 and its roles are left out, see
[a plugin that fails at start](/self-hosting/configuration/#a-plugin-that-fails-at-start).

On the Plugins screen its badge reads **Paused**, and a line under
its name says at which step it failed, then gives the plugin's own
error. For the feed paused by a post count that is not a whole
number, the line reads:

> It failed while the site set it up. Error message:
> `GOPHENBERG_PLUGIN_FEED__ITEMS: must be a whole number, got "banana"`

Each step has a sentence of its own, then *Error message:* and the
error:

| Step | The line under the name starts with |
| --- | --- |
| `register` | It failed while the site set it up. |
| `roles` | It failed while it declared its roles. |
| `types` | It failed while it declared its content types. |
| `hooks` | It failed while the site checked its hooks. |
| `routes` | It failed while it handed over its web addresses. |
| `migrate` | It failed while it updated its database tables. |
| `start` | It failed while it started. |
| Any other step, or none | It failed. |

The sentence shows in the language you read the admin in. The error
after it stays as the plugin wrote it, in English. The server log
writes the same error with the plugin and the step in front, such as
`pluginkit: feed register:`, so you can search the log for it.

**Bringing it back.** The screen only shows the pause. Fix what the
error names, then restart the server. Every start tries a paused
plugin again, and once it starts, its row reads **Active** with
nothing under its name. Until that restart the row still reads
**Paused**, even after you fixed the cause, because the running site
still holds the plugin paused.

Here is the feed, from a bad value to a fix:

1. Whoever runs the server sets `GOPHENBERG_PLUGIN_FEED__ITEMS=banana`
   in `.env` and restarts it. The site starts and serves, but
   `banana` is not a whole number, so the feed is paused at its
   `register` step and its address answers 404.
2. You reload **Plugins**. The Feed row reads **Paused**, with the
   line above under its name. The **Paused** tab now holds the feed,
   and **Active** says *No active plugins*.
3. Whoever runs the server removes the line, or sets a whole number,
   and restarts it. The Feed row reads **Active** again.

[`gophenberg check`](/self-hosting/commands/) catches a bad plugin
setting before the server pauses the plugin, so run it before every
rollout.
