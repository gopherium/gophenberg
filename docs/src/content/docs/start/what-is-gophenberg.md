---
title: What is Gophenberg
description: A self-hosted CMS with the Gutenberg editor, a Go backend, and an Astro theme system.
---

Gophenberg is a content management system you run on your own
server. You write posts in the Gutenberg block editor, the same
editor WordPress uses, and Gophenberg publishes them on your site.

It is one Go binary and one PostgreSQL database, serving three
things:

| Path | What lives there |
| --- | --- |
| `/` | Your public site, showing published posts |
| `/admin` | The admin, where you write and manage content |
| `/api` | The JSON API behind both |

## What makes it different

**A crashed theme does not hide your content.** A theme is a
separate Astro project running beside the binary. If it stops
answering, a built-in renderer serves your posts instead. (A theme
that fails to load at startup is a different story, told in
[how theming works](/themes/how-theming-works/).)

**Plugins are compiled in.** A plugin is a Go package built into
the binary, with its own routes and its own database tables. The
first built-in plugin serves your posts as an RSS feed.

**The editor is real Gutenberg.** Not a lookalike. Autosave,
revision snapshots kept behind the scenes, and a trash you can
undo.

## What ships today

Gophenberg is below 1.0 and honest about it. Working today: posts
with statuses, slugs, and excerpts, autosave and recovery, trash
and restore, a media library, content types with their own
addresses, typed fields, user accounts, themes, plugins, a public
content API, and an RSS feed.

## Where to go next

- [Run it on your machine](/start/local-development/) in a few
  minutes.
- [Put it on a server](/self-hosting/install/) with Docker
  Compose.
- [Change how your site looks](/themes/how-theming-works/) with a
  theme.
