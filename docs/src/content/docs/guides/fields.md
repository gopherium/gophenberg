---
title: Fields
description: Grouping typed fields and choosing which content they appear on, including relations between types.
---

A field is one extra piece of information every item of a type
carries. A car can have a price, a recipe a cooking time, a post a
list of categories. This page shows how to declare fields, choose
where they appear, fill them in, and connect one type to another.

Fields live in groups. A group holds a bundle of fields and carries
rules saying which content it appears on. Declare a Price field in a
group placed on Cars, and every car gets a Price box in the editor.

## Field groups

Open **Field Groups** in the admin menu. Every group is listed with
its name, where it appears, how many fields it holds, and whether it
is active.

Press **Add New Group** and give it a name. A new group starts
placed on one content type, so it is never born appearing nowhere.
You can change that straight away.

A group can be set aside without deleting it. **Deactivate** stops
it appearing anywhere and leaves its fields and their stored values
untouched. **Activate** brings it back.

## Where a group appears

Press **Rules** on a group to say which content it appears on.

A rule is three parts read left to right: a source, a condition and
a value. The only source today is the content type, so a rule reads
like *Content type is Posts*.

Rules are arranged in sets. Every condition inside one set has to be
true at the same time, so **Add condition** narrows a set. Any one
set being true is enough, so **Add rule set** widens the placement.
The groups list shows the result as a sentence, such as *Posts or
Recipes*.

A group with no rules at all appears nowhere. The list says
**Nowhere** beside it, and its fields reach no content.

## Declaring a field

Press **Fields** on a group. The view lists what the group holds and
lets you add more.

Give the field a name and pick a kind. The name is what editors see.
Gophenberg derives the key from it, so a field named Sold On is
stored under `sold-on`. The key and the kind are fixed once the
field exists. **Rename** changes only the name, which is free and
touches nothing stored.

Two fields cannot share a key on the same content. If another group
reaching the same items already holds that key, the save is refused
and says so.

Registering a new content type can still make two groups meet on a
key. When that happens, the first group in the list serves the field
and the other holds it without showing it. The Field Groups screen
warns you, naming the key and both groups, and marks the group that
lost with a **Shadowed** badge. Move or rename one of the fields to
settle it.

## The kinds

| Kind | Holds | Example |
| --- | --- | --- |
| Text | a line of text | a subtitle |
| Number | a number | a price |
| Yes or no | a check | in stock |
| Date | a day | sold on |
| Media | one item from the media library | a cover image |
| Relation | links to items of another type | the categories of a post |

## Moving a field

**Move** carries a field into another group, keeping every value
stored under it. Use it when a field turns out to belong with a
different bundle, or should reach different content.

The button appears only when there is another group to move into.

## Filling fields in

The editor's Document panel shows one control per field reaching the
item, under the excerpt. Fields from every group placed on that
content appear together, in group order. Values save with the item
and travel with revisions, so restoring an old revision also
restores the values it held.

Autosave keeps the fields listed in the table above. It does not
keep relations, which are stored apart from the item. Press **Save
draft** after changing what an item points at.

A field can be marked **required**, either as you declare it or with
**Require** afterwards. A required field never blocks writing:
drafts save freely with it empty. It blocks publishing, so an item
goes public only once the field holds something.

## Relations connect types

A relation field points items of one type at items of another.
Categories work this way: a hierarchical Categories type, plus a
relation field in a group placed on Posts pointing at it. There is
no separate category system to learn, a category is ordinary
content, so it can carry its own body, fields and picture.

A relation field declares which type it points at under **Points
at**, and whether an item holds one target or many under **Holds**.
The editor then offers a picker listing the items of that type.

A type whose items should list what points at them declares the
archive page kind. Such an item serves a term page: its own title
and body first, then the published content pointing at it, newest
first, paginated. The seeded News category is one to look at.

## Deleting a field

**Delete** removes the field and everything stored under it, in
every item the group reaches and in the revisions behind them. The
dialog says so before it happens. This is what keeps old values from
coming back if you later declare a new field under the same key.

Deleting a whole group takes its fields with it, and their stored
values too. If you only want the group to stop appearing, deactivate
it instead.
