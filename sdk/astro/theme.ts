// SPDX-License-Identifier: Apache-2.0

import type { AstroComponentFactory } from 'astro/runtime/server/index.js'

import type { BlockComponentMap } from './blocks.ts'

/** The layouts the injected routes render through. */
export interface ThemeLayouts {
	Base: AstroComponentFactory
	Post: AstroComponentFactory
	Archive: AstroComponentFactory
	NotFound?: AstroComponentFactory
}

/** How a theme names itself to a reader and a search engine. */
export interface ThemeSeo {
	siteName: string
}

/** What a theme declares to the kit. */
export interface GophenbergTheme {
	layouts: ThemeLayouts
	blocks?: BlockComponentMap
	pagination?: { perPage: number }
	seo: ThemeSeo
}
