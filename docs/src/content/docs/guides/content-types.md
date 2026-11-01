---
title: Content types
description: Registering kinds of content and choosing the addresses they answer under.
---

A content type is one kind of content your site holds. Every site
starts with Posts. This page shows how to add more kinds, like Pages
or Guides, and how each kind gets its own addresses.

## The list of types

Open **Content Types** in the admin menu. It is a list like the
[posts list](/guides/posts/), drawn the way WordPress draws its lists.
Each row shows the type's name with its key under it, its
description, the address it answers under, and a Status column. The
Status column marks the **Default** type, a type that **Nests**, a
**Closed** one, and a type a plugin declared, as **From** and the
plugin's name. A type with no description shows a dash.

The newest type comes first. Sort by name or by address from a
column header. **Created**, the moment each type was registered, is
hidden at first. Turn it on under **View options**. Search finds a
type by its plural or singular name, its key or its description, and
**Add filter** narrows the list to the types that hold one of the
Status marks. The search, the order and the filter stay in the page
address, so a reload or a shared link shows the same list.

Every action on a type sits in the menu at the end of its row. On a
phone the list shows one type per row with its Status marks, and the
menu sits on the row too.

When something you asked for is done, a short message at the bottom
of the screen says so. When the server refuses it, that message says
why, in the server's own words.

## Registering a type

Press **Add Content Type** and give it a singular and a plural name, like
Guide and Guides. Gophenberg derives the rest: the plural becomes the
route word, so Guides answer under `/guides`. **Add** stays off until
both names are filled in.

The **Description** is optional. Write one sentence about what the
type holds, like "Every guide this site keeps." Gophenberg stores it
exactly as you typed it. Posts start with "Manage the posts on this
site."

When the server refuses the type, for example because another type
already uses its key, the dialog stays open with what you typed, so
you can change a name and press **Add** again. Closing the dialog
forgets what was typed, and the next one starts empty.

The names and descriptions Gophenberg gives its own types, like Posts,
show in the language you read the admin in while they still hold the
English Gophenberg gave them. Once someone changes one, it shows as
typed, like anything else you write here. A type a plugin declared
always shows the words its plugin gives it.

The description shows under the title of the type's list. A type
with no description shows a sentence of the admin's own there
instead, "Manage the items of this content type.", in the language
you read the admin in.

The new type appears in the admin menu right away, with the same list
screen and editor Posts have. To give its items typed fields of their
own, place a field group on it, as [fields](/guides/fields/) explains.

## Describing a type

**Describe…** in a type's menu opens its description so you can
rewrite it. Empty the field and save to clear it. A type a plugin
declared has no **Describe…**, because it keeps the description its
plugin gives it.

The dialog closes the moment you press **Save**, as WordPress closes
its Rename. When the server refuses the change, the next
**Describe…** on that type opens on what you typed, so you can try
again. **Change address…** keeps a refused route word the same way.

The description travels with its type in **Export definitions**. A
file exported by an older release holds no descriptions, and
importing it leaves each description as it stands. See
[moving definitions between sites](/guides/fields/#moving-definitions-between-sites).

## Addresses and the route word

The route word is the first part of every address a type serves. A
guide with the slug `first-steps` answers at `/guides/first-steps`.

A few words belong to Gophenberg itself, so no type can take them:
`admin`, `api`, `media`, `gophenberg`, `_gophenberg` and `page`. The
front page numbers its listing behind `page`, so `/page/2` always
shows its second page, and no item can sit there either. When the
default type nests, a child of an item called Page cannot take a
number as its slug.

One type owns the root of the site instead of a route word. It is
marked **Default** in the list, and its items answer at `/{slug}`
directly. Pick **Make default…** in another type's menu to hand the
root over. The dialog says where both types will answer before
anything moves, and it stays open until the change is done.

The type that gives up the root takes a route word made from its
plural label, so **Posts** moves to `/posts`. When the label makes no
usable word, such as `3D Models` or a label in another alphabet, the
type uses its key instead. A type keyed `page` cannot fall back this
way, because `/page/2` is the second page of a listing, so give it a
label that makes a word first.

Handing the root over is refused when that word is one Gophenberg keeps, like
the label Page gives. It is also refused when it would move an item to
a numbered page of the front page, like a page filed at
`/pages/page/2`. Relabel the type or rename the item, then try again.
When an import brings a new default type and your site keeps its own
root, the new type gets its route word the same way. The import is
refused before it changes anything when that word is one Gophenberg
keeps or another type already uses, on the site or in the file. The
message names the new type and the word. Change the type's plural label
in the definitions file, then import it again.

An import checks route words in the order the file lists its types. A
type that gives a word up frees it only for the types listed after it.
A type the file leaves out holds its word until the import ends. If you
tick it away, a new type can take that word in a second import.

**Change address…** moves a type to a new route word. Every address of
that type moves with it, at once, including nested items. Links kept
outside your site to the old addresses stop working, which is why the
dialog says so above the field. **Move every address** stays off
while the field is empty.

## Nesting

A type marked **Nests** can hold items inside items. The editor's
Document panel then offers a **Parent** select, and a child's address
carries the chain of its parents: `/pages/about/team` is the page
`team` inside `about`.

Nesting goes at most ten levels deep. An item holding children cannot
be trashed until its children move or go first.

**Let items nest** in a type's menu turns nesting on for it, at any
time. **Stop nesting** turns it off, and is refused while any item of
the type still sits inside another, the ones in the trash included.
The message then counts those items. Move them to the top level
first. An imported definitions file meets the same
rule: a type it marks flat keeps nesting while its items nest, and the
plan says so before you apply it. A plugin that marks its own type
flat meets it too, and the type keeps nesting until those items move.

## Turning a type off

**Deactivate** hides a type without deleting anything. Its menu entry,
its archive, and its items stop being served, and everything comes
back with **Activate**.

**Delete…** removes a type for good, and is refused while the type
still holds content. It asks first, naming the type, and Cancel has
the focus when the dialog opens. The dialog closes once the server
answers, and when the server refuses, the message at the bottom of
the screen says why. The default type can be neither deactivated nor
deleted, so the root always answers.

Tick the box at the start of several rows to act on them at once. The
bar under the list then offers **Activate**, **Deactivate** and
**Delete…**, each for the ticked types it applies to. One message
counts the types it changed, or, when some were refused, the ones it
could not change.
