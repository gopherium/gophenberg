---
title: Licensing
description: Apache-2.0 source, GPL-conveyed bundles, and what that means for your themes and plugins.
---

Gophenberg's licensing has two layers, and the distinction is the
whole page.

## The source is Apache-2.0

The repository is Apache-2.0, and hand-written source files carry
an SPDX header saying so. That covers the Go backend, the admin
frontend sources, the plugin SDK, and the theme kit. Generated
files, such as the database access layer, are unmarked but sit
under the same license. Copyright is held by Manuel "SirLouen"
Camargo.

## Built bundles are conveyed under the GPL

The admin frontend and the theme kit build on WordPress packages,
such as the Gutenberg editor, which are licensed GPL-2.0-or-later.
A built artifact that bundles them, the compiled admin in the
container image or a built theme, combines Apache-2.0 code with
GPL code.

The combination is lawful because GPL-2.0-or-later allows applying
GPL-3.0, and Apache-2.0 is compatible with GPL-3.0. The practical
consequence: **built bundles are conveyed under GPLv3 terms**,
while every original source file stays Apache-2.0.

## What this means for your code

- **A plugin's Go code** is written against the Apache-2.0 SDK
  and compiled with permissively licensed code. You choose its
  license. If your plugin also ships an admin screen, that built
  frontend bundles the same WordPress packages the admin does, so
  the same GPL conveyance applies to that artifact.
- **A theme's source** is yours to license as you wish. Its built
  artifact bundles the GPL block parser, so distributing the
  artifact carries GPL obligations. Distributing the source does
  not.
- **Your content** is yours. Nothing about the software's license
  touches what you write with it.

This page describes the mechanics, not legal advice for your
situation.
