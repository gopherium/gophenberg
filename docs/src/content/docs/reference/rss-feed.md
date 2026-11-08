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
| `GOPHENBERG_FEED_TITLE` | `Gophenberg` | The channel title feed readers display |
| `GOPHENBERG_FEED_ITEMS` | `20` | How many posts the feed carries |

`GOPHENBERG_FEED_ITEMS` must be a positive whole number. With
anything else, the server pauses the feed when it starts and serves
the rest of the site. The feed's address answers 404 until you fix
the value and restart the server, see
[a plugin that fails at start](/self-hosting/configuration/#a-plugin-that-fails-at-start).

`check`, `migrate` and `seed -yes` still fail on a bad value. With
`banana` as the value, `check` prints:

```text
gophenberg: pluginkit: feed register: GOPHENBERG_FEED_ITEMS: must be a whole number, got "banana"
```

The command listing still works. It shows the feed under
**Not loaded**, with the same reason. Run
[`gophenberg check`](/self-hosting/commands/) before a rollout to
catch a bad value before the server pauses the feed.
