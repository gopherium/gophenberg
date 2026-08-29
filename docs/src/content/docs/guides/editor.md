---
title: Writing with the editor
description: The Gutenberg block editor, the blocks you can use, and what reaches your site.
---

Opening a post puts you in a full-screen editor, with the admin's
navigation out of the way. It is the Gutenberg block editor: your
post is a stack of blocks, one per paragraph, heading, image, or
list.

## The layout

The header holds, left to right: a back link, the block inserter,
undo and redo, a list view toggle, the document name, the save
state (Saved, Unsaved changes, or Saving), a preview width select,
and the save and publish buttons.

The title is its own field above the canvas, where blocks live. A
breadcrumb at the bottom shows where in the block structure you
stand.

The sidebar has two tabs: **Document** for status, slug, and
excerpt, covered in [Publishing](/guides/publishing/), and
**Block** for the selected block's settings.

## Working with blocks

The **inserter** (the plus button) opens a searchable library of
blocks. The **list view** shows the post as a tree, the easiest
way to move or select nested blocks. The **preview select**
switches the canvas between Desktop, Tablet, and Mobile widths.

Undo and redo cover block edits only. The title, slug, excerpt,
and status are outside that history.

## The blocks you can use

Paragraph, Heading, List, Quote, Pullquote, Code, Preformatted,
Verse, Table, Image, Separator, Spacer, Group, Columns, Buttons,
Details, Custom HTML, More, and Page Break, plus the structural
blocks inside lists, columns, and buttons.

The Image block fills itself from the
[media library](/guides/media/) or from a web address.

Content using a block Gophenberg does not know still opens and
saves. It shows as an unrecognized block and its markup is kept.

## What reaches your site

Published content passes through an HTML filter that keeps
ordinary writing markup and removes active elements: scripts,
event handlers, iframes, forms, video, audio, and svg. Links are
limited to `http`, `https`, and `mailto` addresses.

The **Custom HTML** block accepts all of those in the editor, but
they are dropped when the page is served. A pasted YouTube or Maps
embed is an iframe, so it will not appear. If something is missing
from your public site, this filter is usually why.
