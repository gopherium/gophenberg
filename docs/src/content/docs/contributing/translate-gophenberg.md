---
title: Translate Gophenberg
description: How to translate the admin into your language, what happens to your work, and when it ships.
---

This page is for you if you speak a language other than English and want
Gophenberg to speak it too. You do not need to write code, and you do not
need to use Git.

## What you are translating

Gophenberg keeps every sentence the interface shows in a list, separate
from the code. Each entry has an English original and a place for your
translation. The English original is called the source string, and it
never changes when you translate it.

The list lives in a file format called PO, short for Portable Object,
which is the format the GNU gettext tools have used for decades. You will
not have to edit that file by hand. A website does it for you.

## Where the work happens

Translation happens on POEditor, a website built for exactly this. You
sign in, pick your language, and you see the English on one side and a box
for your language on the other. You fill in the boxes.

The link to the Gophenberg project is in the repository README.

Ask a maintainer to add your language if it is not listed yet. Any
language can be added, and a language with one contributor is welcome.

## Things worth knowing before you start

**Placeholders must survive.** Some strings carry a marker like `%s` or
`%(name)s`. These are holes the software fills in with a name, a number or
a file. Copy every marker into your translation exactly as it appears. You
may move a marker to wherever your language needs it, and you should when
word order differs. You may never rename one, drop one or add one. A
missing marker breaks the screen that shows it.

**Context tells two identical words apart.** English reuses one word for
different things. "Status" means one thing for a post and another for a
user account. When a string carries a context note, translate the meaning
that note describes. Your language may well need two different words where
English used one, and that is the reason the context exists.

**Some words stay in English.** The name Gophenberg is never translated.
Neither are the size units B, KB, MB, GB and TB.

**Write the way the software speaks.** Gophenberg addresses the reader
directly and plainly. Keep sentences short, and prefer the everyday word
over the technical one when both exist.

## What happens to your translation

Nothing you translate goes live immediately, and that is deliberate.

Once a week, an automated job collects everything translated since the
last collection and opens a single pull request against the repository.
One request carries every language and every string that moved. If nobody
translated anything that week, no request is opened.

A maintainer reviews and merges it, the same way code is reviewed. Your
work then ships with the next release.

Translations are batched rather than sent one at a time so that reviewing
them stays practical. It also means there is no rush. Translate at
whatever pace suits you, and the next collection will pick your work up.

## Marking a translation as needing review

A translation can be marked unverified, which gettext calls fuzzy. It
means the words are there but somebody should check them. Use it when you
are unsure, and a later reviewer will see that you flagged it.

Some Spanish entries are already marked this way. They were drafted to
prove the pipeline works end to end, not by a native speaker, so they are
waiting for exactly the review you might give them.
