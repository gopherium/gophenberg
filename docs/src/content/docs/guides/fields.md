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

The picker offers seventeen entries storing nine kinds of value, so
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
| Fields of its own | Section |
| Rows of fields of its own | Repeater |

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
take answers outside its list. A Many values box, a Radio group and
a Checkbox group each grow an Other box for typing one. A Select or
a Button group offers no way to type, so pick another presentation
when you need one. **Allow empty** adds a None entry that clears the
field.

**Longest**, **Lowest** and **Highest** are limits. Longest stops
your typing. A number outside Lowest or Highest, and an answer a
choice does not list, are named under the control as you fill the
item in, so you can mend them where you wrote them. An address that
does not read as one waits for the save. All three are refused when
you save, drafts included, and autosave keeps the value meanwhile
so you fix it rather than lose it. Tightening any of these leaves
stored values alone, and is refused while the field's own Default
falls outside.

**Steps of** moves a Range field's slider one notch. A Range needs
both Lowest and Highest before it draws a slider, and is a plain
number box without them, where Steps of reaches nothing.

**In list** puts the field in the content list as a column of its
own. Press it on a text, number, switch, date or choice field at the
top of a group. A switch or a choice field also gets a filter above
the table, so you can narrow the list to the items holding one value.
See [the content list](/guides/posts/).

## Fields inside fields

A **Section** bundles fields under one name. An Author section might
hold a Name, a Photo and a Bio, kept together in the editor and
stored together on the item.

A **Repeater** holds rows of fields. A Team repeater whose row has a
Name and a Role lets an author add a row per person, in any order,
with as many or as few as the work needs.

Press **Add field** on a section or a repeater to declare a field
inside it. The field is listed under its container, and every setting
its kind takes works there exactly as it does at the top. Rename it,
require it, settle it and move it up or down in the same way. Two
containers may each hold a field of the same name, because a field is
named inside the container that holds it.

The one control a field inside a container does not offer is the move
to another group, because a field there belongs to its container
rather than to the group directly.

A container may hold another container, up to 32 levels deep. A
Repeater row can hold a Section, and that Section can hold a Repeater
of its own. Deeper than 32 the field is refused, which is far more
nesting than a page needs. Relations are the one kind that stands
outside, so declare a Relation beside a container rather than in it.

Deleting a field inside a container takes the values stored under it,
in every item the group reaches and in the revisions behind them,
exactly as deleting a field at the top does.

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

## Showing a field by rule

A field can wait until another field says so. Press **Rules** beside
it and add a condition: pick a field, pick a comparison, pick or type
a value. The field then shows only while the rule holds, and the
editor hides it the moment you change what it reads.

Add more conditions to one set and every one of them has to hold. Add
a second set and the field shows when either set holds.

A rule reads the fields standing beside it. A field at the top of a
group reads the group's other fields, and a field inside a section or
a repeater row reads the others in that same row, row by row. A rule
cannot read a field in another group, and it cannot lead back to
itself.

What a hidden field already holds stays where it is. Turn the switch
back on and the value is still there. While the field is hidden its
value is not required to publish, and it never reaches visitors.

A field inside a section or a repeater row keeps its value the same
way, with one difference worth knowing. A row is saved whole, so the
editor keeps sending what a hidden field in that row holds. Nothing
is lost when you delete the row above it or drag it somewhere else.

Removing a field another field's rules read is refused, and so is
moving it to another group. Change those rules first.

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

## Moving definitions between sites

**Export definitions** downloads a file holding every content type
and field group the site made, with the fields inside them. It
carries no content and no plugin definitions, only the shapes your
site defined.

**Import definitions** reads that file back. It never applies
straight away. It first shows you what would change, one line per
type, group and field, split into what it would add, what it would
change, and what it would take away.

Anything that would take a definition away carries a tick box, and
nothing is removed unless you tick it. That includes changes that
look small: changing a field's kind, or moving a field to another
group, both mean losing what is stored under it, so both ask.

Two things an import never does. It never hands the site's root to
another content type, because that changes every stored address.
And it never touches a definition a plugin declared.

## Fields a plugin brought

A plugin can declare its own types, groups and fields. They appear
here with a badge naming the plugin, and they are read only: the
plugin's code is where they change.

Two things can happen to them, and both show up as a notice on this
screen:

- **The plugin stopped declaring one.** The definition stays, and
  the notice offers **Adopt**, which hands it to your site. After
  that it edits and deletes like anything you made yourself.
- **A plugin wants a key you already use.** Your own definition
  keeps it and the plugin's is skipped. The notice names the plugin
  so you can decide whether to rename yours.
