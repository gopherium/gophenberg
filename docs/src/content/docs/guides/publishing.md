---
title: Saving and publishing
description: How saving, autosave, statuses, slugs, and publishing work in the editor.
---

This page covers what happens between typing a word and seeing it
on your site.

## Saving

On a screen 782 pixels wide or wider, the header shows one of three
words: **Saved**, **Unsaved changes**, or **Saving**. On a narrower
screen that word leaves the header, and the save button names the
state itself, as in WordPress: **Save draft** when there is
something to store, **Saving** while it saves, and **Saved** once
everything is stored.

**Save draft** appears on any post that is not published, and
lights up only when there is something to store. Edits in the
Document tab, the status, slug, and excerpt, travel with the next
save.

On a screen narrower than 600 pixels, the save button is a cloud
icon. Screen readers hear it by the same three names.

If someone else changed the post while you were editing, saving
shows *This post changed elsewhere. Reload before saving again.*
Nothing is overwritten.

## Autosave

Once a minute, when you leave the editor, and when you close the
page, the editor sends unsaved work to the server. On a published post it is parked
separately, so readers never see half-finished edits. On your own
draft it is written into the draft itself, which is not public
either way. A post in the trash takes no autosave and no edits.
**Move to trash** in the editor sends the unsaved work first. If
that fails, the post stays out of the trash with your words still
in the editor. The trash keeps every author's parked work, and a
permanent delete or **Empty Trash** removes it with the post.

If the server holds newer parked work than the post, opening it
shows a banner with **Restore**, which loads the kept words as
unsaved changes for you to keep or discard.
Work parked after the post's last save stays newer through the
trash. Bring the post back with **Undo** or from the Trash tab, and
opening it shows the banner again. Work parked before the last save
stays hidden.

## Statuses

| Status | Meaning |
| --- | --- |
| Draft | A work in progress, not on your site |
| Pending | Finished but unpublished, waiting for review |
| Private | Kept off the public site deliberately |
| Published | Live on your site |

The status select in the Document tab offers Draft, Pending, and
Private, plus the post's current status when it is something else.

## Publishing

On an unpublished post the primary button is **Publish**, which
saves everything and puts the post live. Once published it becomes
**Update**. To unpublish, choose Draft in the status select and
press Update.

## Slugs and excerpts

The slug is the last part of the post's address on your site, as in
`/hello-world`, editable in the Document tab. If your choice is taken
or invalid, the server settles a valid one during the save.

A new post takes its slug from the title. Accented letters become
plain ones, so "Café" gives `cafe`, and letters of other alphabets,
such as Chinese or Greek, stay as they are. Words in scripts with
vowel signs, such as Hindi or Thai, stay whole.

The excerpt is a short summary shown on listing pages. The RSS
feed does not use it: feed readers receive the full content.
