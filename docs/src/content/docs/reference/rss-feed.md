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

`GOPHENBERG_FEED_ITEMS` must be a positive whole number. Anything
else stops the server at startup.
