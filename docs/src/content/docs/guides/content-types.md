---
title: Content types
description: Registering kinds of content and choosing the addresses they answer under.
---

A content type is one kind of content your site holds. Every site
starts with Posts. This page shows how to add more kinds, like Pages
or Guides, and how each kind gets its own addresses.

Open **Content Types** in the admin menu. The table lists every
registered type, the address it answers under, and what may be done
to it.

## Registering a type

Press **Add New Type** and give it a singular and a plural name, like
Guide and Guides. Gophenberg derives the rest: the plural becomes the
route word, so Guides answer under `/guides`.

The new type appears in the admin menu right away, with the same list
screen and editor Posts have.

## Addresses and the route word

The route word is the first part of every address a type serves. A
guide with the slug `first-steps` answers at `/guides/first-steps`.

One type owns the root of the site instead of a route word. It is
marked **Default** in the table, and its items answer at `/{slug}`
directly. Press **Make default** on another type to hand the root
over.

**Change address** moves a type to a new route word. Every address of
that type moves with it, at once, including nested items. Links kept
outside your site to the old addresses stop working, which is why the
dialog asks you to confirm.

## Nesting

A type marked **Nests** can hold items inside items. The editor's
Document panel then offers a **Parent** select, and a child's address
carries the chain of its parents: `/pages/about/team` is the page
`team` inside `about`.

Nesting goes at most ten levels deep. An item holding children cannot
be trashed until its children move or go first.

## Turning a type off

**Deactivate** hides a type without deleting anything. Its menu entry,
its archive, and its items stop being served, and everything comes
back with **Activate**.

**Delete** removes a type for good, and is refused while the type
still holds content. The default type can be neither deactivated nor
deleted, so the root always answers.
