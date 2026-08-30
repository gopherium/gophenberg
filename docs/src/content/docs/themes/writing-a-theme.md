---
title: Writing a theme
description: The two files a theme author writes, and the layouts Gophenberg fills.
---

A theme is an [Astro](https://astro.build) project using the
`@gophenberg/astro` kit. You write two files: an Astro config
naming the integration, and a theme definition naming your
layouts. The kit does the rest, including the routes.

The best starting point is copying the
[reference theme](https://github.com/gopherium/gophenberg/tree/main/test/theme)
from the Gophenberg repository. This page walks its shape.

## The two files

`astro.config.mjs` turns the project into a theme:

```js
import { gophenberg } from '@gophenberg/astro/config'
import { defineConfig } from 'astro/config'

export default defineConfig({
	integrations: [gophenberg()],
})
```

`src/theme.ts` declares what your theme provides:

```ts
import { defineTheme } from '@gophenberg/astro'

import Archive from './layouts/Archive.astro'
import Base from './layouts/Base.astro'
import NotFound from './layouts/NotFound.astro'
import Post from './layouts/Post.astro'
import Term from './layouts/Term.astro'

export default defineTheme({
	layouts: { Base, Post, Archive, Term, NotFound },
	pagination: { perPage: 10 },
	seo: { siteName: 'My Site' },
})
```

`NotFound` is optional, and a `blocks` map can join the object to
override single block types, covered in
[rendering blocks](/themes/rendering-blocks/).

## The routes come from the kit

You write no pages for content. The integration injects the front
page, one catch-all route serving every stored address, the 404
page, and the health route Gophenberg probes at startup. The
catch-all asks Gophenberg what an address holds and renders your
Post layout for an item, your Archive layout for a listing, or your
Term layout for an item that lists what points at it, with the
content already fetched.

Your own pages coexist with them: `src/pages/about.astro` serves
`/about` as in any Astro site. To put posts on a page of your own,
read the [content API](/reference/content-api/) directly.

## The layouts

**Base** wraps every page. It receives `seo` and an optional
`title`, and its one obligation is rendering `GophenbergHead`
inside `<head>`, which emits the page title and the stylesheets
that make block content look right:

```astro
---
import { GophenbergHead } from '@gophenberg/astro/components'

interface Props {
	seo: { siteName: string }
	title?: string
}

const { seo, title } = Astro.props
---

<html lang="en">
	<head>
		<GophenbergHead title={title ? `${title} | ${seo.siteName}` : seo.siteName} />
	</head>
	<body>
		<header><a href="/">{seo.siteName}</a></header>
		<main><slot /></main>
	</body>
</html>
```

**Post** receives the `post`, your `blocks` overrides, and `seo`,
and renders the content through the `Blocks` component:

```astro
---
import { Blocks } from '@gophenberg/astro/components'
import type { BlockComponentMap, Post } from '@gophenberg/astro'
import Base from './Base.astro'

interface Props {
	post: Post
	blocks?: BlockComponentMap
	seo: { siteName: string }
}

const { post, blocks, seo } = Astro.props
---

<Base seo={seo} title={post.title}>
	<article>
		<h1>{post.title}</h1>
		<Blocks html={post.content} components={blocks} post={post} />
	</article>
</Base>
```

**Archive** receives `posts` (summaries without content), `total`,
`page`, `perPage`, `type` (the content type being listed, carrying
its labels, its route word, and the settings each of its fields
holds), and `seo`. **NotFound** receives `seo`.

**Term** renders an item that lists what points at it, a category
page for example. It receives the Archive props plus `term`, the
item itself as a full `Post`, and `blocks`.

A post's `fields` object carries its values, and relation values
name the items they point at. The kit's `relatedFields` helper
picks the relations out, so linking a post to its categories is:

```astro
---
import { relatedFields } from '@gophenberg/astro'

const related = relatedFields(post)
---

{
	related.map((field) => (
		<ul>
			{field.items.map((item) => (
				<li>
					<a href={`/${item.path}`}>{item.title}</a>
				</li>
			))}
		</ul>
	))
}
```

`relatedFields` returns relations only, and a choice value comes
back as the stored value. The shapes are covered in the
[content API](/reference/content-api/).

Media fields work the same way through `mediaFields`, which reads a
Media field and a Gallery alike, so one loop renders either:

```astro
---
import { mediaFields, mediaUrl } from '@gophenberg/astro'

const pictured = mediaFields(post)
---

{
	pictured.map((field) => (
		<ul>
			{field.items.map((item) => (
				<li>
					<img
						src={mediaUrl(item.src)}
						alt={item.alt_text}
						width={item.width}
						height={item.height}
						loading="lazy"
					/>
				</li>
			))}
		</ul>
	))
}
```

Always pass `mediaUrl`, which is what lets a theme running on its
own address during development still load files from the instance.
Giving `width` and `height` stops the page jumping as images
arrive. `item.sizes` holds the other renditions when you want a
`srcset`, and it can be empty, so check before reaching into it.

## Trying it

`astro dev` runs your theme against a running Gophenberg. It needs
two addresses, one for content and one for the block stylesheets,
which the dev server does not serve itself:

```sh
GOPHENBERG_API_URL=http://localhost:8081 \
GOPHENBERG_ASSET_ORIGIN=http://localhost:8081 \
pnpm astro dev
```

When it looks right,
[installing a theme](/themes/installing-a-theme/) takes it to
production.
