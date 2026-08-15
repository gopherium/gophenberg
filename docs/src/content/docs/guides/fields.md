---
title: Fields
description: Giving a content type its own typed fields, including relations between types.
---

A field is one extra piece of information every item of a type
carries. A car can have a price, a recipe a cooking time, a post a
list of categories. This page shows how to declare fields, fill them
in, and connect one type to another.

Fields belong to a type, not to a single item. Declare a Price field
on Cars and every car gets a Price box in the editor.

## Declaring a field

Open **Content Types** in the admin menu and press **Fields** on a
type. The view lists what the type declares and lets you add more.

Give the field a name and pick a kind. The name is what editors see.
Gophenberg derives the key from it, so a field named Sold On is
stored under `sold-on`. The key and the kind are fixed once the
field exists. **Rename** changes only the name, which is free and
touches nothing stored.

## The kinds

| Kind | Holds | Example |
| --- | --- | --- |
| Text | a line of text | a subtitle |
| Number | a number | a price |
| Yes or no | a check | in stock |
| Date | a day | sold on |
| Media | one item from the media library | a cover image |
| Relation | links to items of another type | the categories of a post |

## Filling fields in

The editor's Document panel shows one control per declared field,
under the excerpt. Values save with the item, autosave with it, and
travel with revisions, so restoring an old revision also restores
the values it held.

A field can be marked **required**. A required field never blocks
writing: drafts save freely with it empty. It blocks publishing, so
an item goes public only once the field holds something.

## Relations connect types

A relation field points items of one type at items of another.
Categories work this way: a hierarchical Categories type, plus a
relation field on Posts pointing at it. There is no separate
category system to learn, a category is ordinary content, so it can
carry its own body, fields and picture.

A relation field declares which type it points at, and whether an
item holds one target or many. The editor then offers a picker
listing the items of that type.

A type whose items should list what points at them declares the
archive page kind. Such an item serves a term page: its own title
and body first, then the published content pointing at it, newest
first, paginated. The seeded News category is one to look at.

## Deleting a field

**Delete** removes the field and everything stored under it, in
every item of the type and in the revisions behind them. The dialog
says so before it happens. This is what keeps old values from
coming back if you later declare a new field under the same key.
