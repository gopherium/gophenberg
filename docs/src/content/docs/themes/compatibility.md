---
title: Theme compatibility
description: What a theme kit version promises, and what happens when a site cannot serve your theme.
---

A theme you build today should keep working when the site it runs
on is updated. This page says exactly how far that promise goes.

## The kit has its own version

Your theme is built against `@gophenberg/astro`, the theme kit. The
kit has its own version, separate from Gophenberg's. Gophenberg
%VERSION% and kit %KIT_VERSION% are different numbers on purpose.

That separation is the point. Gophenberg can change how plugins
work, how the admin looks, or how anything inside it is built,
without touching your theme. Only one thing connects a running
theme to a site, and that is the
[content API](/reference/content-api/) the kit reads through.

## What a kit version promises

The rule is ordinary semantic versioning, read from the site's
side.

**Same major, and the site not older than your theme.** A site
serving kit 2.4.0 answers a theme built on 2.0.0, because everything
that theme asks for is still there. It does not answer a theme built
on 2.6.0, which asks for things the site does not have yet.

**While the kit is at 0.x, the minor counts too.** A site serving
0.4.0 does not answer a theme built on 0.3.0. This is what 0.x means
in semantic versioning: anything may change between minors. Expect
to rebuild your theme on every kit minor until 1.0.

**From 1.0 the promise gets long.** Inside a major version the
content API only gains things. Nothing is removed, renamed, or
reshaped. A theme built on 1.0.0 is meant to keep working for as
long as 1.x is served, which is intended to be years.

Kit 1.0 ships when Gophenberg 1.0 ships, and not before.

## What a site tells you

Ask any Gophenberg site which kits it serves:

```sh
curl https://example.com/api/content/v1
```

```json
{ "gophenberg": "%VERSION%", "api": 0, "kit": ["%KIT_VERSION%"] }
```

`kit` lists every kit version that site answers. A site in the
middle of a major change lists more than one, so old themes keep
working while new ones move across.

## When a site cannot serve your theme

Two things check this, so a mismatch is always clearly refused
rather than a broken page.

**The site refuses it.** Uploading a theme built on a kit the site
does not serve is refused whole, and nothing is installed. A theme
already on disk is listed as **Broken** with the reason beside it,
and activating it is refused.

**The theme refuses itself.** Before reporting itself ready, a theme
asks the site which kits it serves. If the answer does not cover it,
the theme never starts serving, the site keeps answering with the
built-in renderer, and the logs name both versions.

The fix is the same either way: rebuild your theme against a kit the
site serves, and install it again.

## Why the version is not yours to write

You write two fields in `theme.json`:

```json
{ "name": "mytheme", "version": "0.1.0" }
```

The build adds `kit` to `dist/theme.json`, filled in with the kit
version you actually built with. Nobody has to remember to update
it, and it can never be wrong. A range like `^0.1.0` is refused,
because a site has to know which shape your theme reads rather than
which shapes you would accept.
