// SPDX-License-Identifier: Apache-2.0

import { defineTheme } from '@gophenberg/astro'

import Quote from './blocks/Quote.astro'
import Archive from './layouts/Archive.astro'
import Base from './layouts/Base.astro'
import NotFound from './layouts/NotFound.astro'
import Post from './layouts/Post.astro'
import Term from './layouts/Term.astro'

export default defineTheme({
	layouts: { Base, Post, Archive, Term, NotFound },
	blocks: { 'core/quote': Quote },
	pagination: { perPage: 2 },
	seo: { siteName: 'Gophenberg Starter' },
})
