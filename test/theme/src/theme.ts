// SPDX-License-Identifier: Apache-2.0

import { defineTheme } from '@gophenberg/astro'

import Quote from './blocks/Quote.astro'
import Archive from './layouts/Archive.astro'
import Base from './layouts/Base.astro'
import Post from './layouts/Post.astro'

export default defineTheme({
	layouts: { Base, Post, Archive },
	blocks: { 'core/quote': Quote },
	pagination: { perPage: 10 },
	seo: { siteName: 'Gophenberg Starter' },
})
