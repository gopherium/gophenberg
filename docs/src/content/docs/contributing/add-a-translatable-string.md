---
title: Add a translatable string
description: How to write a new interface string so it reaches translators, and what the gates check.
---

This page is for you if you are writing code that shows words to a reader.
Every such word has to reach the translators, and the repository refuses
to build if one does not.

## The short version

Wrap the string, then run `make pot`. That is the whole contract. The
rest of this page explains what to wrap and the few places where wrapping
needs care.

## In the admin, which is TypeScript

Call one of the four gettext functions, giving the text domain
`gophenberg` as the last argument.

```tsx
import { __, _n, _x, sprintf } from '@wordpress/i18n'

__('Add new', 'gophenberg')
sprintf(__('%s renamed.', 'gophenberg'), name)
_n('%d item', '%d items', count, 'gophenberg')
_x('Status', 'post', 'gophenberg')
```

Use `__` for ordinary text. Use `sprintf` around it when a value goes
inside. Use `_n` when the wording changes with a count, since many
languages have more plural forms than English does. Use `_x` when one
English word means two different things, which is covered below.

## On the public site, which is Go

A Go template asks the translator directly.

```html
<p>{{$.T.Get "Nothing published yet."}}</p>
```

A string that never appears literally in the source, because Go builds it
at runtime, is marked instead.

```go
var months = [...]string{i18n.Msgid("January"), i18n.Msgid("February")}
```

`Msgid` returns its argument unchanged. It exists so the extractor can
see a string the compiler would otherwise hide, which is the same trick
the C gettext tools call `N_`.

## Never wrap these

Class names, test ids, route paths, API field names, query keys, and any
status or type value sent to the server. Those are identifiers, not
words, and translating one breaks the software.

Also leave alone the product name Gophenberg, the size units B, KB, MB,
GB and TB, and anything that arrives from the server or from an author.
A content type's plural label reads as its author typed it, in whatever
language they typed it, and no catalogue reaches it.

## When one word means two things

English reuses words that other languages separate. Status means one
thing for a post and another for a user account. Give each use a context
and translators will see them as separate entries.

```tsx
_x('Status', 'post', 'gophenberg')
_x('Status', 'account', 'gophenberg')
```

Only a word standing alone needs this. A word inside a whole sentence
carries its own meaning already.

## The trap that costs an afternoon

Calling a gettext function at the top level of a module runs it once,
when the file is first loaded. In the admin that happens before the
catalogue arrives, so the text freezes in English and never changes
again. Every test still passes, because tests run in English.

The fix is to read the string when it is used rather than when the file
loads.

```tsx
export const mediaNavItem = {
	get label() {
		return _x('Media', 'admin section', 'gophenberg')
	},
	to: '/media',
}
```

This matters for anything imported eagerly. Screens reached through a
lazy route load after the catalogue and are safe either way, but the
getter costs nothing, so prefer it.

## What the gates check

Run `make pot` after adding a string, and commit the template it
regenerates.

A test rebuilds the template and compares it to the committed one byte
for byte, so a forgotten `make pot` fails the build. A second test reads
the Go source and templates with the real Go parsers and names any string
missing from the template. The linter refuses bare text in the admin's
markup. Nothing here depends on a human remembering.

If you also changed a translation, run `make catalogs` too. Compiled
catalogues are committed, and they are compared the same way.
