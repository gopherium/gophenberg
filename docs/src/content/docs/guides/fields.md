---
title: Fields
description: Grouping typed fields and choosing which content they appear on, including relations between types.
---

A field is one extra piece of information every item of a type
carries. A car can have a price, a recipe a cooking time. This page
shows how to declare fields, place them, set them up, fill them in,
and connect one type to another.

Fields live in groups. Declare a Price field in a group placed on
Cars, and every car gets a Price box in the editor.

## Field groups

Open **Field Groups** in the admin menu.

Press **Add New Group** and give it a name. A new group starts
placed on one content type.

A group can be set aside without deleting it. **Deactivate** stops
it appearing anywhere and leaves its fields and their stored values
untouched. **Activate** brings it back.

## Where a group appears

Press **Rules** on a group to say which content it appears on.

A rule reads like *Content type is Posts*, and the content type is
the only source today.

Rules are arranged in sets. Every condition in a set has to be true
at once, so **Add condition** narrows a set, and any one set being
true is enough, so **Add rule set** widens it.

A group with no rules at all appears nowhere, and the list says
**Nowhere** beside it.

## Declaring a field

Press **Fields** on a group, then give the field a name and pick a
kind. The name is what editors see, and Gophenberg derives the key
from it, so a field named Sold On is stored under `sold-on`. The
key and the entry you picked are fixed once the field exists, so an
Email field stays an email box. **Rename** changes only the name,
which is free and touches nothing stored.

Two fields cannot share a key on the same content. If another group
reaching the same items already holds that key, the save is refused
and says so.

A new content type can make two groups meet on a key. The first
group serves the field, the other is marked **Shadowed**, and
changing one group's rules parts them.

**Move** carries a field into another group, keeping every value
stored under it.

## The kinds

The picker offers fifteen entries storing seven kinds of value, so
all four choice entries are listed as Choice.

| Stores | Pick |
| --- | --- |
| A line of text | Text, Text area, Email, Web address |
| A number | Number, Range |
| A check | Yes or no |
| A day | Date |
| One of a list | Select, Radio group, Checkbox group, Button group |
| Items from the media library | Media, Gallery |
| Links to items of another type | Relation |

**Email** and **Web address** are checked when you save, and a web
address has to start with `http://` or `https://`. **Range** draws
a slider. **Gallery** is a Media field holding several items.

## The settings

Press **Settings** on a field to tune its control. **Instructions**
is the help line under the control.

**Default** fills the control in as you press Add New, and never
reaches an item that already exists.

A choice field lists its answers under **Choices**. Each answer has
a **Value**, which is what gets stored, and a **Label**, which is
what the editor reads. A field listing no answers takes whatever is
typed, so add them first. **Many values** lets an item hold several
answers, in a box you add them into. **Allow custom** lets the field
take answers outside its list, which only a Many values box offers a
way to type. **Allow empty** adds a None entry that clears the
field.

**Longest**, **Lowest** and **Highest** are limits. Longest stops
your typing. Anything else the field will not take is refused when
you save, drafts included, and autosave keeps it meanwhile so you
fix the value rather than lose it. That covers a number outside
Lowest or Highest, an address that does not read as one, and an
answer a choice does not list. The editor also names a number
outside Lowest or Highest under its own control as you fill the
item in, so you can mend it before saving rather than after.
Tightening any of these leaves stored values alone, and is refused
while the field's own Default falls outside.

**Steps of** moves a Range field's slider one notch. A Range needs
both Lowest and Highest before it draws a slider, and is a plain
number box without them, where Steps of reaches nothing.

## Filling fields in

The editor's Document panel shows one control per field reaching the
item, under the excerpt, in group order. Values save with the item
and travel with revisions, so restoring an old revision also
restores the values it held.

A **Media** field holds one item and a **Gallery** holds several,
both picked from the [media library](/guides/media/). A Gallery
lists what it holds, and ignores an item it already has.

Autosave does not keep relations, which are stored apart from the
item. Press **Save draft** after changing what an item points at.

A field can be marked **required**, as you declare it or with
**Require** afterwards. It never blocks a draft save, only
publishing, so an item goes public only once the field holds
something.

## Relations connect types

A relation field points items of one type at items of another.
Categories work this way: a hierarchical Categories type, plus a
relation field in a group placed on Posts pointing at it.

A relation field declares which type it points at under **Points
at**, and whether an item holds one target or many under **Holds**.
The editor then offers a picker listing the items of that type. A
type whose items should list what points at them declares the
archive page kind, covered in
[content types](/guides/content-types/).

## Deleting a field

**Delete** removes the field and everything stored under it, in
every item the group reaches and in the revisions behind them. The
dialog says so before it happens.

Deleting a whole group takes its fields with it, and their stored
values too. If you only want the group to stop appearing, deactivate
it instead.
