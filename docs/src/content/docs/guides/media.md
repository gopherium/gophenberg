---
title: The media library
description: Uploading pictures and files, describing them, and placing them in a post.
---

The Media screen holds every file you have uploaded. Pictures,
video, audio and documents all live there, and the editor takes
what it places from the same library.

## Uploading a file

Open Media in the menu and choose a file with **Add media**. You
can also drop a file straight onto the library. Either way the
upload starts at once, and the file appears in the grid when the
server has stored it.

Gophenberg accepts JPEG, PNG, GIF and WebP pictures, MP4 and WebM
video, MP3, M4A, WAV, OGG and FLAC audio, PDF documents and ZIP
archives. Anything else is refused, and the answer says why.

A file is checked before it is stored, so refusing one never leaves
anything behind. An upload is turned away when its type is not on
that list, when its contents do not match its name, when it is
larger than the upload limit, or when it is a picture the server
cannot read.

## What happens to a picture

Every picture you upload is stored twice over: the file exactly as
you sent it, and a set of smaller copies the editor can choose
from. The copies are named Thumbnail, Medium, Large and Full Size,
which is what the editor's Resolution setting lists.

Two things are corrected on the way in. A photo taken sideways is
turned upright, so it is stored the way you saw it on the camera.
A very large photo also gains a display copy bounded to 2560
pixels, and that copy is what a post shows. Your original stays on
disk either way.

An animated GIF is the exception. It is stored exactly as
uploaded and gains no smaller copies, because making them would
throw the animation away.

## Finding and describing

The library opens as a grid of thumbnails. The **Layout** control
switches it to a table, which adds the file name, type and date.
Search matches titles and file names, and the filter narrows the
list to images or to plain files.

Every item carries four descriptions, all editable through
**Describe**:

- **Title** names the item in the library.
- **Alt text** describes a picture to someone who cannot see it.
  Leave it empty when the picture is decorative.
- **Caption** is the line shown under a picture in a post.
- **Description** is a longer note for your own use.

If someone else changes an item while you are describing it, your
save is refused rather than quietly overwriting theirs. Reload and
make the change again.

## Placing media in a post

In the editor, any media block offers two ways to fill itself.
**Upload** takes a file from your computer and adds it to the
library on the way in. **Media Library** opens the same library
you see on the Media screen, so you can place a file you already
uploaded without uploading it twice.

The inserter also has a Media tab listing your images, video and
audio, which places an item with a single click.

A picture placed in a post keeps pointing at the library. Deleting
it from the library does not repair the post, so check where a
file is used before removing it.

## Deleting

**Delete Permanently** removes an item and every copy of it from
disk. There is no media trash, so the confirmation is the only
step between you and a deleted file. You can select several items
and delete them together.
