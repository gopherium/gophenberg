---
title: Rendering blocks
description: How stored block content becomes HTML in a theme, and how to override one block type.
---

A post's content arrives at your theme as the block markup the
editor saved. The kit's `Blocks` component turns it into HTML, and
you only get involved for block types you want to render
differently.

## The default: verbatim

```astro
<Blocks html={post.content} components={blocks} post={post} />
```

Every block you have not overridden renders the markup the editor
saved. A theme with no overrides renders every post correctly,
just unstyled by you.

## Overriding one block type

The `blocks` map in your theme definition assigns a component to a
block name:

```ts
export default defineTheme({
	layouts: { Base, Post, Archive },
	blocks: { 'core/quote': Quote },
	seo: { siteName: 'My Site' },
})
```

This is the reference theme's quote, which pulls the citation out
of the markup and presents it as a caption:

```astro
---
import { InnerBlocks } from '@gophenberg/astro/components'
import type { BlockProps } from '@gophenberg/astro'

const citeMarkup = /<cite\b[^>]*>([\s\S]*?)<\/cite>/i

const { block, components, post } = Astro.props as BlockProps
const cited = citeMarkup.exec(block.innerHTML)?.[1]?.replace(/<[^>]*>/g, '').trim()
---

<figure class="my-quote">
	<blockquote>
		<InnerBlocks block={block} components={components} post={post} />
	</blockquote>
	{cited && <figcaption>{cited}</figcaption>}
</figure>
```

Two pieces to know:

- **The block** carries `name`, `attrs`, `innerHTML`,
  `innerContent`, and `innerBlocks`. Some things, like a quote's
  citation, live in the markup rather than the attributes.
- **`InnerBlocks`** renders the block's children through the same
  machinery, so your overrides apply inside each other. Pass
  `components` and `post` through.

## What content looks like

Content is filtered before any theme sees it: scripts, event
handlers, iframes, and forms are gone, block structure intact.
`style` attributes survive the filter, and any markup your
override adds is your own responsibility.
