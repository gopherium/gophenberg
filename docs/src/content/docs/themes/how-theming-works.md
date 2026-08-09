---
title: How theming works
description: The built-in renderer, Astro themes, and what happens when a theme misbehaves.
---

Gophenberg has two ways to render your public site.

**The built-in renderer** is part of the binary. It serves
published posts with plain, readable markup: the front page lists
posts 20 at a time, each post at `/{type}/{slug}`, older posts at
`/{type}/page/{n}`. It needs nothing installed and it is what
every new deployment serves.

It is deliberately not customizable. No template overrides, no
restyling. When you want the site to look like yours, you install
a theme.

**A theme** is a small [Astro](https://astro.build) project you
build and install next to the binary. Gophenberg runs it as a
supervised Node process and forwards public requests to it.

## When a theme misbehaves

Two failures, with very different outcomes:

- **Fails to load.** A theme you chose in the admin shows as broken
  in the list, and the built-in renderer serves. A theme pinned with
  `GOPHENBERG_THEME` is stricter: the server prints the reason and
  exits, admin included.
- **Crashes while running.** The built-in renderer takes over,
  per request, and readers keep getting pages.

Two edge cases: a theme that hangs without answering holds each
request up to 10 seconds before the renderer steps in, and a theme
that answers an error page is not covered at all, because it did
answer. The safety net catches silence, not mistakes.

Switching in the admin is the safe way round. Pin a theme only
when you want it fixed by deployment, and check the site after.

## How the theme process runs

- It listens only on the machine itself, on a port Gophenberg
  picks. The outside world never reaches it directly.
- It receives exactly three environment variables: its address,
  its port, and the API address. Never the database.
- Gophenberg waits up to 30 seconds for it to report ready, and
  restarts it with growing pauses if it exits. After five starts
  that never served, it logs `theme gave up` and stops trying.
- `/api`, `/admin`, `/gophenberg`, and `/_gophenberg` never reach
  a theme.

In the logs, `mode=theme` at startup means a theme is configured,
not yet that it serves. `theme ready` means it serves.
`theme gave up` means the renderer is serving. `theme exited` also
appears for the theme you just replaced.

## Where to go next

[Writing a theme](/themes/writing-a-theme/) builds one, and
[installing a theme](/themes/installing-a-theme/) puts it in
production.
