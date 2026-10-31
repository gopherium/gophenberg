---
title: The media library
description: Uploading pictures and files, finding and editing them, and placing them in a post.
---

The Media screen holds every file you have uploaded. Pictures,
video, audio and documents all live there, and the editor takes
what it places from the same library.

## Uploading a file

Open Media in the menu and press **Add Media File** at the top of the
screen. You can pick several files at once, or drop them straight
onto the library, which reads "Drop files to upload" while you drag.
Either way the upload starts at once, every file sent side by side,
and the files appear in the grid when the server has stored them.

A short message at the bottom of the screen names the file you
uploaded, or counts the files when you sent several. A file the
server refuses is not counted. Its reason shows in a notice above
the library, and when you sent several files each refused one is
named on its own line.

Gophenberg accepts JPEG, PNG, GIF and WebP pictures, MP4 and WebM
video, MP3, M4A, WAV, OGG and FLAC audio, PDF documents and ZIP
archives. Anything else is refused, and the answer says why.

A file is checked before it is stored, so refusing one never leaves
anything behind. An upload is turned away when its type is not on
that list, when its contents do not match its name, when it is
larger than the upload limit, when it is a picture holding more
pixels than the server takes, 80 million unless the server names
another number, or when it is a picture the server cannot read.

## What happens to a picture

Every picture you upload is stored twice over: the file exactly as
you sent it, and a set of smaller copies the editor can choose
from. The copies are named Thumbnail, Medium, Large and Full Size,
which is what the editor's Resolution setting lists.

Copies saved as JPEG are saved at a quality of 82 out of 100,
unless an administrator chose another number under **Settings**.
A lower number makes lighter files that look rougher. The change
applies to pictures uploaded after it, so copies already on disk
stay as they were.

Two things are corrected in the copies. A photo taken sideways
gains an upright copy named with `-rotated`, so a post shows it the
way you saw it on the camera. A very large photo gains a display
copy bounded to 2560 pixels and named with `-scaled`, upright when
the photo was taken sideways, and that copy is what a post shows.
The file you sent stays on disk as it was, with the details the
camera wrote into it, such as where the photo was taken.

An animated GIF is the exception. It is stored exactly as
uploaded and gains no smaller copies, because making them would
throw the animation away.

## Reading the library

The library opens as a grid of cards, the newest file first. On a
screen narrower than 640 pixels, such as a phone, it shows a list
with one file per row instead. There is no layout switch.

Each card shows the picture, the title in bold and the author. An
untitled file shows `(no title)`. A small round avatar sits before
the author's name, drawn the way the site's
[avatar settings](/self-hosting/configuration/#avatars) say, as in
the [posts list](/guides/posts/).

A file that is no picture shows the icon of its kind, video, audio
or a plain file, above its file name. A picture that fails to load
shows the picture icon the same way. A long file name is cut short
with `…` on the card.

Date added, Date modified, Alt text, Caption, Description, File
name, File size, Dimensions and File type are hidden at first. Turn
them on under **View options**. Both dates show the whole date and
time the way WordPress writes them in the language you read the
admin in, such as `October 8, 2026 5:54 pm` or
`8 de octubre de 2026 17:54`. A file name longer than 15 characters
is cut short with `…` and shows whole when you rest the pointer on
it. Alt text shows on pictures alone. An empty alt text, caption,
description or file type shows `-`.

## Finding a file

The library shows 20 files per page by default. **View options**
offers the other page sizes the site allows, and the arrows under
the library move between pages. It also sorts by Title, Date added,
Date modified or Author.

The search box reads `Search media…`. It waits for you to stop
typing, then matches titles, file names, captions and descriptions.

**Add filter** narrows the library:

- **Author** keeps the files of the people you pick, or with **Is
  none of** leaves them out.
- **Date added** and **Date modified** keep the files added or
  changed before or after a moment you pick.
- **Type** keeps one type of file: Images, Audio, Video, Documents
  for PDF files or Archives for ZIP files. Every type is listed, even
  one the library holds none of.

Filters add up, so a file has to match every one of them. The page,
the page size, the sort, the search and the filters all live in the
address of the screen. Reload it or share the link and the library
opens the same way.

When nothing matches, the library says `No media items found.` An
empty library says `No media items yet.` and how to add one.

## The actions on a file

The actions menu of a file offers:

- **View** opens the file itself in a new tab.
- **Edit…** opens the file's details, as the next part explains.
- **Rename…** changes the title alone, in a dialog sent with
  **Save**.
- **Permanently delete…** removes the file, after a question.

View is the main action and shows on every file. On a phone it also
shows as a button beside the menu of each row. The other three show
only on the files you may change: an author's own uploads, and every
file for an editor or an administrator. Another person's file shows
its title as plain text to an author, and on a phone a tap on its
row opens the file in a new tab, as View does.

## Editing a file

Click the title or the picture of a file you may change to open
**Edit media**, or pick **Edit…** from its actions menu. On a phone,
tap the row. The dialog holds four fields you write in:

- **Title** names the file in the library.
- **Alt text** describes a picture to someone who cannot see it.
  Leave it empty when the picture is decorative. Only pictures have
  it, and its link explains how to write one.
- **Caption** is the line shown under a picture in a post.
- **Description** is a longer note for your own use.

Under them sit the details you cannot change: Date added, Author,
File name, File type, File size and Dimensions. The file name shows
whole here.

Cancel and **Save** stay at the bottom of the dialog while the fields
above them scroll, as in WordPress's own media editor. While the
fields do not fit, the part they scroll in is a stop of its own for
the Tab key, named `Scrollable section` for a screen reader, so the
arrow keys, Page Down and End scroll it. Once it has scrolled, a line
shows under the dialog's header, as in WordPress's dialogs. The dialog
closes the moment you press **Save**, and a short message at the
bottom of the screen says `Media file updated.` once the server
stored the change. The focus then goes back to what opened the
dialog, the title, the picture, the actions menu or the phone row,
wherever the file now sits in the library. It goes to the file's
actions menu when that part no longer shows, and to the library
itself when the file no longer shows there. Only the fields
you changed are sent, so a change someone else made to another field
stays. When the save fails, the message says why, starting with
`Could not save image.` for a picture and `Could not save media.`
for any other file. The dialog then opens on what you typed the next
time you open it for that file.

**Rename…** closes at once too, says `Name updated.`, and keeps a
refused name the same way. Saves to one file go one after the
other, so when you rename a file and then edit it, the last one you
sent wins.

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

A **Media** or **Gallery** [field](/guides/fields/) opens the same
library from the Document panel, and the same warning applies.

## Deleting

**Permanently delete…** removes a file and every copy of it from
disk. There is no media trash, so the question is the only step
between you and a deleted file. It asks in a dialog with no header
that names the file, such as
`Delete "Sunset" for good? This cannot be undone.` Cancel has the
focus when the dialog opens. Press **Delete permanently** and the
button shows the work running until the server answers, then the
dialog closes.

Tick several cards and the **Permanently delete…** button under the
library deletes them all at once, behind one question that counts
them. A phone list has no tick boxes.

A short message at the bottom of the screen names the file, such as
`"Sunset" permanently deleted.`, or counts the files when you deleted
several. The message and the question show the whole title. When
the server refuses, the message gives the reason instead, and when
some of several files fail it counts the ones that failed.
