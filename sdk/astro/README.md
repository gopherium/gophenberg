# @gophenberg/astro

Build a [Gophenberg](https://github.com/gopherium/gophenberg) theme
in Astro.

Gophenberg is a self-hosted CMS. A theme is an Astro project that
renders its public site: you write your layouts, and this kit
supplies the routes, fetches the content, and turns stored block
markup into HTML.

**[Documentation](https://docs.gophenberg.org/themes/writing-a-theme/)**

## Install

```sh
pnpm add @gophenberg/astro
```

Astro 7 is a peer dependency.

## Use

Two files make an Astro project a theme.

`astro.config.mjs`:

```js
import { gophenberg } from '@gophenberg/astro/config'
import { defineConfig } from 'astro/config'

export default defineConfig({
	integrations: [gophenberg()],
})
```

`src/theme.ts`:

```ts
import { defineTheme } from '@gophenberg/astro'

import Archive from './layouts/Archive.astro'
import Base from './layouts/Base.astro'
import Post from './layouts/Post.astro'

export default defineTheme({
	layouts: { Base, Post, Archive },
	seo: { siteName: 'My Site' },
})
```

The integration injects the site's routes, each rendering one of
your layouts with the content already fetched. Your `Base` layout
renders `<GophenbergHead />`, and `Post` renders the content with
`<Blocks />`.

The [reference theme](https://github.com/gopherium/gophenberg/tree/main/test/theme)
is a complete working example to copy.

## What is in it

| Entry point | Contents |
| --- | --- |
| `@gophenberg/astro` | `defineTheme`, `GophenbergClient`, the loader, `parseBlocks`, and the contract types |
| `@gophenberg/astro/components` | `GophenbergHead`, `Blocks`, `InnerBlocks` |
| `@gophenberg/astro/config` | The `gophenberg()` integration |

## License

Apache-2.0, see LICENSE.

Built theme artifacts bundle
`@wordpress/block-serialization-default-parser`, which is
GPL-2.0-or-later, so a distributed artifact is conveyed under
GPLv3 terms. This kit's own source stays Apache-2.0.
