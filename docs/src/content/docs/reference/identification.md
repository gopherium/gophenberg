---
title: Identification signals
description: How a Gophenberg site identifies the software it runs, and exactly what it discloses.
---

Every Gophenberg site identifies the software serving it, the same
way most content management systems do. This page states exactly
what is disclosed so you are not surprised by it.

## The three signals

**Response headers.** Every public page answers with:

```text
X-Generator: Gophenberg %FEATURE_VERSION%
Link: </api/content/v1>; rel="https://gophenberg.org/api"
```

**A generator tag** in the page's head:

```html
<meta name="generator" content="Gophenberg %FEATURE_VERSION%">
```

**Asset addresses.** Pages load their block styles from
`/gophenberg/blocks.css` and `/gophenberg/presets.css`, paths that
name the software.

## What is and is not disclosed

The version is always major.minor, never the patch level, so a
site does not advertise exactly which build it runs. The signals
appear on the public site only: `/api` and `/admin` responses
carry none of them.

On themed sites, the headers are always present, while the
generator tag and the stylesheet addresses come from the
`GophenbergHead` component in the theme's base layout. A theme
that omits it drops those two signals, and its block content loses
the styling the sheets provide, which is why every theme should
render it.

There is no setting to turn the signals off.
