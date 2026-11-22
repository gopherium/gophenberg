---
title: RSS feed
description: The feed address, what it contains, and the two settings that shape it.
---

Every Gophenberg site publishes an RSS feed of its posts, served
by a built-in plugin.

## The address

```text
https://example.com/api/plugins/feed/rss.xml
```

It needs no login. That is the address to hand to feed readers, or
to a newsletter service that syndicates by feed.

The feed also links that address from the head of every page the
built-in site draws, titled with the site's name, so a browser or a
feed reader finds it on its own. A paused feed links nothing.

## What it contains

The newest published posts, most recent first. Each item carries
the post's title, its address on your site, its publication date,
and the **full post content**, not the excerpt.

Item links are built from the address the request arrived on, so
they only come out as `https` when
[`GOPHENBERG_TRUSTED_PROXIES`](/self-hosting/configuration/#trusted-proxies)
is set correctly for your proxy.

## Settings

| Variable | Default | Effect |
| --- | --- | --- |
| `GOPHENBERG_PLUGIN_FEED__TITLE` | `Gophenberg` | The channel title feed readers display |
| `GOPHENBERG_PLUGIN_FEED__ITEMS` | `20` | How many posts the feed carries |

Both are the feed's
[plugin settings](/self-hosting/configuration/#plugin-settings), so an
empty variable keeps the default. Before, they were
`GOPHENBERG_FEED_TITLE` and `GOPHENBERG_FEED_ITEMS`. Those names no
longer apply, so a site that still sets them gets the default title
and 20 posts until you rename them.

Unless it is empty, `GOPHENBERG_PLUGIN_FEED__ITEMS` must be a
positive whole number. With any other value, the server pauses the
feed when it starts and serves the rest of the site. The feed's
address answers 404 until you fix the value and restart the server,
see
[a plugin that fails at start](/self-hosting/configuration/#a-plugin-that-fails-at-start).
Until then the admin's [Plugins screen](/guides/plugins/) shows the
feed as Paused, with the error about the value under its name.

`check`, `migrate` and `seed -yes` still fail on a bad value. With
`banana` as the value, `check` prints:

```text
gophenberg: pluginkit: feed register: GOPHENBERG_PLUGIN_FEED__ITEMS: must be a whole number, got "banana"
```

The command listing still works. It shows the feed under
**Not loaded**, with the same reason. Run
[`gophenberg check`](/self-hosting/commands/) before a rollout to
catch a bad value before the server pauses the feed.
