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

The picker offers twenty one entries. Twenty of them store eleven
kinds of value, so all four choice entries are listed as Choice.

| Stores | Pick |
| --- | --- |
| A line of text | Text, Text area, Email, Web address, Color |
| A number | Number, Range |
| A check | Yes or no |
| A day | Date |
| One of a list | Select, Radio group, Checkbox group, Button group |
| Items from the media library | Media, Gallery |
| An address on this site or away from it | Link |
| Links to items of another type | Relation |
| Fields of its own | Section |
| Rows of fields of its own | Repeater |
| Rows that each pick a shape | Flexible content |

**Email** and **Web address** are checked when you save, and a web
address has to start with `http://` or `https://`. **Color** stores
a hash and six digits, or eight when the one you pick is partly
see through, and offers a color control rather than a text box.
**Range** draws a slider. **Gallery** is a Media field holding
several items.

**Link** holds three things at once: the address, the words it is
read under, and whether it opens in a new tab. The address may be a
web address, an email one written as `mailto:someone@example.com`,
or a path on this site beginning with a slash. A path beginning
with two slashes, or with a slash and a backslash, is refused, since
a browser opens it on another site. Anything else is refused when
you save. The field is empty until you write an address, so a
required Link blocks publishing until it points somewhere.

There is no icon field, and a Media field holding an SVG does the
same job while keeping every icon in one library.

The twenty first entry, **Linked from**, stores nothing of its own.
It reads a relation and lists the items pointing this way. Reading
a relation backwards, below, says how.

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

**Fewest files** and **Most files** bound a Gallery. They count
files, not letters, and they are checked when you save, drafts
included.

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

A **Flexible content** field also holds rows, but each row picks one
of the shapes the field offers, and those shapes are called layouts.
A Features field offering a Hero layout and a Quote layout lets an
author build a page as a hero, then a quote, then another hero, in
whatever order the page needs. Each row carries only the fields of
the layout it picked.

Press **Add field** on a section or a repeater to declare a field
inside it. The field is listed under its container, and every setting
its kind takes works there exactly as it does at the top. Rename it,
require it, settle it and move it up or down in the same way. Two
containers may each hold a field of the same name, because a field is
named inside the container that holds it.

A flexible content field takes layouts rather than fields, so it
offers **Add layout** instead. Press it to name a layout, then press
**Add field** on that layout to declare what its rows carry. Two
layouts may each hold a field of the same name for the same reason
two containers may. A layout stands nowhere but inside a flexible
content field, and a field stands nowhere directly under one.

**Fewest rows** and **Most rows** bound how many rows a repeater or a
flexible content field takes. Set them on a layout instead and they
bound how many rows may pick that one layout, counted across the
whole field.

Deleting a layout takes its rows with it, in every item the group
reaches and in the revisions behind them, because a row without its
layout carries nothing anyone can read. Going back to an earlier
revision does not bring those rows back.

The one control a field inside a container does not offer is the move
to another group, because a field there belongs to its container
rather than to the group directly.

A container may hold another container, up to 32 levels deep. A
Repeater row can hold a Section, and that Section can hold a Repeater
of its own. Deeper than 32 the field is refused, which is far more
nesting than a page needs. A Relation stands inside one as happily
as a text field does, so a Repeater row can point at an item of its
own. Only a Linked from field stands outside, so declare that one
beside the container rather than in it.

Deleting a field inside a container takes the values stored under it,
in every item the group reaches and in the revisions behind them,
exactly as deleting a field at the top does.

A theme reads these the same way it reads any field. The
[theme guide](/themes/writing-a-theme/) shows the helpers that walk
a section and a repeater's rows.

## Filling fields in

The editor's Document panel shows one control per field reaching the
item, under the excerpt. The plain controls come first, then
Sections, Repeaters and Flexible content fields, then the relation
and media pickers, and last the Linked from lists. Each of those
sets follows group order. Values save with the item and travel with
revisions, so restoring an old revision also restores the values it
held.

A **Media** field holds one item and a **Gallery** holds several,
both picked from the [media library](/guides/media/). A Gallery
shows each file it holds by its picture and its name, and ignores an
item it already has. Pick several files in one visit to the library
and they all arrive together. **Move up** and **Move down** order
them, and that order is what a theme reads. A file taken out of the
library shows by its number so you can still remove it.

Autosave keeps what an item points at, the same as every other
value it holds. An item that was deleted stays in the picker of
whatever pointed at it, so you can take it out, and a reader of the
site is never shown it.

A field can be marked **required**, as you declare it or with
**Require** afterwards. It never blocks a draft save, only
publishing, so an item goes public only once the field holds
something. A Linked from field and a layout hold no value of their
own, so neither can be required.

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

Not every kind can be read. A rule reads a text, number, switch,
date, choice, media or link field, a link only for whether it is
filled. A relation, a container and a **Linked from** field can
each be shown by a rule but never read by one, so they are not
offered when you pick what a rule reads.

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

## Reading a relation backwards

A relation points one way. **Linked from** reads it the other way,
and lists every item whose relation points at the one you are
editing. On a category it answers the question the relation cannot,
which is which posts were filed here.

Declaring one asks two things. **Reads from** is the group holding
the relation, and **Through** is the relation in it. Only groups
holding a relation are offered, so declare the relation first.

The Linked from field then has to sit in a group placed on the type
that relation points at. Categories work this way. The relation
sits on Posts and points at Categories, so the Linked from field
sits on Categories.

The list shows the newest published items, as many as the site's
**Posts per page** setting allows, and says how many point in all
when more do than it shows. Nothing in the editor asks for the ones
behind them. Only published items of active types are
listed, so a draft pointing this way waits until it is published.
Each entry links to its own editor.

Nobody fills a Linked from field in, so it takes no value from you
and sends none back when you save. It stands at the top of a group
rather than inside a Section, a Repeater or a Flexible content
field.

Removing the relation a Linked from field reads is refused, and so
is moving it to another group or deleting the group holding it.
Change the Linked from field first.

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
