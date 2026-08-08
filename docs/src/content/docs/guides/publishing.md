---
title: Saving and publishing
description: How saving, autosave, statuses, slugs, and publishing work in the editor.
---

This page covers what happens between typing a word and seeing it
on your site.

## Saving

The header always shows one of three words: **Saved**, **Unsaved
changes**, or **Saving**.

**Save draft** appears on any post that is not published, and
lights up only when there is something to store. Edits in the
Document tab, the status, slug, and excerpt, travel with the next
save.

If someone else changed the post while you were editing, saving
shows *This post changed elsewhere. Reload before saving again.*
Nothing is overwritten.

## Autosave

Once a minute, and when you leave the page, the editor sends
unsaved work to the server. On a published post it is parked
separately, so readers never see half-finished edits. On your own
draft it is written into the draft itself, which is not public
either way.

If the server holds newer parked work than the post, opening it
shows a banner with **Restore**, which loads the kept words as
unsaved changes for you to keep or discard.

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

The slug is the post's address on your site, as in
`/post/hello-world`, editable in the Document tab. If your choice
is taken or invalid, the server settles a valid one during the
save.

The excerpt is a short summary shown on listing pages. The RSS
feed does not use it: feed readers receive the full content.
