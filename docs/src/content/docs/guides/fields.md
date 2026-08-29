---
title: Fields
description: Grouping typed fields and choosing which content they appear on, including relations between types.
---

A field is one extra piece of information every item of a type
carries. A car can have a price, a recipe a cooking time, a post a
list of categories. This page shows how to declare fields, place
them, set them up, fill them in, and connect one type to another.

Fields live in groups. Declare a Price field in a group placed on
Cars, and every car gets a Price box in the editor.

## Field groups

Open **Field Groups** in the admin menu. Every group is listed with
its name, where it appears, how many fields it holds, and whether it
is active.

Press **Add New Group** and give it a name. A new group starts
placed on one content type, which you can change straight away.

A group can be set aside without deleting it. **Deactivate** stops
it appearing anywhere and leaves its fields and their stored values
untouched. **Activate** brings it back.

## Where a group appears

Press **Rules** on a group to say which content it appears on.

A rule reads like *Content type is Posts*, and the content type is
the only source today.

Rules are arranged in sets. Every condition in a set has to be true
at once, so **Add condition** narrows a set, and any one set being
true is enough, so **Add rule set** widens it. The groups list shows
the result as a sentence, such as *Posts or Recipes*.

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

Registering a new content type can make two groups meet on a key.
The first group in the list serves the field, the other holds it
unseen, and the Field Groups screen marks the loser **Shadowed**.
Change one group's rules so the two stop meeting, or move the field
into a group that appears elsewhere. Renaming does not settle it,
because the key stays as it was.

## The kinds

| Kind | Holds | Settings |
| --- | --- | --- |
| Text | a line of text | Instructions, Default, Placeholder, Longest |
| Number | a number | Instructions, Default, Lowest, Highest, Steps of |
| Yes or no | a check | Instructions, Default |
| Date | a day | Instructions |
| Media | one item from the media library | Instructions |
| Relation | links to items of another type | Instructions |

## The settings

Press **Settings** on a field to tune its control, as the table
above lists. **Instructions** is the help line under the control.

**Default** fills the control in as you press Add New, and only
then. It never reaches an item that already exists.

**Longest**, **Lowest** and **Highest** are limits. Longest stops
your typing. A number outside Lowest or Highest is refused when you
save, drafts included, and autosave keeps it meanwhile so you fix
the value rather than lose it. Tightening a limit leaves stored
values alone, and is refused while the field's own Default falls
outside it.

**Steps of** is stored for a theme to read and reaches no control.

## Moving a field

**Move** carries a field into another group, keeping every value
stored under it. The button appears only when there is another
group to move into.

## Filling fields in

The editor's Document panel shows one control per field reaching the
item, under the excerpt. Fields from every group placed on that
content appear together, in group order. Values save with the item
and travel with revisions, so restoring an old revision also
restores the values it held.

Autosave does not keep relations, which are stored apart from the
item. Press **Save draft** after changing what an item points at.

A field can be marked **required**, as you declare it or with
**Require** afterwards. It never blocks a draft save. It blocks
publishing, so an item goes public only once the field holds
something.

## Relations connect types

A relation field points items of one type at items of another.
Categories work this way: a hierarchical Categories type, plus a
relation field in a group placed on Posts pointing at it. A category
is ordinary content, so it carries its own body, fields and picture.

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
