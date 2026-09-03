---
title: Site settings
description: The page size and the picture quality the whole site follows.
---

Two settings decide how the site serves what you publish. Both live
on the **Settings** screen, which only an admin reaches.

## Posts per page

**Posts per page** is how many items a public listing carries. It
counts for the front page, for every archive, and for the listings
the [content API](/reference/content-api/) answers. It starts at 20
and takes a whole number from 1 to 100. A number outside that range
is refused, and the screen says why.

This is the site's default rather than the last word. A theme that
names its own page size uses that instead, and a program reading
the content API can ask for a different number on the request, up
to the same 100.

Two lists do not follow it. The list of posts you work in inside the
admin stays at 20 a page whatever you choose here. The
[RSS feed](/reference/rss-feed/) carries its own number, which
whoever runs the server sets.

## Picture quality

**Picture quality** is how carefully the smaller copies of a
picture are saved. It starts at 82 and takes a whole number from 1
to 100. A lower number makes lighter files that look rougher, and a
higher number makes heavier files that look better. A number
outside the range is refused the same way.

It reaches only the copies written as JPEG, and only pictures
uploaded after you change it. A picture sent as PNG or as a still
GIF gets PNG copies, which ignore the number entirely, and an
animated GIF is stored as it arrived with no copies at all. Copies
already on disk
stay as they were, so lowering the number to save room frees
nothing until new pictures arrive. A JPEG that arrives sideways is
turned upright and stored again at the quality you chose, which
[media](/guides/media/) explains.

## When a change reaches readers

The admin shows the new number as soon as you save it. Readers may
wait a little longer.

Gophenberg tells a shared cache it may hold a content API answer
for a minute, and may serve that old answer for five minutes more
while it fetches a fresh one. Both windows are
[configurable](/self-hosting/configuration/). A theme reads the
site through that API, so a theme's listings can lag by that much.

The pages Gophenberg renders itself carry no caching instruction at
all, so whatever you put in front of them follows its own rules.

## The language the site answers in

That one is not here. It sits on the Language screen, under **The
site default**.
